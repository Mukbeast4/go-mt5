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
    #[error("timed out opening named pipe {0}")]
    TimedOut(String),
    #[error("failed to open named pipe {name}: {source}")]
    Open {
        name: String,
        source: std::io::Error,
    },
    #[error("named pipes are only supported by the Windows bridge executable")]
    UnsupportedPlatform,
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

    let deadline = Instant::now() + timeout;
    loop {
        match ClientOptions::new().read(true).write(true).open(name) {
            Ok(pipe) => return Ok(pipe),
            Err(error) if error.raw_os_error() == Some(231) && Instant::now() < deadline => {
                tokio::time::sleep(Duration::from_millis(50)).await;
            }
            Err(error) if error.raw_os_error() == Some(231) => {
                return Err(PipeError::TimedOut(name.to_owned()));
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

#[cfg(not(windows))]
pub struct NativePipe;

#[cfg(not(windows))]
pub async fn open_pipe(_name: &str, _timeout: Duration) -> Result<NativePipe, PipeError> {
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
