use std::collections::BTreeMap;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use http::{HeaderName, HeaderValue, Method, StatusCode};
use serde_json::Value;
use tokio::sync::Notify;
use vorma_tasks::{CancelToken, Error as TaskError, ExecCtx, Task, Tasks, TasksOptions};

use crate::mux::{
	InputError, InputParser, NestedOptions, NestedRouter, None, RawRequest, RequestCtx,
	RouteExecutionError, Router,
};

#[derive(Clone)]
struct TestEnv {
	shared_runs: Arc<AtomicUsize>,
}

#[derive(Clone, Debug, Eq, Hash, PartialEq)]
struct SharedInput;

fn default_runtime() -> (Arc<TestEnv>, ExecCtx<&'static str>) {
	let tasks = Tasks::new(TasksOptions::default());
	(
		Arc::new(TestEnv {
			shared_runs: Arc::new(AtomicUsize::new(0)),
		}),
		tasks.exec_ctx(CancelToken::new()),
	)
}

fn test_runtime(shared_runs: Arc<AtomicUsize>) -> (Arc<TestEnv>, ExecCtx<&'static str>) {
	let tasks = Tasks::new(TasksOptions::default());
	(
		Arc::new(TestEnv { shared_runs }),
		tasks.exec_ctx(CancelToken::new()),
	)
}

fn no_input() -> InputParser<()> {
	InputParser::default_input()
}

fn empty_public_filemap() -> Arc<BTreeMap<String, String>> {
	Arc::new(BTreeMap::new())
}

fn api_request(path: &str) -> RawRequest {
	if path == "/" {
		return RawRequest::get("/api");
	}
	RawRequest::get(format!("/api{path}"))
}

#[derive(Clone, Default)]
struct GreetingInput {
	name: String,
}

#[test]
fn router_defaults_mount_root_and_best_match() {
	let router: Router = Router::default();
	assert_eq!(router.dynamic_param_prefix(), ':');
	assert_eq!(router.splat_segment_identifier(), '*');
	assert_eq!(router.mount_root(), "/api/");

	let cases = [
		("api", "/api/"),
		("/api", "/api/"),
		("/api/", "/api/"),
		("api/v1", "/api/v1/"),
	];
	for (input, expected) in cases {
		let router: Router = Router::new(crate::mux::Options {
			mount_root: input.to_string(),
			..crate::mux::Options::default()
		})
		.unwrap();
		assert_eq!(router.mount_root(), expected);
	}
	for input in ["", "/"] {
		let error =
			match Router::<(), Box<dyn std::error::Error + Send + Sync>>::new(crate::mux::Options {
				mount_root: input.to_owned(),
				..crate::mux::Options::default()
			}) {
				Ok(_) => panic!("invalid mount root {input:?} should fail"),
				Err(error) => error.to_string(),
			};
		assert!(error.contains("api_base must be a non-root path prefix"));
	}

	let mut router: Router = Router::new(crate::mux::Options {
		mount_root: "/api".to_string(),
		..crate::mux::Options::default()
	})
	.unwrap();
	router
		.add_handler(Method::GET, "/", no_input(), |_| async { Ok(()) })
		.unwrap();
	router
		.add_handler(Method::GET, "/users/new", no_input(), |_| async { Ok(()) })
		.unwrap();
	router
		.add_handler(Method::GET, "/users/:id", no_input(), |_| async { Ok(()) })
		.unwrap();
	router
		.add_handler(Method::POST, "/users", no_input(), |_| async { Ok(()) })
		.unwrap();

	let found = router.find_best(&Method::GET, "/api").unwrap();
	assert_eq!(found.original_pattern(), "/");

	let found = router.find_best(&Method::GET, "/api/").unwrap();
	assert_eq!(found.original_pattern(), "/");

	let found = router.find_best(&Method::GET, "/api/users/new").unwrap();
	assert_eq!(found.original_pattern(), "/users/new");
	assert!(found.params().is_empty());

	let found = router.find_best(&Method::GET, "/api/users/123").unwrap();
	assert_eq!(found.original_pattern(), "/users/:id");
	assert_eq!(found.params().get("id").map(String::as_str), Some("123"));

	let found = router.find_best(&Method::HEAD, "/api/users/123").unwrap();
	assert_eq!(found.method(), &Method::GET);
	assert!(found.is_head_fallback());

	let methods = router
		.allowed_methods_for_path("/api/users/123")
		.into_iter()
		.collect::<Vec<_>>();
	assert_eq!(methods, ["GET", "HEAD"]);

	router
		.add_handler(Method::HEAD, "/users/:id", no_input(), |_| async { Ok(()) })
		.unwrap();
	let found = router.find_best(&Method::HEAD, "/api/users/123").unwrap();
	assert_eq!(found.method(), &Method::HEAD);
	assert!(!found.is_head_fallback());
	assert!(router.find_best(&Method::GET, "/users/123").is_none());

	let methods = router
		.allowed_methods_for_path("/api/users")
		.into_iter()
		.collect::<Vec<_>>();
	assert_eq!(methods, ["POST"]);
}

