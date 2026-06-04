use super::*;

#[tokio::test]
async fn build_view_response_exposes_view_error_client_msg() {
	let mut views = crate::mux::NestedRouter::<(), Box<dyn Error + Send + Sync>>::default();
	views
		.add_handler("/items", InputParser::<()>::default_input(), |_| async {
			let error: Box<dyn Error + Send + Sync> = Box::new(ViewError {
				client_msg: "client visible".to_owned(),
				err: Some("server hidden".into()),
			});
			Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(error)))
		})
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

	assert_eq!(payload["outermost_server_err"], "client visible");
	assert_eq!(payload["outermost_server_err_idx"], 0);
}

#[tokio::test]
async fn build_view_response_input_bad_request_errors_are_bad_requests() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
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
	views
		.add_handler(
			"/items",
			InputParser::<()>::callback(|_| Err(crate::mux::InputError::bad_request("bad input"))),
			|_| async { Ok(json!({"unused": true})) },
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

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.headers().get("x-vorma-middleware").unwrap(), "1");
	assert_eq!(response.headers().get(SET_COOKIE).unwrap(), "middleware=1");
	assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
}

#[tokio::test]
async fn build_view_response_terminal_middleware_does_not_require_view_modules() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.use_middleware(|ctx| async move {
			ctx.response_effects_mut()
				.redirect(false, "/login", Some(StatusCode::FOUND))
				.unwrap();
			Ok(())
		})
		.unwrap();
	views
		.add_handler("/items", InputParser::<()>::default_input(), |_| async {
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
async fn build_view_response_middleware_success_status_does_not_short_circuit_view_handlers() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
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
	views
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok(json!({"loaded": true}))
			},
		)
		.unwrap();
	let manifest = manifest_with_views(&[("/items", "/items.js")]);

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
	assert_eq!(payload["views_data"], json!([{"loaded": true}]));
}

#[tokio::test]
async fn build_view_response_preserves_handler_owned_error_effects() {
	let mut views = crate::mux::NestedRouter::<(), Box<dyn Error + Send + Sync>>::default();
	views
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::BAD_REQUEST, Some("bad input".to_owned()));
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-validation"),
					http::HeaderValue::from_static("1"),
				);
				let error: Box<dyn Error + Send + Sync> =
					Box::new(crate::Error::runtime("bad input"));
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
		.add_handler("/items", InputParser::<()>::default_input(), move |ctx| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response_effects_mut()
					.redirect(false, "/login", Some(StatusCode::FOUND))
					.unwrap();
				Ok(json!({"parent": true}))
			}
		})
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let child_started = child_started_for_child.clone();
				async move {
					ctx.response_effects_mut().set_header(
						http::header::HeaderName::from_static("x-child"),
						HeaderValue::from_static("leaked"),
					);
					child_started.notify_one();
					Ok(json!({"child": true}))
				}
			},
		)
		.unwrap();
	let manifest = manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]);
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
async fn build_view_response_suppresses_descendant_effects_after_ancestor_view_error() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	let child_started = Arc::new(Notify::new());
	let child_started_for_parent = child_started.clone();
	let child_started_for_child = child_started.clone();

	views
		.add_handler("/items", InputParser::<()>::default_input(), move |_| {
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
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let child_started = child_started_for_child.clone();
				async move {
					ctx.response_effects_mut().set_header(
						http::header::HeaderName::from_static("x-child"),
						HeaderValue::from_static("leaked"),
					);
					child_started.notify_one();
					Ok(json!({"child": true}))
				}
			},
		)
		.unwrap();
	let manifest = manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]);
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
	assert!(payload.get("views_data").is_none());
}

#[tokio::test]
async fn build_view_response_deepest_success_status_wins_and_keeps_all_success_effects() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();

	views
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::ACCEPTED, Option::None);
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			},
		)
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				ctx.response_effects_mut().set_header(
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
		manifest: &manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
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
		payload["views_data"],
		json!([{"parent": true}, {"child": true}])
	);
}

