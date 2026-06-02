use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};

use bytes::Buf;
use bytes::Bytes;
use http::header::{ALLOW, CACHE_CONTROL, CONTENT_LENGTH, CONTENT_TYPE, HeaderValue};
use http::{Method, Request, Response, StatusCode};
use http_body::Body;
use http_body_util::{BodyExt, Full, LengthLimitError, Limited};
use path_clean::PathClean;
use vorma_tasks::{CancelToken, Tasks};

use crate::App;
use crate::config::Config;
use crate::config::normalize_api_mount_root;
use crate::core::{RuntimeRoutes, runtime_routes_for};
use crate::document::{DocumentBuildCtx, DocumentBuilder};
use crate::envutil::{is_build, is_dev};
use crate::error::ViewErrorClientMsg;
use crate::handler::{
	ApiResponseInput, ViewResponseResultsInput, build_api_response,
	build_view_response_from_results, build_view_skew_response, refresh_script_inner_html,
	view_not_found_response,
};
use crate::htmlutil::{Element, compute_content_sha256};
use crate::mux::RawRequest;
use crate::response::response_with_client_build_id;
use crate::response::{
	internal_server_error_response, internal_server_error_with_client_build_id, plain_text_response,
};
use crate::r#static::{
	ManifestMode, RuntimeAssetSnapshot, RuntimeAssets, path_is_under_public_static_base,
	static_out_dir,
};

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
		validate_runtime_config(&cfg)?;
		let assets = default_runtime_assets(&cfg);
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

		let request = raw_request(request);

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

	async fn handle_api_request(&self, request: RawRequest) -> Result<Response<Bytes>, String> {
		if !self.routes.resources.method_is_allowed(request.method()) {
			let allow = self.api_allow_header();
			if !allow.is_empty() {
				return method_not_allowed_response(&allow);
			}
		}
		let snapshot = self
			.assets
			.as_ref()
			.map(RuntimeAssets::snapshot)
			.transpose()?;
		let client_build_id = snapshot
			.as_ref()
			.map(|snapshot| snapshot.client_build_id().to_owned())
			.unwrap_or_default();
		let public_filemap = Arc::new(
			snapshot
				.as_ref()
				.map(|snapshot| snapshot.manifest().public_filemap.clone())
				.unwrap_or_default(),
		);
		let raw_path = request.path().to_owned();
		let RequestExecCtx {
			exec_ctx,
			_cancel_on_drop,
		} = self.request_exec_ctx();
		let result = match self
			.routes
			.resources
			.execute_route(request, self.state.clone(), exec_ctx, public_filemap)
			.await
		{
			Ok(result) => result,
			Err(_) => return internal_server_error_with_client_build_id(&client_build_id),
		};
		let Some(result) = result else {
			if let Some(allow) = self.api_allow_header_for_path(&raw_path) {
				let response = method_not_allowed_response(&allow)?;
				return response_with_client_build_id(response, &client_build_id);
			}
			let response = empty_response(StatusCode::NOT_FOUND)?;
			return response_with_client_build_id(response, &client_build_id);
		};
		let response = match build_api_response(ApiResponseInput {
			expected_client_build_id: &client_build_id,
			result: &result,
		}) {
			Ok(response) => response,
			Err(_) => return internal_server_error_with_client_build_id(&client_build_id),
		};
		Ok(response)
	}

	async fn handle_view_request(&self, request: RawRequest) -> Result<Response<Bytes>, String> {
		let Some(assets) = &self.assets else {
			return empty_response(StatusCode::NOT_FOUND);
		};
		let snapshot = assets.snapshot()?;
		match build_view_skew_response(request.uri(), snapshot.client_build_id()) {
			Ok(Some(response)) => return Ok(response),
			Ok(Option::None) => {}
			Err(_) => {
				return internal_server_error_with_client_build_id(snapshot.client_build_id());
			}
		}
		let match_results = match self.routes.views.find_nested_matches(request.path()) {
			Ok(Some(match_results)) => match_results,
			Ok(Option::None) => return view_not_found_response(snapshot.client_build_id()),
			Err(_) => {
				return internal_server_error_with_client_build_id(snapshot.client_build_id());
			}
		};
		let RequestExecCtx {
			exec_ctx,
			_cancel_on_drop,
		} = self.request_exec_ctx();
		let public_filemap = Arc::new(snapshot.manifest().public_filemap.clone());
		let document = self.document.build(DocumentBuildCtx::manifest(
			snapshot.manifest_handle(),
			request.clone(),
		));
		let tasks_results = self.routes.views.run_nested_tasks(
			self.state.clone(),
			exec_ctx,
			request.clone(),
			match_results.clone(),
			public_filemap,
		);
		let (document, tasks_results) = match tokio::try_join!(document, async {
			tasks_results.await.map_err(|err| err.to_string())
		}) {
			Ok(results) => results,
			Err(_) => {
				return internal_server_error_with_client_build_id(snapshot.client_build_id());
			}
		};
		match build_view_response_from_results(ViewResponseResultsInput {
			expected_client_build_id: snapshot.client_build_id(),
			request: &request,
			manifest: snapshot.manifest(),
			document: &document,
			match_results: &match_results,
			tasks_results: &tasks_results,
		}) {
			Ok(response) => Ok(response),
			Err(_) => internal_server_error_with_client_build_id(snapshot.client_build_id()),
		}
	}

	fn public_asset_response(
		&self,
		method: &Method,
		path: &str,
	) -> Result<Option<Response<Bytes>>, String> {
		if method != Method::GET && method != Method::HEAD {
			return Ok(None);
		}
		let Some(assets) = &self.assets else {
			return Ok(None);
		};
		if method == Method::HEAD {
			let Some(asset) = assets.stat_public_asset(path)? else {
				return self.public_asset_missing_response(assets, path);
			};
			return public_asset_found_response(
				Bytes::new(),
				asset.content_length,
				asset.cache_control,
				asset.content_type,
			)
			.map(Some);
		}
		let Some(asset) = assets.read_public_asset(path)? else {
			return self.public_asset_missing_response(assets, path);
		};
		let content_length = asset.bytes.len() as u64;
		public_asset_found_response(
			Bytes::from(asset.bytes),
			content_length,
			asset.cache_control,
			asset.content_type,
		)
		.map(Some)
	}

	fn public_asset_missing_response(
		&self,
		assets: &RuntimeAssets,
		path: &str,
	) -> Result<Option<Response<Bytes>>, String> {
		let snapshot = assets.snapshot()?;
		if path_is_under_public_static_base(snapshot.manifest(), path) {
			return empty_response(StatusCode::NOT_FOUND).map(Some);
		}
		Ok(None)
	}

	fn api_allow_header_for_path(&self, path: &str) -> Option<String> {
		let methods = self.routes.resources.allowed_methods_for_path(path);
		if methods.is_empty() {
			return None;
		}
		Some(allow_header_value(&methods))
	}

	fn api_allow_header(&self) -> String {
		allow_header_value(&self.routes.resources.allowed_methods())
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

fn default_runtime_assets(cfg: &Config) -> Option<RuntimeAssets> {
	if is_build() {
		return None;
	}

	let static_out = static_out_dir(config_path(&cfg.root_dir, &cfg.dist_dir));
	if is_dev() {
		return Some(RuntimeAssets::live_fs(static_out, ManifestMode::Dev));
	}
	Some(RuntimeAssets::cached_fs(static_out, ManifestMode::Prod))
}

fn config_path(root_dir: &Path, path: &str) -> PathBuf {
	let path = PathBuf::from(path);
	if path.is_absolute() {
		return path.clean();
	}
	root_dir.join(path).clean()
}

fn validate_runtime_config(cfg: &Config) -> Result<(), String> {
	if cfg.root_dir.as_os_str().is_empty() {
		return Err("root_dir cannot be empty".to_owned());
	}
	if !cfg.root_dir.is_absolute() {
		return Err("root_dir must be absolute".to_owned());
	}
	if !cfg.root_dir.exists() {
		return Err(format!(
			"root dir does not exist: {}",
			cfg.root_dir.display()
		));
	}
	if !cfg.root_dir.is_dir() {
		return Err(format!(
			"root dir is not a directory: {}",
			cfg.root_dir.display()
		));
	}
	let root_dir = cfg.root_dir.clean();
	let dist_dir = config_path(&root_dir, &cfg.dist_dir);
	if !dist_dir.starts_with(&root_dir) {
		return Err("dist_dir must be inside root_dir".to_owned());
	}
	Ok(())
}

fn raw_request(request: Request<Bytes>) -> RawRequest {
	let (parts, body) = request.into_parts();
	RawRequest::with_extensions(
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

fn public_asset_found_response(
	body: Bytes,
	content_length: u64,
	cache_control: &'static str,
	content_type: String,
) -> Result<Response<Bytes>, String> {
	let mut response = Response::new(body);
	response
		.headers_mut()
		.insert(CACHE_CONTROL, HeaderValue::from_static(cache_control));
	response.headers_mut().insert(
		CONTENT_TYPE,
		HeaderValue::from_str(&content_type).map_err(|err| err.to_string())?,
	);
	response.headers_mut().insert(
		CONTENT_LENGTH,
		HeaderValue::from_str(&content_length.to_string()).map_err(|err| err.to_string())?,
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
#[path = "init_tests.rs"]
mod init_tests;
