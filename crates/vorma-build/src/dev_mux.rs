//! Stable loopback development mux for browser traffic and refresh events.
//!
//! The dev mux is the one port a browser talks to for an entire dev
//! session: it proxies ordinary requests to whichever app-server port is
//! currently active (see [`DevMuxServer::set_active_app_server_port`]) and
//! serves the dev-refresh WebSocket endpoint directly. Its own port never
//! changes even though the app server restarts on every server-affecting
//! rebuild, so a developer's browser tab, its open WebSocket connection,
//! and any bookmarked URL all stay valid across rebuilds — only the proxy
//! target underneath moves.

use std::net::{SocketAddr, TcpListener};
use std::sync::Arc;
use std::sync::atomic::{AtomicU16, Ordering};
use std::thread::{self, JoinHandle};

use axum::body::Body;
use axum::extract::State;
use axum::extract::ws::WebSocketUpgrade;
use axum::http::{HeaderMap, HeaderName, HeaderValue, Request, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Router, routing::any};
use hyper::body::Incoming;
use hyper::client::conn::http1;
use hyper::upgrade;
use hyper_util::rt::TokioIo;
use tokio::net::TcpStream;
use tokio::sync::oneshot;

#[cfg(test)]
use crate::dev_refresh::DevRefreshClientSubscription;
use crate::dev_refresh::{
	DevRefreshClients, DevRefreshError, DevRefreshOriginPolicy, RefreshPayload,
	dev_refresh_endpoint, dev_refresh_socket,
};
use crate::vite_plugin_contract::VITE_PLUGIN_LOOPBACK_HOST;

const HEADER_CONNECTION: &str = "connection";
const HEADER_KEEP_ALIVE: &str = "keep-alive";
const HEADER_PROXY_AUTHENTICATE: &str = "proxy-authenticate";
const HEADER_PROXY_AUTHORIZATION: &str = "proxy-authorization";
const HEADER_PROXY_CONNECTION: &str = "proxy-connection";
const HEADER_TE: &str = "te";
const HEADER_TRAILER: &str = "trailer";
const HEADER_TRANSFER_ENCODING: &str = "transfer-encoding";
const HEADER_UPGRADE: &str = "upgrade";

/// Started stable dev mux server, returned by
/// [`start_loopback_dev_mux_server`]. Owns a background thread serving the
/// proxy and refresh WebSocket; dropping it sends a graceful-shutdown
/// signal and joins that thread (see the `Drop` impl below).
#[derive(Debug)]
pub struct DevMuxServer {
	/*
	Retained for test observation only: production callers already know the
	port they asked the mux to bind.
	*/
	_port: u16,
	clients: DevRefreshClients,
	backend: DevMuxBackend,
	shutdown: Option<oneshot::Sender<()>>,
	thread: Option<JoinHandle<()>>,
}

#[derive(Clone, Debug)]
struct DevMuxState {
	clients: DevRefreshClients,
	origin_policy: DevRefreshOriginPolicy,
	backend: DevMuxBackend,
}

#[derive(Clone, Debug)]
struct DevMuxBackend {
	active_app_server_port: Arc<AtomicU16>,
}

struct ProxiedBackendResponse {
	response: Response,
	backend_upgrade: Option<upgrade::OnUpgrade>,
}

impl DevMuxServer {
	/// Bound browser-facing dev mux port.
	#[cfg(test)]
	pub fn port(&self) -> u16 {
		self._port
	}

	/// Current active app-server backend port.
	#[cfg(test)]
	pub fn active_app_server_port(&self) -> u16 {
		self.backend.active_app_server_port()
	}

	/// Switch browser traffic to a ready app-server backend. Called once the
	/// dev loop has a newly rebuilt app server that has passed its readiness
	/// check — every subsequent proxied request goes to `port` instead of
	/// whichever port was active before, with no interruption to the mux's
	/// own browser-facing port or any open refresh WebSocket connection.
	pub fn set_active_app_server_port(&self, port: u16) -> Result<(), DevMuxError> {
		self.backend.set_active_app_server_port(port)
	}

