//! Public-API integration tests for the in-memory test harness.
//!
//! These tests prove that a complete app declared through the public macros
//! can be booted and exercised at the request level with no build artifacts
//! on disk — the property the retired black-box suites lacked.

use serde::{Deserialize, Serialize};
use vorma::testing::{TEST_CLIENT_BUILD_ID, TestApp};

#[derive(Default)]
struct HarnessState {
	prefix: String,
}

vorma::app!(mod harness_app for crate::HarnessState);

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
struct HarnessViewInput {
	#[serde(default)]
	q: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct HarnessViewOutput {
	message: String,
	query: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize, vorma::TsGen)]
struct HarnessResourceInput {
	name: String,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct HarnessResourceOutput {
	greeting: String,
}

vorma::tasks::task! {
	static HARNESS_TASK: vorma::tasks::Task<String, String, vorma::Error> =
		memoized(|_ctx, id: String| async move {
			Ok(format!("task-{id}"))
		});
}

const HARNESS_VIEW: harness_app::View = harness_app::view! {
	client_file: "src/client/views/harness.view.tsx";
	pattern: "/contract/:id";
	input: HarnessViewInput;
	output: HarnessViewOutput;

	handler: |ctx| {
		ctx.head().title("Harness view");
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-harness-view"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(HarnessViewOutput {
			message: format!("{}{}", ctx.state().prefix, ctx.param("id")),
			query: ctx.input().q.clone(),
		})
	};
};

const HARNESS_TASK_VIEW: harness_app::View = harness_app::view! {
	client_file: "src/client/views/harness-task.view.tsx";
	pattern: "/task/:id";
	input: ();
	output: HarnessViewOutput;

	handler: |ctx| {
		let id = ctx.param("id").to_owned();
		let message = HARNESS_TASK
			.run(ctx.exec_ctx(), id)
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		Ok(HarnessViewOutput {
			message: (*message).clone(),
			query: None,
		})
	};
};

const HARNESS_RESOURCE: harness_app::Resource = harness_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/contract/:id";
	input: HarnessResourceInput;
	output: HarnessResourceOutput;

	handler: |ctx| {
		ctx.response().set_status(vorma::HttpStatusCode::CREATED);
		Ok(HarnessResourceOutput {
			greeting: format!(
				"{}{}:{}",
				ctx.state().prefix,
				ctx.param("id"),
				ctx.input().name
			),
		})
	};
};

fn app_config() -> vorma::AppConfig<HarnessState> {
	app_config_with_tasks_options(vorma::tasks::TasksOptions::default())
}

fn app_config_with_tasks_options(
	tasks_options: vorma::tasks::TasksOptions<vorma::Error>,
) -> vorma::AppConfig<HarnessState> {
	vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		server_target: vorma::ServerTarget {
			cargo_package: "harness-package".to_owned(),
			cargo_bin: "harness-server".to_owned(),
		},
		dist_dir: "target/vorma-harness-test-dist".to_owned(),
		public_static_base: "/assets/".to_owned(),
		frontend_config: vorma::FrontendConfig {
			ui_variant: vorma::UiVariant::React,
			js_package_manager_base_cmd: "pnpm exec".to_owned(),
			js_package_manager_dir: ".".to_owned(),
			vite_config_file: "vite.config.ts".to_owned(),
			entry_file: "src/client/entry.tsx".to_owned(),
			public_static_src_dir: "public".to_owned(),
			critical_css_file: "src/client/styles/critical.css".to_owned(),
		},
		ts_gen_config: vorma::TsGenConfig {
			out_file: "src/client/vorma.gen.ts".to_owned(),
			..vorma::TsGenConfig::default()
		},
		dev_watch_config: vorma::DevWatchConfig::default(),
		state: HarnessState {
			prefix: "harness-".to_owned(),
		},
		views: harness_app::views![HARNESS_VIEW, HARNESS_TASK_VIEW],
		resources: harness_app::resources![HARNESS_RESOURCE],
		middlewares: harness_app::middlewares![harness_app::Middleware::new(|ctx| async move {
			ctx.response().set_header(
				vorma::HttpHeaderName::from_static("x-harness-middleware"),
				vorma::HttpHeaderValue::from_static("1"),
			);
			Ok(())
		})],
		tasks_options,
		document: harness_app::DocumentBuilder::new(|_ctx| async move {
			let mut document = vorma::Document::new();
			document.html().lang("en");
			document
				.head()
				.meta_charset("utf-8")
				.title("Harness app")
				.description("In-memory harness test app");
			Ok(document)
		}),
		request_body_limit: 1024 * 1024,
	}
}

