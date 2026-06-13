//! Promotion-facing runtime host assembly.
//!
//! Top of the serve side: `RuntimeHost` is the public service users mount
//! into their HTTP stack. It loads the committed manifest from disk
//! (`runtime_assets`), compiles the snapshot, and assembles the
//! `runtime_service` stack — including dev-mode behavior driven by the
//! build's committed outputs.

use std::convert::Infallible;
use std::marker::PhantomData;
use std::pin::Pin;
use std::task::{Context, Poll};

use bytes::{Buf, Bytes};
use http::{Method, Request, Response, StatusCode};
use http_body::Body;
use http_body_util::Full;
use tower_service::Service;

use crate::asset_body_provider::{PublicAssetBodyProvider, PublicAssetDirectory};
use crate::document_renderer::DocumentRenderError;
use crate::error::Error;
use crate::public_app::App;
use crate::runtime_app::CommittedRuntimeApp;
use crate::runtime_assets::{ManifestMode, RuntimeAssetBundle};
use crate::runtime_document::RuntimeDocumentProvider;
use crate::runtime_service::CommittedRuntimeService;
use crate::runtime_snapshot::RuntimeSnapshot;
use crate::view_response::{ViewHtmlResponseInput, dev_refresh_script_content_sha256};

const BUILD_MODE_RUNTIME_HOST_UNAVAILABLE_BODY: &[u8] =
	b"Runtime assets are unavailable during Vorma build mode\n";
const DEV_HEALTH_RESPONSE_BODY: &[u8] = b"ok";

/// Public runtime service that serves committed Vorma assets, resources, and views.
pub struct RuntimeHost<S> {
	committed: Option<CommittedRuntimeHost<PublicAssetDirectory>>,
	_marker: PhantomData<fn(S)>,
}

impl<S> RuntimeHost<S>
where
	S: Send + Sync + 'static,
{
	/// Build a runtime host from a normalized app declaration.
	pub(crate) fn new(app: App<S>) -> crate::Result<Self> {
		if crate::is_build() {
			return Ok(Self {
				_marker: PhantomData,
				committed: None,
			});
		}
		let (config, assembly) = app.into_config_and_assembly();
		let mode = if crate::is_dev() {
			ManifestMode::Dev
		} else {
			ManifestMode::Prod
		};
		let asset_bundle = RuntimeAssetBundle::load(&config, mode)
			.map_err(|source| Error::new(source.to_string()))?;
		let (manifest, public_asset_directory) =
			asset_bundle.into_manifest_and_public_asset_directory();
		let committed = assembly
			.compile()
			.map_err(|source| Error::new(format!("{source:?}")))?
			.into_runtime_host(manifest, public_asset_directory)
			.map_err(|source| Error::new(source.to_string()))?;
		Ok(Self {
			_marker: PhantomData,
			committed: Some(committed),
		})
	}

	/// Resolve a public static source path through the committed runtime manifest.
	pub fn public_url(&self, src_path: &str) -> crate::Result<String> {
		if crate::is_build() {
			return Ok(String::new());
		}
		let host = self.committed_host()?;
		host.snapshot()
			.asset_capabilities()
			.public_url(src_path)
			.map(ToOwned::to_owned)
			.map_err(|source| Error::new(source.to_string()))
	}

	/// Current deterministic client build ID.
	pub fn client_build_id(&self) -> crate::Result<String> {
		let Some(host) = &self.committed else {
			return Ok(String::new());
		};
		Ok(host.client_build_id().to_owned())
	}

	/// SHA-256 content hash for the runtime critical CSS element.
	pub fn critical_css_content_sha256(&self) -> crate::Result<String> {
		let Some(host) = &self.committed else {
			return Ok(String::new());
		};
		Ok(host.critical_css_content_sha256().to_owned())
	}

	/// SHA-256 content hash for the dev refresh script element.
	pub fn dev_refresh_script_content_sha256(&self) -> crate::Result<String> {
		if !crate::is_dev() {
			return Ok(String::new());
		}
		let Some(host) = &self.committed else {
			return Ok(String::new());
		};
		Ok(host.dev_refresh_script_content_sha256().to_owned())
	}

	/// Handle an already-collected request.
	///
	/// Mounting [`RuntimeHost`] as a Tower service is usually preferable because the
	/// service implementation enforces the configured request body limit before calling
	/// this lower-level entry point.
	pub async fn handle_request(&self, request: Request<Bytes>) -> Result<Response<Bytes>, String> {
		if is_dev_health_request(&request) {
			return Ok(Response::new(Bytes::from_static(DEV_HEALTH_RESPONSE_BODY)));
		}
		let host = self
			.committed
			.as_ref()
			.ok_or_else(|| build_mode_runtime_assets_error().to_string())?;
		Ok(host.service().handle_request(request).await)
	}

	fn committed_host(&self) -> crate::Result<&CommittedRuntimeHost<PublicAssetDirectory>> {
		self.committed
			.as_ref()
			.ok_or_else(build_mode_runtime_assets_error)
	}
}

