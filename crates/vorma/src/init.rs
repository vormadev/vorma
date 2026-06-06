mod public_asset_service;
mod resource_service;
mod view_service;

use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};

use bytes::Buf;
use bytes::Bytes;
use http::header::{ALLOW, CONTENT_LENGTH, HeaderValue};
#[cfg(test)]
use http::header::{CACHE_CONTROL, CONTENT_TYPE};
use http::{Method, Request, Response, StatusCode};
use http_body::Body;
use http_body_util::{BodyExt, Full, LengthLimitError, Limited};
use path_clean::PathClean;
use vorma_tasks::{CancelToken, Tasks};

use crate::App;
use crate::config::Config;
use crate::config::normalize_api_mount_root;
use crate::core::{RuntimeRoutes, runtime_routes_for};
use crate::document::DocumentBuilder;
use crate::envutil::{is_build, is_dev};
use crate::error::ViewErrorClientMsg;
use crate::handler::refresh_script_inner_html;
use crate::htmlutil::{Element, compute_content_sha256};
use crate::manifest::{MANIFEST_STATIC_OUT_DEV, MANIFEST_STATIC_OUT_PROD};
use crate::mux::RawRequest;
use crate::response::{internal_server_error_response, plain_text_response};
use crate::r#static::{ManifestMode, RuntimeAssetSnapshot, RuntimeAssets, static_out_dir};

const VERCEL_SYSTEM_ENV_KEY: &str = "VERCEL";
const VERCEL_ENVIRONMENT_ENV_KEY: &str = "VERCEL_ENV";

/// Runtime service that serves Vorma static assets, resources, and views.
pub struct RuntimeHost<S, E = Box<dyn std::error::Error + Send + Sync>> {
	state: Arc<S>,
	tasks: Tasks<E>,
	document: DocumentBuilder,
	routes: Arc<RuntimeRoutes<S, E>>,
	assets: Option<RuntimeAssets>,
	request_body_limit: usize,
}

