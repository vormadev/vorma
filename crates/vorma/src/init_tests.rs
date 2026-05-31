use std::collections::BTreeMap;
use std::fs;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use http::HeaderName;
use http::header::{LOCATION, SET_COOKIE};
use http_body_util::BodyExt;
use serde_json::json;
use tokio::sync::{Notify, oneshot};

use super::*;
use crate::api::FormData;
use crate::constants::X_VORMA_CLIENT_BUILD_ID;
use crate::core::{
	ApiRoute, ApiRouteKind, ApiRoutes, TaskMiddleware, TaskMiddlewares, View, Views,
};
use crate::manifest::{ClientModule, Manifest};
use crate::mux::{InputError, InputParser};
use crate::r#static::PUBLIC_ASSET_CACHE_CONTROL;

fn temp_dist_dir(name: &str) -> PathBuf {
	let nanos = SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.unwrap()
		.as_nanos();
	let root = std::env::temp_dir().join(format!(
		"vorma-runtime-host-{name}-{}-{nanos}",
		std::process::id()
	));
	fs::create_dir_all(root.join(".vorma/static/public")).unwrap();
	root
}

fn cfg(dist_dir: &Path) -> Config {
	Config {
		root_dir: dist_dir.to_path_buf(),
		dist_dir: ".".to_owned(),
		path_config: crate::config::PathConfig {
			public_static_base: "/static/".to_owned(),
			api_base: "/api/".to_owned(),
		},
		..Config::default()
	}
}

fn app(
	config: Config,
	views: Views<(), &'static str>,
	api_routes: ApiRoutes<(), &'static str>,
	task_middlewares: TaskMiddlewares<(), &'static str>,
	document: DocumentBuilder,
) -> App<(), &'static str> {
	let Config {
		root_dir,
		server_config,
		dist_dir,
		path_config,
		frontend_config,
		ts_gen_config,
		dev_watch_config,
	} = config;
	App::from_app_config(crate::AppConfig {
		root_dir,
		server_config,
		dist_dir,
		path_config,
		frontend_config,
		ts_gen_config,
		dev_watch_config,
		state: (),
		views,
		api_routes,
		task_middlewares,
		tasks_options: vorma_tasks::TasksOptions::default(),
		document,
		request_body_limit: crate::DEFAULT_REQUEST_BODY_LIMIT,
	})
}

fn default_app(
	config: Config,
	views: Views<(), &'static str>,
	api_routes: ApiRoutes<(), &'static str>,
	task_middlewares: TaskMiddlewares<(), &'static str>,
) -> App<(), &'static str> {
	app(
		config,
		views,
		api_routes,
		task_middlewares,
		DocumentBuilder::default(),
	)
}

fn runtime_host(
	dist_dir: &Path,
	views: Views<(), &'static str>,
	api_routes: ApiRoutes<(), &'static str>,
	task_middlewares: TaskMiddlewares<(), &'static str>,
) -> RuntimeHost<(), &'static str> {
	RuntimeHost::new(default_app(
		cfg(dist_dir),
		views,
		api_routes,
		task_middlewares,
	))
	.unwrap()
}

fn runtime_host_with_document(
	dist_dir: &Path,
	views: Views<(), &'static str>,
	api_routes: ApiRoutes<(), &'static str>,
	task_middlewares: TaskMiddlewares<(), &'static str>,
	document: DocumentBuilder,
) -> RuntimeHost<(), &'static str> {
	RuntimeHost::new(app(
		cfg(dist_dir),
		views,
		api_routes,
		task_middlewares,
		document,
	))
	.unwrap()
}

fn empty_request(method: Method, uri: &str) -> Request<Bytes> {
	request_with_body(method, uri, Bytes::new())
}

struct DropSignal(Arc<Mutex<Option<oneshot::Sender<()>>>>);

impl Drop for DropSignal {
	fn drop(&mut self) {
		if let Some(tx) = self.0.lock().unwrap().take() {
			let _ = tx.send(());
		}
	}
}

fn request_with_body(method: Method, uri: &str, body: Bytes) -> Request<Bytes> {
	Request::builder()
		.method(method)
		.uri(uri)
		.body(body)
		.unwrap()
}

fn manifest() -> Manifest {
	Manifest {
		vorma_version: "0.1.0".to_owned(),
		public_static_base_path: "/static/".to_owned(),
		api_mount_root: "/api/".to_owned(),
		ui_variant: "react".to_owned(),
		root_document_shell_hash: "shell".to_owned(),
		public_filepaths: vec![
			"/static/app.css".to_owned(),
			"/static/favicon.ico".to_owned(),
		],
		public_filemap: BTreeMap::from([
			("app.css".to_owned(), "/static/app.css".to_owned()),
			("favicon.ico".to_owned(), "/static/favicon.ico".to_owned()),
		]),
		client_entry: ClientModule {
			url: "/static/entry.js".to_owned(),
			..ClientModule::default()
		},
		..Manifest::default()
	}
}

fn write_manifest(dist_dir: &Path) {
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest()).unwrap(),
	)
	.unwrap();
}

fn write_manifest_with_api_mount_root(dist_dir: &Path, api_mount_root: &str) -> Manifest {
	let mut manifest = manifest();
	manifest.api_mount_root = api_mount_root.to_owned();
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest).unwrap(),
	)
	.unwrap();
	manifest
}

fn manifest_with_view_assets() -> Manifest {
	Manifest {
		critical_css: "body { color: black; }".to_owned(),
		client_entry: ClientModule {
			url: "/static/entry.js".to_owned(),
			dep_urls: vec![
				"/static/entry.js".to_owned(),
				"/static/shared.js".to_owned(),
			],
			css_bundle_urls: vec![
				"/static/entry.css".to_owned(),
				"/static/shared.css".to_owned(),
			],
		},
		client_routes: BTreeMap::from([(
			"/items/:id".to_owned(),
			ClientModule {
				url: "/static/items.js".to_owned(),
				dep_urls: vec![
					"/static/shared.js".to_owned(),
					"/static/items.js".to_owned(),
				],
				css_bundle_urls: vec![
					"/static/shared.css".to_owned(),
					"/static/items.css".to_owned(),
					"/static/entry.css".to_owned(),
				],
			},
		)]),
		search_schemas: BTreeMap::from([("/items/:id".to_owned(), serde_json::Value::Null)]),
		..manifest()
	}
}

fn write_view_manifest(dist_dir: &Path) {
	let manifest = manifest_with_view_assets();
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest).unwrap(),
	)
	.unwrap();
}

fn write_root_base_view_manifest(dist_dir: &Path) {
	let mut manifest = manifest_with_view_assets();
	manifest.public_static_base_path = "/".to_owned();
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest).unwrap(),
	)
	.unwrap();
}

