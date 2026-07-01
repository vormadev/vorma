//! Captured wire-contract fixtures shared with the TypeScript suite.
//!
//! This test boots a representative app through the public API and captures
//! real HTTP responses for the route-payload, redirect, build-skew, and
//! resource wire contracts. The captured fixtures are committed at
//! `packages/vorma/core/wire_contract_fixtures.json`, where the TypeScript
//! suite consumes them, so both runtimes are pinned to the same bytes.
//!
//! When the server's wire output changes intentionally, regenerate with:
//!
//! ```sh
//! VORMA_UPDATE_WIRE_FIXTURES=1 cargo test -p vorma --test wire_contract_fixtures
//! ```

use std::collections::BTreeMap;

use bytes::Bytes;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use vorma::testing::TestApp;

const FIXTURES_PATH: &str = concat!(
	env!("CARGO_MANIFEST_DIR"),
	"/../../packages/vorma/core/wire_contract_fixtures.json"
);
const UPDATE_ENV_KEY: &str = "VORMA_UPDATE_WIRE_FIXTURES";

#[derive(Default)]
struct FixtureState {
	prefix: String,
}

vorma::app!(mod fixture_app for crate::FixtureState);

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct RootViewOutput {
	section: String,
}

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
struct StoryViewInput {
	#[serde(default)]
	q: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct StoryViewOutput {
	story: String,
	query: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize, vorma::TsGen)]
struct SearchResourceInput {
	q: String,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct SearchResourceOutput {
	echo: String,
}

#[derive(Clone, Debug, Deserialize, Serialize, vorma::TsGen)]
struct StoryMutationInput {
	name: String,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct StoryMutationOutput {
	created: String,
}

const ROOT_VIEW: fixture_app::View = fixture_app::view! {
	client_file: "src/client/views/root.view.tsx";
	pattern: "/";
	input: ();
	output: RootViewOutput;

	handler: |ctx| {
		ctx.head().title("Fixture root");
		Ok(RootViewOutput {
			section: format!("{}root", ctx.state().prefix),
		})
	};
};

const STORY_VIEW: fixture_app::View = fixture_app::view! {
	client_file: "src/client/views/story.view.tsx";
	pattern: "/stories/:id";
	input: StoryViewInput;
	output: StoryViewOutput;

	handler: |ctx| {
		ctx.head()
			.title("Fixture story")
			.description("Captured story view")
			.preload("/assets/fixture-font.woff2", "font");
		Ok(StoryViewOutput {
			story: format!("{}story-{}", ctx.state().prefix, ctx.param("id")),
			query: ctx.input().q.clone(),
		})
	};
};

const SPLAT_VIEW: fixture_app::View = fixture_app::view! {
	client_file: "src/client/views/files.view.tsx";
	pattern: "/files/*";
	input: ();
	output: RootViewOutput;

	handler: |ctx| {
		Ok(RootViewOutput {
			section: format!("files:{}", ctx.splat_values().join("/")),
		})
	};
};

const REDIRECT_VIEW: fixture_app::View = fixture_app::view! {
	client_file: "src/client/views/legacy.view.tsx";
	pattern: "/legacy";
	input: ();
	output: RootViewOutput;

	handler: |ctx| {
		/*
		Redirect is a returned early exit: no fabricated output value, no
		discarded data — the exit IS the outcome.
		*/
		ctx.redirect("/stories/1")
	};
};

const BROKEN_VIEW: fixture_app::View = fixture_app::view! {
	client_file: "src/client/views/broken.view.tsx";
	pattern: "/broken";
	input: ();
	output: RootViewOutput;

	handler: |_ctx| {
		let error: vorma::BoxError = "database down".into();
		Err(error.into())
	};
};

const SEARCH_RESOURCE: fixture_app::Resource = fixture_app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/search";
	input: SearchResourceInput;
	output: SearchResourceOutput;

	handler: |ctx| {
		Ok(SearchResourceOutput {
			echo: format!("{}echo-{}", ctx.state().prefix, ctx.input().q),
		})
	};
};

const STORY_MUTATION: fixture_app::Resource = fixture_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/stories/:id";
	input: StoryMutationInput;
	output: StoryMutationOutput;

