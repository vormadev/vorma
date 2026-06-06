use super::*;

#[tokio::test]
async fn runtime_host_serves_public_assets_from_manifest() {
	let dist_dir = temp_dist_dir("static");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/app.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::GET)
				.uri("/static/app.css")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(
		response.headers().get(CACHE_CONTROL).unwrap(),
		PUBLIC_ASSET_CACHE_CONTROL
	);
	assert_eq!(response.body(), &Bytes::from_static(b"body{}"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_serves_public_assets_from_percent_decoded_path() {
	let dist_dir = temp_dist_dir("static-percent-decoded");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/app.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/static/%61pp.css"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.body(), &Bytes::from_static(b"body{}"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_rejects_invalid_percent_decoded_path_utf8() {
	let dist_dir = temp_dist_dir("invalid-path-utf8");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/static/%FF.css"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::BAD_REQUEST);
	assert_eq!(response.body(), &Bytes::from_static(b"Bad Request\n"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_non_root_public_base_returns_404_for_unmanifested_assets() {
	let dist_dir = temp_dist_dir("static-unmanifested-non-root");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/missing.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/static/missing.css"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_root_public_base_falls_through_for_unmanifested_paths() {
	let dist_dir = temp_dist_dir("static-unmanifested-root");
	write_root_base_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	let response = host
		.handle_request(empty_request(Method::GET, "/items/42?vorma-json=_"))
		.await
		.unwrap();
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(payload["views_data"][0]["id"], "42");
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_public_asset_preserves_content_length_without_body() {
	let dist_dir = temp_dist_dir("static-head");
	write_manifest(&dist_dir);
	fs::write(dist_dir.join(".vorma/static/public/app.css"), b"body{}").unwrap();
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::HEAD)
				.uri("/static/app.css")
				.body(Bytes::new())
				.unwrap(),
		)
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.headers().get(CONTENT_LENGTH).unwrap(), "6");
	assert!(response.body().is_empty());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_resources_under_api_base() {
	let dist_dir = temp_dist_dir("api");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

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
	assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_resources_under_percent_decoded_api_base() {
	let dist_dir = temp_dist_dir("api-percent-decoded");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	for uri in ["/%61pi/ping", "/api%2Fping"] {
		let response = host
			.handle_request(empty_request(Method::GET, uri))
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	}
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_resources_under_manifest_api_mount_root() {
	let dist_dir = temp_dist_dir("api-manifest-mount-root");
	let manifest = write_manifest_with_api_mount_root(&dist_dir, "/server-api/");
	let mut config = cfg(&dist_dir);
	config.path_config.api_base = "/wrong-api/".to_owned();
	let views = Views::new();
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let middlewares = Middlewares::new();
	let host = RuntimeHost::new(default_app(config, views, resources, middlewares)).unwrap();

	let response = host
		.handle_request(empty_request(Method::GET, "/server-api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::OK);
	assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	assert_eq!(
		response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
		manifest.to_client_build_id().unwrap().as_str()
	);

	let response = host
		.handle_request(empty_request(Method::GET, "/wrong-api/ping"))
		.await
		.unwrap();

	assert_eq!(response.status(), StatusCode::NOT_FOUND);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_rejects_root_api_mount_root_from_manifest() {
	let dist_dir = temp_dist_dir("api-root-mount-rejected");
	write_manifest_with_api_mount_root(&dist_dir, "/");
	let views = Views::new();
	let resources = Resources::new();
	let middlewares = Middlewares::new();
	let error = RuntimeHost::new(default_app(cfg(&dist_dir), views, resources, middlewares))
		.err()
		.unwrap();

	assert_eq!(
		error,
		"error initializing runtime assets: api_base must be a non-root path prefix such as /api/"
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_rejects_public_static_base_under_api_mount_root() {
	let dist_dir = temp_dist_dir("api-static-shadow-rejected");
	let mut manifest = manifest();
	manifest.public_static_base_path = "/api/assets/".to_owned();
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		serde_json::to_vec(&manifest).unwrap(),
	)
	.unwrap();

	let error = RuntimeHost::new(default_app(
		cfg(&dist_dir),
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	))
	.err()
	.unwrap();

	assert_eq!(
		error,
		"error initializing runtime assets: public_static_base \"/api/assets/\" must not be under api_base \"/api/\""
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_resources_under_normalized_api_base() {
	let dist_dir = temp_dist_dir("api-normalized");
	write_manifest(&dist_dir);
	let mut config = cfg(&dist_dir);
	config.path_config.api_base = "api".to_owned();
	let views = Views::new();
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/ping",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"pong": true})) },
	));
	let middlewares = Middlewares::new();
	let host = RuntimeHost::new(default_app(config, views, resources, middlewares)).unwrap();

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
	assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_dispatches_api_root_at_mount_root() {
	let dist_dir = temp_dist_dir("api-root");
	write_manifest(&dist_dir);
	let views = Views::new();
	let mut resources = Resources::new();
	resources.push(Resource::new(
		Method::GET,
		"/",
		Some(ResourceKind::Query),
		InputParser::<()>::default_input(),
		|_: crate::core::ResourceCtx<(), &'static str, ()>| async { Ok(json!({"root": true})) },
	));
	let middlewares = Middlewares::new();
	let host = runtime_host(&dist_dir, views, resources, middlewares);

	for uri in ["/api", "/api/"] {
		let response = host
			.handle_request(empty_request(Method::GET, uri))
			.await
			.unwrap();

		assert_eq!(response.status(), StatusCode::OK);
		assert_eq!(response.body(), &Bytes::from_static(br#"{"root":true}"#));
	}
	fs::remove_dir_all(dist_dir).unwrap();
}

#[tokio::test]
async fn runtime_host_head_api_preserves_content_length_without_body() {
	let dist_dir = temp_dist_dir("api-head");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	let response = host
		.handle_request(
			Request::builder()
				.method(Method::HEAD)
				.uri("/api/ping")
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

#[test]
fn runtime_host_public_url_reads_manifest() {
	let dist_dir = temp_dist_dir("public-url");
	write_manifest(&dist_dir);
	let host = host(&dist_dir);

	assert_eq!(host.public_url(" /app.css ").unwrap(), "/static/app.css");
	assert!(host.public_url("missing.css").is_err());
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_client_build_id_and_critical_css_hash_use_manifest() {
	let dist_dir = temp_dist_dir("manifest-helpers");
	write_view_manifest(&dist_dir);
	let host = view_host(&dist_dir);

	assert_eq!(
		host.client_build_id().unwrap(),
		manifest_with_view_assets().to_client_build_id().unwrap()
	);
	assert_eq!(
		host.critical_css_content_sha256().unwrap(),
		manifest_with_view_assets().critical_css_content_sha256()
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_init_requires_manifest() {
	let dist_dir = temp_dist_dir("missing-manifest");
	let error = match RuntimeHost::new(default_app(
		cfg(&dist_dir),
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject missing manifest"),
		Err(error) => error,
	};

	assert!(error.contains("error initializing runtime assets"));
	assert!(error.contains("vorma.manifest.prod.json"));
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_init_rejects_empty_root_dir() {
	let error = match RuntimeHost::new(default_app(
		Config::default(),
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject empty root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "root_dir cannot be empty");
}

#[test]
fn runtime_host_init_rejects_relative_root_dir() {
	let mut config = Config {
		root_dir: PathBuf::from("."),
		dist_dir: ".".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject relative root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "root_dir must be absolute");
}

#[test]
fn runtime_host_init_rejects_missing_root_dir() {
	let nanos = SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.unwrap()
		.as_nanos();
	let root_dir = std::env::temp_dir().join(format!(
		"vorma-runtime-host-missing-root-{}-{nanos}",
		std::process::id(),
	));
	let mut config = Config {
		root_dir: root_dir.clone(),
		dist_dir: ".".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject missing root_dir"),
		Err(error) => error,
	};

	assert_eq!(
		error,
		format!("root dir does not exist: {}", root_dir.display())
	);
}

#[test]
fn runtime_host_init_rejects_file_root_dir() {
	let dist_dir = temp_dist_dir("file-root-dir");
	let root_file = dist_dir.join("root-file");
	fs::write(&root_file, b"not a directory").unwrap();
	let mut config = Config {
		root_dir: root_file.clone(),
		dist_dir: ".".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject file root_dir"),
		Err(error) => error,
	};

	assert_eq!(
		error,
		format!("root dir is not a directory: {}", root_file.display())
	);
	fs::remove_dir_all(dist_dir).unwrap();
}

#[test]
fn runtime_host_init_rejects_relative_dist_dir_escape() {
	let root_dir = temp_dist_dir("relative-dist-escape");
	let mut config = Config {
		root_dir: root_dir.clone(),
		dist_dir: "../outside".to_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject dist_dir outside root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "dist_dir must be inside root_dir");
	fs::remove_dir_all(root_dir).unwrap();
}

#[test]
fn runtime_host_init_rejects_absolute_dist_dir_outside_root() {
	let root_dir = temp_dist_dir("absolute-dist-outside-root");
	let outside = temp_dist_dir("absolute-dist-outside-dist");
	let mut config = Config {
		root_dir: root_dir.clone(),
		dist_dir: outside.to_string_lossy().into_owned(),
		..Config::default()
	};
	config.path_config.api_base = "/api/".to_owned();
	config.path_config.public_static_base = "/static/".to_owned();

	let error = match RuntimeHost::new(default_app(
		config,
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject absolute dist_dir outside root_dir"),
		Err(error) => error,
	};

	assert_eq!(error, "dist_dir must be inside root_dir");
	fs::remove_dir_all(root_dir).unwrap();
	fs::remove_dir_all(outside).unwrap();
}

#[test]
fn runtime_host_init_rejects_invalid_manifest() {
	let dist_dir = temp_dist_dir("invalid-manifest");
	fs::write(
		dist_dir.join(".vorma/static/vorma.manifest.prod.json"),
		b"not json",
	)
	.unwrap();

	let error = match RuntimeHost::new(default_app(
		cfg(&dist_dir),
		Views::new(),
		Resources::new(),
		Middlewares::new(),
	)) {
		Ok(_) => panic!("runtime host init should reject invalid manifest"),
		Err(error) => error,
	};

	assert!(error.contains("error initializing runtime assets"));
	assert!(error.contains("error parsing prod manifest"));
	fs::remove_dir_all(dist_dir).unwrap();
}