	/// Broadcast one payload to every connected dev-refresh WebSocket client.
	pub fn broadcast(&self, payload: RefreshPayload) {
		self.clients.broadcast(payload);
	}

	/// Broadcast multiple payloads to connected clients.
	pub fn broadcast_all(&self, payloads: impl IntoIterator<Item = RefreshPayload>) {
		for payload in payloads {
			self.broadcast(payload);
		}
	}

	/// Add an in-process test subscription.
	#[cfg(test)]
	pub fn add_test_client(&self) -> DevRefreshClientSubscription {
		self.clients.add_client()
	}
}

impl Drop for DevMuxServer {
	fn drop(&mut self) {
		if let Some(shutdown) = self.shutdown.take() {
			let _ = shutdown.send(());
		}
		if let Some(thread) = self.thread.take() {
			let _ = thread.join();
		}
	}
}

impl DevMuxBackend {
	fn new(active_app_server_port: u16) -> Result<Self, DevMuxError> {
		if active_app_server_port == 0 {
			return Err(DevMuxError::InvalidAppServerPort);
		}
		Ok(Self {
			active_app_server_port: Arc::new(AtomicU16::new(active_app_server_port)),
		})
	}

	fn active_app_server_port(&self) -> u16 {
		self.active_app_server_port.load(Ordering::SeqCst)
	}

	fn set_active_app_server_port(&self, port: u16) -> Result<(), DevMuxError> {
		if port == 0 {
			return Err(DevMuxError::InvalidAppServerPort);
		}
		self.active_app_server_port.store(port, Ordering::SeqCst);
		Ok(())
	}
}

/// Start a loopback dev mux server bound to `port`, proxying to
/// `active_app_server_port` until [`DevMuxServer::set_active_app_server_port`]
/// changes it, and authenticating dev-refresh WebSocket connections against
/// `refresh_token`.
///
/// `port` must be nonzero: it also becomes the allowed browser `Origin` for
/// dev-refresh WebSocket connections (see
/// [`crate::dev_refresh::DevRefreshOriginPolicy`]), and an OS-assigned
/// ephemeral port would be meaningless there — this dev session's browser
/// tab needs one stable, known port for its entire lifetime (see the module
/// docs above), not one discovered after the fact.
pub fn start_loopback_dev_mux_server(
	port: u16,
	active_app_server_port: u16,
	refresh_token: impl Into<String>,
) -> Result<DevMuxServer, DevMuxError> {
	if port == 0 {
		return Err(DevMuxError::InvalidPort);
	}
	let origin_policy =
		DevRefreshOriginPolicy::new(port).map_err(|source| DevMuxError::DevRefresh { source })?;
	let refresh_token = refresh_token.into();
	if refresh_token.is_empty() {
		return Err(DevMuxError::EmptyRefreshToken);
	}
	let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, port)).map_err(|source| {
		DevMuxError::Bind {
			message: source.to_string(),
		}
	})?;
	listener
		.set_nonblocking(true)
		.map_err(|source| DevMuxError::Bind {
			message: source.to_string(),
		})?;
	let port = listener
		.local_addr()
		.map_err(|source| DevMuxError::Bind {
			message: source.to_string(),
		})?
		.port();
	let clients = DevRefreshClients::new();
	let backend = DevMuxBackend::new(active_app_server_port)?;
	let (shutdown_tx, shutdown_rx) = oneshot::channel::<()>();
	let state = DevMuxState {
		clients: clients.clone(),
		origin_policy,
		backend: backend.clone(),
	};
	let endpoint = dev_refresh_endpoint(&refresh_token);
	let thread = thread::spawn(move || {
		let _ = serve_loopback_dev_mux(listener, state, endpoint, shutdown_rx);
	});
	Ok(DevMuxServer {
		_port: port,
		clients,
		backend,
		shutdown: Some(shutdown_tx),
		thread: Some(thread),
	})
}

