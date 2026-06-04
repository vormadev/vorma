use super::*;

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
	let resources = Resources::new();
	let middlewares = Middlewares::new();
	let mut app = default_app(cfg(&dist_dir), views, resources, middlewares);
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
		Resources::new(),
		Middlewares::new(),
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
	let resources = Resources::new();
	let middlewares = Middlewares::new();
	let mut app = default_app(cfg(&dist_dir), views, resources, middlewares);
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
		Resources::new(),
		Middlewares::new(),
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
	assert_eq!(payload["views_data"][0]["id"], "42");
	assert_eq!(
		payload["views_data"][0]["filter"],
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
	let host = runtime_host(&dist_dir, views, Resources::new(), Middlewares::new());

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
	let host = runtime_host(&dist_dir, views, Resources::new(), Middlewares::new());

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
	assert_eq!(payload["views_data"][0]["marker"], "from-extension");
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
		Resources::new(),
		Middlewares::new(),
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
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/pending",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ResourceCtx<(), &'static str, ()>| {
			let token_tx = token_tx.clone();
			async move {
				if let Some(token_tx) = token_tx.lock().unwrap().take() {
					let _ = token_tx.send(ctx.exec_ctx().cancel_token().clone());
				}
				std::future::pending::<vorma_tasks::Result<serde_json::Value, &'static str>>().await
			}
		},
	));
	let host = runtime_host(&dist_dir, views, resources, Middlewares::new());
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
	let host = runtime_host(&dist_dir, views, Resources::new(), Middlewares::new());

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["views_data"][0]["app_css"], "/static/app.css");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_api_context_resolves_public_urls() {
	let dist_dir = temp_dist_dir("api-public-url");
	write_manifest(&dist_dir);
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/asset",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|ctx: crate::core::ResourceCtx<(), &'static str, ()>| async move {
			Ok(json!({"app_css": ctx.public_url("app.css").unwrap()}))
		},
	));
	let host = runtime_host(&dist_dir, Views::new(), resources, Middlewares::new());

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
