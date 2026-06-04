use super::*;

#[tokio::test]
async fn runtime_host_document_defaults_feed_view_payload() {
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
async fn runtime_host_builds_document_and_view_handler_concurrently() {
	let dist_dir = temp_dist_dir("view-document-handler-parallel");
	write_view_manifest(&dist_dir);
	let document_started = Arc::new(Notify::new());
	let handler_started = Arc::new(Notify::new());
	let document_started_for_builder = document_started.clone();
	let handler_started_for_builder = handler_started.clone();
	let mut views = Views::new();
	let document_started_for_handler = document_started.clone();
	let handler_started_for_handler = handler_started.clone();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let document_started = document_started_for_handler.clone();
			let handler_started = handler_started_for_handler.clone();
			async move {
				handler_started.notify_one();
				document_started.notified().await;
				Ok(json!({"id": ctx.param("id")}))
			}
		},
	));
	let host = runtime_host_with_document(
		&dist_dir,
		views,
		Resources::new(),
		Middlewares::new(),
		DocumentBuilder::new(move |_| {
			let document_started = document_started_for_builder.clone();
			let handler_started = handler_started_for_builder.clone();
			async move {
				document_started.notify_one();
				handler_started.notified().await;
				Ok(crate::document::Document::new())
			}
		}),
	);

	let response = tokio::time::timeout(
		Duration::from_secs(1),
		host.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_")),
	)
	.await
	.expect("document and view handler should not deadlock")
	.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["views_data"][0]["id"], "42");
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
async fn runtime_host_view_document_error_cancels_running_view_handler() {
	let dist_dir = temp_dist_dir("view-document-error-cancels-handler");
	write_view_manifest(&dist_dir);
	let handler_started = Arc::new(Notify::new());
	let (cancel_tx, cancel_rx) = oneshot::channel();
	let cancel_tx = Arc::new(Mutex::new(Some(cancel_tx)));
	let mut views = Views::new();
	let handler_started_for_handler = handler_started.clone();
	let cancel_tx_for_handler = cancel_tx.clone();
	views.push(View::new(
		"/items/:id",
		"items.tsx",
		InputParser::<()>::default_input(),
		move |_ctx: crate::core::ViewCtx<(), &'static str, ()>| {
			let handler_started = handler_started_for_handler.clone();
			let cancel_tx = cancel_tx_for_handler.clone();
			async move {
				let _drop_signal = DropSignal(cancel_tx);
				handler_started.notify_one();
				std::future::pending::<vorma_tasks::Result<serde_json::Value, &'static str>>().await
			}
		},
	));
	let handler_started_for_document = handler_started.clone();
	let host = runtime_host_with_document(
		&dist_dir,
		views,
		Resources::new(),
		Middlewares::new(),
		DocumentBuilder::new(move |_| {
			let handler_started = handler_started_for_document.clone();
			async move {
				handler_started.notified().await;
				Err("document failed".to_owned())
			}
		}),
	);

	let response = tokio::time::timeout(
		Duration::from_secs(1),
		host.handle_request(empty_request(Method::GET, "/items/42")),
	)
	.await
	.expect("document error should not hang on view handler")
	.unwrap();

	assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
	tokio::time::timeout(Duration::from_secs(1), cancel_rx)
		.await
		.expect("document failure should cancel running view handler")
		.expect("view handler should report cancellation");
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