/// Error from [`start_loopback_dev_mux_server`] or
/// [`DevMuxServer::set_active_app_server_port`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevMuxError {
	/// Dev mux port cannot be zero.
	InvalidPort,
	/// App server port cannot be zero.
	InvalidAppServerPort,
	/// Dev refresh token cannot be empty.
	EmptyRefreshToken,
	/// Dev refresh origin policy failed.
	DevRefresh {
		/// Source refresh error.
		source: DevRefreshError,
	},
	/// Dev mux server bind failed.
	Bind {
		/// Network error message.
		message: String,
	},
}

impl std::fmt::Display for DevMuxError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidPort => f.write_str("dev mux port cannot be zero"),
			Self::InvalidAppServerPort => f.write_str("app server port cannot be zero"),
			Self::EmptyRefreshToken => f.write_str("dev refresh token cannot be empty"),
			Self::DevRefresh { source } => write!(f, "{source}"),
			Self::Bind { message } => write!(f, "bind dev mux server: {message}"),
		}
	}
}

impl std::error::Error for DevMuxError {}

fn serve_loopback_dev_mux(
	listener: TcpListener,
	state: DevMuxState,
	dev_refresh_path: String,
	shutdown_rx: oneshot::Receiver<()>,
) -> Result<(), std::io::Error> {
	let runtime = tokio::runtime::Builder::new_current_thread()
		.enable_io()
		.build()?;
	runtime.block_on(async move {
		let listener = tokio::net::TcpListener::from_std(listener)?;
		let app = Router::new()
			.route(&dev_refresh_path, get(dev_refresh_handler))
			.fallback(any(dev_mux_proxy_handler))
			.with_state(state);
		axum::serve(listener, app)
			.with_graceful_shutdown(async {
				let _ = shutdown_rx.await;
			})
			.await
	})
}

async fn dev_refresh_handler(
	headers: HeaderMap,
	ws: WebSocketUpgrade,
	State(state): State<DevMuxState>,
) -> Response {
	let allowed = headers
		.get(header::ORIGIN)
		.and_then(|value| value.to_str().ok())
		.is_some_and(|origin| state.origin_policy.allows_origin(origin));
	if !allowed {
		return StatusCode::FORBIDDEN.into_response();
	}
	ws.on_upgrade(move |socket| dev_refresh_socket(socket, state.clients))
}

async fn dev_mux_proxy_handler(
	State(state): State<DevMuxState>,
	mut request: Request<Body>,
) -> Response {
	let backend_port = state.backend.active_app_server_port();
	if backend_port == 0 {
		return proxy_error_response(
			StatusCode::BAD_GATEWAY,
			"dev mux app server port cannot be zero",
		);
	}
	let is_upgrade = is_upgrade_request(request.headers());
	let client_upgrade = is_upgrade.then(|| upgrade::on(&mut request));
	prepare_proxy_request(&mut request, backend_port, is_upgrade);
	let proxied = match proxy_request_to_active_app_server(backend_port, request, is_upgrade).await
	{
		Ok(proxied) => proxied,
		Err(message) => return proxy_error_response(StatusCode::BAD_GATEWAY, message),
	};
	if is_upgrade
		&& proxied.response.status() == StatusCode::SWITCHING_PROTOCOLS
		&& let (Some(client_upgrade), Some(backend_upgrade)) =
			(client_upgrade, proxied.backend_upgrade)
	{
		tokio::spawn(async move {
			let Ok(client) = client_upgrade.await else {
				return;
			};
			let Ok(backend) = backend_upgrade.await else {
				return;
			};
			let mut client = TokioIo::new(client);
			let mut backend = TokioIo::new(backend);
			let _ = tokio::io::copy_bidirectional(&mut client, &mut backend).await;
		});
	}
	proxied.response
}

