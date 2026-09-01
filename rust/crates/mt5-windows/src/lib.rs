//! Windows NamedPipe connection support and MT5 pipe-name derivation.

use std::{
    path::Path,
    time::{Duration, Instant},
};

use sha2::{Digest, Sha256};
use thiserror::Error;

#[derive(Debug, Error)]
pub enum PipeError {
    #[error("pipe path must be non-empty")]
    EmptyPath,
    #[error("named pipe {0} was not found")]
    NotFound(String),
    #[error("timed out opening named pipe {0}")]
    TimedOut(String),
    #[error("failed to open named pipe {name}: {source}")]
    Open {
        name: String,
        source: std::io::Error,
    },
    #[error("failed to start terminal {path}: {source}")]
    StartTerminal {
        path: String,
        source: std::io::Error,
    },
    #[error("named pipes are only supported by the Windows bridge executable")]
    UnsupportedPlatform,
}

impl PipeError {
    /// Returns whether the pipe endpoint itself is absent and may be created
    /// by starting the configured terminal.
    pub fn is_missing(&self) -> bool {
        matches!(self, Self::NotFound(_))
    }
}

/// Derive the same name as the Go reference implementation.  MT5 uses a
/// SHA-256 of the lower-cased `\\?\` executable path encoded as UTF-16LE.
pub fn pipe_name_for_terminal_path(path: impl AsRef<Path>) -> Result<String, PipeError> {
    let path = path.as_ref().to_string_lossy();
    if path.is_empty() {
        return Err(PipeError::EmptyPath);
    }
    let input = format!(r"\\?\{}", path.to_lowercase());
    let mut encoded = Vec::with_capacity(input.len() * 2);
    for unit in input.encode_utf16() {
        encoded.extend_from_slice(&unit.to_le_bytes());
    }
    let digest = Sha256::digest(encoded);
    Ok(format!(
        r"\\.\pipe\MT5.Terminal.{}",
        hex::encode_upper(digest)
    ))
}

#[cfg(windows)]
pub type NativePipe = tokio::net::windows::named_pipe::NamedPipeClient;

#[cfg(windows)]
pub async fn open_pipe(name: &str, timeout: Duration) -> Result<NativePipe, PipeError> {
    use tokio::net::windows::named_pipe::ClientOptions;

    open_pipe_until(name, timeout, false, |name| {
        ClientOptions::new().read(true).write(true).open(name)
    })
    .await
}

/// Open a pipe while it is being created by a terminal startup.
///
/// Unlike [`open_pipe`], a missing pipe is treated as transient until the
/// timeout expires. This is used only after the bridge has started the
/// configured terminal path.
#[cfg(windows)]
pub async fn wait_for_pipe(name: &str, timeout: Duration) -> Result<NativePipe, PipeError> {
    use tokio::net::windows::named_pipe::ClientOptions;

    open_pipe_until(name, timeout, true, |name| {
        ClientOptions::new().read(true).write(true).open(name)
    })
    .await
}

#[cfg(windows)]
async fn open_pipe_until<T, F>(
    name: &str,
    timeout: Duration,
    wait_for_missing: bool,
    mut open: F,
) -> Result<T, PipeError>
where
    F: FnMut(&str) -> Result<T, std::io::Error>,
{
    let deadline = Instant::now() + timeout;
    loop {
        match open(name) {
            Ok(pipe) => return Ok(pipe),
            Err(error) if is_pipe_busy(&error) && Instant::now() < deadline => {
                tokio::time::sleep(Duration::from_millis(50)).await;
            }
            Err(error)
                if is_pipe_missing_os_error(&error)
                    && wait_for_missing
                    && Instant::now() < deadline =>
            {
                tokio::time::sleep(Duration::from_millis(50)).await;
            }
            Err(error) if is_pipe_busy(&error) => {
                return Err(PipeError::TimedOut(name.to_owned()));
            }
            Err(error) if is_pipe_missing_os_error(&error) => {
                if wait_for_missing {
                    return Err(PipeError::TimedOut(name.to_owned()));
                }
                return Err(PipeError::NotFound(name.to_owned()));
            }
            Err(source) => {
                return Err(PipeError::Open {
                    name: name.to_owned(),
                    source,
                });
            }
        }
    }
}

#[cfg(windows)]
fn is_pipe_busy(error: &std::io::Error) -> bool {
    error.raw_os_error() == Some(231)
}

