use std::collections::BTreeMap;
use std::error::Error;
use std::sync::Arc;
use std::time::Duration;

use http::Method;
use http::header::{HeaderName, HeaderValue, LOCATION, SET_COOKIE};
use serde_json::json;
use tokio::sync::Notify;
use vorma_tasks::{CancelToken, Tasks, TasksOptions};

use super::*;
use crate::constants::X_VORMA_CLIENT_BUILD_ID;
use crate::error::{LoaderError, LoaderErrorClientMsg};
use crate::manifest::{ClientCoreAssets, ClientModule, Manifest};
use crate::mux::{InputParser, RawRequest, Router};

fn exec_ctx() -> vorma_tasks::ExecCtx<&'static str> {
	Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new())
}

fn boxed_exec_ctx() -> vorma_tasks::ExecCtx<Box<dyn Error + Send + Sync>> {
	Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new())
}

fn manifest_with_routes(routes: &[(&str, &str)]) -> Manifest {
	Manifest {
		client_routes: routes
			.iter()
			.map(|(pattern, url)| {
				(
					(*pattern).to_owned(),
					ClientModule {
						url: (*url).to_owned(),
						..ClientModule::default()
					},
				)
			})
			.collect(),
		search_schemas: routes
			.iter()
			.map(|(pattern, _)| ((*pattern).to_owned(), serde_json::Value::Null))
			.collect(),
		..Manifest::default()
	}
}

fn manifest_with_client_route_modules(routes: &[(&str, ClientModule)]) -> Manifest {
	Manifest {
		client_routes: routes
			.iter()
			.map(|(pattern, module)| ((*pattern).to_owned(), module.clone()))
			.collect(),
		search_schemas: routes
			.iter()
			.map(|(pattern, _)| ((*pattern).to_owned(), serde_json::Value::Null))
			.collect(),
		..Manifest::default()
	}
}

fn api_get(path: &str) -> RawRequest {
	RawRequest::get(api_path(path))
}

fn api_request(method: Method, path: &str) -> RawRequest {
	RawRequest::new(
		method,
		api_path(path).parse().unwrap(),
		Default::default(),
		Bytes::new(),
	)
}

fn api_path(path: &str) -> String {
	assert!(path.starts_with('/'));
	format!("/api{path}")
}

#[derive(Debug)]
struct EmptyClientMsgError;

impl LoaderErrorClientMsg for EmptyClientMsgError {
	fn loader_error_client_msg(&self) -> Option<&str> {
		Some("")
	}
}

#[test]
fn build_view_skew_response_reports_stale_client_build_before_loader_work() {
	let uri = "/items?vorma-json=old-build".parse::<Uri>().unwrap();
	let response = build_view_skew_response(&uri, "current-build")
		.unwrap()
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		"current-build"
	);
	assert_eq!(response.headers().get(X_VORMA_BUILD_SKEW).unwrap(), "1");
	assert_eq!(response.headers().get(CACHE_CONTROL).unwrap(), "no-store");

	let fresh_uri = "/items?vorma-json=current-build".parse::<Uri>().unwrap();
	assert!(
		build_view_skew_response(&fresh_uri, "current-build")
			.unwrap()
			.is_none()
	);

	let manual_uri = "/items?vorma-json=_".parse::<Uri>().unwrap();
	assert!(
		build_view_skew_response(&manual_uri, "current-build")
			.unwrap()
			.is_none()
	);

	let empty_uri = "/items?vorma-json=".parse::<Uri>().unwrap();
	assert!(
		build_view_skew_response(&empty_uri, "current-build")
			.unwrap()
			.is_none()
	);
}