#[tokio::test]
async fn build_view_response_child_redirect_keeps_parent_effects_and_short_circuits_output() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();

	views
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			},
		)
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut()
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
		manifest: &manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
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
		.add_handler("/items", InputParser::<()>::default_input(), move |ctx| {
			let parent_started = parent_started_for_parent.clone();
			let child_redirected = child_redirected_for_parent.clone();
			async move {
				parent_started.notify_one();
				child_redirected.notified().await;
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			}
		})
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let parent_started = parent_started_for_child.clone();
				let child_redirected = child_redirected_for_child.clone();
				async move {
					parent_started.notified().await;
					ctx.response_effects_mut()
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
			manifest: &manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
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
		.add_handler("/items", InputParser::<()>::default_input(), move |ctx| {
			let child_started = child_started_for_parent.clone();
			async move {
				child_started.notified().await;
				ctx.response_effects_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			}
		})
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let child_started = child_started_for_child.clone();
				async move {
					ctx.response_effects_mut().set_header(
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
		manifest: &manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]),
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
		.add_handler(
			"/items",
			InputParser::<()>::default_input(),
			|ctx| async move {
				ctx.response_effects_mut().set_header(
					http::header::HeaderName::from_static("x-parent"),
					HeaderValue::from_static("1"),
				);
				Ok(json!({"parent": true}))
			},
		)
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			move |ctx| {
				let grandchild_started = grandchild_started_for_child.clone();
				async move {
					grandchild_started.notified().await;
					ctx.response_effects_mut()
						.set_status(StatusCode::CREATED, Option::None);
					ctx.response_effects_mut().set_header(
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
		.add_handler(
			"/items/:id/details",
			InputParser::<()>::default_input(),
			move |ctx| {
				let grandchild_started = grandchild_started_for_grandchild.clone();
				async move {
					ctx.response_effects_mut().set_header(
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
		manifest: &manifest_with_views(&[
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
	assert_eq!(payload["views_data"], json!([{"parent": true}]));
}

#[tokio::test]
async fn build_view_response_keeps_match_metadata_when_payload_truncates_on_parent_error() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_handler("/items", InputParser::<()>::default_input(), |_| async {
			Err::<Value, _>(RouteExecutionError::Task(vorma_tasks::Error::from(
				"parent failed",
			)))
		})
		.unwrap();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"child": true})) },
		)
		.unwrap();
	let mut manifest = manifest_with_views(&[("/items", "/items.js"), ("/items/:id", "/item.js")]);
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
	assert!(payload.get("views_data").is_none());
}

#[tokio::test]
async fn build_view_response_errors_when_manifest_missing_search_schema_for_matched_pattern() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_handler("/", InputParser::<()>::default_input(), |_| async {
			Ok(json!({"root": true}))
		})
		.unwrap();
	let manifest = Manifest {
		client_views: BTreeMap::from([(
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
async fn build_view_response_truncates_route_assets_after_first_view_error() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_handler("/items", InputParser::<()>::default_input(), |_| async {
			Ok(json!({"parent": true}))
		})
		.unwrap();
	views
		.add_handler(
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
		.add_handler(
			"/items/:id/details",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"grandchild": true})) },
		)
		.unwrap();
	let mut manifest = manifest_with_client_view_modules(&[
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
	assert_eq!(payload["views_data"], json!([{"parent": true}]));
}

#[tokio::test]
async fn build_view_response_prod_html_preloads_dedupe_route_and_core_assets() {
	let mut views = crate::mux::NestedRouter::<(), &'static str>::default();
	views
		.add_handler(
			"/items/:id",
			InputParser::<()>::default_input(),
			|_| async { Ok(json!({"id": "123"})) },
		)
		.unwrap();
	let mut manifest = manifest_with_client_view_modules(&[(
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
