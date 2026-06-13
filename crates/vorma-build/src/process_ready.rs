//! Loopback process readiness polling.

use std::io::{Read, Write};
use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpStream};
use std::time::{Duration, Instant};

use crate::process_runner::{BuildProcessCancel, BuildProcessError, StartedBuildProcess};
use crate::vite_plugin_contract::VITE_PLUGIN_LOOPBACK_HOST;

const READY_POLL_INTERVAL: Duration = Duration::from_millis(25);
const READY_CONNECT_TIMEOUT: Duration = Duration::from_millis(25);
const READY_READ_TIMEOUT: Duration = Duration::from_millis(25);

/// Wait for a started loopback HTTP process to return a successful readiness response.
pub fn wait_for_loopback_http_ready(
	process: &mut dyn StartedBuildProcess,
	port: u16,
	path: &str,
	timeout: Duration,
) -> Result<(), ProcessReadyError> {
	let cancel = BuildProcessCancel::new();
	wait_for_loopback_http_ready_until_cancelled(process, port, path, timeout, &cancel)
}

/// Wait for a started loopback HTTP process to become ready unless cancellation fires.
pub fn wait_for_loopback_http_ready_until_cancelled(
	process: &mut dyn StartedBuildProcess,
	port: u16,
	path: &str,
	timeout: Duration,
	cancel: &BuildProcessCancel,
) -> Result<(), ProcessReadyError> {
	if port == 0 {
		return Err(ProcessReadyError::InvalidPort);
	}
	if !path.starts_with('/') {
		return Err(ProcessReadyError::InvalidPath {
			path: path.to_owned(),
		});
	}
	let deadline = Instant::now() + timeout;
	loop {
		if cancel.is_cancelled() {
			return Err(ProcessReadyError::Cancelled);
		}
		if http_ready(port, path) {
			return Ok(());
		}
		if let Some(status) = process
			.exit_status()
			.map_err(|source| ProcessReadyError::Process { source })?
		{
			return Err(ProcessReadyError::Exited { status });
		}
		if Instant::now() >= deadline {
			return Err(ProcessReadyError::Timeout {
				port,
				path: path.to_owned(),
			});
		}
		std::thread::sleep(READY_POLL_INTERVAL);
	}
}

/// Process readiness error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProcessReadyError {
	/// Ready port cannot be zero.
	InvalidPort,
	/// Ready endpoint path must be absolute.
	InvalidPath {
		/// Rejected path.
		path: String,
	},
	/// Process status could not be checked.
	Process {
		/// Source process error.
		source: BuildProcessError,
	},
	/// Process exited before it became ready.
	Exited {
		/// Exit status.
		status: String,
	},
	/// Process did not become ready in time.
	Timeout {
		/// Ready port.
		port: u16,
		/// Ready endpoint path.
		path: String,
	},
	/// Readiness waiting was cancelled.
	Cancelled,
}

impl std::fmt::Display for ProcessReadyError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidPort => f.write_str("ready port cannot be zero"),
			Self::InvalidPath { path } => {
				write!(f, "ready endpoint path must start with slash: {path:?}")
			}
			Self::Process { source } => write!(f, "{source}"),
			Self::Exited { status } => {
				write!(f, "process exited before becoming ready: {status}")
			}
			Self::Timeout { port, path } => {
				write!(
					f,
					"process did not become ready at http://{VITE_PLUGIN_LOOPBACK_HOST}:{port}{path}"
				)
			}
			Self::Cancelled => f.write_str("process readiness wait cancelled"),
		}
	}
}

impl std::error::Error for ProcessReadyError {}

fn http_ready(port: u16, path: &str) -> bool {
	let addr = SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), port);
	let Ok(mut stream) = TcpStream::connect_timeout(&addr, READY_CONNECT_TIMEOUT) else {
		return false;
	};
	let _ = stream.set_read_timeout(Some(READY_READ_TIMEOUT));
	let request = format!(
		"GET {path} HTTP/1.1\r\nHost: {VITE_PLUGIN_LOOPBACK_HOST}:{port}\r\nConnection: close\r\n\r\n"
	);
	if stream.write_all(request.as_bytes()).is_err() {
		return false;
	}
	let mut response_prefix = [0_u8; 16];
	let Ok(read) = stream.read(&mut response_prefix) else {
		return false;
	};
	matches!(
		std::str::from_utf8(&response_prefix[..read]),
		Ok(prefix) if prefix.starts_with("HTTP/1.1 2") || prefix.starts_with("HTTP/1.0 2")
	)
}

#[cfg(test)]
mod tests {
	use std::net::TcpListener;
	use std::thread;

	use super::*;

	const READY_PATH: &str = "/ready";

	struct FakeProcess {
		status: Option<String>,
	}

	impl StartedBuildProcess for FakeProcess {
		fn process_id(&self) -> Option<u32> {
			Some(7)
		}

		fn exit_status(&mut self) -> Result<Option<String>, BuildProcessError> {
			Ok(self.status.clone())
		}

		fn terminate(&mut self) -> Result<(), BuildProcessError> {
			Ok(())
		}
	}

	#[test]
	fn readiness_probe_intervals_stay_below_local_latency_budget() {
		assert!(READY_POLL_INTERVAL <= Duration::from_millis(25));
		assert!(READY_CONNECT_TIMEOUT <= Duration::from_millis(25));
		assert!(READY_READ_TIMEOUT <= Duration::from_millis(25));
	}

	#[test]
	fn readiness_poll_returns_when_loopback_endpoint_is_ok() {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut request = [0_u8; 256];
			let _ = stream.read(&mut request);
			stream
				.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
				.unwrap();
		});
		let mut process = FakeProcess { status: None };

		wait_for_loopback_http_ready(&mut process, port, READY_PATH, Duration::from_secs(1))
			.unwrap();
	}

	#[test]
	fn readiness_poll_reports_early_exit() {
		let mut process = FakeProcess {
			status: Some("exit status: 1".to_owned()),
		};
		let error =
			wait_for_loopback_http_ready(&mut process, 1, READY_PATH, Duration::from_millis(1))
				.unwrap_err();

		assert_eq!(
			error,
			ProcessReadyError::Exited {
				status: "exit status: 1".to_owned()
			}
		);
	}

	#[test]
	fn readiness_wait_returns_immediately_when_cancelled() {
		let mut process = FakeProcess { status: None };
		let cancel = BuildProcessCancel::new();
		cancel.cancel();

		let error = wait_for_loopback_http_ready_until_cancelled(
			&mut process,
			1,
			READY_PATH,
			Duration::from_secs(90),
			&cancel,
		)
		.unwrap_err();

		assert_eq!(error, ProcessReadyError::Cancelled);
	}
}