impl<S, E> RuntimeHost<S, E>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	pub(crate) fn new(app: App<S, E>) -> Result<Self, String> {
		let App {
			config: cfg,
			state,
			views,
			resources,
			middlewares,
			tasks_options,
			document,
			request_body_limit,
		} = app;
		let assets = default_runtime_assets(&cfg)?;
		if assets.is_none() {
			validate_runtime_config(&cfg)?;
		}
		let initial_snapshot = assets
			.as_ref()
			.map(RuntimeAssets::snapshot)
			.transpose()
			.map_err(|err| format!("error initializing runtime assets: {err}"))?;
		let api_mount_root = initial_snapshot
			.as_ref()
			.map(|snapshot| snapshot.manifest().api_mount_root.as_str())
			.unwrap_or(&cfg.path_config.api_base);
		let api_mount_root = normalize_api_mount_root(api_mount_root)
			.map_err(|err| format!("error with API mount root: {err}"))?;
		let routes = runtime_routes_for(&views, &resources, &middlewares, &api_mount_root)
			.map_err(|err| err.to_string())?;
		Ok(Self {
			state: Arc::new(state),
			tasks: Tasks::new(tasks_options),
			document,
			routes: Arc::new(routes),
			assets,
			request_body_limit,
		})
	}

	/// Resolve a public static source path through the current runtime manifest.
	pub fn public_url(&self, src_path: &str) -> Result<String, String> {
		if is_build() {
			return Ok(String::new());
		}
		let snapshot = self.snapshot()?;
		snapshot
			.manifest()
			.public_url(src_path)
			.map(ToOwned::to_owned)
			.ok_or_else(|| format!("file {src_path} not found in manifest public filemap"))
	}

	/// Current deterministic client build ID.
	pub fn client_build_id(&self) -> Result<String, String> {
		if is_build() {
			return Ok(String::new());
		}
		let snapshot = self.snapshot()?;
		Ok(snapshot.client_build_id().to_owned())
	}

	/// SHA-256 content hash for the runtime critical CSS element.
	pub fn critical_css_content_sha256(&self) -> Result<String, String> {
		if is_build() {
			return Ok(String::new());
		}
		let snapshot = self.snapshot()?;
		Ok(snapshot.manifest().critical_css_content_sha256())
	}

	/// SHA-256 content hash for the dev refresh script element.
	pub fn dev_refresh_script_content_sha256(&self) -> Result<String, String> {
		if !is_dev() {
			return Ok(String::new());
		}
		let snapshot = self.snapshot()?;
		let inner_html = refresh_script_inner_html(snapshot.manifest());
		Ok(compute_content_sha256(&Element {
			dangerous_inner_html: inner_html,
			..Element::default()
		}))
	}

	/// Handle an already-collected request.
	///
	/// Mounting [`RuntimeHost`] as a Tower service is usually preferable because the
	/// service implementation enforces the configured request body limit before calling
	/// this lower-level entry point.
	pub async fn handle_request(&self, request: Request<Bytes>) -> Result<Response<Bytes>, String> {
		let head_request = request.method() == Method::HEAD;
		let response = self.handle_request_with_body(request).await?;
		if head_request {
			return Ok(head_response_without_body(response));
		}
		Ok(response)
	}

	async fn handle_request_with_body(
		&self,
		request: Request<Bytes>,
	) -> Result<Response<Bytes>, String> {
		if request.body().len() > self.request_body_limit {
			return Ok(payload_too_large_response());
		}

		if let Some(response) = dev_health_check_response(&request)? {
			return Ok(response);
		}

		let request = match raw_request(request) {
			Ok(request) => request,
			Err(_) => {
				return Ok(plain_text_response(
					StatusCode::BAD_REQUEST,
					Bytes::from_static(b"Bad Request\n"),
				));
			}
		};

		if request_path_is_under_mount_root(request.path(), self.routes.resources.mount_root()) {
			return self.handle_api_request(request).await;
		}

		if let Some(response) = self.public_asset_response(request.method(), request.path())? {
			return Ok(response);
		}

		if request.method() == Method::GET || request.method() == Method::HEAD {
			return self.handle_view_request(request).await;
		}

		empty_response(StatusCode::NOT_FOUND)
	}

	fn snapshot(&self) -> Result<Arc<RuntimeAssetSnapshot>, String> {
		self.assets
			.as_ref()
			.ok_or_else(|| "runtime assets are unavailable".to_owned())?
			.snapshot()
	}

	fn request_exec_ctx(&self) -> RequestExecCtx<E> {
		let cancel = CancelToken::new();
		RequestExecCtx {
			exec_ctx: self.tasks.exec_ctx(cancel.clone()),
			_cancel_on_drop: CancelOnDrop(cancel),
		}
	}
}

impl<S, E> Clone for RuntimeHost<S, E> {
	fn clone(&self) -> Self {
		Self {
			state: self.state.clone(),
			tasks: self.tasks.clone(),
			document: self.document.clone(),
			routes: self.routes.clone(),
			assets: self.assets.clone(),
			request_body_limit: self.request_body_limit,
		}
	}
}

impl<S, E, B> tower_service::Service<Request<B>> for RuntimeHost<S, E>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
	B: Body + Send + 'static,
	B::Data: Buf + Send + 'static,
	B::Error:
		std::fmt::Display + Send + Sync + 'static + Into<Box<dyn std::error::Error + Send + Sync>>,
{
	type Response = Response<Full<Bytes>>;
	type Error = std::convert::Infallible;
	type Future = Pin<
		Box<
			dyn std::future::Future<Output = Result<Self::Response, std::convert::Infallible>>
				+ Send
				+ 'static,
		>,
	>;

	fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), std::convert::Infallible>> {
		Poll::Ready(Ok(()))
	}

	fn call(&mut self, request: Request<B>) -> Self::Future {
		let host = self.clone();
		Box::pin(async move {
			let head_request = request.method() == Method::HEAD;
			let response = match collect_http_request(request, host.request_body_limit).await {
				Ok(request) => host
					.handle_request_with_body(request)
					.await
					.unwrap_or_else(|_| internal_server_error_response()),
				Err(RequestBodyError::PayloadTooLarge) => payload_too_large_response(),
				Err(RequestBodyError::Read(_)) => internal_server_error_response(),
			};
			let response = if head_request {
				head_response_without_body(response)
			} else {
				response
			};
			Ok(bytes_response_to_full(response))
		})
	}
}