	handler: |ctx| {
		ctx.response().set_status(vorma::HttpStatusCode::CREATED);
		Ok(StoryMutationOutput {
			created: format!(
				"{}created-{}-{}",
				ctx.state().prefix,
				ctx.param("id"),
				ctx.input().name
			),
		})
	};
};

const BROKEN_RESOURCE: fixture_app::Resource = fixture_app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/broken";
	input: SearchResourceInput;
	output: SearchResourceOutput;

	handler: |_ctx| {
		let error: vorma::BoxError = "resource exploded".into();
		Err(error.into())
	};
};

fn app_config() -> vorma::AppConfig<FixtureState> {
	vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		server_target: vorma::ServerTarget {
			cargo_package: "fixture-package".to_owned(),
			cargo_bin: "fixture-server".to_owned(),
		},
		dist_dir: "target/vorma-fixture-test-dist".to_owned(),
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
		state: FixtureState {
			prefix: "fx-".to_owned(),
		},
		views: fixture_app::views![
			ROOT_VIEW,
			STORY_VIEW,
			SPLAT_VIEW,
			REDIRECT_VIEW,
			BROKEN_VIEW
		],
		resources: fixture_app::resources![SEARCH_RESOURCE, STORY_MUTATION, BROKEN_RESOURCE],
		middlewares: fixture_app::middlewares![],
		tasks_options: vorma::tasks::TasksOptions::default(),
		document: fixture_app::DocumentBuilder::new(|_ctx| async move {
			let mut document = vorma::Document::new();
			document.html().lang("en");
			document.head().meta_charset("utf-8").title("Fixture app");
			Ok(document)
		}),
		request_body_limit: 1024 * 1024,
	}
}

/// One captured request/response pair.
#[derive(Debug, Deserialize, PartialEq, Serialize)]
struct WireFixture {
	name: String,
	request: WireRequest,
	response: WireResponse,
}

#[derive(Debug, Deserialize, PartialEq, Serialize)]
struct WireRequest {
	method: String,
	path: String,
	#[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
	headers: BTreeMap<String, String>,
	#[serde(default, skip_serializing_if = "Option::is_none")]
	body_json: Option<Value>,
}

#[derive(Debug, Deserialize, PartialEq, Serialize)]
struct WireResponse {
	status: u16,
	headers: BTreeMap<String, Vec<String>>,
	#[serde(default, skip_serializing_if = "Option::is_none")]
	body_json: Option<Value>,
	#[serde(default, skip_serializing_if = "Option::is_none")]
	body_text: Option<String>,
}

#[derive(Debug, Deserialize, PartialEq, Serialize)]
struct WireFixtureFile {
	description: String,
	fixtures: Vec<WireFixture>,
}

async fn capture(app: &TestApp, name: &str, request: WireRequest) -> WireFixture {
	let mut builder = http::Request::builder()
		.method(request.method.as_str())
		.uri(&request.path);
	for (header_name, header_value) in &request.headers {
		builder = builder.header(header_name, header_value);
	}
	if request.body_json.is_some() {
		builder = builder.header("content-type", "application/json");
	}
	let body = request
		.body_json
		.as_ref()
		.map(|body| serde_json::to_vec(body).expect("fixture request body serializes"))
		.unwrap_or_default();
	let http_request = builder
		.body(Bytes::from(body))
		.expect("fixture request is valid");

	let response = app.handle_request(http_request).await;

	let status = response.status().as_u16();
	let mut headers = BTreeMap::<String, Vec<String>>::new();
	for (header_name, header_value) in response.headers() {
		headers
			.entry(header_name.as_str().to_owned())
			.or_default()
			.push(String::from_utf8_lossy(header_value.as_bytes()).into_owned());
	}
	let is_json = response
		.headers()
		.get("content-type")
		.and_then(|value| value.to_str().ok())
		.is_some_and(|value| value.starts_with("application/json"));
	let (body_json, body_text) = if is_json {
		(
			Some(serde_json::from_slice(response.body()).expect("JSON response body parses")),
			None,
		)
	} else {
		(
			None,
			Some(String::from_utf8_lossy(response.body()).into_owned()),
		)
	};
	WireFixture {
		name: name.to_owned(),
		request,
		response: WireResponse {
			status,
			headers,
			body_json,
			body_text,
		},
	}
}