async fn proxy_request_to_active_app_server(
	backend_port: u16,
	request: Request<Body>,
	preserve_upgrade_headers: bool,
) -> Result<ProxiedBackendResponse, String> {
	let address: SocketAddr = format!("{VITE_PLUGIN_LOOPBACK_HOST}:{backend_port}")
		.parse()
		.map_err(|source| format!("parse app server address: {source}"))?;
	let stream = TcpStream::connect(address)
		.await
		.map_err(|source| format!("connect app server: {source}"))?;
	let (mut sender, connection) = http1::handshake(TokioIo::new(stream))
		.await
		.map_err(|source| format!("connect app server HTTP: {source}"))?;
	tokio::spawn(async move {
		let _ = connection.with_upgrades().await;
	});
	let mut response = sender
		.send_request(request)
		.await
		.map_err(|source| format!("proxy app server request: {source}"))?;
	let backend_upgrade = preserve_upgrade_headers.then(|| upgrade::on(&mut response));
	Ok(ProxiedBackendResponse {
		response: proxy_response(response, preserve_upgrade_headers),
		backend_upgrade,
	})
}

fn prepare_proxy_request(
	request: &mut Request<Body>,
	backend_port: u16,
	preserve_upgrade_headers: bool,
) {
	let headers = request.headers_mut();
	let header_names = headers.keys().cloned().collect::<Vec<_>>();
	for name in header_names {
		if should_drop_proxy_request_header(&name, preserve_upgrade_headers) {
			headers.remove(name);
		}
	}
	// Preserve the browser-facing Host so app code sees the same origin it
	// would in production; fall back to the backend address when the inbound
	// request carried no Host header (e.g. HTTP/2 authority form).
	if !headers.contains_key(header::HOST) {
		let host = HeaderValue::from_str(&format!("{VITE_PLUGIN_LOOPBACK_HOST}:{backend_port}"))
			.expect("loopback host header is valid");
		headers.insert(header::HOST, host);
	}
}

fn proxy_response(
	backend_response: hyper::Response<Incoming>,
	preserve_upgrade_headers: bool,
) -> Response {
	let (parts, body) = backend_response.into_parts();
	let mut response = Response::new(Body::new(body));
	*response.status_mut() = parts.status;
	*response.version_mut() = parts.version;
	for (name, value) in parts.headers {
		let Some(name) = name else {
			continue;
		};
		if should_drop_proxy_response_header(&name, preserve_upgrade_headers) {
			continue;
		}
		response.headers_mut().append(name, value);
	}
	response
}

fn proxy_error_response(status: StatusCode, message: impl Into<String>) -> Response {
	let mut response = Response::new(Body::from(message.into()));
	*response.status_mut() = status;
	response
}

fn is_upgrade_request(headers: &HeaderMap) -> bool {
	headers
		.get(header::UPGRADE)
		.is_some_and(|value| !value.as_bytes().is_empty())
		&& headers
			.get(header::CONNECTION)
			.is_some_and(|value| ascii_header_value_contains_token(value.as_bytes(), "upgrade"))
}

fn should_drop_proxy_request_header(name: &HeaderName, preserve_upgrade_headers: bool) -> bool {
	if preserve_upgrade_headers
		&& (name.as_str().eq_ignore_ascii_case(HEADER_CONNECTION)
			|| name.as_str().eq_ignore_ascii_case(HEADER_UPGRADE))
	{
		return false;
	}
	is_proxy_connection_header(name.as_str())
}

fn should_drop_proxy_response_header(name: &HeaderName, preserve_upgrade_headers: bool) -> bool {
	if preserve_upgrade_headers
		&& (name.as_str().eq_ignore_ascii_case(HEADER_CONNECTION)
			|| name.as_str().eq_ignore_ascii_case(HEADER_UPGRADE))
	{
		return false;
	}
	is_proxy_connection_header(name.as_str())
}