fn write_nested_view_manifest(dist_dir: &Path) {
	let mut manifest = manifest_with_view_assets();
	manifest.client_routes.insert(
		"/items".to_owned(),
		ClientModule {
			url: "/static/items-parent.js".to_owned(),
			..ClientModule::default()
		},
	);
	manifest
		.search_schemas
		.insert("/items".to_owned(), serde_json::Value::Null);
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest).unwrap(),
	)
	.unwrap();
}

fn host(dist_dir: &Path) -> RuntimeHost<(), &'static str> {
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let task_middlewares = TaskMiddlewares::new();
	runtime_host(dist_dir, views, api_routes, task_middlewares)
}

fn task_middleware_host(dist_dir: &Path) -> RuntimeHost<(), &'static str> {
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-global-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		if ctx.request().method() != Method::GET {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-method-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		if ctx.matched_pattern() != "/ping" {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-pattern-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let ping = ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	);
	api_routes.push(ping);
	runtime_host(dist_dir, views, api_routes, task_middlewares)
}

fn static_json_api_route(
	ctx: crate::core::ErasedRequestCtx<(), &'static str>,
) -> crate::core::ErasedRouteFuture<&'static str> {
	crate::core::run_static_api_route::<(), &'static str, serde_json::Value, (), serde_json::Value>(
		ctx,
		static_json_api_handler,
	)
}

fn static_json_api_handler(
	_: crate::core::ApiCtx<(), &'static str, serde_json::Value>,
) -> crate::core::RouteFuture<serde_json::Value, &'static str> {
	Box::pin(async { panic!("bad JSON should fail before the API handler runs") })
}

fn static_form_data_api_route(
	ctx: crate::core::ErasedRequestCtx<(), &'static str>,
) -> crate::core::ErasedRouteFuture<&'static str> {
	crate::core::run_static_api_route::<(), &'static str, FormData, (), serde_json::Value>(
		ctx,
		static_form_data_api_handler,
	)
}

fn static_form_data_api_handler(
	ctx: crate::core::ApiCtx<(), &'static str, FormData>,
) -> crate::core::RouteFuture<serde_json::Value, &'static str> {
	Box::pin(async move {
		Ok(json!({
			"name": ctx.input().text("name"),
			"tags": ctx.input().texts("tag").collect::<Vec<_>>(),
			"content_type": ctx.input().content_type(),
		}))
	})
}

fn view_host(dist_dir: &Path) -> RuntimeHost<(), &'static str> {
	view_host_with_document(dist_dir, DocumentBuilder::default())
}

fn view_host_with_document(
	dist_dir: &Path,
	document: DocumentBuilder,
) -> RuntimeHost<(), &'static str> {
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			Ok(json!({
				"id": ctx.param("id"),
				"filter": ctx.request().query().unwrap_or_default(),
			}))
		},
	));
	runtime_host_with_document(
		dist_dir,
		views,
		ApiRoutes::new(),
		TaskMiddlewares::new(),
		document,
	)
}

fn custom_document() -> DocumentBuilder {
	DocumentBuilder::new(|ctx| async move {
		let favicon = ctx.public_url("favicon.ico")?;
		let mut document = crate::Document::new();
		document.html().data("shell", "custom");
		document.body().data("body", "custom");
		document.head().title("Default Title");
		document
			.head()
			.description("Default description from document");
		let head = document.head();
		head.link([head.rel("icon").into(), head.href(favicon).into()]);
		document.push_body_prefix(Element {
			tag: "div".to_owned(),
			attributes: BTreeMap::from([("data-document-prefix".to_owned(), "1".to_owned())]),
			text_content: "Document prefix".to_owned(),
			..Element::default()
		});
		Ok(document)
	})
}

