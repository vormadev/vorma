//! Pure Vite plugin RPC contract handling.
//!
//! This is the inbound half of dev-time Vite communication: the running
//! Vite plugin asks this crate's build process for its config, the public
//! URL for a source path, or reports its own control port back
//! ([`VitePluginRpcState::handle_rpc`] dispatches
//! [`crate::vite_plugin_contract::VitePluginRpcRequest`]'s three variants).
//! [`crate::vite_plugin_control`] is the outbound half (the build process
//! telling the plugin something changed). The request-handling logic
//! ([`VitePluginRpcState::handle_rpc`] and its private helpers) is pure —
//! no I/O, fully testable without a real server — while
//! [`LoopbackVitePluginRpcServer`] is the one place that actually binds a
//! socket and serves it via axum.

use std::collections::BTreeMap;
use std::io;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use axum::Router;
use axum::body::Bytes;
use axum::extract::State;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use http::header::CONTENT_TYPE;
use http::{HeaderMap, HeaderValue, StatusCode};
use tokio::sync::oneshot;
use vorma::build_interface::assets::public_source_path_key;

use crate::generation_inputs::PreparedBuildInputs;
use crate::vite_plugin_contract::{
	VITE_PLUGIN_BASE_PATH, VITE_PLUGIN_LOOPBACK_HOST, VITE_PLUGIN_RPC_PATH,
	VITE_PLUGIN_TOKEN_HEADER, VitePluginConfig, VitePluginRpcRequest,
};

/// Maximum accepted Vite plugin RPC body size: requests larger than this
/// are rejected with `413 Payload Too Large` before JSON parsing, in
/// [`VitePluginRpcState::handle_rpc`].
pub const VITE_PLUGIN_RPC_BODY_LIMIT: usize = 64 * 1024;
const CONTENT_TYPE_HEADER_VALUE_JSON: &str = "application/json";
const CONTENT_TYPE_HEADER_VALUE_TEXT: &str = "text/plain; charset=utf-8";

/// Started Vite plugin RPC server handle, returned by
/// [`VitePluginRpcServer::start_vite_plugin_rpc_server`]. Dropping this
/// value sends a graceful-shutdown signal and joins the serving thread.
#[derive(Debug)]
pub struct VitePluginRpcServerHandle {
	port: u16,
	shutdown: Option<oneshot::Sender<()>>,
	thread: Option<JoinHandle<()>>,
}

impl VitePluginRpcServerHandle {
	/// Create a started Vite plugin RPC server handle without shutdown state.
	#[cfg(test)]
	pub fn new(port: u16) -> Self {
		Self {
			port,
			shutdown: None,
			thread: None,
		}
	}

	/// Create a started Vite plugin RPC server handle with owned shutdown state.
	pub fn with_shutdown(port: u16, shutdown: oneshot::Sender<()>, thread: JoinHandle<()>) -> Self {
		Self {
			port,
			shutdown: Some(shutdown),
			thread: Some(thread),
		}
	}

	/// Bound loopback port.
	pub fn port(&self) -> u16 {
		self.port
	}
}

impl Drop for VitePluginRpcServerHandle {
	fn drop(&mut self) {
		if let Some(shutdown) = self.shutdown.take() {
			let _ = shutdown.send(());
		}
		if let Some(thread) = self.thread.take() {
			let _ = thread.join();
		}
	}
}

/// Runtime boundary that serves Vite plugin RPC requests while Vite runs.
/// Trait so tests can substitute a fake that records the served state
/// without binding a real socket; [`LoopbackVitePluginRpcServer`] is the
/// production implementation.
pub trait VitePluginRpcServer {
	/// Start serving the provided Vite plugin RPC state on an OS-assigned
	/// loopback port.
	fn start_vite_plugin_rpc_server(
		&mut self,
		state: VitePluginRpcState,
	) -> Result<VitePluginRpcServerHandle, VitePluginServerError>;
}

/// Error from [`VitePluginRpcServer::start_vite_plugin_rpc_server`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VitePluginServerError {
	message: String,
}

impl VitePluginServerError {
	/// Create a Vite plugin RPC server error.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}
}

impl std::fmt::Display for VitePluginServerError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		f.write_str(&self.message)
	}
}

impl std::error::Error for VitePluginServerError {}