fn is_proxy_connection_header(name: &str) -> bool {
	matches!(
		name.to_ascii_lowercase().as_str(),
		HEADER_CONNECTION
			| HEADER_KEEP_ALIVE
			| HEADER_PROXY_AUTHENTICATE
			| HEADER_PROXY_AUTHORIZATION
			| HEADER_PROXY_CONNECTION
			| HEADER_TE
			| HEADER_TRAILER
			| HEADER_TRANSFER_ENCODING
			| HEADER_UPGRADE
	)
}

fn ascii_header_value_contains_token(value: &[u8], token: &str) -> bool {
	let Ok(value) = std::str::from_utf8(value) else {
		return false;
	};
	value
		.split(',')
		.any(|part| part.trim().eq_ignore_ascii_case(token))
}

#[cfg(test)]
mod tests {
	use std::io::{Read, Write};
	use std::net::TcpStream as StdTcpStream;
	use std::sync::mpsc;
	use std::thread;
	use std::time::{Duration, Instant};

	use super::*;

	const TEST_REFRESH_TOKEN: &str = "refresh-token";
	const TEST_PROXY_PATH: &str = "/hello?name=vorma";
	/*
	Bounded retry count for `start_dev_mux_retrying_port_collisions` below.
	Purpose-built 64-thread ephemeral-port contention measured a 0.03%
	collision rate (38/128,000 attempts); ordinary `cargo test` parallel load
	is far gentler than that, so this bound is generous headroom, not a tight
	fit — a genuinely broken caller (empty token, zero backend port) fails on
	its first attempt via a different `DevMuxError` variant and is never
	retried at all (see the function below), so this bound only governs how
	many times a real transient port collision gets a fresh port to try.
	*/
	const MAX_DEV_MUX_PORT_COLLISION_RETRIES: u32 = 8;

	fn allocate_test_port() -> u16 {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		listener.local_addr().unwrap().port()
	}

	/// Start a loopback dev mux server for a test, retrying on the
	/// discover-then-drop-then-rebind TOCTOU race between
	/// [`allocate_test_port`] and this function's own
	/// [`start_loopback_dev_mux_server`] call: `allocate_test_port` binds
	/// `:0`, reads back the OS-assigned port, and drops the listener so
	/// `start_loopback_dev_mux_server` can bind that same port number for
	/// real a moment later — any other process racing to claim the
	/// newly-freed ephemeral port in that gap steals it out from under this
	/// test. Retrying with a freshly allocated candidate port closes the
	/// race by construction: a stale-port retry would not help (the most
	/// likely contender under `cargo test` parallelism is another
	/// concurrently-running dev-mux test holding its own allocated port for
	/// its entire lifetime, so retrying the SAME port would just collide
	/// again), but a fresh port sidesteps whichever port lost the race.
	/// Only a bind collision (`DevMuxError::Bind`) is retried; any other
	/// error — a real, deterministic bug — panics on its very first
	/// occurrence, exactly as the un-retried call did before.
	fn start_dev_mux_retrying_port_collisions(
		active_app_server_port: u16,
		refresh_token: &str,
	) -> DevMuxServer {
		for attempt in 0..=MAX_DEV_MUX_PORT_COLLISION_RETRIES {
			match start_loopback_dev_mux_server(
				allocate_test_port(),
				active_app_server_port,
				refresh_token,
			) {
				Ok(mux) => return mux,
				Err(DevMuxError::Bind { .. }) if attempt < MAX_DEV_MUX_PORT_COLLISION_RETRIES => {}
				Err(source) => panic!(
					"start_loopback_dev_mux_server failed on attempt {} of {}: {source}",
					attempt + 1,
					MAX_DEV_MUX_PORT_COLLISION_RETRIES + 1
				),
			}
		}
		unreachable!("loop above always returns or panics")
	}

