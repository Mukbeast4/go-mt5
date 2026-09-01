//! Request-only TCP service and serialized MT5 pipe worker.

use std::{
    collections::{HashMap, VecDeque},
    net::{IpAddr, Ipv4Addr, SocketAddr},
    sync::{
        Arc, RwLock,
        atomic::{AtomicBool, AtomicU64, Ordering},
    },
    time::Duration,
};

use async_trait::async_trait;
use bridge_protocol::{
    Cancel, DEFAULT_CAPABILITIES, DEFAULT_CHUNK_BYTES, DEFAULT_CREDIT_BYTES, ErrorMessage,
    ExecutionCertainty, Frame, Hello, HelloAck, MAX_FRAME_LENGTH, MAX_METADATA_LENGTH, MessageType,
    Operation, PayloadSchema, Ping, ProtocolError, Request, Response, ResponseChunk, ResponseEnd,
    ResponseStart, Value, WindowUpdate, value,
};
use mt5_wire::{
    CMD_ACCOUNT_INFO, CMD_MARKET_BOOK_ADD, CMD_MARKET_BOOK_RELEASE, Pipe, ResponseHead,
    ResponseKind, WireError, WireRequest, array_count, build_request, decode_small,
    decode_value_record, initialize_request,
};
use prost::Message;
use rand::random;
use thiserror::Error;
use tokio::{
    io::{AsyncRead, AsyncWrite},
    net::{TcpListener, TcpStream},
    sync::{Notify, Semaphore, mpsc},
    time::{Instant, timeout},
};
use tracing::{debug, warn};

#[derive(Clone, Debug)]
pub struct ExpectedAccount {
    pub login: i64,
    pub server: String,
}

#[derive(Clone, Debug)]
pub struct RuntimeConfig {
    pub listen: SocketAddr,
    pub token: Vec<u8>,
    pub expected_account: Option<ExpectedAccount>,
    pub pipe_io_inactivity_timeout: Option<Duration>,
    pub handshake_timeout: Duration,
    pub tcp_write_stall_timeout: Duration,
    pub request_queue_capacity: usize,
    pub max_connections: usize,
}

impl Default for RuntimeConfig {
    fn default() -> Self {
        Self {
            listen: SocketAddr::from((Ipv4Addr::LOCALHOST, 19550)),
            token: Vec::new(),
            expected_account: None,
            pipe_io_inactivity_timeout: Some(Duration::from_secs(60)),
            handshake_timeout: Duration::from_secs(5),
            tcp_write_stall_timeout: Duration::from_secs(15),
            request_queue_capacity: 64,
            max_connections: 32,
        }
    }
}

impl RuntimeConfig {
    pub fn validate(&self) -> Result<(), RuntimeError> {
        if self.token.is_empty() {
            return Err(RuntimeError::Configuration(
                "bridge token cannot be empty".into(),
            ));
        }
        if !matches!(self.listen.ip(), IpAddr::V4(ip) if ip.is_loopback()) {
            return Err(RuntimeError::Configuration(
                "v1 only permits a 127.0.0.1 listener".into(),
            ));
        }
        if self.request_queue_capacity == 0 {
            return Err(RuntimeError::Configuration(
                "request queue capacity must be positive".into(),
            ));
        }
        if self.max_connections == 0 {
            return Err(RuntimeError::Configuration(
                "maximum connection count must be positive".into(),
            ));
        }
        if self.handshake_timeout.is_zero() || self.tcp_write_stall_timeout.is_zero() {
            return Err(RuntimeError::Configuration(
                "handshake and TCP write-stall timeouts must be positive".into(),
            ));
        }
        Ok(())
    }
}

#[derive(Clone, Debug)]
pub struct TerminalStatus {
    pub state: &'static str,
    pub epoch: u64,
    pub build: u32,
    pub account_login: i64,
    pub account_server: String,
}

impl Default for TerminalStatus {
    fn default() -> Self {
        Self {
            state: "Connecting",
            epoch: 1,
            build: 0,
            account_login: 0,
            account_server: String::new(),
        }
    }
}

#[derive(Debug, Error)]
pub enum RuntimeError {
    #[error("configuration error: {0}")]
    Configuration(String),
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
    #[error("protocol error: {0}")]
    Protocol(#[from] ProtocolError),
    #[error("worker unavailable")]
    WorkerUnavailable,
}

#[derive(Clone)]
pub struct BridgeHandle {
    sender: mpsc::Sender<Work>,
    status: Arc<RwLock<TerminalStatus>>,
    config: Arc<RuntimeConfig>,
    bridge_instance_id: [u8; 16],
    connections: Arc<Semaphore>,
}

/// Reconnects the bridge to the same configured terminal.  Implementations
/// must never select an arbitrary terminal: the connector receives no client
/// request data and is configured at bridge startup.
#[async_trait]
pub trait PipeConnector<S>: Send + Sync {
    async fn connect(&self) -> Result<S, String>;
}

impl BridgeHandle {
    pub fn status(&self) -> TerminalStatus {
        self.status
            .read()
            .expect("terminal status lock poisoned")
            .clone()
    }

