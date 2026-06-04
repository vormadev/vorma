use super::*;

#[tokio::test]
async fn middleware_runs_in_scope_order_and_merges_response_effects() {
	let mut router: Router<TestEnv, &'static str> = Router::default();
	router
		.add_handler(Method::GET, "/items/:id", no_input(), |ctx| async move {
			{
				let mut effects = ctx.response_effects_mut();
				effects.add_header(
					HeaderName::from_static("x-mux"),
					HeaderValue::from_static("handler"),
				);
			}
			Ok(ctx.param("id").unwrap().to_string())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			let mut effects = ctx.response_effects_mut();
			effects.add_header(
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
			let mut effects = ctx.response_effects_mut();
			effects.add_header(
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
			let mut effects = ctx.response_effects_mut();
			effects.add_header(
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
		.response_effects()
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
		result.response_effects().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_error_preserves_explicit_error_effects() {
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
			ctx.response_effects_mut()
				.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
			ctx.response_effects_mut().set_header(
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
		result.response_effects().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert_eq!(
		result
			.response_effects()
			.header(&HeaderName::from_static("x-authz"))
			.unwrap(),
		HeaderValue::from_static("denied")
	);
	assert_eq!(handler_runs.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn middleware_cancelled_later_sibling_does_not_mask_real_error_effects() {
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
				ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::CONFLICT)
	);
	assert_eq!(result.response_effects().status().1, "drifted");
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
			ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert!(result.response_effects().is_redirect());
	assert_eq!(result.response_effects().location(), "/ok");
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
				ctx.response_effects_mut()
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
				ctx.response_effects_mut().set_header(
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
		result.response_effects().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert_eq!(result.response_effects().location(), "/login");
	assert!(
		result
			.response_effects()
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
				ctx.response_effects_mut()
					.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
				Ok(())
			}
		})
		.unwrap();
	router
		.use_middleware(move |ctx: RequestCtx<TestEnv, &'static str, None>| {
			let later_ran = later_ran_for_later_middleware.clone();
			async move {
				ctx.response_effects_mut().set_header(
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
		result.response_effects().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert_eq!(result.response_effects().status().1, "denied");
	assert!(
		result
			.response_effects()
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
			ctx.response_effects_mut().set_header(
				HeaderName::from_static("x-prior"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert_eq!(result.response_effects().location(), "/login");
	assert_eq!(
		result
			.response_effects()
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
				ctx.response_effects_mut().set_header(
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
				ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::SEE_OTHER)
	);
	assert_eq!(result.response_effects().location(), "/login");
	assert_eq!(
		result
			.response_effects()
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
			ctx.response_effects_mut().set_header(
				HeaderName::from_static("x-prior"),
				HeaderValue::from_static("kept"),
			);
			Ok(())
		})
		.unwrap();
	router
		.use_middleware(|ctx: RequestCtx<TestEnv, &'static str, None>| async move {
			ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::FORBIDDEN)
	);
	assert_eq!(result.response_effects().status().1, "denied");
	assert_eq!(
		result
			.response_effects()
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
				ctx.response_effects_mut()
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
		result.response_effects().status().0,
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
				ctx.response_effects_mut().set_header(
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
		result.response_effects().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert!(
		result
			.response_effects()
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
			ctx.response_effects_mut().set_header(
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
		result.response_effects().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert_eq!(
		result
			.response_effects()
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
			ctx.response_effects_mut()
				.set_status(StatusCode::ACCEPTED, Option::None);
			ctx.response_effects_mut().set_header(
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
		result.response_effects().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert!(
		result
			.response_effects()
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
				ctx.response_effects_mut().set_header(
					HeaderName::from_static("x-prior"),
					HeaderValue::from_static("kept"),
				);
				ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::INTERNAL_SERVER_ERROR)
	);
	assert_eq!(
		result
			.response_effects()
			.header(&HeaderName::from_static("x-prior"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(result.response_effects().cookies()[0].name(), "prior");
	assert_eq!(result.response_effects().cookies()[0].value(), "kept");
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
	assert_eq!(
		result.response_effects().status().0,
		Some(StatusCode::BAD_REQUEST)
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
				ctx.response_effects_mut()
					.set_status(StatusCode::CREATED, Option::None);
				ctx.response_effects_mut().set_header(
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
	assert_eq!(result.response_effects().status().0, Option::None);
	assert!(
		result
			.response_effects()
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
		.add_handler(Method::GET, "/success", no_input(), move |ctx| {
			let handler_runs = handler_runs_for_handler.clone();
			async move {
				handler_runs.fetch_add(1, Ordering::SeqCst);
				ctx.response_effects_mut()
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
		result.response_effects().status().0,
		Some(StatusCode::CREATED)
	);
	assert_eq!(
		result
			.response_effects()
			.header(&HeaderName::from_static("x-middleware"))
			.unwrap(),
		HeaderValue::from_static("kept")
	);
	assert_eq!(result.data().and_then(Value::as_str), Some("handler"));
}