#[tokio::test]
async fn build_api_response_serializes_json_value() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::GET,
			"/ping",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"ok": true})) },
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_get("/ping"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		"build-id"
	);
	assert_eq!(response.body(), &Bytes::from_static(br#"{"ok":true}"#));
}

#[tokio::test]
async fn build_api_response_middleware_success_status_does_not_short_circuit_handler() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.use_task_middleware(|ctx| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::ACCEPTED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-middleware"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.add_task_handler(
			Method::GET,
			"/success",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok(json!({"ok": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_get("/success"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::CREATED);
	assert_eq!(response.headers().get("x-middleware").unwrap(), "kept");
	assert_eq!(response.body(), &Bytes::from_static(br#"{"ok":true}"#));
}

#[tokio::test]
async fn build_api_response_short_circuits_proxy_redirects() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::POST,
			"/sessions",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.redirect(false, "/dashboard", Some(StatusCode::FOUND))
					.unwrap();
				Ok(json!({"unused": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_request(Method::POST, "/sessions"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::FOUND);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/dashboard");
	assert!(response.body().is_empty());
}

#[tokio::test]
async fn build_api_response_task_errors_are_internal_server_errors() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::GET,
			"/boom",
			InputParser::<()>::default_input(),
			|_| async {
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from("boom")))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_get("/boom"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	assert_eq!(
		response.body(),
		&Bytes::from_static(b"Internal Server Error\n")
	);
}

#[tokio::test]
async fn build_api_response_preserves_handler_owned_error_proxy() {
	let mut router = Router::<(), Box<dyn Error + Send + Sync>>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::POST,
			"/bad",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::BAD_REQUEST, Some("bad input".to_owned()));
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-validation"),
					http::HeaderValue::from_static("1"),
				);
				ctx.response_proxy_mut()
					.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					Box::new(crate::Error::runtime("bad input")) as Box<dyn Error + Send + Sync>,
				)))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_request(Method::POST, "/bad"),
			Arc::new(()),
			boxed_exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-validation").unwrap(), "1");
	assert_eq!(response.headers().get(CACHE_CONTROL).unwrap(), "no-store");
	assert_eq!(response.headers().get_all(CACHE_CONTROL).iter().count(), 1);
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
}

#[tokio::test]
async fn build_api_response_preserves_handler_owned_redirect_proxy_on_error() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::POST,
			"/redirect",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.redirect(false, "/login", Some(StatusCode::FOUND))
					.unwrap();
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-auth"),
					HeaderValue::from_static("required"),
				);
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					"unauthorized",
				)))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_request(Method::POST, "/redirect"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::FOUND);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/login");
	assert_eq!(response.headers().get("x-auth").unwrap(), "required");
	assert!(response.body().is_empty());
}

#[tokio::test]
async fn build_api_response_suppresses_success_proxy_when_handler_errors() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.use_task_middleware(|ctx| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-vorma-middleware"),
				HeaderValue::from_static("1"),
			);
			ctx.response_proxy_mut()
				.set_cookie(cookie::Cookie::new("middleware", "1"));
			Ok(())
		})
		.unwrap();
	router
		.add_task_handler(
			Method::POST,
			"/boom",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::ACCEPTED, Option::None);
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-success"),
					HeaderValue::from_static("should-not-leak"),
				);
				ctx.response_proxy_mut()
					.set_cookie(cookie::Cookie::new("success", "should-not-leak"));
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from("boom")))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_request(Method::POST, "/boom"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
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
}

