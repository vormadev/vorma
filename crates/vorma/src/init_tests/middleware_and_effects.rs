use super::*;

#[tokio::test]
async fn runtime_host_runs_api_middleware_scopes() {
	let dist_dir = temp_dist_dir("api-task-middleware");
	write_manifest(&dist_dir);
	let host = middleware_host(&dist_dir);

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
async fn runtime_host_api_preserves_handler_owned_error_effects_once() {
	let dist_dir = temp_dist_dir("api-error-effects");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::POST,
		"/drifted",
		Some(ResourceKind::Mutation),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ResourceCtx<(), &'static str, ()>| async move {
			ctx.response()
				.set_error_status(StatusCode::CONFLICT, "drifted");
			ctx.response()
				.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));
			Err::<serde_json::Value, _>(vorma_tasks::Error::from("drifted"))
		},
	));
	let host = runtime_host(&dist_dir, views, resources, Middlewares::new());

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
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::POST,
		"/moved",
		Some(ResourceKind::Mutation),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ResourceCtx<(), &'static str, ()>| async move {
			ctx.response()
				.redirect_with_status("/new-home", StatusCode::MOVED_PERMANENTLY)
				.unwrap();
			Ok(json!({"unreachable": true}))
		},
	));
	let host = runtime_host(&dist_dir, views, resources, Middlewares::new());

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
async fn runtime_host_api_suppresses_success_effects_when_handler_errors() {
	let dist_dir = temp_dist_dir("api-error-suppresses-success-effects");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::CREATED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		ctx.response()
			.set_cookie(cookie::Cookie::new("middleware", "1"));
		Ok(())
	}));
	resources.push(Resource::new(
		Method::POST,
		"/boom",
		Some(ResourceKind::Mutation),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ResourceCtx<(), &'static str, ()>| async move {
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
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let mut resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::ACCEPTED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	resources.push(Resource::new(
		Method::GET,
		"/success",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ResourceCtx<(), &'static str, ()>| async move {
			ctx.response().set_status(StatusCode::CREATED);
			Ok(json!({"ok": true}))
		},
	));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		move |_: crate::core::ResourceCtx<(), &'static str, ()>| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"pong": true}))
			}
		},
	));
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(move |ctx| {
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
	middlewares.push(Middleware::new(move |ctx| {
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
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		move |_: crate::core::ResourceCtx<(), &'static str, ()>| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"pong": true}))
			}
		},
	));
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(move |ctx| {
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
	middlewares.push(Middleware::new(move |ctx| {
		let prior_started = prior_started_for_redirect.clone();
		let redirect_done = redirect_done_for_redirect.clone();
		async move {
			prior_started.notified().await;
			ctx.response().redirect("/login").unwrap();
			redirect_done.notify_one();
			Ok(())
		}
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|_| async move {
		panic!("middleware panic");
		#[allow(unreachable_code)]
		Ok::<(), vorma_tasks::Error<&'static str>>(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let mut resources = Resources::new();
	resources.push(Resource::from_static(
		Method::POST,
		"/json",
		Some(ResourceKind::Mutation),
		crate::core::type_resolver::<serde_json::Value>,
		crate::core::type_resolver::<serde_json::Value>,
		static_json_resource,
	));
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::CREATED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		ctx.response()
			.set_cookie(cookie::Cookie::new("middleware", "1"));
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let mut resources = Resources::new();
	resources.push(Resource::from_static(
		Method::POST,
		"/form",
		Some(ResourceKind::Mutation),
		crate::core::type_resolver::<FormData>,
		crate::core::type_resolver::<serde_json::Value>,
		static_form_data_resource,
	));
	let host = runtime_host(&dist_dir, views, resources, Middlewares::new());

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
async fn runtime_host_runs_view_middleware_scopes() {
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
	let resources = Resources::new();
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
		if ctx.matched_pattern() != "/items/:id" {
			return Ok(());
		}
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-pattern-mw"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	assert_eq!(payload["views_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_middleware_success_status_does_not_short_circuit_view_handler() {
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
	let resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::ACCEPTED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::CREATED);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(payload["views_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_middleware_head_merges_before_view_head() {
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
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.head().title("Middleware Title");
		ctx.head().description("Middleware description");
		ctx.head().meta_property_content("og:type", "article");
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, Resources::new(), middlewares);

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
async fn runtime_host_view_middleware_short_circuits_view_handlers() {
	let dist_dir = temp_dist_dir("view-task-middleware-short-circuit");
	write_view_manifest(&dist_dir);
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_view = handler_runs.clone();
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let handler_runs = handler_runs_for_view.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"id": ctx.param("id")}))
			}
		},
	));
	let resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		if ctx.matched_pattern() != "/items/:id" {
			return Ok(());
		}
		ctx.response().set_status(StatusCode::UNAUTHORIZED);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::UNAUTHORIZED);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_bad_input_preserves_prior_middleware_effects() {
	let dist_dir = temp_dist_dir("view-bad-input-middleware");
	write_view_manifest(&dist_dir);
	let handler_runs = Arc::new(AtomicUsize::new(0));
	let handler_runs_for_view = handler_runs.clone();
	let mut views = Views::new();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::callback(|_| Err(InputError::bad_request("bad input"))),
		move |_: crate::core::ViewCtx<(), &'static str, ()>| {
			let handler_runs = handler_runs_for_view.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				Ok(json!({"unused": true}))
			}
		},
	));
	let resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::CREATED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		ctx.response()
			.set_cookie(cookie::Cookie::new("middleware", "1"));
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_view_suppresses_success_effects_when_handler_errors() {
	let dist_dir = temp_dist_dir("view-error-suppresses-success-effects");
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
	let resources = Resources::new();
	let mut middlewares = Middlewares::new();
	middlewares.push(Middleware::new(|ctx| async move {
		ctx.response().set_status(StatusCode::ACCEPTED);
		ctx.response().set_header(
			HeaderName::from_static("x-vorma-middleware"),
			HeaderValue::from_static("1"),
		);
		Ok(())
	}));
	let host = runtime_host(&dist_dir, views, resources, middlewares);

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
	let host = runtime_host(&dist_dir, views, Resources::new(), Middlewares::new());

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
	let host = runtime_host(&dist_dir, views, Resources::new(), Middlewares::new());

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
	let host = runtime_host(&dist_dir, views, Resources::new(), Middlewares::new());

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::TEMPORARY_REDIRECT);
	assert_eq!(response.headers().get(LOCATION).unwrap(), "/items/next");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}
