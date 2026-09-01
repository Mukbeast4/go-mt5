use std::{env, net::SocketAddr, path::PathBuf, time::Duration};

use bridge_runtime::{ExpectedAccount, RuntimeConfig};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .with_target(false)
        .init();

    let token = env::var("MT5_BRIDGE_TOKEN")?.into_bytes();
    let listen: SocketAddr = env::var("MT5_BRIDGE_LISTEN")
        .unwrap_or_else(|_| "127.0.0.1:19550".to_owned())
        .parse()?;
    let expected_account = match (
        env::var("MT5_ACCOUNT_LOGIN"),
        env::var("MT5_ACCOUNT_SERVER"),
    ) {
        (Ok(login), Ok(server)) => Some(ExpectedAccount {
            login: login.parse()?,
            server,
        }),
        (Err(_), Err(_)) => None,
        _ => return Err("MT5_ACCOUNT_LOGIN and MT5_ACCOUNT_SERVER are required together".into()),
    };
    let mut config = RuntimeConfig {
        token,
        listen,
        expected_account,
        ..RuntimeConfig::default()
    };
    if env::var("MT5_PIPE_IO_TIMEOUT_SECONDS").ok().as_deref() == Some("0") {
        config.pipe_io_inactivity_timeout = None;
    } else if let Ok(seconds) = env::var("MT5_PIPE_IO_TIMEOUT_SECONDS") {
        config.pipe_io_inactivity_timeout = Some(Duration::from_secs(seconds.parse()?));
    }
    if let Some(timeout) = optional_positive_duration("MT5_HANDSHAKE_TIMEOUT_SECONDS")? {
        config.handshake_timeout = timeout;
    }
    if let Some(timeout) = optional_positive_duration("MT5_TCP_WRITE_STALL_TIMEOUT_SECONDS")? {
        config.tcp_write_stall_timeout = timeout;
    }
    if let Some(capacity) = optional_positive_usize("MT5_REQUEST_QUEUE_CAPACITY")? {
        config.request_queue_capacity = capacity;
    }
    if let Some(capacity) = optional_positive_usize("MT5_MAX_CONNECTIONS")? {
        config.max_connections = capacity;
    }

    #[cfg(windows)]
    {
        let connector = WinePipeConnector {
            pipe_name: pipe_name()?,
            open_timeout: optional_positive_duration("MT5_PIPE_OPEN_TIMEOUT_SECONDS")?
                .unwrap_or(Duration::from_secs(60)),
        };
        let bridge = bridge_runtime::start_with_connector(connector, config)?;
        bridge.serve().await?;
        Ok(())
    }

    #[cfg(not(windows))]
    {
        let _ = config;
        Err("mt5-bridge must be built as a Windows executable and run inside Wine".into())
    }
}

fn optional_positive_duration(name: &str) -> Result<Option<Duration>, Box<dyn std::error::Error>> {
    let Ok(raw) = env::var(name) else {
        return Ok(None);
    };
    let seconds: u64 = raw.parse()?;
    if seconds == 0 {
        return Err(format!("{name} must be greater than zero").into());
    }
    Ok(Some(Duration::from_secs(seconds)))
}

fn optional_positive_usize(name: &str) -> Result<Option<usize>, Box<dyn std::error::Error>> {
    let Ok(raw) = env::var(name) else {
        return Ok(None);
    };
    let value: usize = raw.parse()?;
    if value == 0 {
        return Err(format!("{name} must be greater than zero").into());
    }
    Ok(Some(value))
}

#[cfg(windows)]
fn pipe_name() -> Result<String, Box<dyn std::error::Error>> {
    if let Ok(name) = env::var("MT5_PIPE_NAME")
        && !name.is_empty()
    {
        return Ok(name);
    }
    if let Ok(path) = env::var("MT5_TERMINAL_PATH") {
        return Ok(mt5_windows::pipe_name_for_terminal_path(PathBuf::from(
            path,
        ))?);
    }
    Err("set MT5_PIPE_NAME or MT5_TERMINAL_PATH; automatic terminal discovery is intentionally disabled until Wine process-discovery integration is verified".into())
}

#[cfg(windows)]
#[derive(Clone)]
struct WinePipeConnector {
    pipe_name: String,
    open_timeout: Duration,
}

#[cfg(windows)]
#[async_trait::async_trait]
impl bridge_runtime::PipeConnector<mt5_windows::NativePipe> for WinePipeConnector {
    async fn connect(&self) -> Result<mt5_windows::NativePipe, String> {
        mt5_windows::open_pipe(&self.pipe_name, self.open_timeout)
            .await
            .map_err(|error| error.to_string())
    }
}