#[tokio::test]
async fn build_api_response_does_not_double_apply_successful_proxy_short_circuit() {
	let mut router = Router::<(), Box<dyn Error + Send + Sync>>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::POST,
			"/drifted",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CONFLICT, Some("drifted".to_owned()));
				ctx.response_proxy_mut()
					.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));
				Ok(json!({"unused": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_request(Method::POST, "/drifted"),
			Arc::new(()),
			boxed_exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::CONFLICT);
	assert_eq!(response.headers().get(CACHE_CONTROL).unwrap(), "no-store");
	assert_eq!(response.headers().get_all(CACHE_CONTROL).iter().count(), 1);
	assert_eq!(response.body(), &Bytes::from_static(b"drifted\n"));
}

#[tokio::test]
async fn build_api_response_set_headers_replace_existing_response_headers() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_task_handler(
			Method::GET,
			"/typed",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut().set_header(
					CONTENT_TYPE,
					HeaderValue::from_static("application/problem+json"),
				);
				Ok(json!({"ok": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_get("/typed"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(
		response.headers().get(CONTENT_TYPE).unwrap(),
		"application/problem+json"
	);
	assert_eq!(response.headers().get_all(CONTENT_TYPE).iter().count(), 1);
	assert_eq!(response.body(), &Bytes::from_static(br#"{"ok":true}"#));
}

#[tokio::test]
async fn build_api_response_input_bad_request_errors_are_bad_requests() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.use_task_middleware(|ctx| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-vorma-middleware"),
				HeaderValue::from_static("1"),
			);
			ctx.response_proxy_mut()
				.set_cookie(cookie::Cookie::new("middleware", "1"));
			Ok(())
		})
		.unwrap();
	router
		.add_task_handler(
			Method::GET,
			"/bad",
			InputParser::<()>::callback(|_| Err(crate::mux::InputError::bad_request("bad input"))),
			|_| async { Ok(json!({"unused": true})) },
		)
		.unwrap();

	let result = router
		.execute_task_route(
			api_get("/bad"),
			Arc::new(()),
			exec_ctx(),
			Arc::new(BTreeMap::new()),
		)
		.await
		.unwrap()
		.unwrap();
	let response = build_api_response(ApiResponseInput {
		expected_client_build_id: "build-id",
		result: &result,
	})
	.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
}

#[tokio::test]
async fn build_view_response_exposes_loader_error_client_msg() {
	let mut views = crate::mux::NestedRouter::<(), Box<dyn Error + Send + Sync>>::default();
	views
		.add_task_handler("/items", InputParser::<()>::default_input(), |_| async {
			let error: Box<dyn Error + Send + Sync> = Box::new(LoaderError {
				client_msg: "client visible".to_owned(),
				err: Some("server hidden".into()),
			});
			Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(error)))
		})
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js")]);
	let document = Document::new();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: boxed_exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(payload["outermost_server_err"], "client visible");
	assert_eq!(payload["outermost_server_err_idx"], 0);
}

#[tokio::test]
async fn build_view_response_input_bad_request_errors_are_bad_requests() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.use_task_middleware(|ctx| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-vorma-middleware"),
				HeaderValue::from_static("1"),
			);
			ctx.response_proxy_mut()
				.set_cookie(cookie::Cookie::new("middleware", "1"));
			Ok(())
		})
		.unwrap();
	views
		.add_task_handler(
			"/items",
			InputParser::<()>::callback(|_| Err(crate::mux::InputError::bad_request("bad input"))),
			|_| async { Ok(json!({"unused": true})) },
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js")]);
	let document = Document::new();
	let exec_ctx = Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new());

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx,
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
}

#[tokio::test]
async fn build_view_response_terminal_middleware_does_not_require_route_modules() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.use_task_middleware(|ctx| async move {
			ctx.response_proxy_mut()
				.redirect(false, "/login", Some(StatusCode::FOUND))
				.unwrap();
			Ok(())
		})
		.unwrap();
	views
		.add_task_handler("/items", InputParser::<()>::default_input(), |_| async {
			Ok(json!({"unused": true}))
		})
		.unwrap();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &Manifest::default(),
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();

	assert_eq!(response.status(), StatusCode::FOUND);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/login");
	assert!(response.body().is_empty());
}

#[tokio::test]
async fn build_view_response_middleware_success_status_does_not_short_circuit_loaders() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.use_task_middleware(|ctx| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::ACCEPTED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-middleware"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok(json!({"loaded": true}))
			},
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js")]);

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::CREATED);
	assert_eq!(response.headers().get("x-middleware").unwrap(), "kept");
	assert_eq!(payload["loaders_data"], json!([{"loaded": true}]));
}

#[tokio::test]
async fn build_view_response_preserves_handler_owned_error_proxy() {
	let mut views = crate::mux::NestedRouter::<(), Box<dyn Error + Send + Sync>>::default();
	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::BAD_REQUEST, Some("bad input".to_owned()));
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-validation"),
					http::HeaderValue::from_static("1"),
				);
				let error: Box<dyn Error + Send + Sync> =
					Box::new(crate::Error::runtime("bad input"));
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(error)))
			},
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js")]);
	let document = Document::new();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: boxed_exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-validation").unwrap(), "1");
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
}