/// Loopback Axum implementation of the Vite plugin RPC server boundary:
/// binds an OS-assigned ephemeral port and serves
/// [`VitePluginRpcState::handle_rpc`]'s pure request handling through it on
/// a dedicated thread with its own current-thread tokio runtime.
#[derive(Clone, Copy, Debug, Default)]
pub struct LoopbackVitePluginRpcServer;

impl VitePluginRpcServer for LoopbackVitePluginRpcServer {
	fn start_vite_plugin_rpc_server(
		&mut self,
		state: VitePluginRpcState,
	) -> Result<VitePluginRpcServerHandle, VitePluginServerError> {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).map_err(|source| {
			VitePluginServerError::new(format!("bind Vite plugin RPC server: {source}"))
		})?;
		let port = listener
			.local_addr()
			.map_err(|source| {
				VitePluginServerError::new(format!("read Vite plugin RPC server addr: {source}"))
			})?
			.port();
		listener.set_nonblocking(true).map_err(|source| {
			VitePluginServerError::new(format!("configure Vite plugin RPC listener: {source}"))
		})?;
		let (shutdown_tx, shutdown_rx) = tokio::sync::oneshot::channel::<()>();
		let thread = thread::spawn(move || {
			let _ = serve_loopback_rpc(listener, state, shutdown_rx);
		});
		Ok(VitePluginRpcServerHandle::with_shutdown(
			port,
			shutdown_tx,
			thread,
		))
	}
}

/// Vite plugin RPC state for one generation candidate: the config and
/// public filemap the plugin currently sees, plus the reported Vite
/// control port, all behind a shared lock so
/// [`Self::update_generation_contract`] can swap them for a fresh
/// generation while requests keep being served. `Clone`-cheap (`Arc`-backed
/// internally); this is the value handed to both
/// [`VitePluginRpcServer::start_vite_plugin_rpc_server`] and
/// [`crate::dev_build`]'s own generation-update code, so both sides see the
/// same live state.
#[derive(Clone, Debug)]
pub struct VitePluginRpcState {
	token: String,
	inner: Arc<Mutex<VitePluginRpcSharedState>>,
}

#[derive(Clone, Debug)]
struct VitePluginRpcSharedState {
	config: VitePluginConfig,
	public_filemap: BTreeMap<String, String>,
	control_port: Option<u16>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct VitePluginGenerationContract {
	config: VitePluginConfig,
	public_filemap: BTreeMap<String, String>,
}

impl VitePluginGenerationContract {
	pub(crate) fn config(&self) -> &VitePluginConfig {
		&self.config
	}

	pub(crate) fn public_filemap(&self) -> &BTreeMap<String, String> {
		&self.public_filemap
	}
}

impl VitePluginRpcState {
	/// Create Vite plugin RPC state.
	pub fn new(
		token: impl Into<String>,
		config: VitePluginConfig,
		public_filemap: BTreeMap<String, String>,
	) -> Self {
		Self {
			token: token.into(),
			inner: Arc::new(Mutex::new(VitePluginRpcSharedState {
				config,
				public_filemap,
				control_port: None,
			})),
		}
	}

	/// Create Vite plugin RPC state from prepared build inputs: the
	/// convenience constructor most callers use, reading the initial config
	/// and public filemap straight off an already-prepared
	/// [`PreparedBuildInputs`] instead of supplying them separately.
	pub fn from_prepared_build_inputs(
		token: impl Into<String>,
		inputs: &PreparedBuildInputs,
	) -> Self {
		Self::new(
			token,
			inputs.vite_plugin_config().clone(),
			inputs.public_static_outputs().public_filemap().clone(),
		)
	}

	/// Replace the generation contract served to the Vite plugin: after this
	/// call, [`Self::handle_rpc`]'s `Cfg`/`Hash` responses reflect the new
	/// `config`/`public_filemap` for every subsequent request. Callers
	/// typically follow this with a
	/// [`crate::vite_plugin_control::VitePluginControlClient`] notification
	/// so the plugin actually re-fetches rather than serving stale cached
	/// values.
	pub fn update_generation_contract(
		&self,
		config: VitePluginConfig,
		public_filemap: BTreeMap<String, String>,
	) {
		let mut inner = self.inner.lock().expect("Vite plugin RPC lock poisoned");
		inner.config = config;
		inner.public_filemap = public_filemap;
	}

	pub(crate) fn generation_contract(&self) -> VitePluginGenerationContract {
		let inner = self.inner.lock().expect("Vite plugin RPC lock poisoned");
		VitePluginGenerationContract {
			config: inner.config.clone(),
			public_filemap: inner.public_filemap.clone(),
		}
	}

