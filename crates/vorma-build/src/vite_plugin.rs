use std::net::TcpStream;
use std::path::Path;

use axum::Json;
use axum::body::to_bytes;
use axum::extract::Request;
use axum::extract::State;
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use path_slash::PathExt;
use serde::{Deserialize, Serialize};

use crate::config::to_cfg;
use crate::constants::{DEV_LOOPBACK_HOST, VITE_PLUGIN_TOKEN_HEADER};
use crate::dev_mux::{DevMuxState, VitePluginTokenError};
use crate::generation::DevMuxGeneration;

const VITE_PLUGIN_RPC_BODY_LIMIT: usize = 64 * 1024;

#[derive(Debug, Eq, PartialEq, Serialize)]
pub(crate) struct ViteConfigResponse {
	#[serde(rename = "PublicStaticBasePath")]
	pub(crate) public_static_base_path: String,
	#[serde(rename = "EntryModule")]
	pub(crate) entry_module: String,
	#[serde(rename = "ViewModules")]
	pub(crate) view_modules: Vec<String>,
	#[serde(rename = "IgnoredPatterns")]
	pub(crate) ignored_patterns: Vec<String>,
	#[serde(rename = "DedupeList")]
	pub(crate) dedupe_list: Vec<String>,
}

#[derive(Debug, Deserialize)]
#[serde(tag = "method", rename_all = "snake_case")]
pub(crate) enum VitePluginRpcRequest {
	Cfg,
	Hash { src_path: String },
	SetPort { port: u16 },
}

pub(crate) async fn vite_plugin_rpc_handler(
	State(dev_mux_state): State<DevMuxState>,
	request: Request,
) -> Response {
	if let Some(response) = vite_plugin_token_error_response(&dev_mux_state, request.headers()) {
		return response;
	}
	let body = match to_bytes(request.into_body(), VITE_PLUGIN_RPC_BODY_LIMIT).await {
		Ok(body) => body,
		Err(_) => return StatusCode::PAYLOAD_TOO_LARGE.into_response(),
	};
	let request = match serde_json::from_slice::<VitePluginRpcRequest>(&body) {
		Ok(request) => request,
		Err(_) => return (StatusCode::BAD_REQUEST, "invalid JSON RPC request").into_response(),
	};

	match request {
		VitePluginRpcRequest::Cfg => {
			let result = dev_mux_state
				.generation()
				.and_then(|generation| vite_config_response_from_generation(&generation));
			match result {
				Ok(response) => Json(response).into_response(),
				Err(_) => {
					(StatusCode::INTERNAL_SERVER_ERROR, "error getting config").into_response()
				}
			}
		}
		VitePluginRpcRequest::Hash { src_path } => {
			if src_path.is_empty() {
				return (StatusCode::BAD_REQUEST, "missing src_path parameter").into_response();
			}
			match dev_mux_state.resolve_public_url(&src_path) {
				Ok(Some(public_url)) => public_url.into_response(),
				Ok(None) => StatusCode::NOT_FOUND.into_response(),
				Err(_) => (
					StatusCode::INTERNAL_SERVER_ERROR,
					"dev generation not available",
				)
					.into_response(),
			}
		}
		VitePluginRpcRequest::SetPort { port } => {
			if TcpStream::connect((DEV_LOOPBACK_HOST, port)).is_err() {
				return (
					StatusCode::INTERNAL_SERVER_ERROR,
					"error connecting to vite restart port",
				)
					.into_response();
			}

			dev_mux_state.set_vite_plugin_control_port(port);
			(StatusCode::OK, "OK").into_response()
		}
	}
}

fn vite_config_response_from_generation(
	generation: &DevMuxGeneration,
) -> Result<ViteConfigResponse, String> {
	let cfg =
		to_cfg(&generation.config).map_err(|err| format!("error converting config: {err}"))?;
	let mut view_modules = Vec::new();
	for r in generation.view_modules.values() {
		view_modules.push(abs_path(cfg.root_path_string(&r.import_path))?);
	}

	Ok(ViteConfigResponse {
		public_static_base_path: cfg.public_static_base_path(),
		entry_module: cfg.ts_entry_abs_slash(),
		view_modules,
		ignored_patterns: cfg.vite_ignored_patterns(),
		dedupe_list: cfg.vite_dedupe_list(),
	})
}

#[cfg(test)]
fn vite_hash_response_from_pub_fm(
	pub_fm: &std::collections::BTreeMap<String, String>,
	src_path: &str,
) -> Option<String> {
	let src_path = src_path.trim();
	let clean = src_path.strip_prefix('/').unwrap_or(src_path);
	pub_fm.get(clean).cloned()
}

fn vite_plugin_token_error_response(
	dev_mux_state: &DevMuxState,
	headers: &HeaderMap,
) -> Option<Response> {
	let provided = headers
		.get(VITE_PLUGIN_TOKEN_HEADER)
		.and_then(|value| value.to_str().ok());
	match dev_mux_state.check_vite_plugin_token(provided) {
		Ok(()) => None,
		Err(VitePluginTokenError::MissingInternalToken) => Some(
			(
				StatusCode::INTERNAL_SERVER_ERROR,
				"Vite plugin control token not available",
			)
				.into_response(),
		),
		Err(VitePluginTokenError::InvalidToken) => Some(StatusCode::FORBIDDEN.into_response()),
	}
}

fn to_slash(path: &str) -> String {
	Path::new(path).to_slash_lossy().into_owned()
}