#[tokio::test]
async fn build_view_response_suppresses_descendant_effects_after_ancestor_redirect() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	views
		.add_task_handler("/items", InputParser::<()>::default_input(), move |ctx| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response_proxy_mut()
					.redirect(false, "/login", Some(StatusCode::FOUND))
					.unwrap();
				Ok(json!({"parent": true}))
			}
		})
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let child_started = child_started_for_child.clone();
				async move {
					ctx.response_proxy_mut().set_header(
						http::header::HeaderName::from_static("x-child"),
						HeaderValue::from_static("leaked"),
					);
					child_started.notify_one();
					Ok(json!({"child": true}))
				}
			},
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]);
	let document = Document::new();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();

	assert_eq!(response.status(), StatusCode::FOUND);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/login");
	assert!(response.headers().get("x-child").is_none());
}

#[tokio::test]
async fn build_view_response_suppresses_descendant_effects_after_ancestor_loader_error() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	views
		.add_task_handler("/items", InputParser::<()>::default_input(), move |_| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					"parent failed",
				)))
			}
		})
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let child_started = child_started_for_child.clone();
				async move {
					ctx.response_proxy_mut().set_header(
						http::header::HeaderName::from_static("x-child"),
						HeaderValue::from_static("leaked"),
					);
					child_started.notify_one();
					Ok(json!({"child": true}))
				}
			},
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]);
	let document = Document::new();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(response.headers().get("x-child").is_none());
	assert_eq!(
		payload["outermost_server_err"],
		"An unexpected error occurred."
	);
	assert_eq!(payload["outermost_server_err_idx"], 0);
	assert_eq!(payload["import_urls"], json!(["/items.js"]));
	assert!(payload.get("loaders_data").is_none());
}

#[tokio::test]
async fn build_view_response_deepest_success_status_wins_and_keeps_all_success_effects() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();

	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::ACCEPTED, Option::None);
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			},
		)
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-child"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"child": true}))
			},
		)
		.unwrap();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::CREATED);
	assert_eq!(response.headers().get("x-parent").unwrap(), "1");
	assert_eq!(response.headers().get("x-child").unwrap(), "1");
	assert_eq!(
		payload["loaders_data"],
		json!([{"parent": true}, {"child": true}])
	);
}

#[tokio::test]
async fn build_view_response_child_redirect_keeps_parent_effects_and_short_circuits_output() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();

	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			},
		)
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.redirect(false, "/child", Some(StatusCode::FOUND))
					.unwrap();
				Ok(json!({"child": true}))
			},
		)
		.unwrap();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();

	assert_eq!(response.status(), StatusCode::FOUND);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/child");
	assert_eq!(response.headers().get("x-parent").unwrap(), "1");
	assert!(response.body().is_empty());
}

#[tokio::test]
async fn build_view_response_child_redirect_waits_for_parent_success_effects() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	let parent_started = Arc::new(Notify::new());
	let child_redirected = Arc::new(Notify::new());
	let parent_started_for_parent = parent_started.clone();
	let parent_started_for_child = parent_started.clone();
	let child_redirected_for_parent = child_redirected.clone();
	let child_redirected_for_child = child_redirected.clone();

	views
		.add_task_handler("/items", InputParser::<()>::default_input(), move |ctx| {
			let parent_started = parent_started_for_parent.clone();
			let child_redirected = child_redirected_for_parent.clone();
			async move {
				parent_started.notify_one();
				child_redirected.notified().await;
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			}
		})
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let parent_started = parent_started_for_child.clone();
				let child_redirected = child_redirected_for_child.clone();
				async move {
					parent_started.notified().await;
					ctx.response_proxy_mut()
						.redirect(false, "/child", Some(StatusCode::FOUND))
						.unwrap();
					child_redirected.notify_one();
					Ok(json!({"child": true}))
				}
			},
		)
		.unwrap();

	let response = tokio::time::timeout(
		Duration::from_millis(250),
		build_view_response(ViewResponseInput {
			expected_client_build_id: "build-id",
			request: RawRequest::get("/items/123?vorma-json=_"),
			state: Arc::new(()),
			exec_ctx: exec_ctx(),
			views: &views,
			manifest: &manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
			document: &Document::new(),
			public_filemap: Arc::new(BTreeMap::new()),
		}),
	)
	.await
	.expect("view response should not hang")
	.unwrap();

	assert_eq!(response.status(), StatusCode::FOUND);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/child");
	assert_eq!(response.headers().get("x-parent").unwrap(), "1");
	assert!(response.body().is_empty());
}