	fn spawn_backend_response(body: &'static str) -> (u16, thread::JoinHandle<()>) {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let thread = thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut request = [0_u8; 1024];
			let read = stream.read(&mut request).unwrap();
			let request = std::str::from_utf8(&request[..read]).unwrap();
			assert!(request.starts_with("GET /hello?name=vorma HTTP/1.1\r\n"));
			let response = format!(
				"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: {}\r\n\r\n{}",
				body.len(),
				body
			);
			stream.write_all(response.as_bytes()).unwrap();
		});
		(port, thread)
	}

	fn spawn_streaming_backend_response() -> (u16, mpsc::Receiver<()>, mpsc::Sender<()>) {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let (first_chunk_tx, first_chunk_rx) = mpsc::channel();
		let (release_tx, release_rx) = mpsc::channel();
		thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut request = [0_u8; 1024];
			let _ = stream.read(&mut request).unwrap();
			stream
				.write_all(
					b"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nfirst\r\n",
				)
				.unwrap();
			first_chunk_tx.send(()).unwrap();
			release_rx.recv().unwrap();
			stream.write_all(b"6\r\nsecond\r\n0\r\n\r\n").unwrap();
		});
		(port, first_chunk_rx, release_tx)
	}

	fn spawn_upgrade_backend_response() -> (u16, thread::JoinHandle<()>) {
		let listener = TcpListener::bind((VITE_PLUGIN_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let thread = thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut request = [0_u8; 1024];
			let read = stream.read(&mut request).unwrap();
			let request = std::str::from_utf8(&request[..read]).unwrap();
			let lowercase_request = request.to_ascii_lowercase();
			assert!(lowercase_request.contains("connection: upgrade\r\n"));
			assert!(lowercase_request.contains("upgrade: test-protocol\r\n"));
			stream
				.write_all(
					b"HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: test-protocol\r\n\r\n",
				)
				.unwrap();
			let mut message = [0_u8; 4];
			stream.read_exact(&mut message).unwrap();
			assert_eq!(&message, b"ping");
			stream.write_all(b"pong").unwrap();
		});
		(port, thread)
	}

	fn get_through_mux(port: u16, path: &str) -> String {
		let deadline = Instant::now() + Duration::from_secs(1);
		loop {
			match StdTcpStream::connect((VITE_PLUGIN_LOOPBACK_HOST, port)) {
				Ok(mut stream) => {
					let request = format!(
						"GET {path} HTTP/1.1\r\nHost: {VITE_PLUGIN_LOOPBACK_HOST}:{port}\r\nConnection: close\r\n\r\n"
					);
					stream.write_all(request.as_bytes()).unwrap();
					let mut response = String::new();
					stream.read_to_string(&mut response).unwrap();
					return response.split("\r\n\r\n").nth(1).unwrap_or("").to_owned();
				}
				Err(error) if Instant::now() < deadline => {
					let _ = error;
					thread::sleep(Duration::from_millis(10));
				}
				Err(error) => panic!("connect dev mux: {error}"),
			}
		}
	}

	#[test]
	fn dev_mux_proxy_filters_headers_without_stripping_content_length() {
		assert!(!should_drop_proxy_request_header(&header::HOST, false));
		assert!(should_drop_proxy_request_header(&header::CONNECTION, false));
		assert!(should_drop_proxy_request_header(&header::UPGRADE, false));
		assert!(!should_drop_proxy_request_header(
			&HeaderName::from_static("content-length"),
			false
		));
		assert!(!should_drop_proxy_request_header(
			&HeaderName::from_static("x-test"),
			false
		));
		assert!(!should_drop_proxy_request_header(&header::CONNECTION, true));
		assert!(!should_drop_proxy_request_header(&header::UPGRADE, true));
	}

	#[test]
	fn dev_mux_backend_switches_active_app_server_port() {
		let backend = DevMuxBackend::new(3000).unwrap();

		backend.set_active_app_server_port(3001).unwrap();

		assert_eq!(backend.active_app_server_port(), 3001);
	}

	#[test]
	fn start_loopback_dev_mux_server_rejects_zero_port() {
		let error = start_loopback_dev_mux_server(0, 3000, TEST_REFRESH_TOKEN).unwrap_err();

		assert_eq!(error, DevMuxError::InvalidPort);
	}

	#[test]
	fn loopback_dev_mux_proxies_to_active_backend_and_switches() {
		let (first_backend_port, first_backend_thread) = spawn_backend_response("first");
		let mux = start_dev_mux_retrying_port_collisions(first_backend_port, TEST_REFRESH_TOKEN);

		let first_body = get_through_mux(mux.port(), TEST_PROXY_PATH);
		first_backend_thread.join().unwrap();
		let (second_backend_port, second_backend_thread) = spawn_backend_response("second");
		mux.set_active_app_server_port(second_backend_port).unwrap();
		let second_body = get_through_mux(mux.port(), TEST_PROXY_PATH);
		second_backend_thread.join().unwrap();

		assert_eq!(mux.active_app_server_port(), second_backend_port);
		assert_eq!(first_body, "first");
		assert_eq!(second_body, "second");
	}

	#[test]
	fn loopback_dev_mux_streams_chunked_backend_body_without_buffering_until_end() {
		let (backend_port, first_chunk_rx, release_tx) = spawn_streaming_backend_response();
		let mux = start_dev_mux_retrying_port_collisions(backend_port, TEST_REFRESH_TOKEN);
		let mut stream = StdTcpStream::connect((VITE_PLUGIN_LOOPBACK_HOST, mux.port())).unwrap();
		stream
			.set_read_timeout(Some(Duration::from_secs(1)))
			.unwrap();
		let request = format!(
			"GET {TEST_PROXY_PATH} HTTP/1.1\r\nHost: {VITE_PLUGIN_LOOPBACK_HOST}:{}\r\nConnection: close\r\n\r\n",
			mux.port()
		);
		stream.write_all(request.as_bytes()).unwrap();
		first_chunk_rx.recv_timeout(Duration::from_secs(1)).unwrap();
		let mut response = [0_u8; 512];
		let read = stream.read(&mut response).unwrap();
		let response = std::str::from_utf8(&response[..read]).unwrap();

		assert!(response.contains("first"));
		assert!(!response.contains("second"));
		release_tx.send(()).unwrap();
	}

	#[test]
	fn loopback_dev_mux_proxies_upgrade_connections_bidirectionally() {
		let (backend_port, backend_thread) = spawn_upgrade_backend_response();
		let mux = start_dev_mux_retrying_port_collisions(backend_port, TEST_REFRESH_TOKEN);
		let mut stream = StdTcpStream::connect((VITE_PLUGIN_LOOPBACK_HOST, mux.port())).unwrap();
		stream
			.set_read_timeout(Some(Duration::from_secs(1)))
			.unwrap();
		let request = format!(
			"GET /socket HTTP/1.1\r\nHost: {VITE_PLUGIN_LOOPBACK_HOST}:{}\r\nConnection: Upgrade\r\nUpgrade: test-protocol\r\n\r\n",
			mux.port()
		);
		stream.write_all(request.as_bytes()).unwrap();
		let mut response = [0_u8; 512];
		let read = stream.read(&mut response).unwrap();
		let response = std::str::from_utf8(&response[..read]).unwrap();
		assert!(response.starts_with("HTTP/1.1 101 Switching Protocols\r\n"));

		stream.write_all(b"ping").unwrap();
		let mut message = [0_u8; 4];
		stream.read_exact(&mut message).unwrap();

		assert_eq!(&message, b"pong");
		backend_thread.join().unwrap();
	}
}