    pub async fn serve(self) -> Result<(), RuntimeError> {
        self.config.validate()?;
        let listener = TcpListener::bind(self.config.listen).await?;
        loop {
            let (stream, peer) = listener.accept().await?;
            let bridge = self.clone();
            let permit = match bridge.connections.clone().try_acquire_owned() {
                Ok(permit) => permit,
                Err(_) => {
                    debug!(%peer, "bridge connection rejected at configured capacity");
                    drop(stream);
                    continue;
                }
            };
            tokio::spawn(async move {
                let _permit = permit;
                if let Err(error) = serve_connection(stream, bridge).await {
                    debug!(%peer, %error, "bridge connection closed");
                }
            });
        }
    }
}

pub fn start_with_pipe<S>(io: S, config: RuntimeConfig) -> Result<BridgeHandle, RuntimeError>
where
    S: AsyncRead + AsyncWrite + Unpin + Send + Sync + 'static,
{
    start_with_optional_connector(Some(io), None, config)
}

pub fn start_with_connector<S, C>(
    connector: C,
    config: RuntimeConfig,
) -> Result<BridgeHandle, RuntimeError>
where
    S: AsyncRead + AsyncWrite + Unpin + Send + Sync + 'static,
    C: PipeConnector<S> + 'static,
{
    start_with_optional_connector(None, Some(Arc::new(connector)), config)
}

fn start_with_optional_connector<S>(
    io: Option<S>,
    connector: Option<Arc<dyn PipeConnector<S>>>,
    config: RuntimeConfig,
) -> Result<BridgeHandle, RuntimeError>
where
    S: AsyncRead + AsyncWrite + Unpin + Send + Sync + 'static,
{
    config.validate()?;
    let status = Arc::new(RwLock::new(TerminalStatus::default()));
    let (sender, receiver) = mpsc::channel(config.request_queue_capacity);
    let max_connections = config.max_connections;
    let handle = BridgeHandle {
        sender,
        status: Arc::clone(&status),
        config: Arc::new(config),
        bridge_instance_id: random(),
        connections: Arc::new(Semaphore::new(max_connections)),
    };
    let worker_config = WorkerConfig {
        expected_account: handle.config.expected_account.clone(),
        pipe_io_inactivity_timeout: handle.config.pipe_io_inactivity_timeout,
    };
    tokio::spawn(
        PipeWorker::new(io.map(Pipe::new), connector, status, worker_config).run(receiver),
    );
    Ok(handle)
}

struct WorkerConfig {
    expected_account: Option<ExpectedAccount>,
    pipe_io_inactivity_timeout: Option<Duration>,
}

struct Work {
    session_key: u64,
    request_id: u64,
    request: Request,
    flow: Arc<FlowControl>,
    cancel: Arc<Cancellation>,
    output: mpsc::Sender<WorkerOutput>,
}

struct WorkerOutput {
    frame: Frame,
    completed: bool,
}

fn enqueue_fair_work(
    queues: &mut HashMap<u64, VecDeque<Work>>,
    schedule: &mut VecDeque<u64>,
    work: Work,
) {
    let session_key = work.session_key;
    let queue = queues.entry(session_key).or_default();
    if queue.is_empty() {
        schedule.push_back(session_key);
    }
    queue.push_back(work);
}

struct PipeWorker<S> {
    pipe: Option<Pipe<S>>,
    connector: Option<Arc<dyn PipeConnector<S>>>,
    status: Arc<RwLock<TerminalStatus>>,
    config: WorkerConfig,
    ready: bool,
    reconnect_delay: Duration,
    next_reconnect_at: Instant,
}

impl<S> PipeWorker<S>
where
    S: AsyncRead + AsyncWrite + Unpin + Send + Sync + 'static,
{
    fn new(
        pipe: Option<Pipe<S>>,
        connector: Option<Arc<dyn PipeConnector<S>>>,
        status: Arc<RwLock<TerminalStatus>>,
        config: WorkerConfig,
    ) -> Self {
        Self {
            pipe,
            connector,
            status,
            config,
            ready: false,
            reconnect_delay: Duration::from_millis(500),
            next_reconnect_at: Instant::now(),
        }
    }

    async fn run(mut self, mut receiver: mpsc::Receiver<Work>) {
        if let Err(error) = self.ensure_connected().await {
            warn!(%error, "MT5 pipe initialization failed");
            self.set_unavailable("Unavailable");
        }
        let mut queues: HashMap<u64, VecDeque<Work>> = HashMap::new();
        let mut schedule = VecDeque::new();
        loop {
            if schedule.is_empty() {
                match receiver.recv().await {
                    Some(work) => enqueue_fair_work(&mut queues, &mut schedule, work),
                    None => break,
                }
            }
            while let Ok(work) = receiver.try_recv() {
                enqueue_fair_work(&mut queues, &mut schedule, work);
            }
            let Some(session_key) = schedule.pop_front() else {
                continue;
            };
            let queue = queues
                .get_mut(&session_key)
                .expect("fair scheduler has a queue for every scheduled session");
            let work = queue
                .pop_front()
                .expect("fair scheduler never schedules an empty queue");
            if queue.is_empty() {
                queues.remove(&session_key);
            } else {
                schedule.push_back(session_key);
            }
            self.execute(work).await;
        }
    }

    async fn initialize(&mut self) -> Result<(), WireError> {
        let request = initialize_request();
        let mut head = self.begin(&request).await?;
        if !head.success {
            return Err(self.pipe()?.read_remote_error(&mut head).await?);
        }
        let body = self
            .pipe()?
            .read_small(&mut head, MAX_METADATA_LENGTH)
            .await?;
        if body.len() < 4 {
            return Err(WireError::InvalidResponse(
                "initialization response has no build".into(),
            ));
        }
        let build = u32::from_le_bytes(body[..4].try_into().unwrap());

        let account = self.read_account().await?;
        if let Some(expected) = &self.config.expected_account {
            if !account_identity_is_ready(&account) {
                return Err(WireError::InvalidResponse(
                    "MT5 terminal account is not ready".into(),
                ));
            }
            if account.0 != expected.login || account.1 != expected.server {
                let mut status = self.status.write().expect("terminal status lock poisoned");
                status.state = "AccountMismatch";
                status.build = build;
                status.account_login = account.0;
                status.account_server = account.1;
                self.ready = false;
                return Ok(());
            }
        }

        let mut status = self.status.write().expect("terminal status lock poisoned");
        status.state = "Ready";
        status.build = build;
        status.account_login = account.0;
        status.account_server = account.1;
        self.ready = true;
        Ok(())
    }

    async fn ensure_connected(&mut self) -> Result<(), WireError> {
        if self.pipe.is_none() {
            let connector = self.connector.clone().ok_or_else(|| {
                WireError::InvalidResponse("no pipe reconnection configured".into())
            })?;
            if Instant::now() < self.next_reconnect_at {
                return Err(WireError::InvalidResponse(
                    "MT5 pipe reconnect is waiting for backoff".into(),
                ));
            }
            {
                let mut status = self.status.write().expect("terminal status lock poisoned");
                status.state = "Reconnecting";
            }
            match connector.connect().await {
                Ok(io) => self.pipe = Some(Pipe::new(io)),
                Err(error) => {
                    self.schedule_reconnect();
                    debug!(%error, next_attempt = ?self.next_reconnect_at, "MT5 pipe reconnect failed");
                    return Err(WireError::InvalidResponse(format!(
                        "MT5 pipe reconnect failed: {error}"
                    )));
                }
            }
        }
        if let Err(error) = self.initialize().await {
            // A failed initialization can leave a partial MT5 response on the
            // byte stream.  Reusing that pipe would desynchronize every later
            // request, so the next job must establish a fresh session.
            self.retire_pipe();
            return Err(error);
        }
        self.reconnect_delay = Duration::from_millis(500);
        self.next_reconnect_at = Instant::now();
        Ok(())
    }

    async fn read_account(&mut self) -> Result<(i64, String), WireError> {
        let request = WireRequest {
            command: CMD_ACCOUNT_INFO,
            params: Vec::new(),
            response_kind: ResponseKind::Small,
            mutation: false,
        };
        let mut head = self.begin(&request).await?;
        if !head.success {
            return Err(self.pipe()?.read_remote_error(&mut head).await?);
        }
        let body = self
            .pipe()?
            .read_small(&mut head, MAX_METADATA_LENGTH)
            .await?;
        let account = decode_small(Operation::AccountInfo, &body)?;
        let fields = account
            .as_object()
            .ok_or_else(|| WireError::InvalidResponse("account result is not an object".into()))?;
        let login = match fields.get("login").and_then(|v| v.kind.as_ref()) {
            Some(value::Kind::I64(login)) => *login,
            _ => return Err(WireError::InvalidResponse("account has no login".into())),
        };
        let server = match fields.get("server").and_then(|v| v.kind.as_ref()) {
            Some(value::Kind::String(server)) => server.clone(),
            _ => return Err(WireError::InvalidResponse("account has no server".into())),
        };
        Ok((login, server))
    }

    async fn execute(&mut self, work: Work) {
        let operation =
            Operation::try_from(work.request.operation).unwrap_or(Operation::Unspecified);
        if let Some(stop) = work.cancel.stop_reason() {
            self.error(
                &work,
                operation,
                "bridge",
                stop.code(),
                stop.before_dispatch_message(),
                0,
                ExecutionCertainty::NotDispatched,
                true,
            )
            .await;
            return;
        }
        if operation == Operation::BridgeStatus {
            self.respond_status(&work, operation).await;
            return;
        }
        if !self.ready
            && self.pipe.is_none()
            && self.connector.is_some()
            && let Err(error) = self.ensure_connected().await
        {
            self.error_from_wire(
                &work,
                operation,
                error,
                ExecutionCertainty::NotDispatched,
                true,
            )
            .await;
            return;
        }
        let status = self
            .status
            .read()
            .expect("terminal status lock poisoned")
            .clone();
        if !self.ready || status.state != "Ready" {
            self.error(
                &work,
                operation,
                "bridge",
                "Unavailable",
                "MT5 terminal is not ready",
                0,
                ExecutionCertainty::NotDispatched,
                true,
            )
            .await;
            return;
        }
        if work.request.expected_terminal_epoch != status.epoch {
            self.error(
                &work,
                operation,
                "bridge",
                "StaleEpoch",
                "request terminal epoch is stale",
                0,
                ExecutionCertainty::NotDispatched,
                true,
            )
            .await;
            return;
        }
        let request = match build_request(operation, work.request.params.as_ref()) {
            Ok(request) => request,
            Err(error) => {
                self.error_from_wire(
                    &work,
                    operation,
                    error,
                    ExecutionCertainty::NotDispatched,
                    true,
                )
                .await;
                return;
            }
        };
        // Adopt an unconfigured account only when the first real request is
        // dispatched. A freshly launched terminal can expose its pipe before
        // it has restored the account, so pinning the initialization snapshot
        // can mistake normal startup for an account switch. Once adopted,
        // continue checking every non-diagnostic operation.
        if !matches!(
            operation,
            Operation::Version | Operation::TerminalInfo | Operation::BridgeStatus
        ) {
            match self.verify_or_adopt_account().await {
                Ok(()) => {}
                Err(error) => {
                    self.error_from_wire(
                        &work,
                        operation,
                        error,
                        ExecutionCertainty::NotDispatched,
                        true,
                    )
                    .await;
                    return;
                }
            }
        }
        let certainty = if request.mutation {
            ExecutionCertainty::OutcomeUnknown
        } else {
            ExecutionCertainty::NotDispatched
        };
        if let Some(stop) = work.cancel.stop_reason() {
            self.error(
                &work,
                operation,
                "bridge",
                stop.code(),
                stop.before_dispatch_message(),
                0,
                ExecutionCertainty::NotDispatched,
                true,
            )
            .await;
            return;
        }
        if let Err(error) = self.execute_request(&work, operation, request).await {
            self.retire_pipe();
            self.set_unavailable("Unavailable");
            self.error_from_wire(&work, operation, error, certainty, true)
                .await;
        }
    }

    async fn verify_or_adopt_account(&mut self) -> Result<(), WireError> {
        let account = self.read_account().await?;
        if let Some(expected) = self.config.expected_account.clone() {
            if account.0 != expected.login || account.1 != expected.server {
                self.ready = false;
                self.set_unavailable("AccountMismatch");
                return Err(WireError::InvalidResponse(
                    "configured account no longer matches terminal account".into(),
                ));
            }
            return Ok(());
        }

        if !account_identity_is_ready(&account) {
            return Err(WireError::InvalidResponse(
                "MT5 terminal account is not ready".into(),
            ));
        }

        debug!(login = account.0, server = %account.1, "adopted terminal account");
        self.config.expected_account = Some(ExpectedAccount {
            login: account.0,
            server: account.1.clone(),
        });
        let mut status = self.status.write().expect("terminal status lock poisoned");
        status.account_login = account.0;
        status.account_server = account.1;
        Ok(())
    }

    async fn respond_status(&mut self, work: &Work, operation: Operation) {
        // Status has no expected-epoch requirement. It refreshes account
        // identity when a pipe is ready, while still returning the last known
        // state if the terminal is disconnected.
        if self.ready {
            match self.read_account().await {
                Ok((login, server)) => {
                    let mut status = self.status.write().expect("terminal status lock poisoned");
                    status.account_login = login;
                    status.account_server = server;
                    if let Some(expected) = &self.config.expected_account
                        && (status.account_login != expected.login
                            || status.account_server != expected.server)
                    {
                        status.state = "AccountMismatch";
                        self.ready = false;
                    }
                }
                Err(error) => {
                    warn!(%error, "fresh terminal status request failed");
                    self.retire_pipe();
                    self.set_unavailable("Unavailable");
                }
            }
        }
        let status = self
            .status
            .read()
            .expect("terminal status lock poisoned")
            .clone();
        let value = Value::object([
            ("state", Value::string(status.state)),
            ("terminal_epoch", Value::u64(status.epoch)),
            ("terminal_build", Value::u64(u64::from(status.build))),
            ("account_login", Value::i64(status.account_login)),
            ("account_server", Value::string(status.account_server)),
        ]);
        self.response(work, operation, value).await;
    }

    async fn execute_request(
        &mut self,
        work: &Work,
        operation: Operation,
        request: WireRequest,
    ) -> Result<(), WireError> {
        if operation == Operation::MarketBookSnapshot {
            return self
                .execute_market_book_snapshot(work, operation, request)
                .await;
        }
        let mut head = self.begin_for_work(&request, &work.cancel).await?;
        if !head.success {
            return Err(self.pipe()?.read_remote_error(&mut head).await?);
        }
        self.consume_success(work, operation, request.response_kind, &mut head, true)
            .await
            .map(|_| ())
    }

    async fn execute_market_book_snapshot(
        &mut self,
        work: &Work,
        operation: Operation,
        get: WireRequest,
    ) -> Result<(), WireError> {
        let add = WireRequest {
            command: CMD_MARKET_BOOK_ADD,
            params: get.params.clone(),
            response_kind: ResponseKind::Unit,
            mutation: false,
        };
        let mut add_head = self.begin_for_work(&add, &work.cancel).await?;
        if !add_head.success {
            return Err(self.pipe()?.read_remote_error(&mut add_head).await?);
        }
        self.pipe()?.discard(&mut add_head).await?;

        let mut get_head = self.begin_for_work(&get, &work.cancel).await?;
        if !get_head.success {
            return Err(self.pipe()?.read_remote_error(&mut get_head).await?);
        }
        let result = self
            .consume_success(work, operation, ResponseKind::Books, &mut get_head, false)
            .await?;
        let Some(rows) = result else {
            // The streaming helper already sent a failed ResponseEnd and
            // retired the pipe. A release on the retired session cannot be
            // meaningful, and a second terminal response would violate the
            // per-request completion contract.
            return Ok(());
        };

        let release = WireRequest {
            command: CMD_MARKET_BOOK_RELEASE,
            params: get.params,
            response_kind: ResponseKind::Unit,
            mutation: false,
        };
        let mut release_head = self.begin_for_work(&release, &work.cancel).await?;
        if !release_head.success {
            let error = self.pipe()?.read_remote_error(&mut release_head).await?;
            self.fail_stream(work, operation, rows, error).await;
            return Ok(());
        }
        if let Err(error) = self.pipe()?.discard(&mut release_head).await {
            self.fail_stream(work, operation, rows, error).await;
            return Ok(());
        }
        self.end(work, true, rows, ExecutionCertainty::ResultReceived, None)
            .await;
        Ok(())
    }

    async fn consume_success(
        &mut self,
        work: &Work,
        operation: Operation,
        kind: ResponseKind,
        head: &mut ResponseHead,
        finish_stream: bool,
    ) -> Result<Option<u64>, WireError> {
        if kind == ResponseKind::Unit {
            self.pipe()?.discard(head).await?;
            self.response(
                work,
                operation,
                Value::object(Vec::<(String, Value)>::new()),
            )
            .await;
            return Ok(Some(0));
        }
        if matches!(kind, ResponseKind::Small) {
            let bytes = self.pipe()?.read_small(head, MAX_METADATA_LENGTH).await?;
            let value = decode_small(operation, &bytes)?;
            self.response(work, operation, value).await;
            return Ok(Some(0));
        }

        let mut count_bytes = [0_u8; 4];
        self.pipe()?.read_exact_body(head, &mut count_bytes).await?;
        let record_bytes = kind
            .record_layout()
            .map(|(_, bytes)| bytes)
            .or_else(|| kind.value_record_bytes())
            .expect("array result kind");
        let count = array_count(head.remaining, count_bytes, record_bytes)?;
        if let Some((schema, _)) = kind.record_layout() {
            self.raw_stream(
                work,
                operation,
                schema,
                record_bytes,
                count,
                head,
                finish_stream,
            )
            .await
        } else {
            self.value_stream(
                work,
                operation,
                kind,
                record_bytes,
                count,
                head,
                finish_stream,
            )
            .await
        }
    }

    #[allow(clippy::too_many_arguments)] // wire framing requires each layout component explicitly.
    async fn raw_stream(
        &mut self,
        work: &Work,
        operation: Operation,
        schema: PayloadSchema,
        record_bytes: usize,
        count: u64,
        head: &mut ResponseHead,
        finish_stream: bool,
    ) -> Result<Option<u64>, WireError> {
        self.send_message(
            work,
            MessageType::ResponseStart,
            &ResponseStart {
                operation: operation as i32,
                schema: schema as i32,
                total_rows_known: true,
                total_rows: count,
            },
            false,
            false,
        )
        .await;
        let target = (DEFAULT_CHUNK_BYTES / record_bytes) * record_bytes;
        let mut sequence = 0_u64;
        let mut rows = 0_u64;
        while head.remaining > 0 {
            if let Some(stop) = work.cancel.stop_reason() {
                self.stop_stream(work, operation, rows, stop).await;
                return Ok(None);
            }
            let bytes = match self.read_chunk_for_work(head, target, &work.cancel).await {
                Ok(bytes) => bytes,
                Err(WireError::Cancelled) => {
                    self.stop_stream(work, operation, rows, StopReason::Cancelled)
                        .await;
                    return Ok(None);
                }
                Err(WireError::DeadlineExceeded) => {
                    self.stop_stream(work, operation, rows, StopReason::Deadline)
                        .await;
                    return Ok(None);
                }
                Err(error) => {
                    self.fail_stream(work, operation, rows, error).await;
                    return Ok(None);
                }
            };
            let row_count = (bytes.len() / record_bytes) as u64;
            let metadata = ResponseChunk {
                sequence,
                row_offset: rows,
                row_count,
                records: Vec::new(),
            };
            self.send_message_with_payload(
                work,
                MessageType::ResponseChunk,
                &metadata,
                bytes,
                true,
                false,
            )
            .await;
            sequence += 1;
            rows += row_count;
        }
        if finish_stream {
            self.end(work, true, rows, ExecutionCertainty::ResultReceived, None)
                .await;
        }
        Ok(Some(rows))
    }

    #[allow(clippy::too_many_arguments)] // wire framing requires each layout component explicitly.
    async fn value_stream(
        &mut self,
        work: &Work,
        operation: Operation,
        kind: ResponseKind,
        record_bytes: usize,
        count: u64,
        head: &mut ResponseHead,
        finish_stream: bool,
    ) -> Result<Option<u64>, WireError> {
        self.send_message(
            work,
            MessageType::ResponseStart,
            &ResponseStart {
                operation: operation as i32,
                schema: PayloadSchema::ProtoValues as i32,
                total_rows_known: true,
                total_rows: count,
            },
            false,
            false,
        )
        .await;
        let mut sequence = 0_u64;
        let mut rows = 0_u64;
        let mut batch = Vec::new();
        while head.remaining > 0 {
            if let Some(stop) = work.cancel.stop_reason() {
                self.stop_stream(work, operation, rows, stop).await;
                return Ok(None);
            }
            let bytes = match self
                .read_chunk_for_work(head, record_bytes, &work.cancel)
                .await
            {
                Ok(bytes) => bytes,
                Err(WireError::Cancelled) => {
                    self.stop_stream(work, operation, rows, StopReason::Cancelled)
                        .await;
                    return Ok(None);
                }
                Err(WireError::DeadlineExceeded) => {
                    self.stop_stream(work, operation, rows, StopReason::Deadline)
                        .await;
                    return Ok(None);
                }
                Err(error) => {
                    self.fail_stream(work, operation, rows, error).await;
                    return Ok(None);
                }
            };
            let record = match decode_value_record(kind, &bytes) {
                Ok(record) => record,
                Err(error) => {
                    self.fail_stream(work, operation, rows, error).await;
                    return Ok(None);
                }
            };
            batch.push(record);
            let candidate = ResponseChunk {
                sequence,
                row_offset: rows,
                row_count: batch.len() as u64,
                records: batch.clone(),
            };
            if candidate.encoded_len() > MAX_METADATA_LENGTH && batch.len() > 1 {
                let last = batch.pop().expect("batch has one record");
                let metadata = ResponseChunk {
                    sequence,
                    row_offset: rows,
                    row_count: batch.len() as u64,
                    records: std::mem::take(&mut batch),
                };
                let sent = metadata.row_count;
                self.send_message(work, MessageType::ResponseChunk, &metadata, true, false)
                    .await;
                rows += sent;
                sequence += 1;
                batch.push(last);
            }
            if batch.len() == 1 {
                let single = ResponseChunk {
                    sequence,
                    row_offset: rows,
                    row_count: 1,
                    records: batch.clone(),
                };
                if single.encoded_len() > MAX_METADATA_LENGTH {
                    return Err(WireError::InvalidResponse(
                        "one record exceeds protocol metadata maximum".into(),
                    ));
                }
            }
        }
        if !batch.is_empty() {
            let metadata = ResponseChunk {
                sequence,
                row_offset: rows,
                row_count: batch.len() as u64,
                records: batch,
            };
            rows += metadata.row_count;
            self.send_message(work, MessageType::ResponseChunk, &metadata, true, false)
                .await;
        }
        if finish_stream {
            self.end(work, true, rows, ExecutionCertainty::ResultReceived, None)
                .await;
        }
        Ok(Some(rows))
    }

    async fn response(&self, work: &Work, operation: Operation, result: Value) {
        let metadata = Response {
            operation: operation as i32,
            result: Some(result),
        };
        self.send_message(work, MessageType::Response, &metadata, true, true)
            .await;
    }

    async fn end(
        &self,
        work: &Work,
        success: bool,
        delivered_rows: u64,
        certainty: ExecutionCertainty,
        error: Option<ErrorMessage>,
    ) {
        let metadata = ResponseEnd {
            success,
            delivered_rows,
            certainty: certainty as i32,
            error,
        };
        // Completion must be deliverable even when a client has consumed all
        // of its data credit.  It contains no data payload and unblocks the
        // client-side request state machine.
        self.send_message(work, MessageType::ResponseEnd, &metadata, false, true)
            .await;
    }

    #[allow(clippy::too_many_arguments)] // preserves the protocol error fields at call sites.
    async fn error(
        &self,
        work: &Work,
        operation: Operation,
        origin: &str,
        code: &str,
        message: &str,
        native_code: i64,
        certainty: ExecutionCertainty,
        completed: bool,
    ) {
        let metadata = error_message(origin, code, operation, message, native_code, certainty);
        self.send_message(work, MessageType::Error, &metadata, false, completed)
            .await;
    }

    async fn error_from_wire(
        &self,
        work: &Work,
        operation: Operation,
        error: WireError,
        certainty: ExecutionCertainty,
        completed: bool,
    ) {
        let (code, native) = match &error {
            WireError::Remote { code, .. } => ("Mt5Error", i64::from(*code)),
            WireError::Io(_) => ("PipeIo", 0),
            WireError::DeadlineExceeded => ("DeadlineExceeded", 0),
            WireError::Cancelled => ("Cancelled", 0),
            _ => ("Wire", 0),
        };
        self.error(
            work,
            operation,
            "mt5",
            code,
            &error.to_string(),
            native,
            certainty,
            completed,
        )
        .await;
    }

    async fn send_message<M: Message>(
        &self,
        work: &Work,
        kind: MessageType,
        message: &M,
        credit: bool,
        completed: bool,
    ) {
        match Frame::encode_message(kind, work.request_id, message) {
            Ok(frame) => self.send_frame(work, frame, credit, completed).await,
            Err(error) => warn!(%error, "unable to encode bridge response"),
        }
    }

    async fn send_message_with_payload<M: Message>(
        &self,
        work: &Work,
        kind: MessageType,
        message: &M,
        payload: Vec<u8>,
        credit: bool,
        completed: bool,
    ) {
        match Frame::encode_message(kind, work.request_id, message) {
            Ok(frame) => {
                self.send_frame(work, frame.with_payload(payload), credit, completed)
                    .await
            }
            Err(error) => warn!(%error, "unable to encode bridge response chunk"),
        }
    }

    async fn send_frame(&self, work: &Work, frame: Frame, credit: bool, completed: bool) {
        if credit
            && !work
                .flow
                .acquire(
                    (frame.metadata.len() + frame.payload.len()) as u64,
                    &work.cancel,
                )
                .await
        {
            return;
        }
        let _ = work.output.send(WorkerOutput { frame, completed }).await;
    }

    async fn begin(&mut self, request: &WireRequest) -> Result<ResponseHead, WireError> {
        match self.config.pipe_io_inactivity_timeout {
            Some(duration) => timeout(duration, self.pipe()?.begin(request))
                .await
                .map_err(|_| {
                    WireError::InvalidResponse("MT5 pipe I/O inactivity timeout".into())
                })?,
            None => self.pipe()?.begin(request).await,
        }
    }

    async fn begin_for_work(
        &mut self,
        request: &WireRequest,
        cancel: &Cancellation,
    ) -> Result<ResponseHead, WireError> {
        if let Some(stop) = cancel.stop_reason() {
            return Err(stop.as_wire_error());
        }
        let inactivity = self.config.pipe_io_inactivity_timeout;
        let begin = self.pipe()?.begin(request);
        tokio::pin!(begin);
        match (inactivity, cancel.deadline) {
            (Some(inactivity), Some(deadline)) => {
                tokio::select! {
                    result = timeout(inactivity, &mut begin) => result.map_err(|_| WireError::InvalidResponse("MT5 pipe I/O inactivity timeout".into()))?,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                    _ = tokio::time::sleep_until(deadline) => Err(WireError::DeadlineExceeded),
                }
            }
            (Some(inactivity), None) => {
                tokio::select! {
                    result = timeout(inactivity, &mut begin) => result.map_err(|_| WireError::InvalidResponse("MT5 pipe I/O inactivity timeout".into()))?,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                }
            }
            (None, Some(deadline)) => {
                tokio::select! {
                    result = &mut begin => result,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                    _ = tokio::time::sleep_until(deadline) => Err(WireError::DeadlineExceeded),
                }
            }
            (None, None) => {
                tokio::select! {
                    result = &mut begin => result,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                }
            }
        }
    }

    async fn read_chunk_for_work(
        &mut self,
        head: &mut ResponseHead,
        max_bytes: usize,
        cancel: &Cancellation,
    ) -> Result<Vec<u8>, WireError> {
        if let Some(stop) = cancel.stop_reason() {
            return Err(stop.as_wire_error());
        }
        let inactivity = self.config.pipe_io_inactivity_timeout;
        let read = self.pipe()?.read_chunk(head, max_bytes);
        tokio::pin!(read);
        match (inactivity, cancel.deadline) {
            (Some(inactivity), Some(deadline)) => {
                tokio::select! {
                    result = timeout(inactivity, &mut read) => result.map_err(|_| WireError::InvalidResponse("MT5 pipe I/O inactivity timeout".into()))?,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                    _ = tokio::time::sleep_until(deadline) => Err(WireError::DeadlineExceeded),
                }
            }
            (Some(inactivity), None) => {
                tokio::select! {
                    result = timeout(inactivity, &mut read) => result.map_err(|_| WireError::InvalidResponse("MT5 pipe I/O inactivity timeout".into()))?,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                }
            }
            (None, Some(deadline)) => {
                tokio::select! {
                    result = &mut read => result,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                    _ = tokio::time::sleep_until(deadline) => Err(WireError::DeadlineExceeded),
                }
            }
            (None, None) => {
                tokio::select! {
                    result = &mut read => result,
                    _ = cancel.notify.notified() => Err(WireError::Cancelled),
                }
            }
        }
    }

    async fn stop_stream(
        &mut self,
        work: &Work,
        operation: Operation,
        delivered_rows: u64,
        stop: StopReason,
    ) {
        self.retire_pipe();
        self.set_unavailable("Unavailable");
        self.end(
            work,
            false,
            delivered_rows,
            ExecutionCertainty::NotDispatched,
            Some(error_message(
                "bridge",
                stop.code(),
                operation,
                stop.streaming_message(),
                0,
                ExecutionCertainty::NotDispatched,
            )),
        )
        .await;
    }

    async fn fail_stream(
        &mut self,
        work: &Work,
        operation: Operation,
        delivered_rows: u64,
        error: WireError,
    ) {
        let (origin, code, native) = match &error {
            WireError::Remote { code, .. } => ("mt5", "Mt5Error", i64::from(*code)),
            WireError::Io(_) => ("mt5", "PipeIo", 0),
            _ => ("mt5", "Wire", 0),
        };
        self.retire_pipe();
        self.set_unavailable("Unavailable");
        self.end(
            work,
            false,
            delivered_rows,
            ExecutionCertainty::NotDispatched,
            Some(error_message(
                origin,
                code,
                operation,
                &error.to_string(),
                native,
                ExecutionCertainty::NotDispatched,
            )),
        )
        .await;
    }

    fn pipe(&mut self) -> Result<&mut Pipe<S>, WireError> {
        self.pipe
            .as_mut()
            .ok_or_else(|| WireError::InvalidResponse("MT5 pipe was retired".into()))
    }

    fn retire_pipe(&mut self) {
        // Dropping Tokio's NamedPipeClient closes the Windows handle and
        // cancels pending overlapped work before a later reconnect creates a
        // fresh terminal epoch.
        if self.pipe.take().is_some() {
            self.schedule_reconnect();
        }
    }

    fn schedule_reconnect(&mut self) {
        // Jitter prevents several bridges restarted by the same supervisor
        // from attempting to open the same Wine pipe in lockstep.
        let basis_points = 8_000_u64 + (u64::from(random::<u16>()) * 4_000 / 65_535);
        let delay = self.reconnect_delay.mul_f64(basis_points as f64 / 10_000.0);
        self.next_reconnect_at = Instant::now() + delay;
        self.reconnect_delay = (self.reconnect_delay * 2).min(Duration::from_secs(30));
    }

    fn set_unavailable(&mut self, state: &'static str) {
        self.ready = false;
        let mut status = self.status.write().expect("terminal status lock poisoned");
        status.state = state;
        status.epoch = status.epoch.saturating_add(1);
    }
}

fn error_message(
    origin: &str,
    code: &str,
    operation: Operation,
    message: &str,
    native_code: i64,
    certainty: ExecutionCertainty,
) -> ErrorMessage {
    ErrorMessage {
        origin: origin.into(),
        code: code.into(),
        operation: format!("{operation:?}"),
        message: message.into(),
        native_code,
        certainty: certainty as i32,
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum StopReason {
    Cancelled,
    Deadline,
}

impl StopReason {
    fn code(self) -> &'static str {
        match self {
            Self::Cancelled => "Cancelled",
            Self::Deadline => "DeadlineExceeded",
        }
    }

    fn before_dispatch_message(self) -> &'static str {
        match self {
            Self::Cancelled => "request was cancelled before dispatch",
            Self::Deadline => "caller-imposed request deadline expired before dispatch",
        }
    }

    fn streaming_message(self) -> &'static str {
        match self {
            Self::Cancelled => "request cancelled while streaming",
            Self::Deadline => "caller-imposed request deadline expired while streaming",
        }
    }