	pub(crate) fn restore_generation_contract(&self, contract: VitePluginGenerationContract) {
		self.update_generation_contract(contract.config, contract.public_filemap);
	}

	/// Current Vite control port reported by the plugin, if
	/// [`VitePluginRpcRequest::SetPort`] has been handled successfully at
	/// least once for this state. `None` until then.
	pub fn control_port(&self) -> Option<u16> {
		self.inner
			.lock()
			.expect("Vite plugin RPC lock poisoned")
			.control_port
	}

	/// Record the Vite control port reported by the plugin. Called only
	/// after [`Self::handle_rpc`] has confirmed the reported port is
	/// actually reachable (see the doctrine comment on the private
	/// `loopback_control_port_reachable` in this module for why that check
	/// happens before recording rather than at every later use).
	pub fn record_control_port(&self, port: u16) {
		self.inner
			.lock()
			.expect("Vite plugin RPC lock poisoned")
			.control_port = Some(port);
	}

	/// Handle one Vite plugin RPC request: validates the auth token header
	/// (see [`crate::vite_plugin_contract::VITE_PLUGIN_TOKEN_HEADER`]),
	/// enforces [`VITE_PLUGIN_RPC_BODY_LIMIT`], parses the JSON body as a
	/// [`VitePluginRpcRequest`], and dispatches to the matching handler.
	/// Pure: no I/O, so this is directly testable without a server.
	pub fn handle_rpc(&self, headers: &HeaderMap, body: &[u8]) -> VitePluginRpcResponse {
		if let Some(response) = self.token_error_response(headers) {
			return response;
		}
		if body.len() > VITE_PLUGIN_RPC_BODY_LIMIT {
			return VitePluginRpcResponse::text(StatusCode::PAYLOAD_TOO_LARGE, "");
		}
		let request = match serde_json::from_slice::<VitePluginRpcRequest>(body) {
			Ok(request) => request,
			Err(_) => {
				return VitePluginRpcResponse::text(
					StatusCode::BAD_REQUEST,
					"invalid JSON RPC request",
				);
			}
		};
		match request {
			VitePluginRpcRequest::Cfg => self.config_response(),
			VitePluginRpcRequest::Hash { src_path } => self.hash_response(&src_path),
			VitePluginRpcRequest::SetPort { port } => self.set_port_response(port),
		}
	}

	fn token_error_response(&self, headers: &HeaderMap) -> Option<VitePluginRpcResponse> {
		if self.token.is_empty() {
			return Some(VitePluginRpcResponse::text(
				StatusCode::INTERNAL_SERVER_ERROR,
				"Vite plugin control token not available",
			));
		}
		let provided = headers
			.get(VITE_PLUGIN_TOKEN_HEADER)
			.and_then(|value| value.to_str().ok());
		if provided != Some(self.token.as_str()) {
			return Some(VitePluginRpcResponse::text(StatusCode::FORBIDDEN, ""));
		}
		None
	}

	fn config_response(&self) -> VitePluginRpcResponse {
		let config = self
			.inner
			.lock()
			.expect("Vite plugin RPC lock poisoned")
			.config
			.clone();
		match serde_json::to_vec(&config) {
			Ok(body) => VitePluginRpcResponse::json(StatusCode::OK, body),
			Err(source) => VitePluginRpcResponse::text(
				StatusCode::INTERNAL_SERVER_ERROR,
				&format!("error encoding config: {source}"),
			),
		}
	}

	fn hash_response(&self, src_path: &str) -> VitePluginRpcResponse {
		let Some(key) = public_source_path_key(src_path) else {
			return VitePluginRpcResponse::text(
				StatusCode::BAD_REQUEST,
				"missing src_path parameter",
			);
		};
		let public_url = self
			.inner
			.lock()
			.expect("Vite plugin RPC lock poisoned")
			.public_filemap
			.get(key)
			.cloned();
		let Some(public_url) = public_url else {
			return VitePluginRpcResponse::text(StatusCode::NOT_FOUND, "");
		};
		VitePluginRpcResponse::text(StatusCode::OK, &public_url)
	}