#[cfg(windows)]
fn is_pipe_missing_os_error(error: &std::io::Error) -> bool {
    fn cause_is_missing(cause: &(dyn std::error::Error + 'static)) -> bool {
        if let Some(io_error) = cause.downcast_ref::<std::io::Error>()
            && (io_error.kind() == std::io::ErrorKind::NotFound
                || matches!(io_error.raw_os_error(), Some(2 | 3)))
        {
            return true;
        }

        if let Some(io_error) = cause.downcast_ref::<std::io::Error>()
            && let Some(inner) = io_error.get_ref()
            && cause_is_missing(inner)
        {
            return true;
        }

        cause.source().is_some_and(cause_is_missing)
    }

    cause_is_missing(error)
}

/// Start a terminal executable and return its process handle for guarded
/// relaunch decisions by the bridge connector.
#[cfg(windows)]
pub fn start_terminal(path: impl AsRef<Path>) -> Result<std::process::Child, PipeError> {
    let path = path.as_ref();
    if path.as_os_str().is_empty() {
        return Err(PipeError::EmptyPath);
    }

    let mut command = std::process::Command::new(path);
    if let Some(parent) = path
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
    {
        command.current_dir(parent);
    }
    command.spawn().map_err(|source| PipeError::StartTerminal {
        path: path.to_string_lossy().into_owned(),
        source,
    })
}

#[cfg(not(windows))]
pub struct NativePipe;

#[cfg(not(windows))]
pub async fn open_pipe(_name: &str, _timeout: Duration) -> Result<NativePipe, PipeError> {
    Err(PipeError::UnsupportedPlatform)
}

#[cfg(not(windows))]
pub async fn wait_for_pipe(_name: &str, _timeout: Duration) -> Result<NativePipe, PipeError> {
    Err(PipeError::UnsupportedPlatform)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn path_hash_matches_go_fixture() {
        let name =
            pipe_name_for_terminal_path(r"C:\Program Files\MetaTrader 5\terminal64.exe").unwrap();
        assert_eq!(
            name,
            r"\\.\pipe\MT5.Terminal.781AEDD6B227148DB36F632AFAB710BBA441CCEA07ED9EF5BC7B94FAED25BD12"
        );
    }

    #[test]
    fn path_hash_is_case_insensitive() {
        assert_eq!(
            pipe_name_for_terminal_path(r"C:\Program Files\MetaTrader 5\terminal64.exe").unwrap(),
            pipe_name_for_terminal_path(r"c:\program files\metatrader 5\terminal64.exe").unwrap(),
        );
    }
}

#[cfg(windows)]
#[cfg(test)]
mod windows_tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    #[test]
    fn missing_pipe_errors_are_classified() {
        assert!(is_pipe_missing_os_error(
            &std::io::Error::from_raw_os_error(2)
        ));
        assert!(is_pipe_missing_os_error(
            &std::io::Error::from_raw_os_error(3)
        ));
        assert!(is_pipe_missing_os_error(&std::io::Error::new(
            std::io::ErrorKind::NotFound,
            "Wine reported a missing named pipe without a raw Win32 code",
        )));
        assert!(is_pipe_missing_os_error(&std::io::Error::other(
            std::io::Error::from_raw_os_error(2),
        )));
        assert!(!is_pipe_missing_os_error(
            &std::io::Error::from_raw_os_error(231)
        ));
        assert!(!is_pipe_missing_os_error(
            &std::io::Error::from_raw_os_error(5)
        ));
    }

    #[tokio::test]
    async fn wait_for_missing_pipe_retries_until_available() {
        let attempts = AtomicUsize::new(0);
        let result = open_pipe_until(r"\\.\pipe\test", Duration::from_millis(250), true, |_| {
            if attempts.fetch_add(1, Ordering::Relaxed) < 2 {
                Err(std::io::Error::from_raw_os_error(2))
            } else {
                Ok(())
            }
        })
        .await;

        assert_eq!(result.unwrap(), ());
        assert_eq!(attempts.load(Ordering::Relaxed), 3);
    }

    #[tokio::test]
    async fn explicit_pipe_open_returns_missing_immediately() {
        let attempts = AtomicUsize::new(0);
        let result = open_pipe_until(r"\\.\pipe\test", Duration::from_secs(1), false, |_| {
            attempts.fetch_add(1, Ordering::Relaxed);
            Err::<(), _>(std::io::Error::from_raw_os_error(2))
        })
        .await;

        assert!(matches!(result, Err(PipeError::NotFound(name)) if name == r"\\.\pipe\test"));
        assert_eq!(attempts.load(Ordering::Relaxed), 1);
    }
}