    fn as_wire_error(self) -> WireError {
        match self {
            Self::Cancelled => WireError::Cancelled,
            Self::Deadline => WireError::DeadlineExceeded,
        }
    }
}

struct Cancellation {
    cancelled: AtomicBool,
    notify: Notify,
    deadline: Option<Instant>,
}
impl Cancellation {
    fn new(deadline: Option<Instant>) -> Self {
        Self {
            cancelled: AtomicBool::new(false),
            notify: Notify::new(),
            deadline,
        }
    }
    fn cancel(&self) {
        self.cancelled.store(true, Ordering::Release);
        // `notify_one` retains a permit if cancellation wins the race before
        // the pipe-read or credit waiter has been polled.
        self.notify.notify_one();
    }
    fn is_cancelled(&self) -> bool {
        self.cancelled.load(Ordering::Acquire)
    }
    fn stop_reason(&self) -> Option<StopReason> {
        if self.is_cancelled() {
            Some(StopReason::Cancelled)
        } else if self
            .deadline
            .is_some_and(|deadline| Instant::now() >= deadline)
        {
            Some(StopReason::Deadline)
        } else {
            None
        }
    }
}

struct FlowControl {
    credit: AtomicU64,
    notify: Notify,
}
impl FlowControl {
    fn new() -> Self {
        Self {
            credit: AtomicU64::new(DEFAULT_CREDIT_BYTES),
            notify: Notify::new(),
        }
    }
    fn add(&self, amount: u64) {
        self.credit.fetch_saturating_add(amount, Ordering::AcqRel);
        self.notify.notify_waiters();
    }
    async fn acquire(&self, amount: u64, cancel: &Cancellation) -> bool {
        loop {
            if cancel.stop_reason().is_some() {
                return false;
            }
            let current = self.credit.load(Ordering::Acquire);
            if current >= amount
                && self
                    .credit
                    .compare_exchange(
                        current,
                        current - amount,
                        Ordering::AcqRel,
                        Ordering::Acquire,
                    )
                    .is_ok()
            {
                return true;
            }
            // `Notify::notify_waiters` intentionally has no stored permit.
            // The short timer closes the check/register race if cancellation
            // happens immediately before the waiter is first polled.
            if let Some(deadline) = cancel.deadline {
                tokio::select! {
                    _ = self.notify.notified() => {},
                    _ = cancel.notify.notified() => return false,
                    _ = tokio::time::sleep_until(deadline) => return false,
                    _ = tokio::time::sleep(Duration::from_millis(50)) => {},
                }
            } else {
                tokio::select! {
                    _ = self.notify.notified() => {},
                    _ = cancel.notify.notified() => return false,
                    _ = tokio::time::sleep(Duration::from_millis(50)) => {},
                }
            }
            if cancel.stop_reason().is_some() {
                return false;
            }
        }
    }
}

