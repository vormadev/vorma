use super::*;

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
		.execute_view_stack(
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
	assert_eq!(results.view_results().len(), 2);
	assert_eq!(results.matched_patterns(), ["/parallel", "/parallel/:id"]);
	assert_eq!(results.terminal_boundary(), Option::None);
	assert_eq!(results.view_results()[0].pattern(), "/parallel");
	assert_eq!(results.view_results()[1].pattern(), "/parallel/:id");
	assert_eq!(
		results.view_results()[0].data().and_then(Value::as_str),
		Some("parent-ok")
	);
	assert_eq!(
		results.view_results()[1].data().and_then(Value::as_str),
		Some("123")
	);
}

#[tokio::test]
async fn nested_middleware_terminal_effects_suppresses_matched_handlers() {
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
			ctx.response_effects_mut()
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
		.execute_view_stack(
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
	assert_eq!(
		results.terminal_boundary(),
		Some(ViewStackTerminalBoundary::Middleware)
	);
	assert_eq!(results.matched_patterns(), ["/items", "/items/:id"]);
	let middleware_effects = results.middleware_effects();
	assert_eq!(middleware_effects.status().0, Some(StatusCode::FOUND));
	assert_eq!(middleware_effects.location(), "/login");
	assert!(
		results
			.view_results()
			.iter()
			.all(|result| result.data().is_none())
	);
	assert!(
		results
			.view_results()
			.iter()
			.all(|result| !result.ran_task())
	);
	assert!(results.view_results().iter().all(|result| {
		result
			.response_effects()
			.is_some_and(|effects| !effects.is_terminal_response())
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
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				Ok(ctx.param("id").unwrap().to_string())
			}
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = router
		.execute_view_stack(
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
		results.middleware_effects().status().0,
		Some(StatusCode::ACCEPTED)
	);
	assert_eq!(
		results.view_results()[1]
			.response_effects()
			.unwrap()
			.status()
			.0,
		Some(StatusCode::CREATED)
	);
	assert_eq!(
		results.view_results()[0].data().and_then(Value::as_str),
		Some("parent")
	);
	assert_eq!(
		results.view_results()[1].data().and_then(Value::as_str),
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
		router.execute_view_stack(
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
		results.view_results()[0].error(),
		Some(RouteExecutionError::Task(TaskError::Failed(_)))
	));
	assert_eq!(
		results.terminal_boundary(),
		Some(ViewStackTerminalBoundary::View { index: 0 })
	);
	assert!(matches!(
		results.view_results()[1].error(),
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
				ctx.response_effects_mut()
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
		router.execute_view_stack(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		),
	)
	.await
	.expect("nested response-effects cancellation should not hang")
	.unwrap();

	assert_eq!(
		results.view_results()[0]
			.response_effects()
			.unwrap()
			.status()
			.0,
		Some(StatusCode::FORBIDDEN)
	);
	assert!(matches!(
		results.view_results()[1].error(),
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
				ctx.response_effects_mut()
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
		router.execute_view_stack(
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
		results.view_results()[0]
			.response_effects()
			.unwrap()
			.status()
			.0,
		Some(StatusCode::FORBIDDEN)
	);
	assert!(results.view_results()[1].data().is_none());
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
		router.execute_view_stack(
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
		results.view_results()[0]
			.error()
			.is_some_and(RouteExecutionError::is_bad_request)
	);
	assert_eq!(
		results.view_results()[0]
			.response_effects()
			.unwrap()
			.status()
			.0,
		Some(StatusCode::BAD_REQUEST)
	);
	assert!(matches!(
		results.view_results()[1].error(),
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
			ctx.response_effects_mut()
				.set_status(StatusCode::CREATED, Option::None);
			ctx.response_effects_mut().set_header(
				HeaderName::from_static("x-child-success"),
				HeaderValue::from_static("suppressed"),
			);
			Err::<(), _>(RouteExecutionError::Task(TaskError::from("child failed")))
		})
		.unwrap();

	let matches = router.find_nested_matches("/items/123").unwrap().unwrap();
	let (state, exec_ctx) = default_runtime();
	let results = router
		.execute_view_stack(
			state,
			exec_ctx,
			RawRequest::get("/items/123"),
			matches,
			empty_public_filemap(),
		)
		.await
		.unwrap();

	assert!(matches!(
		results.view_results()[1].error(),
		Some(RouteExecutionError::Task(TaskError::Failed(_)))
	));
	let child_effects = results.view_results()[1].response_effects().unwrap();
	assert_eq!(child_effects.status().0, Option::None);
	assert!(
		child_effects
			.header(&HeaderName::from_static("x-child-success"))
			.is_none()
	);
}