fn get(path: &str) -> WireRequest {
	WireRequest {
		method: "GET".to_owned(),
		path: path.to_owned(),
		headers: BTreeMap::new(),
		body_json: None,
	}
}

fn get_with_header(path: &str, header_name: &str, header_value: &str) -> WireRequest {
	WireRequest {
		method: "GET".to_owned(),
		path: path.to_owned(),
		headers: BTreeMap::from([(header_name.to_owned(), header_value.to_owned())]),
		body_json: None,
	}
}

fn post_json(path: &str, body: Value) -> WireRequest {
	WireRequest {
		method: "POST".to_owned(),
		path: path.to_owned(),
		headers: BTreeMap::new(),
		body_json: Some(body),
	}
}

async fn capture_all() -> WireFixtureFile {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut fixtures = Vec::new();
	fixtures.push(capture(&app, "view_payload_root", get("/?vorma-json=_")).await);
	fixtures.push(
		capture(
			&app,
			"view_payload_nested_with_input",
			get("/stories/5?q=ada&vorma-json=_"),
		)
		.await,
	);
	fixtures.push(
		capture(
			&app,
			"view_payload_splat",
			get("/files/docs/readme.md?vorma-json=_"),
		)
		.await,
	);
	fixtures.push(
		capture(
			&app,
			"view_payload_server_error",
			get("/broken?vorma-json=_"),
		)
		.await,
	);
	fixtures.push(capture(&app, "view_redirect_native", get("/legacy?vorma-json=_")).await);
	fixtures.push(
		capture(
			&app,
			"view_redirect_client",
			get_with_header("/legacy?vorma-json=_", "x-accepts-client-redirect", "1"),
		)
		.await,
	);
	fixtures.push(
		capture(
			&app,
			"view_payload_build_skew",
			get("/?vorma-json=stale-build-id"),
		)
		.await,
	);
	fixtures.push(capture(&app, "resource_query", get("/api/search?q=ada")).await);
	fixtures.push(
		capture(
			&app,
			"resource_mutation",
			post_json("/api/stories/5", serde_json::json!({"name": "ada"})),
		)
		.await,
	);
	fixtures.push(capture(&app, "resource_error", get("/api/broken?q=x")).await);
	fixtures.push(capture(&app, "resource_not_found", get("/api/nope")).await);
	WireFixtureFile {
		description: "Captured wire-contract fixtures. Generated by the Rust suite from a real \
			in-memory app (crates/vorma/tests/wire_contract_fixtures.rs) and consumed by the \
			TypeScript suite, so both runtimes pin the same bytes. Regenerate with \
			VORMA_UPDATE_WIRE_FIXTURES=1 cargo test -p vorma --test wire_contract_fixtures."
			.to_owned(),
		fixtures,
	}
}

#[tokio::test]
async fn captured_wire_fixtures_match_committed_file() {
	let captured = capture_all().await;
	let rendered = format!(
		"{}\n",
		vorma_contract::contracts::to_tab_indented_json_string(&captured)
			.expect("fixtures serialize")
	);
	if std::env::var(UPDATE_ENV_KEY).is_ok() {
		std::fs::write(FIXTURES_PATH, &rendered).expect("write fixture file");
		return;
	}
	let committed = std::fs::read_to_string(FIXTURES_PATH).unwrap_or_else(|error| {
		panic!(
			"missing committed wire fixtures at {FIXTURES_PATH} ({error}); \
			run {UPDATE_ENV_KEY}=1 cargo test -p vorma --test wire_contract_fixtures"
		)
	});
	assert_eq!(
		committed, rendered,
		"server wire output drifted from committed fixtures; if intentional, regenerate with \
		{UPDATE_ENV_KEY}=1 cargo test -p vorma --test wire_contract_fixtures and review the \
		TypeScript suite against the new fixtures"
	);
}