#[tokio::test]
async fn runtime_host_serves_public_assets_from_manifest() {
	let dist_dir = temp_dist_dir("static");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/app.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/static/app.css")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.headers().get(CACHE_CONTROL).unwrap(),
		PUBLIC_ASSET_CACHE_CONTROL
	);
	assert_eq!(response.body(), &Bytes::from_static(b"body{}"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_serves_public_assets_from_percent_decoded_path() {
	let dist_dir = temp_dist_dir("static-percent-decoded");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/app.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/static/%61pp.css"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.body(), &Bytes::from_static(b"body{}"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_non_root_public_base_returns_404_for_unmanifested_assets() {
	let dist_dir = temp_dist_dir("static-unmanifested-non-root");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/missing.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/static/missing.css"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_root_public_base_falls_through_for_unmanifested_paths() {
	let dist_dir = temp_dist_dir("static-unmanifested-root");
	write_root_base_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["loaders_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_public_asset_preserves_content_length_without_body() {
	let dist_dir = temp_dist_dir("static-head");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/app.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::HEAD)
				.uri("/static/app.css")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "6");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_api_routes_under_api_base() {
	let dist_dir = temp_dist_dir("api");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/api/ping")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_api_routes_under_percent_decoded_api_base() {
	let dist_dir = temp_dist_dir("api-percent-decoded");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	for uri in ["/%61pi/ping", "/api%2Fping"] {
		let response = host
			.handle_request(empty_request(Method::GET, uri))
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	}
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_api_routes_under_manifest_api_mount_root() {
	let dist_dir = temp_dist_dir("api-manifest-mount-root");
	let manifest = write_manifest_with_api_mount_root(&dist_dir, "/server-api/");
	let mut config = cfg(&dist_dir);
	config.path_config.api_base = "/wrong-api/".to_owned();
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let task_middlewares = TaskMiddlewares::new();
	let host = RuntimeHost::new(default_app(config, views, api_routes, task_middlewares)).unwrap();

	let response = host
		.handle_request(empty_request(Method::GET, "/server-api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest.to_client_build_id().unwrap().as_str()
	);

	let response = host
		.handle_request(empty_request(Method::GET, "/wrong-api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_rejects_root_api_mount_root_from_manifest() {
	let dist_dir = temp_dist_dir("api-root-mount-rejected");
	write_manifest_with_api_mount_root(&dist_dir, "/");
	let views = Views::new();
	let api_routes = ApiRoutes::new();
	let task_middlewares = TaskMiddlewares::new();
	let error = RuntimeHost::new(default_app(
		cfg(&dist_dir),
		views,
		api_routes,
		task_middlewares,
	))
	.err()
	.unwrap();

	assert_eq!(
		error,
		"error initializing runtime assets: api_base must be a non-root path prefix such as /api/"
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_rejects_public_static_base_under_api_mount_root() {
	let dist_dir = temp_dist_dir("api-static-shadow-rejected");
	let mut manifest = manifest();
	manifest.public_static_base_path = "/api/assets/".to_owned();
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest).unwrap(),
	)
	.unwrap();

	let error = RuntimeHost::new(default_app(
		cfg(&dist_dir),
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	))
	.err()
	.unwrap();

	assert_eq!(
		error,
		"error initializing runtime assets: public_static_base \"/api/assets/\" must not be under api_base \"/api/\""
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_api_routes_under_normalized_api_base() {
	let dist_dir = temp_dist_dir("api-normalized");
	write_manifest(&dist_dir);
	let mut config = cfg(&dist_dir);
	config.path_config.api_base = "api".to_owned();
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let task_middlewares = TaskMiddlewares::new();
	let host = RuntimeHost::new(default_app(config, views, api_routes, task_middlewares)).unwrap();

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/api/ping")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_api_root_at_mount_root() {
	let dist_dir = temp_dist_dir("api-root");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"root": true})) },
	));
	let task_middlewares = TaskMiddlewares::new();
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	for uri in ["/api", "/api/"] {
		let response = host
			.handle_request(empty_request(Method::GET, uri))
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.body(), &Bytes::from_static(br#"{"root":true}"#));
	}
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_api_preserves_content_length_without_body() {
	let dist_dir = temp_dist_dir("api-head");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::HEAD)
				.uri("/api/ping")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(response.headers().contains_key(CONTENT_LENGTH));
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_public_url_reads_manifest() {
	let dist_dir = temp_dist_dir("public-url");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	assert_eq!(host.public_url(" /app.css ").unwrap(), "/static/app.css");
	assert!(host.public_url("missing.css").is_err());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_client_build_id_and_critical_css_hash_use_manifest() {
	let dist_dir = temp_dist_dir("manifest-helpers");
	write_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	assert_eq!(
		host.client_build_id().unwrap(),
		manifest_with_view_assets().to_client_build_id().unwrap()
	);
	assert_eq!(
		host.critical_css_content_sha256().unwrap(),
		manifest_with_view_assets().critical_css_content_sha256()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_init_requires_manifest() {
	let dist_dir = temp_dist_dir("missing-manifest");
	let error = match RuntimeHost::new(default_app(
		cfg(&dist_dir),
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject missing manifest"),
		Err(error) => error,
	};

	assert!(error.contains("error initializing runtime assets"));
	assert!(error.contains("vorma.manifest.prod.json"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_init_rejects_empty_root_dir() {
	let error = match RuntimeHost::new(default_app(
		Config::default(),
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject empty root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "root_dir cannot be empty");
}

#[test]
fn runtime_host_init_rejects_relative_root_dir() {
	let mut config = Config {
		root_dir: PathBuf::from("."),
		dist_dir: ".".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject relative root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "root_dir must be absolute");
}

#[test]
fn runtime_host_init_rejects_missing_root_dir() {
	let nanos = SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.unwrap()
		.as_nanos();
	let root_dir = std::env::temp_dir().join(format!(
		"vorma-runtime-host-missing-root-{}-{nanos}",
		std::process::id(),
	));
	let mut config = Config {
		root_dir: root_dir.clone(),
		dist_dir: ".".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject missing root_dir"),
		Err(error) => error,
	};

	assert_eq!(
		error,
		format!("root dir does not exist: {}", root_dir.display())
	);
}

#[test]
fn runtime_host_init_rejects_file_root_dir() {
	let dist_dir = temp_dist_dir("file-root-dir");
	let root_file = dist_dir.join("root-file");
	fs::write(&root_file, b"not a directory").unwrap();
	let mut config = Config {
		root_dir: root_file.clone(),
		dist_dir: ".".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject file root_dir"),
		Err(error) => error,
	};

	assert_eq!(
		error,
		format!("root dir is not a directory: {}", root_file.display())
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_init_rejects_relative_dist_dir_escape() {
	let root_dir = temp_dist_dir("relative-dist-escape");
	let mut config = Config {
		root_dir: root_dir.clone(),
		dist_dir: "../outside".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject dist_dir outside root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "dist_dir must be inside root_dir");
	fs::remove_dir_all(root_dir).unwrap();
}

#[test]
fn runtime_host_init_rejects_absolute_dist_dir_outside_root() {
	let root_dir = temp_dist_dir("absolute-dist-outside-root");
	let outside = temp_dist_dir("absolute-dist-outside-dist");
	let mut config = Config {
		root_dir: root_dir.clone(),
		dist_dir: outside.to_string_lossy().into_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject absolute dist_dir outside root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "dist_dir must be inside root_dir");
	fs::remove_dir_all(root_dir).unwrap();
	fs::remove_dir_all(outside).unwrap();
}

#[test]
fn runtime_host_init_rejects_invalid_manifest() {
	let dist_dir = temp_dist_dir("invalid-manifest");
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		b"not json",
	)
	.unwrap();

	let error = match RuntimeHost::new(default_app(
		cfg(&dist_dir),
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject invalid manifest"),
		Err(error) => error,
	};

	assert!(error.contains("error initializing runtime assets"));
	assert!(error.contains("error parsing prod manifest"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_runs_api_task_middleware_scopes() {
	let dist_dir = temp_dist_dir("api-task-middleware");
	write_manifest(&dist_dir);
	let host = task_middleware_host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/api/ping")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.headers().get("x-vorma-global-mw").unwrap(),
		HeaderValue::from_static("1")
	);
	assert_eq!(
		response.headers().get("x-vorma-method-mw").unwrap(),
		HeaderValue::from_static("1")
	);
	assert_eq!(
		response.headers().get("x-vorma-pattern-mw").unwrap(),
		HeaderValue::from_static("1")
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_preserves_handler_owned_error_proxy_once() {
	let dist_dir = temp_dist_dir("api-error-proxy");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::POST,
		"/drifted",
		Some(ApiRouteKind::Mutation),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ApiCtx<(), &'static str, ()>| async move {
			ctx.response()
				.set_error_status(StatusCode::CONFLICT, "drifted");
			ctx.response()
				.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));
			Err::<serde_json::Value, _>(vorma_tasks::Error::from("drifted"))
		},
	));
	let host = runtime_host(&dist_dir, views, api_routes, TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::POST, "/api/drifted"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::CONFLICT);
	assert_eq!(response.headers().get(CACHE_CONTROL).unwrap(), "no-store");
	assert_eq!(response.headers().get_all(CACHE_CONTROL).iter().count(), 1);
	assert_eq!(response.body(), &Bytes::from_static(b"drifted\n"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_response_handle_supports_redirect_status_codes() {
	let dist_dir = temp_dist_dir("api-custom-redirect-status");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::POST,
		"/moved",
		Some(ApiRouteKind::Mutation),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ApiCtx<(), &'static str, ()>| async move {
			ctx.response()
				.redirect_with_status("/new-home", StatusCode::MOVED_PERMANENTLY)
				.unwrap();
			Ok(json!({"unreachable": true}))
		},
	));
	let host = runtime_host(&dist_dir, views, api_routes, TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::POST, "/api/moved"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::MOVED_PERMANENTLY);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/new-home");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_suppresses_success_proxy_when_handler_errors() {
	let dist_dir = temp_dist_dir("api-error-suppresses-success-proxy");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::CREATED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		ctx.response()
			.set_cookie(cookie::Cookie::new("middleware", "1"));
		Ok(())
	}));
	api_routes.push(ApiRoute::new(
		Method::POST,
		"/boom",
		Some(ApiRouteKind::Mutation),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ApiCtx<(), &'static str, ()>| async move {
			ctx.response().set_status(StatusCode::ACCEPTED);
			ctx.response().set_header(
				HeaderName::from_static("x-success"),
				HeaderValue::from_static("should-not-leak"),
			);
			ctx.response()
				.set_cookie(cookie::Cookie::new("success", "should-not-leak"));
			Err::<serde_json::Value, _>(vorma_tasks::Error::from("boom"))
		},
	));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::POST, "/api/boom"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert!(!response.headers().contains_key("x-success"));
	assert_eq!(response.headers().get_all(SET_COOKIE).iter().count(), 1);
	assert_eq!(
		response.body(),
		&Bytes::from_static(b"Internal Server Error\n")
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_middleware_success_status_does_not_short_circuit_handler() {
	let dist_dir = temp_dist_dir("api-middleware-success-status");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::ACCEPTED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/success",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ApiCtx<(), &'static str, ()>| async move {
			ctx.response().set_status(StatusCode::CREATED);
			Ok(json!({"ok": true}))
		},
	));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/api/success"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::CREATED);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.body(), &Bytes::from_static(br#"{"ok":true}"#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_prior_middleware_terminal_suppresses_finished_later_effects() {
	let dist_dir = temp_dist_dir("api-middleware-terminal-suppresses-later");
	write_manifest(&dist_dir);
	let later_finished = Arc::new(Notify::new());
	let later_finished_for_first = later_finished.clone();
	let later_finished_for_later = later_finished.clone();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		move |_: crate::core::ApiCtx<(), &'static str, ()>| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"pong": true}))
			}
		},
	));
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(move |ctx| {
		let later_finished = later_finished_for_first.clone();
		async move {
			later_finished.notified().await;
			ctx.response()
				.set_error_status(StatusCode::FORBIDDEN, "denied");
			ctx.response().set_header(
				HeaderName::from_static("x-first"),
				HeaderValue::from_static("1"),
			);
			Ok(())
		}
	}));
	task_middlewares.push(TaskMiddleware::new(move |ctx| {
		let later_finished = later_finished_for_later.clone();
		async move {
			ctx.response().set_header(
				HeaderName::from_static("x-later"),
				HeaderValue::from_static("should-not-leak"),
			);
			later_finished.notify_one();
			Ok(())
		}
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::FORBIDDEN);
	assert_eq!(response.headers().get("x-first").unwrap(), "1");
	assert!(!response.headers().contains_key("x-later"));
	assert_eq!(response.body(), &Bytes::from_static(b"denied\n"));
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_later_middleware_redirect_preserves_prior_success_effects() {
	let dist_dir = temp_dist_dir("api-later-middleware-redirect-preserves-prior");
	write_manifest(&dist_dir);
	let prior_started = Arc::new(Notify::new());
	let redirect_done = Arc::new(Notify::new());
	let prior_started_for_prior = prior_started.clone();
	let prior_started_for_redirect = prior_started.clone();
	let redirect_done_for_prior = redirect_done.clone();
	let redirect_done_for_redirect = redirect_done.clone();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		move |_: crate::core::ApiCtx<(), &'static str, ()>| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"pong": true}))
			}
		},
	));
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(move |ctx| {
		let prior_started = prior_started_for_prior.clone();
		let redirect_done = redirect_done_for_prior.clone();
		async move {
			prior_started.notify_one();
			redirect_done.notified().await;
			ctx.response().set_header(
				HeaderName::from_static("x-prior"),
				HeaderValue::from_static("kept"),
			);
			ctx.response()
				.set_cookie(cookie::Cookie::new("prior", "kept"));
			Ok(())
		}
	}));
	task_middlewares.push(TaskMiddleware::new(move |ctx| {
		let prior_started = prior_started_for_redirect.clone();
		let redirect_done = redirect_done_for_redirect.clone();
		async move {
			prior_started.notified().await;
			ctx.response().redirect("/login").unwrap();
			redirect_done.notify_one();
			Ok(())
		}
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = tokio::time::timeout(
		Duration::from_millis(250),
		host.handle_request(empty_request(Method::GET, "/api/ping")),
	)
	.await
	.expect("API response should not hang")
	.unwrap();

	assert_eq!(response.status(), StatusCode::SEE_OTHER);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/login");
	assert_eq!(response.headers().get("x-prior").unwrap(), "kept");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "prior=kept");
	assert!(response.body().is_empty());
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_internal_middleware_join_error_preserves_client_build_id() {
	let dist_dir = temp_dist_dir("api-middleware-join-error");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|_| async move {
		panic!("middleware panic");
		#[allow(unreachable_code)]
		Ok::<(), vorma_tasks::Error<&'static str>>(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest().to_client_build_id().unwrap().as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_bad_json_is_bad_request_and_skips_handler() {
	let dist_dir = temp_dist_dir("api-bad-json");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::from_static(
		Method::POST,
		"/json",
		Some(ApiRouteKind::Mutation),
		crate::core::type_resolver::<serde_json::Value>,
		crate::core::type_resolver::<serde_json::Value>,
		static_json_api_route,
	));
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::CREATED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		ctx.response()
			.set_cookie(cookie::Cookie::new("middleware", "1"));
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::POST)
				.uri("/api/json")
				.header(CONTENT_TYPE, "application/json")
				.body(Bytes::from_static(b"{"))
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert!(
		std::str::from_utf8(response.body())
			.unwrap()
			.contains("error decoding JSON")
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_parses_form_data_urlencoded_body() {
	let dist_dir = temp_dist_dir("api-form-data");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::from_static(
		Method::POST,
		"/form",
		Some(ApiRouteKind::Mutation),
		crate::core::type_resolver::<FormData>,
		crate::core::type_resolver::<serde_json::Value>,
		static_form_data_api_route,
	));
	let host = runtime_host(&dist_dir, views, api_routes, TaskMiddlewares::new());

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::POST)
				.uri("/api/form")
				.header(CONTENT_TYPE, "application/x-www-form-urlencoded")
				.body(Bytes::from_static(b"tag=a&tag=b&name=jeff"))
				.unwrap(),
		)
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["name"], "jeff");
	assert_eq!(payload["tags"], json!(["a", "b"]));
	assert_eq!(payload["content_type"], "application/x-www-form-urlencoded");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_runs_view_task_middleware_scopes() {
	let dist_dir = temp_dist_dir("view-task-middleware");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			Ok(json!({"id": ctx.param("id")}))
		},
	));
	let api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-global-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		if ctx.request().method() != Method::GET {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-method-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		if ctx.matched_pattern() != "/items/:id" {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-pattern-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.headers().get("x-vorma-global-mw").unwrap(),
		HeaderValue::from_static("1")
	);
	assert_eq!(
		response.headers().get("x-vorma-method-mw").unwrap(),
		HeaderValue::from_static("1")
	);
	assert_eq!(
		response.headers().get("x-vorma-pattern-mw").unwrap(),
		HeaderValue::from_static("1")
	);
	assert_eq!(payload["loaders_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_middleware_success_status_does_not_short_circuit_loader() {
	let dist_dir = temp_dist_dir("view-middleware-success-status");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			ctx.response().set_status(StatusCode::CREATED);
			Ok(json!({"id": ctx.param("id")}))
		},
	));
	let api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::ACCEPTED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::CREATED);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(payload["loaders_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_task_middleware_head_merges_before_view_head() {
	let dist_dir = temp_dist_dir("view-task-middleware-head");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			ctx.head().title("View Title");
			ctx.head().description("View description");
			Ok(json!({"id": ctx.param("id")}))
		},
	));
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.head().title("Middleware Title");
		ctx.head().description("Middleware description");
		ctx.head().meta_property_content("og:type", "article");
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["title"]["dangerous_inner_html"], "View Title");
	assert_eq!(
		payload["meta_head_els"][0]["attributes_known_safe"]["content"],
		"View description"
	);
	assert_eq!(
		payload["meta_head_els"][1]["attributes_known_safe"]["content"],
		"article"
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_task_middleware_short_circuits_loaders() {
	let dist_dir = temp_dist_dir("view-task-middleware-short-circuit");
	write_view_manifest(&dist_dir);
	let loader_runs = Arc::new(AtomicUsize::new(0));
	let loader_runs_for_view = loader_runs.clone();
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let loader_runs = loader_runs_for_view.clone();
			async move {
				loader_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"id": ctx.param("id")}))
			}
		},
	));
	let api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		if ctx.matched_pattern() != "/items/:id" {
			return Ok(());
		}
		ctx.response().set_status(StatusCode::UNAUTHORIZED);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::UNAUTHORIZED);
	assert_eq!(loader_runs.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_bad_input_preserves_prior_middleware_effects() {
	let dist_dir = temp_dist_dir("view-bad-input-middleware");
	write_view_manifest(&dist_dir);
	let loader_runs = Arc::new(AtomicUsize::new(0));
	let loader_runs_for_view = loader_runs.clone();
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::callback(|_| Err(InputError::bad_request("bad input"))),
		move |_: crate::core::ViewCtx<(), &'static str, ()>| {
			let loader_runs = loader_runs_for_view.clone();
			async move {
				loader_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"unused": true}))
			}
		},
	));
	let api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::CREATED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		ctx.response()
			.set_cookie(cookie::Cookie::new("middleware", "1"));
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
	assert_eq!(loader_runs.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_suppresses_success_proxy_when_handler_errors() {
	let dist_dir = temp_dist_dir("view-error-suppresses-success-proxy");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			ctx.response().set_status(StatusCode::CREATED);
			ctx.response().set_header(
				HeaderName::from_static("x-view-success"),
				HeaderValue::from_static("should-not-leak"),
			);
			Err::<serde_json::Value, _>(vorma_tasks::Error::from("boom"))
		},
	));
	let api_routes = ApiRoutes::new();
	let mut task_middlewares = TaskMiddlewares::new();
	task_middlewares.push(TaskMiddleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::ACCEPTED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, api_routes, task_middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert!(!response.headers().contains_key("x-view-success"));
	assert_eq!(
		payload["outermost_server_err"],
		"An unexpected error occurred."
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_ancestor_terminal_effect_suppresses_finished_child_effects() {
	let dist_dir = temp_dist_dir("view-ancestor-terminal-effect");
	write_nested_view_manifest(&dist_dir);
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();
	let mut views = Views::new();
	views.push(View::new(
		"/items",
		"items-parent.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response()
					.set_error_status(StatusCode::FORBIDDEN, "denied");
				ctx.response().set_header(
					HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			}
		},
	));
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let child_started = child_started_for_child.clone();
			async move {
				ctx.response().set_header(
					HeaderName::from_static("x-child"),
					HeaderValue::from_static("leaked"),
				);
				child_started.notify_one();
				Ok(json!({"child": true}))
			}
		},
	));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::FORBIDDEN);
	assert_eq!(response.headers().get("x-parent").unwrap(), "1");
	assert!(response.headers().get("x-child").is_none());
	assert_eq!(response.body(), &Bytes::from_static(b"denied\n"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_child_redirect_preserves_prior_success_effects() {
	let dist_dir = temp_dist_dir("view-child-redirect-preserves-prior");
	write_nested_view_manifest(&dist_dir);
	let parent_started = Arc::new(Notify::new());
	let child_redirected = Arc::new(Notify::new());
	let parent_started_for_parent = parent_started.clone();
	let parent_started_for_child = parent_started.clone();
	let child_redirected_for_parent = child_redirected.clone();
	let child_redirected_for_child = child_redirected.clone();
	let mut views = Views::new();
	views.push(View::new(
		"/items",
		"items-parent.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let parent_started = parent_started_for_parent.clone();
			let child_redirected = child_redirected_for_parent.clone();
			async move {
				parent_started.notify_one();
				child_redirected.notified().await;
				ctx.response().set_header(
					HeaderName::from_static("x-parent"),
					HeaderValue::from_static("kept"),
				);
				ctx.response()
					.set_cookie(cookie::Cookie::new("parent", "kept"));
				Ok(json!({"parent": true}))
			}
		},
	));
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let parent_started = parent_started_for_child.clone();
			let child_redirected = child_redirected_for_child.clone();
			async move {
				parent_started.notified().await;
				ctx.response().redirect("/child").unwrap();
				child_redirected.notify_one();
				Ok(json!({"child": true}))
			}
		},
	));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), TaskMiddlewares::new());

	let response = tokio::time::timeout(
		Duration::from_millis(250),
		host.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_")),
	)
	.await
	.expect("view response should not hang")
	.unwrap();

	assert_eq!(response.status(), StatusCode::SEE_OTHER);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/child");
	assert_eq!(response.headers().get("x-parent").unwrap(), "kept");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "parent=kept");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_response_handle_supports_redirect_status_codes() {
	let dist_dir = temp_dist_dir("view-custom-redirect-status");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			ctx.response()
				.redirect_with_status("/items/next", StatusCode::TEMPORARY_REDIRECT)
				.unwrap();
			Ok(json!({"unreachable": true}))
		},
	));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::TEMPORARY_REDIRECT);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/items/next");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_is_a_tower_service() {
	let dist_dir = temp_dist_dir("tower-service");
	write_manifest(&dist_dir);
	let mut host = host(&dist_dir);

	let response = tower_service::Service::call(
		&mut host,
		Request::builder()
			.method(Method::GET)
			.uri("/api/ping")
			.body(Full::new(Bytes::new()))
			.unwrap(),
	)
	.await
	.unwrap();
	let body = response.into_body().collect().await.unwrap().to_bytes();

	assert_eq!(body, Bytes::from_static(br#"{"pong":true}"#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_enforces_request_body_limit_in_tower_service() {
	let dist_dir = temp_dist_dir("body-limit");
	write_manifest(&dist_dir);
	let views = Views::new();
	let api_routes = ApiRoutes::new();
	let task_middlewares = TaskMiddlewares::new();
	let mut app = default_app(cfg(&dist_dir), views, api_routes, task_middlewares);
	app.request_body_limit = 4;
	let mut host = RuntimeHost::new(app).unwrap();

	let response = tower_service::Service::call(
		&mut host,
		Request::builder()
			.method(Method::POST)
			.uri("/api/ping")
			.body(Full::new(Bytes::from_static(b"12345")))
			.unwrap(),
	)
	.await
	.unwrap();
	let status = response.status();
	let body = response.into_body().collect().await.unwrap().to_bytes();

	assert_eq!(status, StatusCode::PAYLOAD_TOO_LARGE);
	assert_eq!(body, Bytes::from_static(b"Payload Too Large\n"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_body_limit_has_no_body_in_tower_service() {
	let dist_dir = temp_dist_dir("head-body-limit-service");
	write_manifest(&dist_dir);
	let mut app = default_app(
		cfg(&dist_dir),
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	);
	app.request_body_limit = 4;
	let mut host = RuntimeHost::new(app).unwrap();

	let response = tower_service::Service::call(
		&mut host,
		Request::builder()
			.method(Method::HEAD)
			.uri("/api/ping")
			.body(Full::new(Bytes::from_static(b"12345")))
			.unwrap(),
	)
	.await
	.unwrap();
	let status = response.status();
	let content_length = response.headers().get(CONTENT_LENGTH).cloned();
	let body = response.into_body().collect().await.unwrap().to_bytes();

	assert_eq!(status, StatusCode::PAYLOAD_TOO_LARGE);
	assert_eq!(content_length.unwrap(), "18");
	assert!(body.is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_enforces_request_body_limit_in_direct_handler() {
	let dist_dir = temp_dist_dir("direct-body-limit");
	write_manifest(&dist_dir);
	let views = Views::new();
	let api_routes = ApiRoutes::new();
	let task_middlewares = TaskMiddlewares::new();
	let mut app = default_app(cfg(&dist_dir), views, api_routes, task_middlewares);
	app.request_body_limit = 4;
	let host = RuntimeHost::new(app).unwrap();

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::POST)
				.uri("/api/ping")
				.body(Bytes::from_static(b"12345"))
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::PAYLOAD_TOO_LARGE);
	assert_eq!(response.body(), &Bytes::from_static(b"Payload Too Large\n"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_body_limit_has_no_body_in_direct_handler() {
	let dist_dir = temp_dist_dir("head-body-limit-direct");
	write_manifest(&dist_dir);
	let mut app = default_app(
		cfg(&dist_dir),
		Views::new(),
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	);
	app.request_body_limit = 4;
	let host = RuntimeHost::new(app).unwrap();

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::HEAD)
				.uri("/api/ping")
				.body(Bytes::from_static(b"12345"))
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::PAYLOAD_TOO_LARGE);
	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "18");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_serves_view_json_payload() {
	let dist_dir = temp_dist_dir("view-json");
	write_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42?filter=active&vorma-json=_")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["matched_patterns"][0], "/items/:id");
	assert_eq!(payload["params"]["id"], "42");
	assert_eq!(payload["import_urls"][0], "/static/items.js");
	assert_eq!(
		payload["deps"],
		json!(["/static/entry.js", "/static/shared.js", "/static/items.js"])
	);
	assert_eq!(
		payload["css_bundles"],
		json!([
			"/static/entry.css",
			"/static/shared.css",
			"/static/items.css"
		])
	);
	assert_eq!(payload.get("rest_head_els"), None);
	assert_eq!(payload["loaders_data"][0]["id"], "42");
	assert_eq!(
		payload["loaders_data"][0]["filter"],
		"filter=active&vorma-json=_"
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_response_and_head_handles_do_not_deadlock_when_both_are_held() {
	let dist_dir = temp_dist_dir("view-response-head-handles");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			let mut response = ctx.response();
			response.set_header(
				HeaderName::from_static("x-before-head"),
				HeaderValue::from_static("1"),
			);
			ctx.head().title("Both Handles");
			response.set_header(
				HeaderName::from_static("x-after-head"),
				HeaderValue::from_static("1"),
			);
			Ok(json!({"id": ctx.param("id")}))
		},
	));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), TaskMiddlewares::new());

	let response = tokio::time::timeout(
		Duration::from_millis(200),
		host.handle_request(empty_request(Method::GET, "/items/42")),
	)
	.await
	.expect("response/head handle locking should not hang")
	.unwrap();
	let body = String::from_utf8(response.body().to_vec()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.headers().get("x-before-head").unwrap(), "1");
	assert_eq!(response.headers().get("x-after-head").unwrap(), "1");
	assert!(body.contains("<title>Both Handles</title>"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[derive(Clone)]
struct RequestMarker(&'static str);

#[tokio::test]
async fn runtime_host_preserves_request_extensions_for_handlers() {
	let dist_dir = temp_dist_dir("request-extensions");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			let marker = ctx
				.request()
				.extension::<RequestMarker>()
				.map(|marker| marker.0)
				.unwrap_or("");
			Ok(json!({"marker": marker}))
		},
	));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), TaskMiddlewares::new());

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42?vorma-json=_")
				.extension(RequestMarker("from-extension"))
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["loaders_data"][0]["marker"], "from-extension");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_cancels_view_exec_ctx_when_request_future_is_dropped() {
	let dist_dir = temp_dist_dir("request-cancellation");
	write_view_manifest(&dist_dir);
	let (token_tx, token_rx) = tokio::sync::oneshot::channel::<CancelToken>();
	let token_tx = Arc::new(Mutex::new(Some(token_tx)));
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let token_tx = token_tx.clone();
			async move {
				if let Some(token_tx) = token_tx.lock().unwrap().take() {
					let _ = token_tx.send(ctx.exec_ctx().cancel_token().clone());
				}
				std::future::pending::<vorma_tasks::Result<serde_json::Value, &'static str>>().await
			}
		},
	));
	let host = RuntimeHost::new(default_app(
		cfg(&dist_dir),
		views,
		ApiRoutes::new(),
		TaskMiddlewares::new(),
	))
	.unwrap();
	let host_for_request = host.clone();
	let request = Request::builder()
		.method(Method::GET)
		.uri("/items/42?vorma-json=_")
		.body(Bytes::new())
		.unwrap();
	let handle = tokio::spawn(async move { host_for_request.handle_request(request).await });
	let token = tokio::time::timeout(Duration::from_secs(1), token_rx)
		.await
		.unwrap()
		.unwrap();

	handle.abort();
	let _ = handle.await;
	tokio::time::timeout(Duration::from_secs(1), token.cancelled())
		.await
		.unwrap();
	assert!(token.is_cancelled());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_cancels_api_exec_ctx_when_request_future_is_dropped() {
	let dist_dir = temp_dist_dir("api-request-cancellation");
	write_manifest(&dist_dir);
	let (token_tx, token_rx) = tokio::sync::oneshot::channel::<CancelToken>();
	let token_tx = Arc::new(Mutex::new(Some(token_tx)));
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/pending",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ApiCtx<(), &'static str, ()>| {
			let token_tx = token_tx.clone();
			async move {
				if let Some(token_tx) = token_tx.lock().unwrap().take() {
					let _ = token_tx.send(ctx.exec_ctx().cancel_token().clone());
				}
				std::future::pending::<vorma_tasks::Result<serde_json::Value, &'static str>>().await
			}
		},
	));
	let host = runtime_host(&dist_dir, views, api_routes, TaskMiddlewares::new());
	let host_for_request = host.clone();
	let request = Request::builder()
		.method(Method::GET)
		.uri("/api/pending")
		.body(Bytes::new())
		.unwrap();
	let handle = tokio::spawn(async move { host_for_request.handle_request(request).await });
	let token = tokio::time::timeout(Duration::from_secs(1), token_rx)
		.await
		.unwrap()
		.unwrap();

	handle.abort();
	let _ = handle.await;
	tokio::time::timeout(Duration::from_secs(1), token.cancelled())
		.await
		.unwrap();
	assert!(token.is_cancelled());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_context_resolves_public_urls() {
	let dist_dir = temp_dist_dir("view-public-url");
	write_view_manifest(&dist_dir);
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		|ctx: crate::core::ViewCtx<(), &'static str, ()>| async move {
			Ok(json!({"app_css": ctx.public_url("app.css").unwrap()}))
		},
	));
	let host = runtime_host(&dist_dir, views, ApiRoutes::new(), TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["loaders_data"][0]["app_css"], "/static/app.css");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_context_resolves_public_urls() {
	let dist_dir = temp_dist_dir("api-public-url");
	write_manifest(&dist_dir);
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/asset",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ApiCtx<(), &'static str, ()>| async move {
			Ok(json!({"app_css": ctx.public_url("app.css").unwrap()}))
		},
	));
	let host = runtime_host(&dist_dir, Views::new(), api_routes, TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::GET, "/api/asset"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.body(),
		&Bytes::from_static(br#"{"app_css":"/static/app.css"}"#)
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_document_defaults_feed_loader_payload() {
	let dist_dir = temp_dist_dir("view-document-json");
	write_view_manifest(&dist_dir);
	let host = view_host_with_document(&dist_dir, custom_document());

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42?vorma-json=_")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["title"]["dangerous_inner_html"], "Default Title");
	assert_eq!(
		payload["meta_head_els"][0]["attributes_known_safe"]["content"],
		"Default description from document"
	);
	assert_eq!(
		payload["rest_head_els"][0]["attributes_known_safe"]["href"],
		"/static/favicon.ico"
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_skew_bypasses_document_builder() {
	let dist_dir = temp_dist_dir("view-skew-document-bypass");
	write_view_manifest(&dist_dir);
	let calls = Arc::new(AtomicUsize::new(0));
	let calls_for_builder = calls.clone();
	let host = view_host_with_document(
		&dist_dir,
		DocumentBuilder::new(move |_| {
			let calls = calls_for_builder.clone();
			async move {
				calls.fetch_add(1, Ordering::SeqCst);
				Ok(crate::document::Document::new())
			}
		}),
	);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42?vorma-json=stale")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response
			.headers()
			.get(crate::constants::X_VORMA_BUILD_SKEW)
			.unwrap(),
		"1"
	);
	assert_eq!(calls.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_missing_view_does_not_build_document() {
	let dist_dir = temp_dist_dir("view-missing-document-bypass");
	write_view_manifest(&dist_dir);
	let calls = Arc::new(AtomicUsize::new(0));
	let calls_for_builder = calls.clone();
	let host = view_host_with_document(
		&dist_dir,
		DocumentBuilder::new(move |_| {
			let calls = calls_for_builder.clone();
			async move {
				calls.fetch_add(1, Ordering::SeqCst);
				Err("document builder should not run for missing views".to_owned())
			}
		}),
	);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/missing")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	assert_eq!(calls.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_builds_document_and_view_loader_concurrently() {
	let dist_dir = temp_dist_dir("view-document-loader-parallel");
	write_view_manifest(&dist_dir);
	let document_started = Arc::new(Notify::new());
	let loader_started = Arc::new(Notify::new());
	let document_started_for_builder = document_started.clone();
	let loader_started_for_builder = loader_started.clone();
	let mut views = Views::new();
	let document_started_for_loader = document_started.clone();
	let loader_started_for_loader = loader_started.clone();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let document_started = document_started_for_loader.clone();
			let loader_started = loader_started_for_loader.clone();
			async move {
				loader_started.notify_one();
				document_started.notified().await;
				Ok(json!({"id": ctx.param("id")}))
			}
		},
	));
	let host = runtime_host_with_document(
		&dist_dir,
		views,
		ApiRoutes::new(),
		TaskMiddlewares::new(),
		DocumentBuilder::new(move |_| {
			let document_started = document_started_for_builder.clone();
			let loader_started = loader_started_for_builder.clone();
			async move {
				document_started.notify_one();
				loader_started.notified().await;
				Ok(crate::document::Document::new())
			}
		}),
	);

	let response = tokio::time::timeout(
		Duration::from_secs(1),
		host.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_")),
	)
	.await
	.expect("document and loader should not deadlock")
	.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["loaders_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_serves_view_html_shell() {
	let dist_dir = temp_dist_dir("view-html");
	write_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42?filter=active")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();
	let body = String::from_utf8(response.body().to_vec()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(body.contains(r#"id="vorma-data-json""#));
	assert!(body.contains(r#"<div id="vorma-root"></div>"#));
	assert!(body.contains(r#"<script src="/static/entry.js" type="module"></script>"#));
	assert_substrings_in_order(
		&body,
		&[
			r#"data-vorma-css-bundle="/static/entry.css""#,
			r#"data-vorma-css-bundle="/static/shared.css""#,
			r#"data-vorma-css-bundle="/static/items.css""#,
		],
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_document_shell_renders_attrs_and_body_prefix() {
	let dist_dir = temp_dist_dir("view-document-html");
	write_view_manifest(&dist_dir);
	let host = view_host_with_document(&dist_dir, custom_document());

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();
	let body = String::from_utf8(response.body().to_vec()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(body.contains(r#"<html lang="en" data-shell="custom">"#));
	assert!(body.contains(r#"<body data-body="custom">"#));
	assert_substrings_in_order(
		&body,
		&[
			r#"<div data-document-prefix="1">Document prefix</div>"#,
			r#"id="vorma-data-json""#,
			r#"<div id="vorma-root"></div>"#,
		],
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_document_builder_receives_real_request() {
	let dist_dir = temp_dist_dir("view-document-request");
	write_view_manifest(&dist_dir);
	let host = view_host_with_document(
		&dist_dir,
		DocumentBuilder::new(|ctx| async move {
			let mut document = crate::document::Document::new();
			document
				.head()
				.meta_property_content("og:url", ctx.request().path());
			Ok(document)
		}),
	);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/items/42")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();
	let body = String::from_utf8(response.body().to_vec()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(body.contains(r#"content="/items/42" property="og:url""#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_document_error_preserves_client_build_id() {
	let dist_dir = temp_dist_dir("view-document-error");
	write_view_manifest(&dist_dir);
	let host = view_host_with_document(
		&dist_dir,
		DocumentBuilder::new(|_| async move { Err("document failed".to_owned()) }),
	);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest_with_view_assets()
			.to_client_build_id()
			.unwrap()
			.as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_document_error_cancels_running_loader() {
	let dist_dir = temp_dist_dir("view-document-error-cancels-loader");
	write_view_manifest(&dist_dir);
	let loader_started = Arc::new(Notify::new());
	let (cancel_tx, cancel_rx) = oneshot::channel();
	let cancel_tx = Arc::new(Mutex::new(Some(cancel_tx)));
	let mut views = Views::new();
	let loader_started_for_loader = loader_started.clone();
	let cancel_tx_for_loader = cancel_tx.clone();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |_ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let loader_started = loader_started_for_loader.clone();
			let cancel_tx = cancel_tx_for_loader.clone();
			async move {
				let _drop_signal = DropSignal(cancel_tx);
				loader_started.notify_one();
				std::future::pending::<vorma_tasks::Result<serde_json::Value, &'static str>>().await
			}
		},
	));
	let loader_started_for_document = loader_started.clone();
	let host = runtime_host_with_document(
		&dist_dir,
		views,
		ApiRoutes::new(),
		TaskMiddlewares::new(),
		DocumentBuilder::new(move |_| {
			let loader_started = loader_started_for_document.clone();
			async move {
				loader_started.notified().await;
				Err("document failed".to_owned())
			}
		}),
	);

	let response = tokio::time::timeout(
		Duration::from_secs(1),
		host.handle_request(empty_request(Method::GET, "/items/42")),
	)
	.await
	.expect("document error should not hang on loader")
	.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	tokio::time::timeout(Duration::from_secs(1), cancel_rx)
		.await
		.expect("document failure should cancel running loader")
		.expect("loader should report cancellation");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_manifest_mismatch_preserves_client_build_id() {
	let dist_dir = temp_dist_dir("view-manifest-mismatch");
	write_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest().to_client_build_id().unwrap().as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_view_preserves_content_length_without_body() {
	let dist_dir = temp_dist_dir("view-head");
	write_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::HEAD)
				.uri("/items/42")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(response.headers().contains_key(CONTENT_LENGTH));
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_reports_method_not_allowed_for_known_api_path() {
	let dist_dir = temp_dist_dir("allow");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::POST)
				.uri("/api/ping")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "GET, HEAD");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_reports_method_not_allowed_for_unregistered_api_method() {
	let dist_dir = temp_dist_dir("global-allow");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::DELETE)
				.uri("/api/missing")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "GET, HEAD");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_method_not_allowed_has_no_body() {
	let dist_dir = temp_dist_dir("head-method-not-allowed");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::POST,
		"/submit",
		Some(ApiRouteKind::Mutation),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"ok": true})) },
	));
	let host = runtime_host(&dist_dir, views, api_routes, TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::HEAD, "/api/submit"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "POST");
	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "19");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_reports_not_found_for_unknown_api_path_with_registered_method() {
	let dist_dir = temp_dist_dir("api-method-known-path-missing");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/api/missing")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest().to_client_build_id().unwrap().as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_supported_api_method_405_includes_client_build_id() {
	let dist_dir = temp_dist_dir("api-method-known-path-method-missing");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut api_routes = ApiRoutes::new();
	api_routes.push(ApiRoute::new(
		Method::GET,
		"/ping",
		Some(ApiRouteKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	api_routes.push(ApiRoute::new(
		Method::POST,
		"/other",
		Some(ApiRouteKind::Mutation),
		InputParser::<()>::default_input(),
		|_: crate::core::ApiCtx<(), &'static str, ()>| async { Ok(json!({"ok": true})) },
	));
	let host = runtime_host(&dist_dir, views, api_routes, TaskMiddlewares::new());

	let response = host
		.handle_request(empty_request(Method::POST, "/api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "GET, HEAD");
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest().to_client_build_id().unwrap().as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn head_response_preserves_content_length() {
	let response = Response::new(Bytes::from_static(b"hello"));
	let response = head_response_without_body(response);

	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "5");
	assert!(response.body().is_empty());
}

#[test]
fn runtime_host_dev_refresh_script_hash_is_empty_outside_dev() {
	let dist_dir = temp_dist_dir("dev-refresh-script-hash-non-dev");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	assert_eq!(host.dev_refresh_script_content_sha256().unwrap(), "");

	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn request_path_is_under_mount_root_matches_api_base_boundaries() {
	assert!(request_path_is_under_mount_root("/api", "/api/"));
	assert!(request_path_is_under_mount_root("/api/ping", "/api/"));
	assert!(!request_path_is_under_mount_root("/apix", "/api/"));
}

fn assert_substrings_in_order(body: &str, expected: &[&str]) {
	let mut last_idx = Option::None;
	for substring in expected {
		let idx = body
			.find(substring)
			.unwrap_or_else(|| panic!("expected body to contain {substring:?}"));
		if let Some(last_idx) = last_idx {
			assert!(
				idx > last_idx,
				"expected {substring:?} after previous substring"
			);
		}
		last_idx = Some(idx);
	}
}
