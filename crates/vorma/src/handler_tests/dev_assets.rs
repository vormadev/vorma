use super::*;

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
async fn build_view_response_suppresses_success_effects_when_view_errors() {
	let mut views = crate::mux::NestedRouter::<(), Box<dyn Error + Send + Sync>>::default();
	views
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-view-header"),
					http::HeaderValue::from_static("1"),
				);
				let error: Box<dyn Error + Send + Sync> = Box::new(ViewError {
					client_msg: "client visible".to_owned(),
					err: Option::None,
				});
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(error)))
			},
		)
		.unwrap();
	let manifest = manifest_with_views(&[("/items", "/items.js")]);
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
async fn build_view_response_detects_view_error_by_index_not_message_text() {
	let mut views = crate::mux::NestedRouter::<(), EmptyClientMsgError>::default();
	views
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
					EmptyClientMsgError,
				)))
			},
		)
		.unwrap();
	let manifest = manifest_with_views(&[("/items", "/items.js")]);
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