#[tokio::test]
async fn build_view_response_parent_error_status_suppresses_finished_child_effects() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	views
		.add_task_handler("/items", InputParser::<()>::default_input(), move |ctx| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response_proxy_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			}
		})
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let child_started = child_started_for_child.clone();
				async move {
					ctx.response_proxy_mut().set_header(
						http::header::HeaderName::from_static("x-child"),
						HeaderValue::from_static("leaked"),
					);
					child_started.notify_one();
					Ok(json!({"child": true}))
				}
			},
		)
		.unwrap();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();

	assert_eq!(response.status(), StatusCode::FORBIDDEN);
	assert_eq!(response.headers().get("x-parent").unwrap(), "1");
	assert!(response.headers().get("x-child").is_none());
	assert_eq!(response.body(), &Bytes::from_static(b"denied\n"));
}

#[tokio::test]
async fn build_view_response_erroring_child_suppresses_own_success_effects_and_grandchild() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	let grandchild_started = Arc::new(Notify::new());
	let grandchild_started_for_child = grandchild_started.clone();
	let grandchild_started_for_grandchild = grandchild_started.clone();

	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			},
		)
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let grandchild_started = grandchild_started_for_child.clone();
				async move {
					grandchild_started.notified().await;
					ctx.response_proxy_mut()
						.set_status(StatusCode::CREATED, Option::None);
					ctx.response_proxy_mut().set_header(
						http::header::HeaderName::from_static("x-child"),
						HeaderValue::from_static("1"),
					);
					Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
						"child failed",
					)))
				}
			},
		)
		.unwrap();
	views
		.add_task_handler(
			"/items/:id/details",
			InputParser::<()>::default_input(),
			move |ctx| {
				let grandchild_started = grandchild_started_for_grandchild.clone();
				async move {
					ctx.response_proxy_mut().set_header(
						http::header::HeaderName::from_static("x-grandchild"),
						HeaderValue::from_static("leaked"),
					);
					grandchild_started.notify_one();
					Ok(json!({"grandchild": true}))
				}
			},
		)
		.unwrap();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123/details?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest_with_routes(&[
			("/items", "/items.js"),
			("/items/:id", "/item.js"),
			("/items/:id/details", "/details.js"),
		]),
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.headers().get("x-parent").unwrap(), "1");
	assert!(response.headers().get("x-child").is_none());
	assert!(response.headers().get("x-grandchild").is_none());
	assert_eq!(
		payload["outermost_server_err"],
		"An unexpected error occurred."
	);
	assert_eq!(payload["outermost_server_err_idx"], 1);
	assert_eq!(payload["import_urls"], json!(["/items.js", "/item.js"]));
	assert_eq!(payload["loaders_data"], json!([{"parent": true}]));
}

#[tokio::test]
async fn build_view_response_keeps_match_metadata_when_payload_truncates_on_parent_error() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_task_handler("/items", InputParser::<()>::default_input(), |_| async {
			Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
				"parent failed",
			)))
		})
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"child": true})) },
		)
		.unwrap();
	let mut manifest = manifest_with_routes(&[("/items", "/items.js"), ("/items/:id", "/item.js")]);
	manifest
		.search_schemas
		.insert("/items".to_owned(), json!({"parent": "schema"}));
	manifest
		.search_schemas
		.insert("/items/:id".to_owned(), json!({"child": "schema"}));

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["matched_patterns"], json!(["/items", "/items/:id"]));
	assert_eq!(payload["params"], json!({"id": "123"}));
	assert_eq!(
		payload["search_schemas"],
		json!([{"parent": "schema"}, {"child": "schema"}])
	);
	assert_eq!(payload["outermost_server_err_idx"], 0);
	assert_eq!(payload["import_urls"], json!(["/items.js"]));
	assert!(payload.get("loaders_data").is_none());
}