trait SaturatingAdd {
    fn fetch_saturating_add(&self, value: u64, ordering: Ordering);
}
impl SaturatingAdd for AtomicU64 {
    fn fetch_saturating_add(&self, value: u64, ordering: Ordering) {
        let mut current = self.load(Ordering::Acquire);
        loop {
            let next = current.saturating_add(value);
            match self.compare_exchange_weak(current, next, ordering, Ordering::Acquire) {
                Ok(_) => return,
                Err(actual) => current = actual,
            }
        }
    }
}

#[derive(Default)]
struct SessionRequests {
    flows: HashMap<u64, Arc<FlowControl>>,
    cancels: HashMap<u64, Arc<Cancellation>>,
}

impl Drop for SessionRequests {
    fn drop(&mut self) {
        // The worker owns a clone of each cancellation token.  Cancelling in
        // this destructor covers every TCP exit path, including a write
        // failure while a response is blocked behind response credit.
        for cancel in self.cancels.values() {
            cancel.cancel();
        }
    }
}

async fn serve_connection(stream: TcpStream, bridge: BridgeHandle) -> Result<(), RuntimeError> {
    let (mut reader, mut writer) = stream.into_split();
    let hello_frame = timeout(
        bridge.config.handshake_timeout,
        Frame::read_from(&mut reader),
    )
    .await
    .map_err(|_| RuntimeError::Configuration("TCP handshake timed out".into()))??;
    if hello_frame.message_type != MessageType::Hello || hello_frame.request_id != 0 {
        return Err(RuntimeError::Configuration(
            "first client frame must be Hello with request id zero".into(),
        ));
    }
    let hello = hello_frame.decode_message::<Hello>()?;
    if !constant_time_eq(&hello.token, &bridge.config.token) {
        return Err(RuntimeError::Configuration("authentication failed".into()));
    }
    let status = bridge.status();
    let session_id = random::<[u8; 16]>();
    let session_key = random::<u64>();
    let ack = HelloAck {
        bridge_instance_id: bridge.bridge_instance_id.to_vec(),
        session_id: session_id.to_vec(),
        terminal_epoch: status.epoch,
        terminal_state: status.state.into(),
        terminal_build: status.build,
        account_login: status.account_login,
        account_server: status.account_server,
        max_frame_length: u32::try_from(MAX_FRAME_LENGTH).expect("protocol frame limit fits u32"),
        max_metadata_length: u32::try_from(MAX_METADATA_LENGTH)
            .expect("protocol metadata limit fits u32"),
        target_chunk_bytes: u32::try_from(DEFAULT_CHUNK_BYTES)
            .expect("protocol chunk target fits u32"),
        initial_response_credit: DEFAULT_CREDIT_BYTES,
        capabilities: DEFAULT_CAPABILITIES,
    };
    Frame::encode_message(MessageType::HelloAck, 0, &ack)?
        .write_to(&mut writer)
        .await?;

    let (outbound_tx, mut outbound_rx) = mpsc::channel::<WorkerOutput>(32);
    let mut requests = SessionRequests::default();
    let mut last_request_id = 0_u64;
    loop {
        tokio::select! {
            outbound = outbound_rx.recv() => {
                let Some(outbound) = outbound else { return Ok(()); };
                let request_id = outbound.frame.request_id;
                timeout(bridge.config.tcp_write_stall_timeout, outbound.frame.write_to(&mut writer)).await
                    .map_err(|_| RuntimeError::Configuration("TCP write stalled".into()))??;
                if outbound.completed {
                    requests.flows.remove(&request_id);
                    requests.cancels.remove(&request_id);
                }
            }
            incoming = Frame::read_from(&mut reader) => {
                let frame = incoming?;
                match frame.message_type {
                    MessageType::Request => {
                        if frame.request_id == 0 || frame.request_id <= last_request_id { return Err(RuntimeError::Configuration("request ids must be strictly increasing and nonzero".into())); }
                        let request = frame.decode_message::<Request>()?;
                        let deadline = if request.deadline_ms == 0 {
                            None
                        } else {
                            Instant::now().checked_add(Duration::from_millis(request.deadline_ms))
                        };
                        let flow = Arc::new(FlowControl::new());
                        let cancel = Arc::new(Cancellation::new(deadline));
                        let session_flow = Arc::clone(&flow);
                        let session_cancel = Arc::clone(&cancel);
                        last_request_id = frame.request_id;
                        let work = Work { session_key, request_id: frame.request_id, request, flow, cancel, output: outbound_tx.clone() };
                        match bridge.sender.try_send(work) {
                            Ok(()) => {
                                requests.flows.insert(frame.request_id, session_flow);
                                requests.cancels.insert(frame.request_id, session_cancel);
                            }
                            Err(tokio::sync::mpsc::error::TrySendError::Full(work)) => {
                                let error = error_message(
                                    "bridge",
                                    "QueueFull",
                                    Operation::try_from(work.request.operation).unwrap_or(Operation::Unspecified),
                                    "bridge request queue is at capacity",
                                    0,
                                    ExecutionCertainty::NotDispatched,
                                );
                                timeout(
                                    bridge.config.tcp_write_stall_timeout,
                                    Frame::encode_message(MessageType::Error, frame.request_id, &error)?.write_to(&mut writer),
                                )
                                .await
                                .map_err(|_| RuntimeError::Configuration("TCP write stalled".into()))??;
                            }
                            Err(tokio::sync::mpsc::error::TrySendError::Closed(_)) => return Err(RuntimeError::WorkerUnavailable),
                        }
                    }
                    MessageType::WindowUpdate => {
                        if frame.request_id == 0 { return Err(RuntimeError::Configuration("WindowUpdate requires a request id".into())); }
                        if let Some(flow) = requests.flows.get(&frame.request_id) { flow.add(frame.decode_message::<WindowUpdate>()?.credit_bytes); }
                    }
                    MessageType::Cancel => {
                        if frame.request_id == 0 { return Err(RuntimeError::Configuration("Cancel requires a request id".into())); }
                        let _ = frame.decode_message::<Cancel>()?;
                        if let Some(cancel) = requests.cancels.get(&frame.request_id) { cancel.cancel(); }
                    }
                    MessageType::Ping => {
                        if frame.request_id != 0 { return Err(RuntimeError::Configuration("Ping uses request id zero".into())); }
                        let ping = frame.decode_message::<Ping>()?;
                        Frame::encode_message(MessageType::Pong, 0, &ping)?.write_to(&mut writer).await?;
                    }
                    _ => return Err(RuntimeError::Configuration("unexpected client frame type".into())),
                }
            }
        }
    }
}