	fn set_port_response(&self, port: u16) -> VitePluginRpcResponse {
		if !loopback_control_port_reachable(port) {
			return VitePluginRpcResponse::text(
				StatusCode::INTERNAL_SERVER_ERROR,
				"Vite plugin control port is not reachable",
			);
		}
		self.record_control_port(port);
		VitePluginRpcResponse::text(StatusCode::OK, "OK")
	}
}

const CONTROL_PORT_PROBE_TIMEOUT: Duration = Duration::from_millis(250);

/*
The recorded control port is what config-change notifications are later
posted to, so a port the plugin reported incorrectly must be rejected at
recording time instead of failing every later notification.
*/
fn loopback_control_port_reachable(port: u16) -> bool {
	if port == 0 {
		return false;
	}
	let Ok(host) = VITE_PLUGIN_LOOPBACK_HOST.parse() else {
		return false;
	};
	TcpStream::connect_timeout(&SocketAddr::new(host, port), CONTROL_PORT_PROBE_TIMEOUT).is_ok()
}

fn serve_loopback_rpc(
	listener: TcpListener,
	state: VitePluginRpcState,
	shutdown_rx: tokio::sync::oneshot::Receiver<()>,
) -> Result<(), io::Error> {
	let runtime = tokio::runtime::Builder::new_current_thread()
		.enable_io()
		.build()?;
	runtime.block_on(async move {
		let listener = tokio::net::TcpListener::from_std(listener)?;
		let path = format!("{VITE_PLUGIN_BASE_PATH}{VITE_PLUGIN_RPC_PATH}");
		let app = Router::new()
			.route(&path, post(axum_vite_plugin_rpc_handler))
			.with_state(state);
		axum::serve(listener, app)
			.with_graceful_shutdown(async {
				let _ = shutdown_rx.await;
			})
			.await
	})
}

async fn axum_vite_plugin_rpc_handler(
	State(state): State<VitePluginRpcState>,
	headers: HeaderMap,
	body: Bytes,
) -> Response {
	let response = state.handle_rpc(&headers, &body);
	let mut axum_response = response.body().to_vec().into_response();
	*axum_response.status_mut() = response.status();
	axum_response.headers_mut().insert(
		CONTENT_TYPE,
		HeaderValue::from_static(response.content_type()),
	);
	axum_response
}

/// Pure Vite plugin RPC response: a status, content type, and body, decoupled
/// from any actual HTTP framework type — [`VitePluginRpcState::handle_rpc`]'s
/// return type, adapted into a real axum `Response` only at the one call
/// site that actually serves HTTP (the private `axum_vite_plugin_rpc_handler`
/// in this module).
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VitePluginRpcResponse {
	status: StatusCode,
	content_type: &'static str,
	body: Vec<u8>,
}

impl VitePluginRpcResponse {
	/// Response status.
	pub fn status(&self) -> StatusCode {
		self.status
	}

	/// Response content type.
	pub fn content_type(&self) -> &'static str {
		self.content_type
	}

	/// Response body.
	pub fn body(&self) -> &[u8] {
		&self.body
	}

	fn json(status: StatusCode, body: Vec<u8>) -> Self {
		Self {
			status,
			content_type: CONTENT_TYPE_HEADER_VALUE_JSON,
			body,
		}
	}

	fn text(status: StatusCode, body: &str) -> Self {
		Self {
			status,
			content_type: CONTENT_TYPE_HEADER_VALUE_TEXT,
			body: body.as_bytes().to_vec(),
		}
	}
}

#[cfg(test)]
mod tests {
	use std::io::{Read, Write};
	use std::net::TcpStream;

	use super::*;

	const TEST_TOKEN: &str = "token";

	fn headers() -> HeaderMap {
		let mut headers = HeaderMap::new();
		headers.insert(VITE_PLUGIN_TOKEN_HEADER, TEST_TOKEN.parse().unwrap());
		headers
	}

	fn state() -> VitePluginRpcState {
		VitePluginRpcState::new(
			TEST_TOKEN,
			VitePluginConfig {
				public_static_base_path: "/static/".to_owned(),
				entry_module: "src/entry.tsx".to_owned(),
				view_modules: vec!["src/root.tsx".to_owned()],
				ignored_patterns: vec!["**/*.rs".to_owned()],
				dedupe_list: vec!["react".to_owned(), "react-dom".to_owned()],
			},
			BTreeMap::from([(
				"img/logo.svg".to_owned(),
				"/static/logo.hash.svg".to_owned(),
			)]),
		)
	}

