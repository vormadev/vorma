//! Vite plugin development control client.
//!
//! This is the outbound half of dev-time Vite communication (the build
//! process telling the already-running Vite plugin what changed); the
//! inbound half (the plugin asking the build process for config or an
//! asset hash) is [`crate::vite_plugin_rpc`]. See
//! [`crate::vite_plugin_contract::VITE_PLUGIN_CONFIG_CHANGED_PATH`] and
//! [`crate::vite_plugin_contract::VITE_PLUGIN_ASSETS_CHANGED_PATH`] for
//! which of [`VitePluginControlClient`]'s two notifications applies to
//! which kind of change.

use std::io::{Read, Write};
use std::net::TcpStream;

use crate::vite_plugin_contract::{
	VITE_PLUGIN_ASSETS_CHANGED_PATH, VITE_PLUGIN_CONFIG_CHANGED_PATH, VITE_PLUGIN_LOOPBACK_HOST,
	VITE_PLUGIN_TOKEN_HEADER,
};

/// Client that notifies the Vite plugin control server after committed dev
/// config changes. Trait so tests can substitute a fake that records
/// notifications without a real loopback connection;
/// [`LoopbackVitePluginControlClient`] is the production implementation.
pub trait VitePluginControlClient {
	/// Notify the plugin that its config should be reloaded (Vite restarts —
	/// see [`crate::vite_plugin_contract::VITE_PLUGIN_CONFIG_CHANGED_PATH`]'s
	/// docs for when this is and is not the right call).
	fn notify_config_changed(
		&mut self,
		control_port: u16,
		token: &str,
	) -> Result<(), VitePluginControlError>;

	/// Notify the plugin that these public-asset source paths changed
	/// (targeted invalidation, no Vite restart — see
	/// [`crate::vite_plugin_contract::VITE_PLUGIN_ASSETS_CHANGED_PATH`]'s
	/// docs).
	fn notify_assets_changed(
		&mut self,
		control_port: u16,
		token: &str,
		changed_source_paths: &[String],
	) -> Result<(), VitePluginControlError>;
}

/// Loopback HTTP implementation of the Vite plugin control client: a raw
/// HTTP/1.1 `POST` over a plain `TcpStream` (no HTTP client dependency
/// needed for two fixed, tiny, same-machine requests), authenticated with
/// the Vite plugin token header.
#[derive(Clone, Copy, Debug, Default)]
pub struct LoopbackVitePluginControlClient;

impl VitePluginControlClient for LoopbackVitePluginControlClient {
	fn notify_config_changed(
		&mut self,
		control_port: u16,
		token: &str,
	) -> Result<(), VitePluginControlError> {
		post_control_request(control_port, token, VITE_PLUGIN_CONFIG_CHANGED_PATH, "")
	}

	fn notify_assets_changed(
		&mut self,
		control_port: u16,
		token: &str,
		changed_source_paths: &[String],
	) -> Result<(), VitePluginControlError> {
		let body = serde_json::to_string(changed_source_paths).map_err(|source| {
			VitePluginControlError::Io {
				operation: "serialize",
				message: source.to_string(),
			}
		})?;
		post_control_request(control_port, token, VITE_PLUGIN_ASSETS_CHANGED_PATH, &body)
	}
}

fn post_control_request(
	control_port: u16,
	token: &str,
	path: &str,
	body: &str,
) -> Result<(), VitePluginControlError> {
	if control_port == 0 {
		return Err(VitePluginControlError::InvalidControlPort);
	}
	if token.is_empty() {
		return Err(VitePluginControlError::EmptyToken);
	}
	let mut stream =
		TcpStream::connect((VITE_PLUGIN_LOOPBACK_HOST, control_port)).map_err(|source| {
			VitePluginControlError::Io {
				operation: "connect",
				message: source.to_string(),
			}
		})?;
	let content_length = body.len();
	let request = format!(
		"POST {path} HTTP/1.1\r\nHost: {VITE_PLUGIN_LOOPBACK_HOST}:{control_port}\r\n{VITE_PLUGIN_TOKEN_HEADER}: {token}\r\nContent-Type: application/json\r\nContent-Length: {content_length}\r\nConnection: close\r\n\r\n{body}"
	);
	stream
		.write_all(request.as_bytes())
		.map_err(|source| VitePluginControlError::Io {
			operation: "write",
			message: source.to_string(),
		})?;
	let mut response = String::new();
	stream
		.read_to_string(&mut response)
		.map_err(|source| VitePluginControlError::Io {
			operation: "read",
			message: source.to_string(),
		})?;
	validate_response_status(&response)
}

/// Error from [`VitePluginControlClient`]'s methods.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum VitePluginControlError {
	/// Control port cannot be zero.
	InvalidControlPort,
	/// Control token cannot be empty.
	EmptyToken,
	/// Control request I/O failed.
	Io {
		/// Failed operation.
		operation: &'static str,
		/// I/O error message.
		message: String,
	},
	/// Control response did not include a status line.
	InvalidResponse {
		/// Raw response prefix.
		response: String,
	},
	/// Control server returned a non-OK response.
	UnexpectedStatus {
		/// HTTP status line.
		status_line: String,
	},
}

impl std::fmt::Display for VitePluginControlError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidControlPort => f.write_str("Vite plugin control port cannot be zero"),
			Self::EmptyToken => f.write_str("Vite plugin control token cannot be empty"),
			Self::Io { operation, message } => {
				write!(f, "Vite plugin control {operation}: {message}")
			}
			Self::InvalidResponse { response } => {
				write!(f, "invalid Vite plugin control response {response:?}")
			}
			Self::UnexpectedStatus { status_line } => {
				write!(f, "Vite plugin control returned {status_line:?}")
			}
		}
	}
}

impl std::error::Error for VitePluginControlError {}

fn validate_response_status(response: &str) -> Result<(), VitePluginControlError> {
	let status_line =
		response
			.lines()
			.next()
			.ok_or_else(|| VitePluginControlError::InvalidResponse {
				response: response.to_owned(),
			})?;
	if status_line.starts_with("HTTP/1.1 200 ") || status_line.starts_with("HTTP/1.0 200 ") {
		return Ok(());
	}
	Err(VitePluginControlError::UnexpectedStatus {
		status_line: status_line.to_owned(),
	})
}

#[cfg(test)]
mod tests {
	use std::net::TcpListener;
	use std::sync::mpsc;
	use std::thread;

	use super::*;

	#[test]
	fn loopback_control_client_posts_tokenized_config_change() {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let (tx, rx) = mpsc::channel();
		let thread = thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut buffer = [0; 2048];
			let n = stream.read(&mut buffer).unwrap();
			let request = String::from_utf8_lossy(&buffer[..n]).into_owned();
			stream
				.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok")
				.unwrap();
			tx.send(request).unwrap();
		});
		let mut client = LoopbackVitePluginControlClient;

		client.notify_config_changed(port, "token").unwrap();

		let request = rx.recv().unwrap();
		thread.join().unwrap();
		assert!(request.starts_with("POST /cfg-changed HTTP/1.1\r\n"));
		assert!(request.contains("x-vorma-vite-plugin-token: token\r\n"));
	}

	#[test]
	fn control_response_rejects_non_ok_status() {
		let error = validate_response_status("HTTP/1.1 403 Forbidden\r\n\r\n").unwrap_err();

		assert!(matches!(
			error,
			VitePluginControlError::UnexpectedStatus { status_line }
				if status_line == "HTTP/1.1 403 Forbidden"
		));
	}
}