fn default_runtime_assets(cfg: &Config) -> Result<Option<RuntimeAssets>, String> {
	if is_build() {
		return Ok(None);
	}

	if is_dev() {
		let static_out = runtime_static_out_dir(cfg, ManifestMode::Dev)?;
		return Ok(Some(RuntimeAssets::live_fs(static_out, ManifestMode::Dev)));
	}
	let static_out = runtime_static_out_dir(cfg, ManifestMode::Prod)?;
	Ok(Some(RuntimeAssets::cached_fs(
		static_out,
		ManifestMode::Prod,
	)))
}

fn config_path(root_dir: &Path, path: &str) -> PathBuf {
	let path = PathBuf::from(path);
	if path.is_absolute() {
		return path.clean();
	}
	root_dir.join(path).clean()
}

fn runtime_static_out_dir(cfg: &Config, mode: ManifestMode) -> Result<PathBuf, String> {
	validate_runtime_root_dir_shape(&cfg.root_dir)?;
	let configured_root_dir = cfg.root_dir.clean();
	let configured_dist_dir = runtime_dist_dir(&configured_root_dir, &cfg.dist_dir)?;
	let configured_static_out = static_out_dir(configured_dist_dir);
	if mode == ManifestMode::Prod
		&& vercel_runtime_env_is_present()
		&& !runtime_manifest_path(&configured_static_out, mode).is_file()
	{
		return vercel_runtime_static_out_dir(cfg, &configured_static_out, mode);
	}
	validate_existing_runtime_root_dir(&configured_root_dir)?;
	Ok(configured_static_out)
}

fn vercel_runtime_static_out_dir(
	cfg: &Config,
	configured_static_out: &Path,
	mode: ManifestMode,
) -> Result<PathBuf, String> {
	let current_root_dir = std::env::current_dir().map_err(|err| {
		format!("Vercel runtime current dir must be readable while resolving Vorma assets: {err}")
	})?;
	vercel_runtime_static_out_dir_from_current_root(
		cfg,
		configured_static_out,
		mode,
		current_root_dir,
	)
}

fn vercel_runtime_static_out_dir_from_current_root(
	cfg: &Config,
	configured_static_out: &Path,
	mode: ManifestMode,
	current_root_dir: PathBuf,
) -> Result<PathBuf, String> {
	let current_root_dir = current_root_dir.clean();
	validate_existing_runtime_root_dir(&current_root_dir)?;
	let current_dist_dir = runtime_dist_dir(&current_root_dir, &cfg.dist_dir)?;
	let current_static_out = static_out_dir(current_dist_dir);
	if runtime_manifest_path(&current_static_out, mode).is_file() {
		return Ok(current_static_out);
	}
	Err(format!(
		"Vercel runtime Vorma manifest was not found at configured root {} or current dir {}",
		runtime_manifest_path(configured_static_out, mode).display(),
		runtime_manifest_path(&current_static_out, mode).display()
	))
}

fn runtime_manifest_path(static_out: &Path, mode: ManifestMode) -> PathBuf {
	static_out.join(match mode {
		ManifestMode::Dev => MANIFEST_STATIC_OUT_DEV,
		ManifestMode::Prod => MANIFEST_STATIC_OUT_PROD,
	})
}

fn vercel_runtime_env_is_present() -> bool {
	std::env::var_os(VERCEL_SYSTEM_ENV_KEY).is_some()
		|| std::env::var_os(VERCEL_ENVIRONMENT_ENV_KEY).is_some()
}

fn validate_runtime_config(cfg: &Config) -> Result<(), String> {
	validate_runtime_root_dir_shape(&cfg.root_dir)?;
	let root_dir = cfg.root_dir.clean();
	validate_existing_runtime_root_dir(&root_dir)?;
	runtime_dist_dir(&root_dir, &cfg.dist_dir)?;
	Ok(())
}

fn validate_runtime_root_dir_shape(root_dir: &Path) -> Result<(), String> {
	if root_dir.as_os_str().is_empty() {
		return Err("root_dir cannot be empty".to_owned());
	}
	if !root_dir.is_absolute() {
		return Err("root_dir must be absolute".to_owned());
	}
	Ok(())
}

fn validate_existing_runtime_root_dir(root_dir: &Path) -> Result<(), String> {
	if !root_dir.exists() {
		return Err(format!("root dir does not exist: {}", root_dir.display()));
	}
	if !root_dir.is_dir() {
		return Err(format!(
			"root dir is not a directory: {}",
			root_dir.display()
		));
	}
	Ok(())
}

