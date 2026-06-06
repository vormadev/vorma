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
use crate::core::{Middleware, Middlewares, Resource, ResourceKind, Resources, View, Views};
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
	resources: Resources<(), &'static str>,
	middlewares: Middlewares<(), &'static str>,
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
		resources,
		middlewares,
		tasks_options: vorma_tasks::TasksOptions::default(),
		document,
		request_body_limit: crate::DEFAULT_REQUEST_BODY_LIMIT,
	})
}

fn default_app(
	config: Config,
	views: Views<(), &'static str>,
	resources: Resources<(), &'static str>,
	middlewares: Middlewares<(), &'static str>,
) -> App<(), &'static str> {
	app(
		config,
		views,
		resources,
		middlewares,
		DocumentBuilder::default(),
	)
}

fn runtime_host(
	dist_dir: &Path,
	views: Views<(), &'static str>,
	resources: Resources<(), &'static str>,
	middlewares: Middlewares<(), &'static str>,
) -> RuntimeHost<(), &'static str> {
	RuntimeHost::new(default_app(cfg(dist_dir), views, resources, middlewares)).unwrap()
}

fn runtime_host_with_document(
	dist_dir: &Path,
	views: Views<(), &'static str>,
	resources: Resources<(), &'static str>,
	middlewares: Middlewares<(), &'static str>,
	document: DocumentBuilder,
) -> RuntimeHost<(), &'static str> {
	RuntimeHost::new(app(cfg(dist_dir), views, resources, middlewares, document)).unwrap()
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
			"/static/entry.js".to_owned(),
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

#[test]
fn vercel_runtime_static_out_dir_uses_current_dir_manifest_when_configured_manifest_is_missing() {
	let configured_root = temp_dist_dir("vercel-configured-root-missing-manifest");
	let current_root = temp_dist_dir("vercel-current-root-with-manifest");
	write_manifest(&current_root);
	let config = cfg(&configured_root);
	let configured_static_out = configured_root.join(".vorma/static");

	let got = vercel_runtime_static_out_dir_from_current_root(
		&config,
		&configured_static_out,
		ManifestMode::Prod,
		current_root.clone(),
	)
	.unwrap();

	assert_eq!(got, current_root.join(".vorma/static").clean());
	fs::remove_dir_all(configured_root).unwrap();
	fs::remove_dir_all(current_root).unwrap();
}

#[test]
fn vercel_runtime_static_out_dir_reports_configured_and_current_manifest_paths_when_missing() {
	let configured_root = temp_dist_dir("vercel-configured-root-without-manifest");
	let current_root = temp_dist_dir("vercel-current-root-without-manifest");
	let config = cfg(&configured_root);
	let configured_static_out = configured_root.join(".vorma/static");

	let error = vercel_runtime_static_out_dir_from_current_root(
		&config,
		&configured_static_out,
		ManifestMode::Prod,
		current_root.clone(),
	)
	.unwrap_err();

	assert!(
		error.contains(
			&configured_root
				.join(".vorma/static/vorma.manifest.prod.json")
				.display()
				.to_string()
		)
	);
	assert!(
		error.contains(
			&current_root
				.join(".vorma/static/vorma.manifest.prod.json")
				.display()
				.to_string()
		)
	);
	fs::remove_dir_all(configured_root).unwrap();
	fs::remove_dir_all(current_root).unwrap();
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
		public_filepaths: vec![
			"/static/app.css".to_owned(),
			"/static/entry.css".to_owned(),
			"/static/entry.js".to_owned(),
			"/static/favicon.ico".to_owned(),
			"/static/items.css".to_owned(),
			"/static/items.js".to_owned(),
			"/static/shared.css".to_owned(),
			"/static/shared.js".to_owned(),
		],
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
		client_views: BTreeMap::from([(
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
	manifest
		.public_filepaths
		.push("/static/items-parent.js".to_owned());
	manifest.client_views.insert(
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
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let middlewares = Middlewares::new();
	runtime_host(dist_dir, views, resources, middlewares)
}

fn middleware_host(dist_dir: &Path) -> RuntimeHost<(), &'static str> {
	let views = Views::new();
	let mut resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-global-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	middlewares.push(Middleware::new(|ctx| async move {
		if ctx.request().method() != Method::GET {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-method-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	middlewares.push(Middleware::new(|ctx| async move {
		if ctx.matched_pattern() != "/ping" {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-pattern-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let ping = Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	);
	resources.push(ping);
	runtime_host(dist_dir, views, resources, middlewares)
}

fn static_json_resource(
	ctx: crate::core::ErasedRequestCtx<(), &'static str>,
) -> crate::core::ErasedRouteFuture<&'static str> {
	crate::core::run_static_resource::<(), &'static str, serde_json::Value, (), serde_json::Value>(
		ctx,
		static_json_api_handler,
	)
}

fn static_json_api_handler(
	_: crate::core::ResourceCtx<(), &'static str, serde_json::Value>,
) -> crate::core::RouteFuture<serde_json::Value, &'static str> {
	Box::pin(async { panic!("bad JSON should fail before the API handler runs") })
}

fn static_form_data_resource(
	ctx: crate::core::ErasedRequestCtx<(), &'static str>,
) -> crate::core::ErasedRouteFuture<&'static str> {
	crate::core::run_static_resource::<(), &'static str, FormData, (), serde_json::Value>(
		ctx,
		static_form_data_api_handler,
	)
}

fn static_form_data_api_handler(
	ctx: crate::core::ResourceCtx<(), &'static str, FormData>,
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
		Resources::new(),
		Middlewares::new(),
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

mod document_and_view_shell;
mod method_and_request_boundaries;
mod middleware_and_effects;
mod public_assets_and_resources;
mod service_body_and_context;