fn constant_time_eq(left: &[u8], right: &[u8]) -> bool {
    let mut difference = left.len() ^ right.len();
    for index in 0..left.len().max(right.len()) {
        difference |= usize::from(
            left.get(index).copied().unwrap_or(0) ^ right.get(index).copied().unwrap_or(0),
        );
    }
    difference == 0
}

fn account_identity_is_ready(account: &(i64, String)) -> bool {
    account.0 > 0 && !account.1.is_empty()
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};

    #[test]
    fn only_loopback_listeners_are_valid() {
        let mut config = RuntimeConfig {
            token: b"token".to_vec(),
            ..RuntimeConfig::default()
        };
        assert!(config.validate().is_ok());
        config.listen = "0.0.0.0:19550".parse().unwrap();
        assert!(config.validate().is_err());
    }

    #[test]
    fn constant_time_comparison_requires_equal_values() {
        assert!(constant_time_eq(b"abc", b"abc"));
        assert!(!constant_time_eq(b"abc", b"abd"));
        assert!(!constant_time_eq(b"abc", b"ab"));
    }

    #[test]
    fn account_identity_requires_a_login_and_server() {
        assert!(account_identity_is_ready(&(
            7_395_945,
            "FPTradingLLC-Demo".into()
        )));
        assert!(!account_identity_is_ready(&(0, "FPTradingLLC-Demo".into())));
        assert!(!account_identity_is_ready(&(7_395_945, String::new())));
    }

    #[tokio::test]
    async fn flow_control_blocks_until_credit_arrives() {
        let flow = Arc::new(FlowControl::new());
        let cancel = Arc::new(Cancellation::new(None));
        assert!(flow.acquire(DEFAULT_CREDIT_BYTES, &cancel).await);
        let pending = {
            let flow = Arc::clone(&flow);
            let cancel = Arc::clone(&cancel);
            tokio::spawn(async move { flow.acquire(1, &cancel).await })
        };
        tokio::task::yield_now().await;
        flow.add(1);
        assert!(pending.await.unwrap());
    }

    #[tokio::test]
    async fn flow_control_observes_cancellation() {
        let flow = FlowControl::new();
        let cancel = Cancellation::new(None);
        let future = flow.acquire(DEFAULT_CREDIT_BYTES + 1, &cancel);
        tokio::pin!(future);
        cancel.cancel();
        assert!(!future.await);
    }

    #[tokio::test]
    async fn flow_control_observes_expired_deadline() {
        let flow = FlowControl::new();
        let cancel = Cancellation::new(Some(Instant::now() - Duration::from_millis(1)));
        assert_eq!(cancel.stop_reason(), Some(StopReason::Deadline));
        assert!(!flow.acquire(1, &cancel).await);
    }

    #[test]
    fn dropping_a_connection_session_cancels_its_work() {
        let cancel = Arc::new(Cancellation::new(None));
        let mut requests = SessionRequests::default();
        requests.cancels.insert(7, Arc::clone(&cancel));
        drop(requests);
        assert_eq!(cancel.stop_reason(), Some(StopReason::Cancelled));
    }

    #[test]
    fn fair_scheduler_rotates_between_connections() {
        let (output, _) = mpsc::channel(1);
        let mut queues = HashMap::new();
        let mut schedule = VecDeque::new();
        enqueue_fair_work(&mut queues, &mut schedule, test_work(10, 1, output.clone()));
        enqueue_fair_work(&mut queues, &mut schedule, test_work(10, 2, output.clone()));
        enqueue_fair_work(&mut queues, &mut schedule, test_work(20, 1, output));
        assert_eq!(schedule, VecDeque::from([10, 20]));

        let session = schedule.pop_front().unwrap();
        queues.get_mut(&session).unwrap().pop_front();
        schedule.push_back(session);
        assert_eq!(schedule, VecDeque::from([20, 10]));
    }

    #[tokio::test]
    async fn worker_streams_rates_without_buffering_the_native_array() {
        let (bridge_io, mut terminal_io) = tokio::io::duplex(16 * 1024);
        let rates = fixture("rates_h1_50_eurusd.bin");
        let account = fixture("account_info.bin");
        let mut startup_account = account.clone();
        startup_account[..8].fill(0);
        let terminal = tokio::spawn(async move {
            let mut account_requests = 0;
            for _ in 0..4 {
                let length = terminal_io.read_u32_le().await.unwrap() as usize;
                let mut request = vec![0_u8; length];
                terminal_io.read_exact(&mut request).await.unwrap();
                let command = u32::from_le_bytes(request[..4].try_into().unwrap());
                let payload = match command {
                    mt5_wire::CMD_INITIALIZE => 5836_u32.to_le_bytes().to_vec(),
                    mt5_wire::CMD_ACCOUNT_INFO => {
                        account_requests += 1;
                        if account_requests == 1 {
                            startup_account.clone()
                        } else {
                            account.clone()
                        }
                    }
                    mt5_wire::CMD_COPY_RATES_FROM_POS => rates.clone(),
                    other => panic!("unexpected MT5 command {other}"),
                };
                terminal_io
                    .write_u32_le((8 + payload.len()) as u32)
                    .await
                    .unwrap();
                terminal_io.write_u32_le(command).await.unwrap();
                terminal_io.write_u32_le(1).await.unwrap();
                terminal_io.write_all(&payload).await.unwrap();
                terminal_io.flush().await.unwrap();
            }
        });

        let status = Arc::new(RwLock::new(TerminalStatus::default()));
        let (jobs, receiver) = mpsc::channel(2);
        let worker = tokio::spawn(
            PipeWorker::new(
                Some(Pipe::new(bridge_io)),
                None,
                Arc::clone(&status),
                WorkerConfig {
                    expected_account: None,
                    pipe_io_inactivity_timeout: None,
                },
            )
            .run(receiver),
        );
        let (output, mut output_rx) = mpsc::channel(8);
        let params = Value::object([
            ("symbol", Value::string("EURUSD")),
            ("timeframe", Value::u64(16_385)),
            ("start_pos", Value::u64(0)),
            ("count", Value::u64(50)),
        ]);
        jobs.send(Work {
            session_key: 1,
            request_id: 1,
            request: Request {
                operation: Operation::CopyRatesFromPos as i32,
                expected_terminal_epoch: 1,
                params: Some(params),
                deadline_ms: 0,
            },
            flow: Arc::new(FlowControl::new()),
            cancel: Arc::new(Cancellation::new(None)),
            output,
        })
        .await
        .unwrap();

        let first = output_rx.recv().await.unwrap();
        assert_eq!(first.frame.message_type, MessageType::ResponseStart);
        let start = first.frame.decode_message::<ResponseStart>().unwrap();
        assert_eq!(start.schema, PayloadSchema::RateV1 as i32);
        assert_eq!(start.total_rows, 50);

        let second = output_rx.recv().await.unwrap();
        assert_eq!(second.frame.message_type, MessageType::ResponseChunk);
        assert_eq!(second.frame.payload.len(), 50 * mt5_wire::RATE_RECORD_BYTES);
        let chunk = second.frame.decode_message::<ResponseChunk>().unwrap();
        assert_eq!(chunk.row_count, 50);

        let third = output_rx.recv().await.unwrap();
        assert_eq!(third.frame.message_type, MessageType::ResponseEnd);
        assert!(third.completed);
        assert!(third.frame.decode_message::<ResponseEnd>().unwrap().success);
        drop(jobs);
        worker.await.unwrap();
        terminal.await.unwrap();
    }

    fn fixture(name: &str) -> Vec<u8> {
        let root = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../..");
        std::fs::read(root.join("testdata").join(name)).unwrap()
    }

    fn test_work(session_key: u64, request_id: u64, output: mpsc::Sender<WorkerOutput>) -> Work {
        Work {
            session_key,
            request_id,
            request: Request {
                operation: Operation::Version as i32,
                expected_terminal_epoch: 1,
                params: None,
                deadline_ms: 0,
            },
            flow: Arc::new(FlowControl::new()),
            cancel: Arc::new(Cancellation::new(None)),
            output,
        }
    }
}
