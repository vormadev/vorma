use super::*;

#[tokio::test]
async fn runtime_host_reports_method_not_allowed_for_known_api_path() {
	let dist_dir = temp_dist_dir("allow");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::POST)
				.uri("/api/ping")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "GET, HEAD");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_reports_method_not_allowed_for_unregistered_api_method() {
	let dist_dir = temp_dist_dir("global-allow");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::DELETE)
				.uri("/api/missing")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "GET, HEAD");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_method_not_allowed_has_no_body() {
	let dist_dir = temp_dist_dir("head-method-not-allowed");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::POST,
		"/submit",
		Some(ResourceKind::Mutation),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"ok": true})) },
	));
	let host = runtime_host(&dist_dir, views, resources, Middlewares::new());

	let response = host
		.handle_request(empty_request(Method::HEAD, "/api/submit"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "POST");
	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "19");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_reports_not_found_for_unknown_api_path_with_registered_method() {
	let dist_dir = temp_dist_dir("api-method-known-path-missing");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/api/missing")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest().to_client_build_id().unwrap().as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_supported_api_method_405_includes_client_build_id() {
	let dist_dir = temp_dist_dir("api-method-known-path-method-missing");
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
	resources.push(Resource::new(
		Method::POST,
		"/other",
		Some(ResourceKind::Mutation),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"ok": true})) },
	));
	let host = runtime_host(&dist_dir, views, resources, Middlewares::new());

	let response = host
		.handle_request(empty_request(Method::POST, "/api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::METHOD_NOT_ALLOWED);
	assert_eq!(response.headers().get(ALLOW).unwrap(), "GET, HEAD");
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest().to_client_build_id().unwrap().as_str()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn head_response_preserves_content_length() {
	let response = Response::new(Bytes::from_static(b"hello"));
	let response = head_response_without_body(response);

	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "5");
	assert!(response.body().is_empty());
}

#[test]
fn runtime_host_dev_refresh_script_hash_is_empty_outside_dev() {
	let dist_dir = temp_dist_dir("dev-refresh-script-hash-non-dev");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	assert_eq!(host.dev_refresh_script_content_sha256().unwrap(), "");

	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn request_path_is_under_mount_root_matches_api_base_boundaries() {
	assert!(request_path_is_under_mount_root("/api", "/api/"));
	assert!(request_path_is_under_mount_root("/api/ping", "/api/"));
	assert!(!request_path_is_under_mount_root("/apix", "/api/"));
}
