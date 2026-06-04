use super::*;

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