#[tokio::test]
async fn route_executes_with_typed_input_and_request_context() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(
			Method::GET,
			"/orgs/:org_id/files/*",
			InputParser::callback(|request| {
				Ok(GreetingInput {
					name: request.query().unwrap_or("").to_string(),
				})
			}),
			|ctx| async move {
				assert_eq!(ctx.matched_pattern(), "/orgs/:org_id/files/*");
				assert_eq!(ctx.param("org_id"), Some("acme"));
				assert_eq!(ctx.splat_values(), ["reports", "q1"]);
				assert_eq!(ctx.request().path(), "/api/orgs/acme/files/reports/q1");
				assert_eq!(ctx.state().shared_runs.load(Ordering::SeqCst), 0);
				Ok(format!("hello {}", ctx.input().name))
			},
		)
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/orgs/acme/files/reports/q1?Jeff"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.route_match().original_pattern(),
		"/orgs/:org_id/files/*"
	);
	assert_eq!(result.data().and_then(Value::as_str), Some("hello Jeff"));
	assert!(result.error().is_none());
}

#[tokio::test]
async fn middleware_runs_in_scope_order_and_merges_response_proxy() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(Method::GET, "/items/:id", no_input(), |ctx| async move {
			{
				let mut proxy = ctx.response_proxy_mut();
				proxy.add_header(
					HeaderName::from_static("x-mux"),
					HeaderValue::from_static("handler"),
				);
			}
			Ok(ctx.param("id").unwrap().to_string())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			let mut proxy = ctx.response_proxy_mut();
			proxy.add_header(
				HeaderName::from_static("x-mux"),
				HeaderValue::from_static("global"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			if ctx.request().method() != Method::GET {
				return Ok(());
			}
			let mut proxy = ctx.response_proxy_mut();
			proxy.add_header(
				HeaderName::from_static("x-mux"),
				HeaderValue::from_static("method"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			if ctx.matched_pattern() != "/items/:id" {
				return Ok(());
			}
			let mut proxy = ctx.response_proxy_mut();
			proxy.add_header(
				HeaderName::from_static("x-mux"),
				HeaderValue::from_static("route"),
			);
			Ok(())
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/items/123"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(result.data().and_then(Value::as_str), Some("123"));
	let values: Vec<_> = result
		.response_proxy()
		.headers(&HeaderName::from_static("x-mux"))
		.into_iter()
		.map(|value| value.to_str().unwrap())
		.collect();
	assert_eq!(values, ["global", "method", "route", "handler"]);
}

#[tokio::test]
async fn middleware_filter_controls_execution() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let ran = Arc::new(AtomicUsize::new(0));
	let ran_for_middleware = ran.clone();
	router
		.add_handler(Method::GET, "/public", no_input(), |_| async { Ok("ok") })
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let ran = ran_for_middleware.clone();
			async move {
				if !ctx.request().path().starts_with("/private") {
					return Ok(());
				}
				ran.fetch_add(1, Ordering::SeqCst);
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/public"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(result.data().and_then(Value::as_str), Some("ok"));
	assert_eq!(ran.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_error_cancels_later_parallel_sibling_and_short_circuits_handler() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let later_started = Arc::new(Notify::new());
	let later_started_for_error_middleware = later_started.clone();
	let later_started_for_waiting_middleware = later_started.clone();

	router
		.add_handler(Method::GET, "/middleware-cancel", no_input(), move |_| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok("handler")
			}
		})
		.unwrap();
	router
		.use_middleware(move |_| {
			let later_started = later_started_for_error_middleware.clone();
			async move {
				later_started.notified().await;
				tokio::time::sleep(Duration::from_millis(20)).await;
				Err::<(), _>(TaskError::from("blocked"))
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_started = later_started_for_waiting_middleware.clone();
			async move {
				later_started.notify_one();
				ctx.exec_ctx().cancel_token().cancelled().await;
				Err::<(), _>(TaskError::Cancelled)
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = tokio::time::timeout(
		Duration::from_millis(250),
		router.execute_route(
			api_request("/middleware-cancel"),
			state,
			exec_ctx,
			empty_public_filemap(),
		),
	)
	.await
	.expect("middleware cancellation should not hang")
	.unwrap()
	.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_error_preserves_explicit_error_proxy() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();

	router
		.add_handler(Method::GET, "/middleware-error", no_input(), move |_| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok("handler")
			}
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-authz"),
				HeaderValue::from_static("denied"),
			);
			Err::<(), _>(TaskError::from("denied"))
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-authz"))
			.unwrap(),
		HeaderValue::from_static("denied")
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_cancelled_later_sibling_does_not_mask_real_error_proxy() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let later_started = Arc::new(Notify::new());
	let later_started_for_error_middleware = later_started.clone();
	let later_started_for_waiting_middleware = later_started.clone();

	router
		.add_handler(Method::GET, "/middleware-error", no_input(), |_| async {
			Ok("handler")
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_started = later_started_for_error_middleware.clone();
			async move {
				later_started.notified().await;
				ctx.response_proxy_mut()
					.set_status(StatusCode::CONFLICT, Some("drifted".to_owned()));
				Err::<(), _>(TaskError::from("drifted"))
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_started = later_started_for_waiting_middleware.clone();
			async move {
				later_started.notify_one();
				ctx.exec_ctx().cancel_token().cancelled().await;
				Err::<(), _>(TaskError::Cancelled)
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = tokio::time::timeout(
		Duration::from_millis(250),
		router.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		),
	)
	.await
	.expect("middleware cancellation should not hang")
	.unwrap()
	.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::CONFLICT)
	);
	assert_eq!(result.response_proxy().status().1, "drifted");
}

#[tokio::test]
async fn middleware_prior_redirect_short_circuits_later_plain_error() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(Method::GET, "/middleware-error", no_input(), |_| async {
			Ok("handler")
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut()
				.redirect(false, "/ok", Option::None)
				.unwrap();
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|_: RequestCtx<TestEnv, &'static str, None>| async {
			Err::<(), _>(TaskError::from("boom"))
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert!(result.response_proxy().is_redirect());
	assert_eq!(result.response_proxy().location(), "/ok");
}

#[tokio::test]
async fn middleware_prior_redirect_suppresses_later_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let later_ran = Arc::new(Notify::new());
	let later_ran_for_redirect_middleware = later_ran.clone();
	let later_ran_for_later_middleware = later_ran.clone();

	router
		.add_handler(Method::GET, "/middleware-redirect", no_input(), move |_| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok("handler")
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_redirect_middleware.clone();
			async move {
				later_ran.notified().await;
				ctx.response_proxy_mut()
					.redirect(false, "/login", Option::None)
					.unwrap();
				Ok(())
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_later_middleware.clone();
			async move {
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-later"),
					HeaderValue::from_static("leaked"),
				);
				later_ran.notify_one();
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-redirect"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert_eq!(result.response_proxy().location(), "/login");
	assert!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-later"))
			.is_none()
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_prior_error_status_suppresses_later_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let later_ran = Arc::new(Notify::new());
	let later_ran_for_error_middleware = later_ran.clone();
	let later_ran_for_later_middleware = later_ran.clone();

	router
		.add_handler(
			Method::GET,
			"/middleware-error-status",
			no_input(),
			move |_| {
				let handler_runs = handler_runs_for_handler.clone();
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok("handler")
				}
			},
		)
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_error_middleware.clone();
			async move {
				later_ran.notified().await;
				ctx.response_proxy_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				Ok(())
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_later_middleware.clone();
			async move {
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-later"),
					HeaderValue::from_static("leaked"),
				);
				later_ran.notify_one();
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error-status"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert_eq!(result.response_proxy().status().1, "denied");
	assert!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-later"))
			.is_none()
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_later_redirect_keeps_prior_success_effects_and_short_circuits_handler() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();

	router
		.add_handler(Method::GET, "/middleware-redirect", no_input(), move |_| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok("handler")
			}
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-prior"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut()
				.redirect(false, "/login", Option::None)
				.unwrap();
			Ok(())
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-redirect"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert_eq!(result.response_proxy().location(), "/login");
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-prior"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_later_redirect_waits_for_prior_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let prior_started = Arc::new(Notify::new());
	let redirect_done = Arc::new(Notify::new());
	let prior_started_for_prior_middleware = prior_started.clone();
	let prior_started_for_redirect_middleware = prior_started.clone();
	let redirect_done_for_prior_middleware = redirect_done.clone();
	let redirect_done_for_redirect_middleware = redirect_done.clone();

	router
		.add_handler(Method::GET, "/middleware-redirect", no_input(), move |_| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok("handler")
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let prior_started = prior_started_for_prior_middleware.clone();
			let redirect_done = redirect_done_for_prior_middleware.clone();
			async move {
				prior_started.notify_one();
				redirect_done.notified().await;
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-prior"),
					HeaderValue::from_static("kept"),
				);
				Ok(())
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let prior_started = prior_started_for_redirect_middleware.clone();
			let redirect_done = redirect_done_for_redirect_middleware.clone();
			async move {
				prior_started.notified().await;
				ctx.response_proxy_mut()
					.redirect(false, "/login", Option::None)
					.unwrap();
				redirect_done.notify_one();
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = tokio::time::timeout(
		Duration::from_millis(250),
		router.execute_route(
			api_request("/middleware-redirect"),
			state,
			exec_ctx,
			empty_public_filemap(),
		),
	)
	.await
	.expect("middleware cancellation should not hang")
	.unwrap()
	.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert_eq!(result.response_proxy().location(), "/login");
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-prior"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_error_status_without_returned_error_short_circuits_handler() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();

	router
		.add_handler(
			Method::GET,
			"/middleware-error-status",
			no_input(),
			move |_| {
				let handler_runs = handler_runs_for_handler.clone();
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok("handler")
				}
			},
		)
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-prior"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
			Ok(())
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error-status"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert_eq!(result.response_proxy().status().1, "denied");
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-prior"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_prior_terminal_does_not_wait_for_uncancellable_later_sibling() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let later_started = Arc::new(Notify::new());
	let terminal_can_finish = Arc::new(Notify::new());
	let later_started_for_terminal = later_started.clone();
	let later_started_for_later = later_started.clone();
	let terminal_can_finish_for_terminal = terminal_can_finish.clone();

	router
		.add_handler(Method::GET, "/middleware-terminal", no_input(), |_| async {
			Ok("handler should not run")
		})
		.unwrap();
	router
		.use_middleware(move |ctx| {
			let later_started = later_started_for_terminal.clone();
			let terminal_can_finish = terminal_can_finish_for_terminal.clone();
			async move {
				later_started.notified().await;
				ctx.response_proxy_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				terminal_can_finish.notify_one();
				Ok(())
			}
		})
		.unwrap();
	router
		.use_middleware(move |_| {
			let later_started = later_started_for_later.clone();
			async move {
				later_started.notify_one();
				std::future::pending::<()>().await;
				#[allow(unreachable_code)]
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = tokio::time::timeout(
		Duration::from_millis(250),
		router.execute_route(
			api_request("/middleware-terminal"),
			state,
			exec_ctx,
			empty_public_filemap(),
		),
	)
	.await
	.expect("terminal middleware should not wait for irrelevant later work")
	.unwrap()
	.unwrap();

	terminal_can_finish.notified().await;
	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::FORBIDDEN)
	);
}

#[tokio::test]
async fn middleware_prior_plain_error_suppresses_later_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let later_ran = Arc::new(Notify::new());
	let later_ran_for_error_middleware = later_ran.clone();
	let later_ran_for_later_middleware = later_ran.clone();

	router
		.add_handler(Method::GET, "/middleware-error", no_input(), |_| async {
			Ok("handler")
		})
		.unwrap();
	router
		.use_middleware(move |_: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_error_middleware.clone();
			async move {
				later_ran.notified().await;
				Err::<(), _>(TaskError::from("boom"))
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_later_middleware.clone();
			async move {
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-later"),
					HeaderValue::from_static("leaked"),
				);
				later_ran.notify_one();
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-later"))
			.is_none()
	);
}

#[tokio::test]
async fn middleware_plain_error_preserves_prior_response_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(Method::GET, "/middleware-error", no_input(), |_| async {
			Ok("handler")
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-prior"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|_: RequestCtx<TestEnv, &'static str, None>| async {
			Err::<(), _>(TaskError::from("boom"))
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-prior"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
}

#[tokio::test]
async fn middleware_plain_error_suppresses_own_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(Method::GET, "/middleware-error", no_input(), |_| async {
			Ok("handler")
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::ACCEPTED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-own-success"),
				HeaderValue::from_static("suppressed"),
			);
			Err::<(), _>(TaskError::from("boom"))
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-own-success"))
			.is_none()
	);
}

#[tokio::test]
async fn middleware_later_plain_error_waits_for_prior_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	let prior_started = Arc::new(Notify::new());
	let error_done = Arc::new(Notify::new());
	let prior_started_for_prior = prior_started.clone();
	let prior_started_for_error = prior_started.clone();
	let error_done_for_prior = error_done.clone();
	let error_done_for_error = error_done.clone();

	router
		.add_handler(Method::GET, "/middleware-error", no_input(), move |_| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok("handler")
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let prior_started = prior_started_for_prior.clone();
			let error_done = error_done_for_prior.clone();
			async move {
				prior_started.notify_one();
				error_done.notified().await;
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-prior"),
					HeaderValue::from_static("kept"),
				);
				ctx.response_proxy_mut()
					.set_cookie(cookie::Cookie::new("prior", "kept"));
				Ok(())
			}
		})
		.unwrap();
	router
		.use_middleware(move |_: RequestCtx<TestEnv, &'static str, None>| {
			let prior_started = prior_started_for_error.clone();
			let error_done = error_done_for_error.clone();
			async move {
				prior_started.notified().await;
				error_done.notify_one();
				Err::<(), _>(TaskError::from("boom"))
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = tokio::time::timeout(
		Duration::from_millis(250),
		router.execute_route(
			api_request("/middleware-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		),
	)
	.await
	.expect("middleware ordering should not hang")
	.unwrap()
	.unwrap();

	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-prior"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(result.response_proxy().cookies()[0].name(), "prior");
	assert_eq!(result.response_proxy().cookies()[0].value(), "kept");
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn route_bad_request_input_errors_are_reported_without_running_handler() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();
	router
		.add_handler(
			Method::GET,
			"/validate",
			InputParser::<()>::callback(|_| Err(InputError::bad_request("bad input"))),
			move |_| {
				let handler_runs = handler_runs_for_handler.clone();
				async move {
					handler_runs.fetch_add(1, Ordering::SeqCst);
					Ok("handler")
				}
			},
		)
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/validate"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert!(result.data().is_none());
	assert!(
		result
			.error()
			.is_some_and(RouteExecutionError::is_bad_request)
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn route_handler_error_suppresses_own_success_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(
			Method::GET,
			"/handler-error",
			no_input(),
			|ctx| async move {
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				ctx.response_proxy_mut().set_header(
					HeaderName::from_static("x-handler-success"),
					HeaderValue::from_static("suppressed"),
				);
				Err::<(), _>(RouteExecutionError::Task(TaskError::from("boom")))
			},
		)
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/handler-error"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert!(matches!(
		result.error(),
		Some(RouteExecutionError::Task(TaskError::Failed(_)))
	));
	assert_eq!(result.response_proxy().status().0, Option::None);
	assert!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-handler-success"))
			.is_none()
	);
}

#[tokio::test]
async fn route_and_middleware_share_task_scope() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let shared_runs = Arc::new(AtomicUsize::new(0));
	let shared_runs_for_task = shared_runs.clone();
	let shared_task = Task::new(Duration::ZERO, move |_ctx, _input: SharedInput| {
		let shared_runs = shared_runs_for_task.clone();
		async move {
			shared_runs.fetch_add(1, Ordering::SeqCst);
			tokio::time::sleep(Duration::from_millis(20)).await;
			Ok("shared-result".to_string())
		}
	});
	let shared_task_for_handler = shared_task.clone();

	router
		.add_handler(Method::GET, "/shared", no_input(), move |ctx| {
			let shared_task = shared_task_for_handler.clone();
			async move {
				let value = shared_task.run(ctx.exec_ctx(), SharedInput).await?;
				Ok((*value).clone())
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let shared_task = shared_task.clone();
			async move {
				let _ = shared_task.run(ctx.exec_ctx(), SharedInput).await?;
				Ok(())
			}
		})
		.unwrap();

	let (state, exec_ctx) = test_runtime(shared_runs.clone());
	let result = router
		.execute_route(
			api_request("/shared"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(result.data().and_then(Value::as_str), Some("shared-result"));
	assert_eq!(shared_runs.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn middleware_success_status_does_not_short_circuit_handler() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_handler = handler_runs.clone();

	router
		.use_middleware(|ctx| async move {
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
		.add_handler(Method::GET, "/success", no_input(), move |ctx| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok("handler")
			}
		})
		.unwrap();

	let (state, exec_ctx) = default_runtime();
	let result = router
		.execute_route(
			api_request("/success"),
			state,
			exec_ctx,
			empty_public_filemap(),
		)
		.await
		.unwrap()
		.unwrap();

	assert_eq!(handler_runs.load(Ordering::SeqCst), 1);
	assert_eq!(
		result.response_proxy().status().0,
		Some(StatusCode::CREATED)
	);
	assert_eq!(
		result
			.response_proxy()
			.header(&HeaderName::from_static("x-middleware"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(result.data().and_then(Value::as_str), Some("handler"));
}

#[test]
fn nested_router_registration_snapshots_and_matching() {
	let mut router: NestedRouter = NestedRouter::new(NestedOptions {
		explicit_index_segment_identifier: "_index".to_string(),
		..NestedOptions::default()
	})
	.unwrap();

	router.add_pattern_without_handler("/").unwrap();
	router.add_pattern_without_handler("/users").unwrap();
	router
		.add_handler("/users/:id", no_input(), |_| async { Ok("user") })
		.unwrap();
	router
		.add_handler("/users/_index", no_input(), |_| async { Ok("index") })
		.unwrap();

	assert!(router.is_registered("/users/:id").unwrap());
	assert!(router.has_handler("/users/:id").unwrap());

	let mut snapshot = router.all_routes().unwrap();
	snapshot.clear();
	assert!(router.is_registered("/users/:id").unwrap());

	let results = router.find_nested_matches("/users/123").unwrap().unwrap();
	let patterns: Vec<_> = results
		.matches
		.iter()
		.map(|found| found.pattern.original_pattern())
		.collect();
	assert_eq!(patterns, ["/", "/users", "/users/:id"]);
	assert_eq!(results.params.get("id").map(String::as_str), Some("123"));

	let results = router.find_nested_matches("/users/").unwrap().unwrap();
	let patterns: Vec<_> = results
		.matches
		.iter()
		.map(|found| found.pattern.original_pattern())
		.collect();
	assert_eq!(patterns, ["/", "/users", "/users/_index"]);
}

#[tokio::test]
async fn nested_tasks_run_in_parallel_and_preserve_match_order() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let sleep = Duration::from_millis(60);
	router
		.add_handler("/parallel", no_input(), move |_| async move {
			tokio::time::sleep(sleep).await;
			Ok("parent-ok")
		})
		.unwrap();
	router
		.add_handler("/parallel/:id", no_input(), move |ctx| async move {
			tokio::time::sleep(sleep).await;
			Ok(ctx.param("id").unwrap().to_string())
		})
		.unwrap();

	let matches = router
		.find_nested_matches("/parallel/123")
		.unwrap()
		.unwrap();
	let (state, exec_ctx) = default_runtime();
	let start = tokio::time::Instant::now();
	let results = router
		.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/parallel/123"),
			matches,
			empty_public_filemap(),
		)
		.await
		.unwrap();
	let elapsed = start.elapsed();

	assert!(
		elapsed < sleep + Duration::from_millis(40),
		"expected parallel execution, elapsed={elapsed:?}"
	);
	assert_eq!(results.results().len(), 2);
	assert_eq!(results.results()[0].pattern(), "/parallel");
	assert_eq!(results.results()[1].pattern(), "/parallel/:id");
	assert_eq!(
		results.results()[0].data().and_then(Value::as_str),
		Some("parent-ok")
	);
	assert_eq!(
		results.results()[1].data().and_then(Value::as_str),
		Some("123")
	);
}

#[tokio::test]
async fn nested_middleware_terminal_proxy_suppresses_matched_handlers() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let parent_runs = Arc::new(AtomicUsize::new(0));
	let child_runs = Arc::new(AtomicUsize::new(0));
	let parent_runs_for_handler = parent_runs.clone();
	let child_runs_for_handler = child_runs.clone();

	router
		.use_middleware(|ctx| async move {
			assert_eq!(ctx.matched_pattern(), "/items/:id");
			assert_eq!(ctx.param("id"), Some("123"));
			assert_eq!(ctx.request().path(), "/items/123");
			ctx.response_proxy_mut()
				.redirect(false, "/login", Some(StatusCode::FOUND))
				.unwrap();
			Ok(())
		})
		.unwrap();
	router
		.add_handler("/items", no_input(), move |_| {
			let parent_runs = parent_runs_for_handler.clone();
			async move {
				parent_runs.fetch_add(1, Ordering::SeqCst);
				Ok("parent")
			}
		})
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), move |_| {
			let child_runs = child_runs_for_handler.clone();
			async move {
				child_runs.fetch_add(1, Ordering::SeqCst);
				Ok("child")
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = router
		.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		)
		.await
		.unwrap();

	assert_eq!(parent_runs.load(Ordering::SeqCst), 0);
	assert_eq!(child_runs.load(Ordering::SeqCst), 0);
	let middleware_proxy = results.middleware_proxy().unwrap();
	assert_eq!(middleware_proxy.status().0, Some(StatusCode::FOUND));
	assert_eq!(middleware_proxy.location(), "/login");
	assert!(
		results
			.results()
			.iter()
			.all(|result| result.data().is_none())
	);
	assert!(results.results().iter().all(|result| !result.ran_task()));
	assert!(results.results().iter().all(|result| {
		result
			.response_proxy()
			.is_some_and(|proxy| !proxy.is_terminal_response())
	}));
}

#[tokio::test]
async fn nested_middleware_success_status_does_not_short_circuit_matched_handlers() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let parent_runs = Arc::new(AtomicUsize::new(0));
	let child_runs = Arc::new(AtomicUsize::new(0));
	let parent_runs_for_handler = parent_runs.clone();
	let child_runs_for_handler = child_runs.clone();

	router
		.use_middleware(|ctx| async move {
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
		.add_handler("/items", no_input(), move |_| {
			let parent_runs = parent_runs_for_handler.clone();
			async move {
				parent_runs.fetch_add(1, Ordering::SeqCst);
				Ok("parent")
			}
		})
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), move |ctx| {
			let child_runs = child_runs_for_handler.clone();
			async move {
				child_runs.fetch_add(1, Ordering::SeqCst);
				ctx.response_proxy_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok(ctx.param("id").unwrap().to_string())
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = router
		.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		)
		.await
		.unwrap();

	assert_eq!(parent_runs.load(Ordering::SeqCst), 1);
	assert_eq!(child_runs.load(Ordering::SeqCst), 1);
	assert_eq!(
		results.middleware_proxy().unwrap().status().0,
		Some(StatusCode::ACCEPTED)
	);
	assert_eq!(
		results.results()[1].response_proxy().unwrap().status().0,
		Some(StatusCode::CREATED)
	);
	assert_eq!(
		results.results()[0].data().and_then(Value::as_str),
		Some("parent")
	);
	assert_eq!(
		results.results()[1].data().and_then(Value::as_str),
		Some("123")
	);
}

#[tokio::test]
async fn nested_parent_error_cancels_descendants() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	router
		.add_handler("/items", no_input(), move |_| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				Err::<(), _>(RouteExecutionError::Task(TaskError::from("parent failed")))
			}
		})
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), move |ctx| {
			let child_started = child_started_for_child.clone();
			async move {
				child_started.notify_one();
				ctx.exec_ctx().cancel_token().cancelled().await;
				Ok("should not complete")
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = tokio::time::timeout(
		Duration::from_millis(250),
		router.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		),
	)
	.await
	.expect("nested cancellation should not hang")
	.unwrap();

	assert!(matches!(
		results.results()[0].error(),
		Some(RouteExecutionError::Task(TaskError::Failed(_)))
	));
	assert!(matches!(
		results.results()[1].error(),
		Some(RouteExecutionError::Task(TaskError::Cancelled))
	));
}

#[tokio::test]
async fn nested_parent_response_error_cancels_descendants() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	router
		.add_handler("/items", no_input(), move |ctx| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response_proxy_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				Ok("parent")
			}
		})
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), move |ctx| {
			let child_started = child_started_for_child.clone();
			async move {
				child_started.notify_one();
				ctx.exec_ctx().cancel_token().cancelled().await;
				Ok("should not complete")
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = tokio::time::timeout(
		Duration::from_millis(250),
		router.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		),
	)
	.await
	.expect("nested response-proxy cancellation should not hang")
	.unwrap();

	assert_eq!(
		results.results()[0].response_proxy().unwrap().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert!(matches!(
		results.results()[1].error(),
		Some(RouteExecutionError::Task(TaskError::Cancelled))
	));
}

#[tokio::test]
async fn nested_parent_terminal_does_not_wait_for_uncancellable_child() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let child_started = Arc::new(Notify::new());
	let parent_can_finish = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();
	let parent_can_finish_for_parent = parent_can_finish.clone();

	router
		.add_handler("/items", no_input(), move |ctx| {
			let child_started = child_started_for_parent.clone();
			let parent_can_finish = parent_can_finish_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response_proxy_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				parent_can_finish.notify_one();
				Ok("parent")
			}
		})
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), move |_| {
			let child_started = child_started_for_child.clone();
			async move {
				child_started.notify_one();
				std::future::pending::<()>().await;
				#[allow(unreachable_code)]
				Ok("child should not matter")
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = tokio::time::timeout(
		Duration::from_millis(250),
		router.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		),
	)
	.await
	.expect("ancestor terminal effect should not wait for irrelevant descendant work")
	.unwrap();

	parent_can_finish.notified().await;
	assert_eq!(
		results.results()[0].response_proxy().unwrap().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert!(results.results()[1].data().is_none());
}

#[tokio::test]
async fn nested_parent_bad_request_input_error_cancels_descendants() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	router
		.add_handler(
			"/items",
			InputParser::<()>::callback(move |_| {
				child_started_for_parent.notify_one();
				Err(InputError::bad_request("bad input"))
			}),
			|_| async { Ok("parent") },
		)
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), move |ctx| {
			let child_started = child_started_for_child.clone();
			async move {
				child_started.notified().await;
				ctx.exec_ctx().cancel_token().cancelled().await;
				Ok("should not complete")
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = tokio::time::timeout(
		Duration::from_millis(250),
		router.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		),
	)
	.await
	.expect("nested input-error cancellation should not hang")
	.unwrap();

	assert!(
		results.results()[0]
			.error()
			.is_some_and(RouteExecutionError::is_bad_request)
	);
	assert_eq!(
		results.results()[0].response_proxy().unwrap().status().0,
		Some(StatusCode::BAD_REQUEST)
	);
	assert!(matches!(
		results.results()[1].error(),
		Some(RouteExecutionError::Task(TaskError::Cancelled))
	));
}

#[tokio::test]
async fn nested_handler_error_suppresses_own_success_effects_in_execution_results() {
	let mut router: NestedRouter<TestEnv, &'static str> = NestedRouter::default();

	router
		.add_handler("/items", no_input(), |_| async { Ok("parent") })
		.unwrap();
	router
		.add_handler("/items/:id", no_input(), |ctx| async move {
			ctx.response_proxy_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_proxy_mut().set_header(
				HeaderName::from_static("x-child-success"),
				HeaderValue::from_static("suppressed"),
			);
			Err::<(), _>(RouteExecutionError::Task(TaskError::from("child failed")))
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = router
		.run_nested_tasks(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		)
		.await
		.unwrap();

	assert!(matches!(
		results.results()[1].error(),
		Some(RouteExecutionError::Task(TaskError::Failed(_)))
	));
	let child_proxy = results.results()[1].response_proxy().unwrap();
	assert_eq!(child_proxy.status().0, Option::None);
	assert!(
		child_proxy
			.header(&HeaderName::from_static("x-child-success"))
			.is_none()
	);
}
