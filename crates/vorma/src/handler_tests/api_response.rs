use super::*;

#[test]
fn build_view_skew_response_reports_stale_client_build_before_view_handler_work() {
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
		.add_handler(
			Method::GET,
			"/ping",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"ok": true})) },
		)
		.unwrap();

	let result = router
		.execute_route(
			resource_get("/ping"),
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
		.use_middleware(|ctx| async move {
			ctx.response_effects_mut()
				.set_status(StatusCode::ACCEPTED, Option::None);
			ctx.response_effects_mut().set_header(
				HeaderName::from_static("x-middleware"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.add_handler(
			Method::GET,
			"/success",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok(json!({"ok": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
			resource_get("/success"),
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
async fn build_api_response_short_circuits_effect_redirects() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_handler(
			Method::POST,
			"/sessions",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.redirect(false, "/dashboard", Some(StatusCode::FOUND))
					.unwrap();
				Ok(json!({"unused": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
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
		.add_handler(
			Method::GET,
			"/boom",
			InputParser::<()>::default_input(),
			|_| async {
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from("boom")))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
			resource_get("/boom"),
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
async fn build_api_response_preserves_handler_owned_error_effects() {
	let mut router = Router::<(), Box<dyn Error + Send + Sync>>::new(Default::default()).unwrap();
	router
		.add_handler(
			Method::POST,
			"/bad",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::BAD_REQUEST, Some("bad input".to_owned()));
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-validation"),
					http::HeaderValue::from_static("1"),
				);
				ctx.response_effects_mut()
					.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					Box::new(crate::Error::runtime("bad input")) as Box<dyn Error + Send + Sync>,
				)))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
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
async fn build_api_response_preserves_handler_owned_redirect_effects_on_error() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.add_handler(
			Method::POST,
			"/redirect",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.redirect(false, "/login", Some(StatusCode::FOUND))
					.unwrap();
				ctx.response_effects_mut().set_header(
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
		.execute_route(
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
async fn build_api_response_suppresses_success_effects_when_handler_errors() {
	let mut router = Router::<(), &'static str>::new(Default::default()).unwrap();
	router
		.use_middleware(|ctx| async move {
			ctx.response_effects_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_effects_mut().set_header(
				HeaderName::from_static("x-vorma-middleware"),
				HeaderValue::from_static("1"),
			);
			ctx.response_effects_mut()
				.set_cookie(cookie::Cookie::new("middleware", "1"));
			Ok(())
		})
		.unwrap();
	router
		.add_handler(
			Method::POST,
			"/boom",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::ACCEPTED, Option::None);
				ctx.response_effects_mut().set_header(
					HeaderName::from_static("x-success"),
					HeaderValue::from_static("should-not-leak"),
				);
				ctx.response_effects_mut()
					.set_cookie(cookie::Cookie::new("success", "should-not-leak"));
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from("boom")))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
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
async fn build_api_response_does_not_double_apply_successful_effects_short_circuit() {
	let mut router = Router::<(), Box<dyn Error + Send + Sync>>::new(Default::default()).unwrap();
	router
		.add_handler(
			Method::POST,
			"/drifted",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::CONFLICT, Some("drifted".to_owned()));
				ctx.response_effects_mut()
					.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));
				Ok(json!({"unused": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
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
		.add_handler(
			Method::GET,
			"/typed",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut().set_header(
					CONTENT_TYPE,
					HeaderValue::from_static("application/problem+json"),
				);
				Ok(json!({"ok": true}))
			},
		)
		.unwrap();

	let result = router
		.execute_route(
			resource_get("/typed"),
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
		.use_middleware(|ctx| async move {
			ctx.response_effects_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_effects_mut().set_header(
				HeaderName::from_static("x-vorma-middleware"),
				HeaderValue::from_static("1"),
			);
			ctx.response_effects_mut()
				.set_cookie(cookie::Cookie::new("middleware", "1"));
			Ok(())
		})
		.unwrap();
	router
		.add_handler(
			Method::GET,
			"/bad",
			InputParser::<()>::callback(|_| Err(crate::mux::InputError::bad_request("bad input"))),
			|_| async { Ok(json!({"unused": true})) },
		)
		.unwrap();

	let result = router
		.execute_route(
			resource_get("/bad"),
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