fn abs_path(path: impl AsRef<Path>) -> Result<String, String> {
	let path = path.as_ref();
	let abs = if path.is_absolute() {
		path.to_path_buf()
	} else {
		std::env::current_dir()
			.map_err(|err| format!("error getting current dir: {err}"))?
			.join(path)
	};
	Ok(to_slash(&abs.to_string_lossy()))
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::net::TcpListener;

	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::dev_mux::DevMuxRuntime;
	use crate::ts_modules::TsViewModule;

	#[test]
	fn vite_config_response_uses_vite_plugin_field_names_and_absolute_modules() {
		let root = std::env::current_dir().unwrap();
		let cfg = Config {
			root_dir: root.clone(),
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
				entry_file: "src/entry.tsx".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				..crate::test_support::ts_gen_config()
			},
			..Config::default()
		};
		let generation = DevMuxGeneration {
			config: cfg,
			view_modules: BTreeMap::from([(
				"/".to_owned(),
				TsViewModule {
					pattern: "/".to_owned(),
					import_path: "src/root.tsx".to_owned(),
					deps: Vec::new(),
				},
			)]),
			public_filemap: BTreeMap::new(),
		};

		let value =
			serde_json::to_value(vite_config_response_from_generation(&generation).unwrap())
				.unwrap();

		assert_eq!(value["PublicStaticBasePath"], "/static/");
		assert_eq!(
			value["EntryModule"],
			root.join("src/entry.tsx").to_string_lossy().as_ref(),
		);
		assert_eq!(
			value["ViewModules"][0],
			root.join("src/root.tsx").to_string_lossy().as_ref(),
		);
		assert_eq!(value["DedupeList"][0], "react");
	}

	#[test]
	fn vite_hash_response_trims_input() {
		let public_filemap =
			BTreeMap::from([("logo.svg".to_owned(), "/static/vorma_out_logo".to_owned())]);

		assert_eq!(
			vite_hash_response_from_pub_fm(&public_filemap, " /logo.svg "),
			Some("/static/vorma_out_logo".to_owned()),
		);
	}

	#[test]
	fn vite_hash_response_only_trims_one_leading_slash() {
		let public_filemap =
			BTreeMap::from([("logo.svg".to_owned(), "/static/vorma_out_logo".to_owned())]);

		assert_eq!(
			vite_hash_response_from_pub_fm(&public_filemap, "//logo.svg"),
			None
		);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_rejects_missing_token() {
		let dev_mux_state = dev_mux_state_with_token("secret");

		let response =
			vite_plugin_rpc_handler(State(dev_mux_state), rpc_request(HeaderMap::new(), "{}"))
				.await;

		assert_eq!(response.status(), StatusCode::FORBIDDEN);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_checks_token_before_parsing_body() {
		let dev_mux_state = dev_mux_state_with_token("secret");

		let response = vite_plugin_rpc_handler(
			State(dev_mux_state),
			rpc_request(HeaderMap::new(), "not JSON"),
		)
		.await;

		assert_eq!(response.status(), StatusCode::FORBIDDEN);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_treats_empty_src_path_as_missing() {
		let dev_mux_state = dev_mux_state_with_token("secret");
		let response = vite_plugin_rpc_handler(
			State(dev_mux_state),
			rpc_request(
				headers_with_token("secret"),
				r#"{"method":"hash","src_path":""}"#,
			),
		)
		.await;

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_rejects_missing_internal_token() {
		let dev_mux_state = DevMuxState::new();

		let response = vite_plugin_rpc_handler(
			State(dev_mux_state),
			rpc_request(headers_with_token("secret"), r#"{"method":"cfg"}"#),
		)
		.await;

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_rejects_hash_before_generation_is_committed() {
		let dev_mux_state = dev_mux_state_with_token("secret");

		let response = vite_plugin_rpc_handler(
			State(dev_mux_state),
			rpc_request(
				headers_with_token("secret"),
				r#"{"method":"hash","src_path":"/logo.svg"}"#,
			),
		)
		.await;

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_rejects_malformed_authorized_json() {
		let dev_mux_state = dev_mux_state_with_token("secret");

		let response = vite_plugin_rpc_handler(
			State(dev_mux_state),
			rpc_request(headers_with_token("secret"), "not JSON"),
		)
		.await;

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_records_reachable_restart_port() {
		let listener = TcpListener::bind((crate::constants::DEV_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let dev_mux_state = dev_mux_state_with_token("secret");

		let response = vite_plugin_rpc_handler(
			State(dev_mux_state.clone()),
			rpc_request(
				headers_with_token("secret"),
				&format!(r#"{{"method":"set_port","port":{port}}}"#),
			),
		)
		.await;

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(
			dev_mux_state.snapshot().vite_plugin_control_port,
			Some(port),
		);
	}

	#[tokio::test]
	async fn vite_plugin_rpc_does_not_record_unreachable_restart_port() {
		let dev_mux_state = dev_mux_state_with_token("secret");

		let response = vite_plugin_rpc_handler(
			State(dev_mux_state.clone()),
			rpc_request(
				headers_with_token("secret"),
				r#"{"method":"set_port","port":0}"#,
			),
		)
		.await;

		assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
		assert_eq!(dev_mux_state.snapshot().vite_plugin_control_port, None);
	}

	fn dev_mux_state_with_token(token: &str) -> DevMuxState {
		DevMuxRuntime::for_test(0, "", token).state()
	}

	fn headers_with_token(token: &str) -> HeaderMap {
		let mut headers = HeaderMap::new();
		headers.insert(VITE_PLUGIN_TOKEN_HEADER, token.parse().unwrap());
		headers
	}

	fn rpc_request(headers: HeaderMap, body: &str) -> Request {
		let mut request = Request::builder()
			.method("POST")
			.uri("/vite-plugin/rpc")
			.body(axum::body::Body::from(body.to_owned()))
			.unwrap();
		*request.headers_mut() = headers;
		request
	}
}
