use std::net::TcpListener;
use std::sync::{Arc, Mutex};
use std::thread;

use axum::Router;
use axum::routing::{get, post};

use crate::browser_sync::{ChangeType, ClientManager, ClientSubscription, RefreshPayload};
use crate::constants::DEV_LOOPBACK_HOST;
use crate::generation::DevMuxGeneration;
use crate::supervisor::get_random_free_port;
use crate::utils::random_id;

#[derive(Debug)]
pub(crate) struct DevMuxServer {
	shutdown_tx: Option<tokio::sync::oneshot::Sender<()>>,
	thread: Option<thread::JoinHandle<Result<(), String>>>,
}

#[derive(Clone, Debug)]
pub(crate) struct DevMuxState {
	snapshot: Arc<Mutex<DevMuxSnapshot>>,
	prepared: Arc<Mutex<Option<PreparedDevMux>>>,
	client_manager: ClientManager,
}

#[derive(Debug)]
pub(crate) struct DevMuxRuntime {
	state: DevMuxState,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct PreparedDevMux {
	pub(crate) port: u16,
	pub(crate) dev_refresh_token: String,
	pub(crate) vite_plugin_token: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum VitePluginTokenError {
	MissingInternalToken,
	InvalidToken,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct DevMuxSnapshot {
	pub(crate) generation: Option<DevMuxGeneration>,
	pub(crate) vite_plugin_control_port: Option<u16>,
}

impl DevMuxServer {
	pub(crate) fn stop(&mut self) -> Result<(), String> {
		if let Some(shutdown_tx) = self.shutdown_tx.take() {
			let _ = shutdown_tx.send(());
		}
		let Some(thread) = self.thread.take() else {
			return Ok(());
		};
		thread
			.join()
			.map_err(|_| "internal dev server thread panicked".to_owned())?
	}
}

impl Drop for DevMuxServer {
	fn drop(&mut self) {
		let _ = self.stop();
	}
}

impl DevMuxState {
	pub(crate) fn new() -> Self {
		Self {
			snapshot: Arc::new(Mutex::new(DevMuxSnapshot::default())),
			prepared: Arc::new(Mutex::new(None)),
			client_manager: ClientManager::new(),
		}
	}

	#[cfg(test)]
	pub(crate) fn snapshot(&self) -> DevMuxSnapshot {
		self.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned")
			.clone()
	}

	fn prepared(&self) -> Option<PreparedDevMux> {
		self.prepared
			.lock()
			.expect("dev mux prepared lock poisoned")
			.clone()
	}

	fn set_prepared(&self, prepared: PreparedDevMux) {
		*self
			.prepared
			.lock()
			.expect("dev mux prepared lock poisoned") = Some(prepared);
	}

	pub(crate) fn check_vite_plugin_token(
		&self,
		provided: Option<&str>,
	) -> Result<(), VitePluginTokenError> {
		let Some(prepared) = self.prepared() else {
			return Err(VitePluginTokenError::MissingInternalToken);
		};
		if provided == Some(prepared.vite_plugin_token.as_str()) {
			return Ok(());
		}
		Err(VitePluginTokenError::InvalidToken)
	}

	pub(crate) fn generation(&self) -> Result<DevMuxGeneration, String> {
		self.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned")
			.generation
			.clone()
			.ok_or_else(|| "dev generation not available".to_owned())
	}

	pub(crate) fn resolve_public_url(&self, src_path: &str) -> Result<Option<String>, String> {
		let src_path = src_path.trim();
		let clean = src_path.strip_prefix('/').unwrap_or(src_path);
		let snapshot = self
			.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned");
		let generation = snapshot
			.generation
			.as_ref()
			.ok_or_else(|| "dev generation not available".to_owned())?;
		Ok(generation.public_filemap.get(clean).cloned())
	}

	pub(crate) fn clear_generation(&self) {
		self.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned")
			.generation = None;
	}

	pub(crate) fn vite_plugin_control_port(&self) -> Option<u16> {
		self.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned")
			.vite_plugin_control_port
	}

	pub(crate) fn set_vite_plugin_control_port(&self, port: u16) {
		self.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned")
			.vite_plugin_control_port = Some(port);
	}

	pub(crate) fn broadcast(&self, msg: RefreshPayload) {
		self.client_manager.broadcast(msg);
	}

	pub(crate) fn add_client(&self) -> ClientSubscription {
		self.client_manager.add()
	}
}

impl Default for DevMuxRuntime {
	fn default() -> Self {
		Self {
			state: DevMuxState::new(),
		}
	}
}

impl DevMuxRuntime {
	pub(crate) fn state(&self) -> DevMuxState {
		self.state.clone()
	}

	pub(crate) fn prepare(&mut self) -> Result<u16, String> {
		if let Some(prepared) = self.state.prepared() {
			return Ok(prepared.port);
		}

		let port = get_random_free_port()?;
		self.state.set_prepared(PreparedDevMux {
			port,
			dev_refresh_token: random_id(16)?,
			vite_plugin_token: random_id(32)?,
		});
		Ok(port)
	}

	pub(crate) fn prepared(&self) -> Result<PreparedDevMux, String> {
		self.state
			.prepared()
			.ok_or_else(|| "dev mux is not prepared".to_owned())
	}

	pub(crate) fn port_i32(&self) -> Result<i32, String> {
		Ok(i32::from(self.prepared()?.port))
	}

	pub(crate) fn port_u16(&self) -> Result<u16, String> {
		Ok(self.prepared()?.port)
	}

	pub(crate) fn dev_refresh_token(&self) -> Result<String, String> {
		Ok(self.prepared()?.dev_refresh_token)
	}

	#[cfg(test)]
	pub(crate) fn vite_plugin_token(&self) -> Result<String, String> {
		Ok(self.prepared()?.vite_plugin_token)
	}

	pub(crate) fn require_vite_plugin_token(&self) -> Result<String, String> {
		Ok(self.prepared()?.vite_plugin_token)
	}

	pub(crate) fn dev_refresh_endpoint(&self) -> Result<String, String> {
		Ok(crate::browser_sync::dev_refresh_endpoint(
			&self.dev_refresh_token()?,
		))
	}

	pub(crate) fn publish_generation(&self, generation: DevMuxGeneration) {
		self.state
			.snapshot
			.lock()
			.expect("dev mux snapshot lock poisoned")
			.generation = Some(generation);
	}

	pub(crate) fn clear_generation(&self) {
		self.state.clear_generation();
	}

	pub(crate) fn vite_plugin_control_port(&self) -> Option<u16> {
		self.state.vite_plugin_control_port()
	}

	pub(crate) fn broadcast_refresh(
		&self,
		change_type: ChangeType,
		critical_css: &str,
		build_error: &str,
	) {
		self.state.broadcast(RefreshPayload {
			change_type,
			critical_css: critical_css.to_owned(),
			build_error: build_error.to_owned(),
		});
	}

	#[cfg(test)]
	pub(crate) fn for_test(port: i32, dev_refresh_token: &str, vite_plugin_token: &str) -> Self {
		let state = DevMuxState::new();
		if let Some(port) = u16::try_from(port).ok().filter(|port| *port != 0) {
			state.set_prepared(PreparedDevMux {
				port,
				dev_refresh_token: dev_refresh_token.to_owned(),
				vite_plugin_token: vite_plugin_token.to_owned(),
			});
		}
		Self { state }
	}
}

pub(crate) fn start_dev_mux_server(
	dev_mux_state: DevMuxState,
	dev_refresh_endpoint: String,
	port: u16,
) -> Result<DevMuxServer, String> {
	let std_listener =
		TcpListener::bind((DEV_LOOPBACK_HOST, port)).map_err(|err| err.to_string())?;
	std_listener
		.set_nonblocking(true)
		.map_err(|err| err.to_string())?;
	let (shutdown_tx, shutdown_rx) = tokio::sync::oneshot::channel();
	let app = dev_router(dev_mux_state, &dev_refresh_endpoint);
	let thread = thread::spawn(move || {
		let runtime = tokio::runtime::Builder::new_current_thread()
			.enable_all()
			.build()
			.map_err(|err| err.to_string())?;
		runtime.block_on(async move {
			let listener =
				tokio::net::TcpListener::from_std(std_listener).map_err(|err| err.to_string())?;
			axum::serve(listener, app)
				.with_graceful_shutdown(async {
					let _ = shutdown_rx.await;
				})
				.await
				.map_err(|err| err.to_string())
		})
	});

	Ok(DevMuxServer {
		shutdown_tx: Some(shutdown_tx),
		thread: Some(thread),
	})
}

pub(crate) fn dev_router(dev_mux_state: DevMuxState, dev_refresh_endpoint: &str) -> Router {
	Router::new()
		.route(
			"/vite-plugin/rpc",
			post(crate::vite_plugin::vite_plugin_rpc_handler),
		)
		.route(
			dev_refresh_endpoint,
			get(crate::browser_sync::dev_refresh_handler),
		)
		.with_state(dev_mux_state)
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::io::{Read, Write};
	use std::net::TcpStream;
	use std::time::Duration;

	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::constants::VITE_PLUGIN_TOKEN_HEADER;
	use crate::ts_modules::TsViewModule;

	fn config() -> Config {
		Config {
			root_dir: std::env::current_dir().unwrap(),
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				..FrontendConfig::default()
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			..Config::default()
		}
	}

	#[test]
	fn prepare_dev_mux_assigns_refresh_and_plugin_tokens() {
		let mut dev_mux = DevMuxRuntime::default();
		let port = dev_mux.prepare().unwrap();

		assert_eq!(dev_mux.port_i32().unwrap(), i32::from(port));
		assert_eq!(dev_mux.dev_refresh_token().unwrap().len(), 16);
		assert_eq!(dev_mux.vite_plugin_token().unwrap().len(), 32);
		assert!(
			dev_mux
				.dev_refresh_token()
				.unwrap()
				.chars()
				.all(|c| c.is_ascii_alphanumeric())
		);
		assert!(
			dev_mux
				.vite_plugin_token()
				.unwrap()
				.chars()
				.all(|c| c.is_ascii_alphanumeric())
		);
	}

	#[test]
	fn dev_mux_server_serves_tokenized_vite_plugin_rpc_until_stopped() {
		let mut dev_mux = DevMuxRuntime::default();
		let port = dev_mux.prepare().unwrap();
		let token = dev_mux.vite_plugin_token().unwrap();
		dev_mux.publish_generation(DevMuxGeneration {
			config: config(),
			view_modules: BTreeMap::from([(
				"/".to_owned(),
				TsViewModule {
					pattern: "/".to_owned(),
					import_path: "src/root.tsx".to_owned(),
					deps: Vec::new(),
				},
			)]),
			public_filemap: BTreeMap::new(),
		});
		let state = dev_mux.state();
		let endpoint = dev_mux.dev_refresh_endpoint().unwrap();
		let mut server = start_dev_mux_server(state, endpoint, port).unwrap();

		let response = http_post_rpc(port, &token, r#"{"method":"cfg"}"#);
		let second_response = http_post_rpc(port, &token, r#"{"method":"cfg"}"#);

		assert!(response.starts_with("HTTP/1.1 200 "));
		assert!(response.contains("PublicStaticBasePath"));
		assert!(second_response.starts_with("HTTP/1.1 200 "));
		assert!(second_response.contains("PublicStaticBasePath"));
		server.stop().unwrap();
	}

	#[test]
	fn dev_mux_server_owns_published_generation_after_runtime_value_drops() {
		let mut dev_mux = DevMuxRuntime::default();
		let port = dev_mux.prepare().unwrap();
		let token = dev_mux.vite_plugin_token().unwrap();
		dev_mux.publish_generation(DevMuxGeneration {
			config: config(),
			view_modules: BTreeMap::new(),
			public_filemap: BTreeMap::new(),
		});
		let state = dev_mux.state();
		let endpoint = dev_mux.dev_refresh_endpoint().unwrap();
		let mut server = start_dev_mux_server(state, endpoint, port).unwrap();
		drop(dev_mux);

		let response =
			http_post_rpc_with_timeout(port, &token, r#"{"method":"cfg"}"#, Duration::from_secs(1));

		assert!(response.starts_with("HTTP/1.1 200 "));
		server.stop().unwrap();
	}

	fn http_post_rpc(port: u16, token: &str, body: &str) -> String {
		http_post_rpc_with_timeout(port, token, body, Duration::from_secs(5))
	}

	fn http_post_rpc_with_timeout(port: u16, token: &str, body: &str, timeout: Duration) -> String {
		let mut stream = TcpStream::connect((DEV_LOOPBACK_HOST, port)).unwrap();
		stream.set_read_timeout(Some(timeout)).unwrap();
		let req = format!(
			"POST /vite-plugin/rpc HTTP/1.1\r\nHost: {DEV_LOOPBACK_HOST}:{port}\r\n{VITE_PLUGIN_TOKEN_HEADER}: {token}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
			body.len()
		);
		stream.write_all(req.as_bytes()).unwrap();
		let mut response = String::new();
		stream.read_to_string(&mut response).unwrap();
		response
	}
}