fn is_dev_health_request<B>(request: &Request<B>) -> bool {
	crate::is_dev()
		&& request.method() == Method::GET
		&& request.uri().path() == crate::constants::DEV_HEALTH_PATH
}

impl<S> Clone for RuntimeHost<S> {
	fn clone(&self) -> Self {
		Self {
			_marker: PhantomData,
			committed: self.committed.clone(),
		}
	}
}

impl<S, B> Service<Request<B>> for RuntimeHost<S>
where
	S: Send + Sync + 'static,
	B: Body + Send + 'static,
	B::Data: Buf + Send + 'static,
	B::Error:
		std::fmt::Display + Send + Sync + 'static + Into<Box<dyn std::error::Error + Send + Sync>>,
{
	type Response = Response<Full<Bytes>>;
	type Error = Infallible;
	type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

	fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
		Poll::Ready(Ok(()))
	}

	fn call(&mut self, request: Request<B>) -> Self::Future {
		if is_dev_health_request(&request) {
			return Box::pin(async move {
				Ok(Response::new(Full::new(Bytes::from_static(
					DEV_HEALTH_RESPONSE_BODY,
				))))
			});
		}
		let committed = self.committed.clone();
		Box::pin(async move {
			let Some(committed) = committed else {
				let mut response = Response::new(Full::new(Bytes::from_static(
					BUILD_MODE_RUNTIME_HOST_UNAVAILABLE_BODY,
				)));
				*response.status_mut() = StatusCode::INTERNAL_SERVER_ERROR;
				return Ok(response);
			};
			let mut service = committed.into_service();
			service.call(request).await
		})
	}
}

fn build_mode_runtime_assets_error() -> Error {
	Error::new("runtime assets are unavailable during Vorma build mode")
}

/// Committed runtime host ready to be mounted into an HTTP stack.
pub struct CommittedRuntimeHost<P> {
	service: CommittedRuntimeService<P>,
	client_build_id: String,
	critical_css_content_sha256: String,
	dev_refresh_script_content_sha256: String,
}

impl<P> CommittedRuntimeHost<P>
where
	P: PublicAssetBodyProvider + 'static,
{
	/// Build a runtime host from one committed app and asset provider.
	pub fn new(
		app: CommittedRuntimeApp,
		asset_provider: P,
		request_body_limit: usize,
	) -> Result<Self, RuntimeHostError> {
		let client_build_id = app.snapshot().client_build_id().to_owned();
		let critical_css_content_sha256 =
			app.snapshot()
				.manifest()
				.critical_css_content_sha256()
				.map_err(|source| RuntimeHostError::CriticalCssHash { source })?;
		let dev_refresh_script_content_sha256 =
			dev_refresh_script_content_sha256(app.snapshot().manifest());
		Ok(Self {
			service: CommittedRuntimeService::new(app, asset_provider, request_body_limit),
			client_build_id,
			critical_css_content_sha256,
			dev_refresh_script_content_sha256,
		})
	}

	/// Replace the trusted SSR HTML input used for HTML view responses.
	pub fn with_view_html(mut self, view_html: ViewHtmlResponseInput<'static>) -> Self {
		self.service = self.service.with_view_html(view_html);
		self
	}

	/// Replace the runtime document provider used for matched view responses.
	pub fn with_document_provider<D>(mut self, document_provider: D) -> Self
	where
		D: RuntimeDocumentProvider + 'static,
	{
		self.service = self.service.with_document_provider(document_provider);
		self
	}

	/// Committed client build identifier.
	pub fn client_build_id(&self) -> &str {
		&self.client_build_id
	}

	/// CSP-compatible SHA-256 hash of committed critical CSS style contents.
	pub fn critical_css_content_sha256(&self) -> &str {
		&self.critical_css_content_sha256
	}

	/// CSP-compatible SHA-256 hash of committed dev refresh script contents.
	pub fn dev_refresh_script_content_sha256(&self) -> &str {
		&self.dev_refresh_script_content_sha256
	}

	/// Committed runtime snapshot served by this host.
	pub fn snapshot(&self) -> &RuntimeSnapshot {
		self.service.app().snapshot()
	}

	/// Runtime service served by this host.
	pub fn service(&self) -> &CommittedRuntimeService<P> {
		&self.service
	}

	/// Consume this host into its runtime service.
	pub fn into_service(self) -> CommittedRuntimeService<P> {
		self.service
	}
}