#[tokio::test]
async fn build_view_response_errors_when_manifest_missing_search_schema_for_matched_pattern() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_task_handler("/", InputParser::<()>::default_input(), |_| async {
			Ok(json!({"root": true}))
		})
		.unwrap();
	let manifest = Manifest {
		client_routes: BTreeMap::from([(
			"/".to_owned(),
			ClientModule {
				url: "/root.js".to_owned(),
				..ClientModule::default()
			},
		)]),
		..Manifest::default()
	};

	let error = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap_err();

	assert_eq!(error, "no search schema found for matched pattern: /");
}

#[tokio::test]
async fn build_view_response_truncates_route_assets_after_first_loader_error() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_task_handler("/items", InputParser::<()>::default_input(), |_| async {
			Ok(json!({"parent": true}))
		})
		.unwrap();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|_| async {
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					"child failed",
				)))
			},
		)
		.unwrap();
	views
		.add_task_handler(
			"/items/:id/details",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"grandchild": true})) },
		)
		.unwrap();
	let mut manifest = manifest_with_client_route_modules(&[
		(
			"/items",
			ClientModule {
				url: "/items.js".to_owned(),
				dep_urls: vec!["/shared.js".to_owned(), "/parent.js".to_owned()],
				css_bundle_urls: vec!["/parent.css".to_owned()],
			},
		),
		(
			"/items/:id",
			ClientModule {
				url: "/item.js".to_owned(),
				dep_urls: vec!["/child.js".to_owned()],
				css_bundle_urls: vec!["/child.css".to_owned()],
			},
		),
		(
			"/items/:id/details",
			ClientModule {
				url: "/details.js".to_owned(),
				dep_urls: vec!["/grandchild.js".to_owned()],
				css_bundle_urls: vec!["/grandchild.css".to_owned()],
			},
		),
	]);
	manifest.client_entry.dep_urls = vec!["/entry.js".to_owned(), "/shared.js".to_owned()];
	manifest.client_entry.css_bundle_urls = vec!["/entry.css".to_owned()];

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123/details?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(payload["outermost_server_err_idx"], 1);
	assert_eq!(payload["import_urls"], json!(["/items.js", "/item.js"]));
	assert_eq!(
		payload["deps"],
		json!(["/entry.js", "/shared.js", "/parent.js", "/child.js"])
	);
	assert_eq!(
		payload["css_bundles"],
		json!(["/entry.css", "/parent.css", "/child.css"])
	);
	assert_eq!(payload["loaders_data"], json!([{"parent": true}]));
}