fn runtime_dist_dir(root_dir: &Path, dist_dir: &str) -> Result<PathBuf, String> {
	let dist_dir = config_path(root_dir, dist_dir);
	if !dist_dir.starts_with(root_dir) {
		return Err("dist_dir must be inside root_dir".to_owned());
	}
	Ok(dist_dir)
}

fn raw_request(request: Request<Bytes>) -> Result<RawRequest, String> {
	let (parts, body) = request.into_parts();
	RawRequest::try_with_extensions(
		parts.method,
		parts.uri,
		parts.headers,
		body,
		parts.extensions,
	)
}

struct CancelOnDrop(CancelToken);

impl Drop for CancelOnDrop {
	fn drop(&mut self) {
		self.0.cancel();
	}
}

struct RequestExecCtx<E> {
	exec_ctx: vorma_tasks::ExecCtx<E>,
	_cancel_on_drop: CancelOnDrop,
}

async fn collect_http_request<B>(
	request: Request<B>,
	limit: usize,
) -> Result<Request<Bytes>, RequestBodyError>
where
	B: Body + Send + 'static,
	B::Data: Buf + Send + 'static,
	B::Error:
		std::fmt::Display + Send + Sync + 'static + Into<Box<dyn std::error::Error + Send + Sync>>,
{
	let (parts, body) = request.into_parts();
	let bytes = match Limited::new(body, limit).collect().await {
		Ok(collected) => collected.to_bytes(),
		Err(error) if error.is::<LengthLimitError>() => {
			return Err(RequestBodyError::PayloadTooLarge);
		}
		Err(error) => {
			return Err(RequestBodyError::Read(error.to_string()));
		}
	};
	Ok(Request::from_parts(parts, bytes))
}

#[derive(Debug, Eq, PartialEq)]
enum RequestBodyError {
	PayloadTooLarge,
	Read(String),
}

fn bytes_response_to_full(response: Response<Bytes>) -> Response<Full<Bytes>> {
	let (parts, body) = response.into_parts();
	Response::from_parts(parts, Full::new(body))
}

fn payload_too_large_response() -> Response<Bytes> {
	plain_text_response(
		StatusCode::PAYLOAD_TOO_LARGE,
		Bytes::from_static(b"Payload Too Large\n"),
	)
}

fn dev_health_check_response(request: &Request<Bytes>) -> Result<Option<Response<Bytes>>, String> {
	if !is_dev() || request.method() != Method::GET || request.uri().path() != "/.vorma/healthz" {
		return Ok(None);
	}
	Response::builder()
		.status(StatusCode::OK)
		.body(Bytes::from_static(b"ok"))
		.map(Some)
		.map_err(|err| err.to_string())
}

fn empty_response(status: StatusCode) -> Result<Response<Bytes>, String> {
	Response::builder()
		.status(status)
		.body(Bytes::new())
		.map_err(|err| err.to_string())
}

fn method_not_allowed_response(allow: &str) -> Result<Response<Bytes>, String> {
	let mut response = plain_text_response(
		StatusCode::METHOD_NOT_ALLOWED,
		Bytes::from_static(b"Method Not Allowed\n"),
	);
	response.headers_mut().insert(
		ALLOW,
		HeaderValue::from_str(allow).map_err(|err| err.to_string())?,
	);
	Ok(response)
}

fn allow_header_value(methods: &std::collections::BTreeSet<String>) -> String {
	methods
		.iter()
		.map(String::as_str)
		.collect::<Vec<_>>()
		.join(", ")
}

fn head_response_without_body(response: Response<Bytes>) -> Response<Bytes> {
	let (mut parts, body) = response.into_parts();
	if !parts.headers.contains_key(CONTENT_LENGTH)
		&& !parts.headers.contains_key(http::header::TRANSFER_ENCODING)
	{
		let value = HeaderValue::from_str(&body.len().to_string())
			.expect("usize length should be a valid header value");
		parts.headers.insert(CONTENT_LENGTH, value);
	}
	Response::from_parts(parts, Bytes::new())
}

fn request_path_is_under_mount_root(path: &str, mount_root: &str) -> bool {
	let without_trailing = mount_root.trim_end_matches('/');
	path == without_trailing || path.starts_with(mount_root)
}

#[cfg(test)]
#[path = "init_tests/mod.rs"]
mod init_tests;