	#[test]
	fn vite_plugin_rpc_returns_config_with_contract_field_names() {
		let response = state().handle_rpc(&headers(), br#"{"method":"cfg"}"#);
		let value: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.content_type(), CONTENT_TYPE_HEADER_VALUE_JSON);
		assert_eq!(value["public_static_base_path"], "/static/");
		assert_eq!(value["entry_module"], "src/entry.tsx");
		assert_eq!(value["view_modules"][0], "src/root.tsx");
	}

	#[test]
	fn vite_plugin_rpc_resolves_public_hash_paths() {
		let response = state().handle_rpc(
			&headers(),
			br#"{"method":"hash","src_path":" /img/logo.svg "}"#,
		);

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.content_type(), CONTENT_TYPE_HEADER_VALUE_TEXT);
		assert_eq!(response.body(), b"/static/logo.hash.svg");
	}

	#[test]
	fn vite_plugin_rpc_rejects_missing_unknown_and_oversized_requests() {
		let state = state();

		assert_eq!(
			state
				.handle_rpc(&HeaderMap::new(), br#"{"method":"cfg"}"#)
				.status(),
			StatusCode::FORBIDDEN
		);
		assert_eq!(
			state
				.handle_rpc(&headers(), br#"{"method":"hash","src_path":""}"#)
				.status(),
			StatusCode::BAD_REQUEST
		);
		assert_eq!(
			state
				.handle_rpc(&headers(), br#"{"method":"hash","src_path":"missing.svg"}"#)
				.status(),
			StatusCode::NOT_FOUND
		);
		assert_eq!(
			state
				.handle_rpc(&headers(), &vec![b' '; VITE_PLUGIN_RPC_BODY_LIMIT + 1])
				.status(),
			StatusCode::PAYLOAD_TOO_LARGE
		);
	}

	#[test]
	fn vite_plugin_rpc_records_reachable_control_port() {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let state = state();
		let response = state.handle_rpc(
			&headers(),
			format!(r#"{{"method":"set_port","port":{port}}}"#).as_bytes(),
		);

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.body(), b"OK");
		assert_eq!(state.control_port(), Some(port));
	}

	#[test]
	fn vite_plugin_rpc_does_not_record_unreachable_control_port() {
		let state = state();
		let response = state.handle_rpc(&headers(), br#"{"method":"set_port","port":0}"#);

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		assert_eq!(state.control_port(), None);
	}

	#[test]
	fn vite_plugin_rpc_serves_updated_generation_contract() {
		let state = state();
		state.update_generation_contract(
			VitePluginConfig {
				public_static_base_path: "/assets/".to_owned(),
				entry_module: "src/next-entry.tsx".to_owned(),
				view_modules: vec!["src/next-root.tsx".to_owned()],
				ignored_patterns: Vec::new(),
				dedupe_list: Vec::new(),
			},
			BTreeMap::from([(
				"img/logo.svg".to_owned(),
				"/assets/logo.next.svg".to_owned(),
			)]),
		);

		let config_response = state.handle_rpc(&headers(), br#"{"method":"cfg"}"#);
		let hash_response = state.handle_rpc(
			&headers(),
			br#"{"method":"hash","src_path":"img/logo.svg"}"#,
		);
		let config: serde_json::Value = serde_json::from_slice(config_response.body()).unwrap();

		assert_eq!(config["public_static_base_path"], "/assets/");
		assert_eq!(config["entry_module"], "src/next-entry.tsx");
		assert_eq!(hash_response.body(), b"/assets/logo.next.svg");
	}

	#[test]
	fn loopback_vite_plugin_rpc_server_serves_contract_endpoint() {
		let mut server = LoopbackVitePluginRpcServer;
		let handle = server.start_vite_plugin_rpc_server(state()).unwrap();
		let body = r#"{"method":"cfg"}"#;
		let request = format!(
			"POST {VITE_PLUGIN_BASE_PATH}{VITE_PLUGIN_RPC_PATH} HTTP/1.1\r\nHost: {VITE_PLUGIN_LOOPBACK_HOST}\r\n{VITE_PLUGIN_TOKEN_HEADER}: {TEST_TOKEN}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
			body.len()
		);
		let mut stream = TcpStream::connect((VITE_PLUGIN_LOOPBACK_HOST, handle.port())).unwrap();

		stream.write_all(request.as_bytes()).unwrap();
		let mut response = String::new();
		stream.read_to_string(&mut response).unwrap();

		assert!(response.contains("200 OK"), "{response}");
		assert!(response.contains("\"public_static_base_path\":\"/static/\""));
	}
}