#[tokio::test]
async fn build_view_response_prod_html_preloads_dedupe_route_and_core_assets() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_task_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"id": "123"})) },
		)
		.unwrap();
	let mut manifest = manifest_with_client_route_modules(&[(
		"/items/:id",
		ClientModule {
			url: "/items.js".to_owned(),
			dep_urls: vec!["/shared.js".to_owned(), "/items-dep.js".to_owned()],
			css_bundle_urls: Vec::new(),
		},
	)]);
	manifest.client_entry.url = "/entry.js".to_owned();
	manifest.client_entry.dep_urls = vec!["/shared.js".to_owned()];
	manifest.client_core_assets = Some(ClientCoreAssets {
		module_url: "/shared.js".to_owned(),
		wasm_url: "/client.wasm".to_owned(),
	});

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items/123"),
		state: Arc::new(()),
		exec_ctx: exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &Document::new(),
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let html = String::from_utf8(response.body().to_vec()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(html.matches(r#"rel="modulepreload""#).count(), 2);
	assert_eq!(html.matches(r#"href="/shared.js""#).count(), 1);
	assert_eq!(html.matches(r#"href="/items-dep.js""#).count(), 1);
	assert!(html.contains(r#"href="/client.wasm""#));
	assert!(html.contains(r#"as="fetch""#));
	assert!(html.contains(r#"type="application/wasm""#));
}

#[test]
fn to_dev_scripts_uses_client_entry_origin_for_vite_runtime_scripts() {
	let scripts = to_dev_scripts(5173, "http://127.0.0.1:5174/src/client/entry.tsx", true).unwrap();

	assert!(scripts.contains("http://127.0.0.1:5174/@react-refresh"));
	assert!(scripts.contains(r#"src="http://127.0.0.1:5174/@vite/client""#));
	assert!(scripts.contains(r#"src="http://127.0.0.1:5174/src/client/entry.tsx""#));
	assert!(!scripts.contains("http://localhost"));
	assert!(!scripts.contains("http://127.0.0.1:5173/@vite/client"));
}

#[test]
fn to_dev_scripts_falls_back_to_loopback_origin_for_relative_client_entry() {
	let scripts = to_dev_scripts(5173, "/src/client/entry.tsx", false).unwrap();

	assert!(scripts.contains(r#"src="http://127.0.0.1:5173/@vite/client""#));
	assert!(scripts.contains(r#"src="/src/client/entry.tsx""#));
}

#[test]
fn vite_dev_origin_preserves_ipv6_loopback_brackets() {
	assert_eq!(
		vite_dev_origin(5173, "http://[::1]:5174/src/client/entry.tsx"),
		"http://[::1]:5174"
	);
}

#[tokio::test]
async fn build_view_response_suppresses_success_proxy_when_loader_errors() {
	let mut views = crate::mux::NestedRouter::<(), Box<dyn Error + Send + Sync>>::default();
	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				ctx.response_proxy_mut().set_header(
					http::header::HeaderName::from_static("x-view-header"),
					http::HeaderValue::from_static("1"),
				);
				let error: Box<dyn Error + Send + Sync> = Box::new(LoaderError {
					client_msg: "client visible".to_owned(),
					err: Option::None,
				});
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(error)))
			},
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js")]);
	let document = Document::new();

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx: boxed_exec_ctx(),
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(!response.headers().contains_key("x-view-header"));
	assert_eq!(payload["outermost_server_err"], "client visible");
}

#[tokio::test]
async fn build_view_response_detects_loader_error_by_index_not_message_text() {
	let mut views = crate::mux::NestedRouter::<(), EmptyClientMsgError>::default();
	views
		.add_task_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					EmptyClientMsgError,
				)))
			},
		)
		.unwrap();
	let manifest = manifest_with_routes(&[("/items", "/items.js")]);
	let document = Document::new();
	let exec_ctx = Tasks::new(TasksOptions::default()).exec_ctx(CancelToken::new());

	let response = build_view_response(ViewResponseInput {
		expected_client_build_id: "build-id",
		request: RawRequest::get("/items?vorma-json=_"),
		state: Arc::new(()),
		exec_ctx,
		views: &views,
		manifest: &manifest,
		document: &document,
		public_filemap: Arc::new(BTreeMap::new()),
	})
	.await
	.unwrap();
	let payload: Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert!(payload.get("outermost_server_err").is_none());
	assert_eq!(payload["outermost_server_err_idx"], 0);
}

#[test]
fn json_for_script_escapes_html_breakout_bytes() {
	let json = json_for_script(json!({
		"danger": "</script><script>alert(1)</script>",
		"amp": "&",
	}))
	.unwrap();

	assert!(!json.contains("</script>"));
	assert!(json.contains("\\u003c/script\\u003e"));
	assert!(json.contains("\\u0026"));
}

#[test]
fn refresh_script_inner_html_replaces_dev_manifest_values() {
	let script = refresh_script_inner_html(&Manifest {
		dev_mux_port: 4242,
		dev_refresh_token: "token-123".to_owned(),
		..Manifest::default()
	});

	assert!(script.starts_with('\n'));
	assert!(script.contains("4242"));
	assert!(script.contains("token-123"));
	assert!(!script.contains("__REPLACE_ME_WITH_REFRESH_PORT__"));
	assert!(!script.contains("__REPLACE_ME_WITH_REFRESH_TOKEN__"));
}