#[tokio::test]
async fn test_app_boots_in_memory_and_serves_view_json_payloads() {
	let app = TestApp::from_config(app_config()).unwrap();
	assert_eq!(app.client_build_id(), TEST_CLIENT_BUILD_ID);

	let response = app.get_view_payload("/contract/42?q=ada").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	assert_eq!(response.headers()["x-harness-view"], "1");
	assert_eq!(response.headers()["x-harness-middleware"], "1");
	assert_eq!(
		response.headers()["x-vorma-client-build-id"],
		TEST_CLIENT_BUILD_ID
	);
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(
		payload["matched_patterns"],
		serde_json::json!(["/contract/:id"])
	);
	assert_eq!(payload["views_data"][0]["message"], "harness-42");
	assert_eq!(payload["views_data"][0]["query"], "ada");
}

#[tokio::test]
async fn task_override_errors_surface_as_generic_view_payload_errors() {
	let overrides = vorma::tasks::TaskOverrides::new(vorma::tasks::TaskOverrideMode::RunUnmatched)
		.replace(&HARNESS_TASK, |_ctx, _input| async {
			Err(vorma::Error::new("injected task failure").into())
		});
	let app = TestApp::from_config(app_config_with_tasks_options(vorma::tasks::TasksOptions {
		overrides: Some(overrides),
		..vorma::tasks::TasksOptions::default()
	}))
	.unwrap();

	let response = app.get_view_payload("/task/42").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(
		payload["matched_patterns"],
		serde_json::json!(["/task/:id"])
	);
	assert_eq!(
		payload["outermost_server_err"],
		"An unexpected error occurred."
	);
}

#[tokio::test]
async fn test_app_serves_view_html_documents() {
	let app = TestApp::from_config(app_config()).unwrap();

	let response = app.get("/contract/7").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let content_type = response.headers()["content-type"].to_str().unwrap();
	assert!(content_type.starts_with("text/html"), "{content_type}");
	let html = String::from_utf8(response.body().to_vec()).unwrap();
	assert!(html.contains("Harness view"), "missing view title: {html}");
	assert!(html.contains("harness-7"), "missing view data: {html}");
}

#[tokio::test]
async fn test_app_serves_resource_mutations() {
	let app = TestApp::from_config(app_config()).unwrap();

	let response = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/contract/42",
			&HarnessResourceInput {
				name: "ada".to_owned(),
			},
		)
		.await;

	assert_eq!(response.status(), vorma::HttpStatusCode::CREATED);
	let body: serde_json::Value = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(body["greeting"], "harness-42:ada");
}

#[tokio::test]
async fn test_app_returns_not_found_for_unmatched_paths() {
	let app = TestApp::from_config(app_config()).unwrap();

	let response = app.get("/nope").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::NOT_FOUND);
}

#[tokio::test]
async fn test_app_serves_in_memory_public_assets() {
	let app = TestApp::builder(app_config())
		.with_public_asset("logo.png", &b"png-bytes"[..])
		.build()
		.unwrap();

	let public_url = app.public_url("logo.png").unwrap();
	assert_eq!(public_url, "/assets/logo.png");

	let response = app.get(&public_url).await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	assert_eq!(response.body().as_ref(), b"png-bytes");
	assert_eq!(response.headers()["content-type"], "image/png");
}
