use std::{env, net::SocketAddr, path::PathBuf, time::Duration};

#[cfg(windows)]
use std::{
    process::Child,
    sync::{Arc, Mutex},
};

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
        let target = pipe_target()?;
        let (pipe_name, terminal_path) = match target {
            PipeTarget::PipeName(name) => (name, None),
            PipeTarget::Terminal { pipe_name, path } => (pipe_name, Some(path)),
        };
        match terminal_path.as_deref() {
            Some(path) => tracing::debug!(
                source = "MT5_TERMINAL_PATH",
                pipe_name = %pipe_name,
                terminal_path = %path.display(),
                "configured MT5 pipe target"
            ),
            None => tracing::debug!(
                source = "MT5_PIPE_NAME",
                pipe_name = %pipe_name,
                "configured MT5 pipe target"
            ),
        }
        let connector = WinePipeConnector {
            pipe_name,
            terminal_path,
            open_timeout: optional_positive_duration("MT5_PIPE_OPEN_TIMEOUT_SECONDS")?
                .unwrap_or(Duration::from_secs(60)),
            launched_terminal: Arc::new(Mutex::new(None)),
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
enum PipeTarget {
    PipeName(String),
    Terminal { pipe_name: String, path: PathBuf },
}

#[cfg(windows)]
fn pipe_target() -> Result<PipeTarget, Box<dyn std::error::Error>> {
    pipe_target_from_values(
        env::var("MT5_PIPE_NAME").ok(),
        env::var("MT5_TERMINAL_PATH").ok(),
    )
}

#[cfg(windows)]
fn pipe_target_from_values(
    pipe_name: Option<String>,
    terminal_path: Option<String>,
) -> Result<PipeTarget, Box<dyn std::error::Error>> {
    if let Some(name) = pipe_name.filter(|name| !name.is_empty()) {
        return Ok(PipeTarget::PipeName(name));
    }
    if let Some(path) = terminal_path {
        let path = PathBuf::from(path);
        return Ok(PipeTarget::Terminal {
            pipe_name: mt5_windows::pipe_name_for_terminal_path(&path)?,
            path,
        });
    }
    Err("set MT5_PIPE_NAME or MT5_TERMINAL_PATH; automatic terminal discovery is intentionally disabled until Wine process-discovery integration is verified".into())
}

#[cfg(windows)]
#[derive(Clone)]
struct WinePipeConnector {
    pipe_name: String,
    terminal_path: Option<PathBuf>,
    open_timeout: Duration,
    launched_terminal: Arc<Mutex<Option<Child>>>,
}

#[cfg(windows)]
#[async_trait::async_trait]
impl bridge_runtime::PipeConnector<mt5_windows::NativePipe> for WinePipeConnector {
    async fn connect(&self) -> Result<mt5_windows::NativePipe, String> {
        match mt5_windows::open_pipe(&self.pipe_name, self.open_timeout).await {
            Ok(pipe) => return Ok(pipe),
            Err(error) if self.terminal_path.is_some() && error.is_missing() => {
                tracing::debug!(
                    pipe_name = %self.pipe_name,
                    terminal_path = %self
                        .terminal_path
                        .as_deref()
                        .expect("terminal path checked above")
                        .display(),
                    "MT5 pipe is missing; attempting configured terminal startup"
                );
                self.ensure_terminal_started()?;
            }
            Err(error) => return Err(error.to_string()),
        }

        mt5_windows::wait_for_pipe(&self.pipe_name, self.open_timeout)
            .await
            .map_err(|error| error.to_string())
    }
}

#[cfg(windows)]
impl WinePipeConnector {
    fn ensure_terminal_started(&self) -> Result<(), String> {
        let path = self
            .terminal_path
            .as_deref()
            .expect("terminal path is required for terminal startup");
        let mut launched = self
            .launched_terminal
            .lock()
            .map_err(|_| "terminal process lock is poisoned".to_owned())?;

        if let Some(child) = launched.as_mut() {
            match child.try_wait() {
                Ok(None) => return Ok(()),
                Ok(Some(status)) => {
                    tracing::debug!(
                        path = %path.display(),
                        ?status,
                        "configured MT5 terminal exited; relaunching"
                    );
                    *launched = None;
                }
                Err(error) => {
                    return Err(format!(
                        "failed to inspect configured terminal {}: {error}",
                        path.display()
                    ));
                }
            }
        }

        let child = mt5_windows::start_terminal(path).map_err(|error| error.to_string())?;
        tracing::info!(path = %path.display(), pid = child.id(), "started configured MT5 terminal");
        *launched = Some(child);
        Ok(())
    }
}

#[cfg(windows)]
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn explicit_pipe_takes_precedence_over_terminal_path() {
        let target = pipe_target_from_values(
            Some(r"\\.\pipe\explicit".to_owned()),
            Some(r"C:\missing\terminal64.exe".to_owned()),
        )
        .unwrap();

        assert!(matches!(target, PipeTarget::PipeName(name) if name == r"\\.\pipe\explicit"));
    }

    #[test]
    fn terminal_path_derives_pipe_name() {
        let target = pipe_target_from_values(
            None,
            Some(r"C:\Program Files\MetaTrader 5\terminal64.exe".to_owned()),
        )
        .unwrap();

        assert!(matches!(
            target,
            PipeTarget::Terminal { pipe_name, path }
                if pipe_name == r"\\.\pipe\MT5.Terminal.781AEDD6B227148DB36F632AFAB710BBA441CCEA07ED9EF5BC7B94FAED25BD12"
                    && path.to_string_lossy() == r"C:\Program Files\MetaTrader 5\terminal64.exe"
        ));
    }

    #[test]
    fn empty_pipe_name_falls_back_to_terminal_path() {
        let target = pipe_target_from_values(
            Some(String::new()),
            Some(r"C:\Program Files\MetaTrader 5\terminal64.exe".to_owned()),
        )
        .unwrap();

        assert!(matches!(target, PipeTarget::Terminal { .. }));
    }
}