impl<P> Clone for CommittedRuntimeHost<P> {
	fn clone(&self) -> Self {
		Self {
			service: self.service.clone(),
			client_build_id: self.client_build_id.clone(),
			critical_css_content_sha256: self.critical_css_content_sha256.clone(),
			dev_refresh_script_content_sha256: self.dev_refresh_script_content_sha256.clone(),
		}
	}
}

/// Runtime host assembly error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RuntimeHostError {
	/// Critical CSS hash calculation failed.
	CriticalCssHash {
		/// Source document render error.
		source: DocumentRenderError,
	},
}

impl std::fmt::Display for RuntimeHostError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::CriticalCssHash { source } => {
				write!(f, "calculate critical CSS content hash: {source}")
			}
		}
	}
}

impl std::error::Error for RuntimeHostError {
	fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
		match self {
			Self::CriticalCssHash { source } => Some(source),
		}
	}
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;
	use std::path::PathBuf;

	use bytes::Bytes;
	use http::Method;
	use http_body_util::{BodyExt, Full};
	use serde::{Deserialize, Serialize};
	use tower_service::Service;

	use super::*;
	use crate::asset_body_provider::PublicAssetBodyError;
	use crate::config::{DevWatchConfig, FrontendConfig, ServerTarget, TsGenConfig, UiVariant};
	use crate::document_builder::DocumentBuilder;
	use crate::execution_engine::{HandlerOutput, HandlerRegistry};
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ResourceDeclaration,
	};
	use crate::public_app::{App, AppConfig, Middlewares, Resource, Resources, Views};
	use crate::runtime_assets::{MANIFEST_STATIC_OUT_PROD, PUBLIC_OUT_DIR, static_out_dir};
	use crate::runtime_manifest::RuntimeManifest;
	use crate::runtime_snapshot::{RuntimeSnapshot, RuntimeSnapshotInput};
	use crate::test_support::{route_type_contract, unique_temp_root};
	use crate::tsgen::{FieldDef, Result as TsResult, Type, TypeDef, TypeRef, TypeRegistry};

	const TEST_CLIENT_BUILD_ID: &str = "build-id";
	const TEST_CRITICAL_CSS: &str = "body{color:black}";
	const TEST_PUBLIC_RUNTIME_INPUT_TYPE_NAME: &str = "PublicRuntimeInput";
	const TEST_PUBLIC_RUNTIME_OUTPUT_TYPE_NAME: &str = "PublicRuntimeOutput";
	const TEST_PUBLIC_RUNTIME_RESOURCE_PATTERN: &str = "/api/hello";
	const TEST_PUBLIC_RUNTIME_RESOURCE_URI: &str = "/api/hello";
	const TEST_PUBLIC_STATIC_BASE: &str = "/static/";
	const TEST_PUBLIC_SOURCE_PATH: &str = "app.css";
	const TEST_PUBLIC_OUTPUT_PATH: &str = "/static/app.hash.css";
	const TEST_PUBLIC_OUTPUT_FILE: &str = "app.hash.css";
	const TEST_PUBLIC_OUTPUT_BODY: &[u8] = b"body{}";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn runtime_app(manifest: RuntimeManifest) -> CommittedRuntimeApp {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			None,
			route_type_contract(),
			handler_id("ping"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let snapshot =
			RuntimeSnapshot::compile(RuntimeSnapshotInput::new(graph, manifest)).unwrap();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("ping"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"ok": true})))
		});
		CommittedRuntimeApp::new(snapshot, handlers)
	}

	fn manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			TEST_CLIENT_BUILD_ID,
			"/static/",
			TEST_CRITICAL_CSS,
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		)
	}

	fn asset_provider() -> impl PublicAssetBodyProvider {
		|_| async { Err(PublicAssetBodyError::new("no asset")) }
	}

	#[derive(Clone, Debug, Default)]
	struct PublicRuntimeState {
		prefix: String,
	}

	#[derive(Clone, Debug, Deserialize)]
	struct PublicRuntimeInput {
		name: String,
	}

	impl Type for PublicRuntimeInput {
		fn type_ref() -> TypeRef {
			TypeRef::named(TEST_PUBLIC_RUNTIME_INPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record(
				TEST_PUBLIC_RUNTIME_INPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("name")],
			));
			Ok(())
		}
	}

	#[derive(Clone, Debug, Serialize)]
	struct PublicRuntimeOutput {
		message: String,
	}

	impl Type for PublicRuntimeOutput {
		fn type_ref() -> TypeRef {
			TypeRef::named(TEST_PUBLIC_RUNTIME_OUTPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record(
				TEST_PUBLIC_RUNTIME_OUTPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("message")],
			));
			Ok(())
		}
	}

	const PUBLIC_RUNTIME_RESOURCE: Resource<PublicRuntimeState> = Resource::from_static(
		Method::POST,
		TEST_PUBLIC_RUNTIME_RESOURCE_PATTERN,
		None,
		crate::route_input::type_resolver::<PublicRuntimeInput>,
		crate::route_input::type_resolver::<PublicRuntimeOutput>,
		public_runtime_resource_handler,
	);

	fn public_runtime_resource_handler(
		ctx: crate::static_route::ErasedRequestCtx<PublicRuntimeState>,
	) -> crate::static_route::ErasedRouteFuture {
		crate::static_route::run_static_resource::<
			PublicRuntimeState,
			PublicRuntimeInput,
			(),
			PublicRuntimeOutput,
		>(ctx, public_runtime_resource_inner)
	}

	fn public_runtime_resource_inner(
		ctx: crate::ResourceCtx<PublicRuntimeState, PublicRuntimeInput>,
	) -> crate::static_route::RouteFuture<PublicRuntimeOutput, crate::HttpExit> {
		Box::pin(async move {
			Ok(PublicRuntimeOutput {
				message: format!("{}{}", ctx.state().prefix, ctx.input().name),
			})
		})
	}

	fn temp_root() -> PathBuf {
		unique_temp_root("vorma-runtime-host")
	}

	fn public_runtime_app_config(root_dir: PathBuf) -> AppConfig<PublicRuntimeState> {
		let mut resources: Resources<PublicRuntimeState> = Resources::new();
		resources.push(PUBLIC_RUNTIME_RESOURCE);
		AppConfig {
			root_dir,
			server_target: ServerTarget {
				cargo_package: "server-package".to_owned(),
				cargo_bin: "server-bin".to_owned(),
			},
			dist_dir: "dist".to_owned(),
			public_static_base: TEST_PUBLIC_STATIC_BASE.to_owned(),
			frontend_config: FrontendConfig {
				ui_variant: UiVariant::React,
				js_package_manager_base_cmd: "pnpm".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: "src/critical.css".to_owned(),
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				extra_types: Vec::new(),
				extra_ts: crate::tsgen::TsDrafter::new(),
			},
			dev_watch_config: DevWatchConfig::default(),
			state: PublicRuntimeState {
				prefix: "hello ".to_owned(),
			},
			views: Views::new(),
			resources,
			middlewares: Middlewares::new(),
			tasks_options: vorma_tasks::TasksOptions::default(),
			document: DocumentBuilder::default(),
			request_body_limit: 1024,
		}
	}

	fn write_public_runtime_assets(root_dir: &std::path::Path) {
		let static_out = static_out_dir(root_dir.join("dist"));
		let public_out = static_out.join(PUBLIC_OUT_DIR);
		fs::create_dir_all(&public_out).unwrap();
		fs::write(
			public_out.join(TEST_PUBLIC_OUTPUT_FILE),
			TEST_PUBLIC_OUTPUT_BODY,
		)
		.unwrap();
		fs::write(
			static_out.join(MANIFEST_STATIC_OUT_PROD),
			public_runtime_manifest().to_json_vec().unwrap(),
		)
		.unwrap();
	}

	fn public_runtime_manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			TEST_CLIENT_BUILD_ID,
			TEST_PUBLIC_STATIC_BASE,
			TEST_CRITICAL_CSS,
			BTreeMap::new(),
			vec![TEST_PUBLIC_OUTPUT_PATH.to_owned()],
			BTreeMap::from([(
				TEST_PUBLIC_SOURCE_PATH.to_owned(),
				TEST_PUBLIC_OUTPUT_PATH.to_owned(),
			)]),
			Vec::new(),
		)
	}

	#[test]
	fn runtime_host_exposes_committed_build_metadata() {
		let manifest = manifest();
		let expected_hash = manifest.critical_css_content_sha256().unwrap();
		let expected_refresh_hash =
			crate::view_response::dev_refresh_script_content_sha256(&manifest);

		let host =
			CommittedRuntimeHost::new(runtime_app(manifest), asset_provider(), 1024).unwrap();

		assert_eq!(host.client_build_id(), TEST_CLIENT_BUILD_ID);
		assert_eq!(host.critical_css_content_sha256(), expected_hash);
		assert_eq!(
			host.dev_refresh_script_content_sha256(),
			expected_refresh_hash
		);
		assert_eq!(host.snapshot().client_build_id(), TEST_CLIENT_BUILD_ID);
	}

	#[tokio::test]
	async fn public_runtime_host_loads_committed_assets_and_serves_public_app() {
		let root_dir = temp_root();
		fs::create_dir_all(root_dir.join("dist")).unwrap();
		write_public_runtime_assets(&root_dir);
		let loaded_manifest =
			RuntimeManifest::from_json_slice(&public_runtime_manifest().to_json_vec().unwrap())
				.unwrap();
		let app = App::from_app_config(public_runtime_app_config(root_dir.clone())).unwrap();
		let mut host = RuntimeHost::new(app).unwrap();

		assert_eq!(
			host.client_build_id().unwrap(),
			loaded_manifest.client_build_id()
		);
		assert_eq!(
			host.public_url(TEST_PUBLIC_SOURCE_PATH).unwrap(),
			TEST_PUBLIC_OUTPUT_PATH
		);
		assert_eq!(
			host.critical_css_content_sha256().unwrap(),
			manifest().critical_css_content_sha256().unwrap()
		);
		assert_eq!(host.dev_refresh_script_content_sha256().unwrap(), "");

		let request = http::Request::builder()
			.method(Method::POST)
			.uri(TEST_PUBLIC_RUNTIME_RESOURCE_URI)
			.body(Full::new(Bytes::from_static(br#"{"name":"Ada"}"#)))
			.unwrap();
		let response = host.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();
		let body: serde_json::Value = serde_json::from_slice(&body).unwrap();

		assert_eq!(body["message"], "hello Ada");

		let request = http::Request::builder()
			.method(Method::GET)
			.uri(TEST_PUBLIC_OUTPUT_PATH)
			.body(Full::new(Bytes::new()))
			.unwrap();
		let response = host.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(body, Bytes::from_static(TEST_PUBLIC_OUTPUT_BODY));

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn public_runtime_host_public_url_is_inert_in_build_mode() {
		let root_dir = temp_root();
		let host = crate::envutil::with_build_mode(|| {
			let app = App::from_app_config(public_runtime_app_config(root_dir)).unwrap();
			RuntimeHost::new(app).unwrap()
		});

		let resolved =
			crate::envutil::with_build_mode(|| host.public_url(TEST_PUBLIC_SOURCE_PATH)).unwrap();

		assert_eq!(resolved, "");
	}

	#[tokio::test]
	async fn runtime_host_owns_mountable_committed_service() {
		let mut service =
			CommittedRuntimeHost::new(runtime_app(manifest()), asset_provider(), 1024)
				.unwrap()
				.into_service();
		let request = http::Request::builder()
			.method(Method::GET)
			.uri("/api/ping")
			.body(Full::new(Bytes::new()))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let body = response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(body, Bytes::from_static(br#"{"ok":true}"#));
	}

	#[tokio::test]
	async fn runtime_host_serves_health_endpoint_only_in_dev_mode() {
		let mut host = RuntimeHost::<()> {
			_marker: PhantomData,
			committed: None,
		};
		let dev_request = http::Request::builder()
			.method(Method::GET)
			.uri(crate::constants::DEV_HEALTH_PATH)
			.body(Full::new(Bytes::new()))
			.unwrap();
		let dev_response = crate::envutil::with_dev_mode(|| host.call(dev_request))
			.await
			.unwrap();
		let dev_body = dev_response.into_body().collect().await.unwrap().to_bytes();

		assert_eq!(dev_body, Bytes::from_static(DEV_HEALTH_RESPONSE_BODY));

		let dev_request = http::Request::builder()
			.method(Method::GET)
			.uri(crate::constants::DEV_HEALTH_PATH)
			.body(Bytes::new())
			.unwrap();

		assert!(crate::envutil::with_dev_mode(|| is_dev_health_request(
			&dev_request
		)));

		let prod_request = http::Request::builder()
			.method(Method::GET)
			.uri(crate::constants::DEV_HEALTH_PATH)
			.body(Full::new(Bytes::new()))
			.unwrap();
		let prod_response = host.call(prod_request).await.unwrap();

		assert_eq!(prod_response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	}
}
