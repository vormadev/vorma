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

/*
The remaining harness routes exist to exercise `vorma::testing::TestSession`
against a real login-style cookie flow: a session cookie set by one
resource, read back by another, alongside a second, independent cookie name
to prove the jar holds more than one entry and only overwrites by matching
name.
*/
const HARNESS_SESSION_COOKIE: &str = "harness_session";
const HARNESS_THEME_COOKIE: &str = "harness_theme";

fn harness_cookie_value(headers: &vorma::HttpHeaderMap, name: &str) -> Option<String> {
	let header = headers.get("cookie")?.to_str().ok()?;
	for piece in vorma::HttpCookie::split_parse(header.to_owned()) {
		let Ok(cookie) = piece else { continue };
		if cookie.name() == name {
			return Some(cookie.value().to_owned());
		}
	}
	None
}

#[derive(Clone, Debug, Deserialize, Serialize, vorma::TsGen)]
struct HarnessLoginInput {
	username: String,
}

#[derive(Clone, Debug, PartialEq, Serialize, vorma::TsGen)]
struct HarnessLoginOutput {
	username: String,
}

const HARNESS_LOGIN_RESOURCE: harness_app::Resource = harness_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/login";
	input: HarnessLoginInput;
	output: HarnessLoginOutput;

	handler: |ctx| {
		let username = ctx.input().username.clone();
		ctx.response().set_cookie(
			vorma::HttpCookie::build((HARNESS_SESSION_COOKIE, username.clone()))
				.path("/")
				.build(),
		);
		Ok(HarnessLoginOutput { username })
	};
};

const HARNESS_SET_THEME_RESOURCE: harness_app::Resource = harness_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/theme/:value";
	input: ();
	output: ();

	handler: |ctx| {
		ctx.response().set_cookie(
			vorma::HttpCookie::build((HARNESS_THEME_COOKIE, ctx.param("value").to_owned()))
				.path("/")
				.build(),
		);
		Ok(())
	};
};

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize, vorma::TsGen)]
struct HarnessWhoamiOutput {
	username: Option<String>,
	theme: Option<String>,
}

const HARNESS_WHOAMI_RESOURCE: harness_app::Resource = harness_app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/whoami";
	input: ();
	output: HarnessWhoamiOutput;

	handler: |ctx| {
		Ok(HarnessWhoamiOutput {
			username: harness_cookie_value(ctx.request().headers(), HARNESS_SESSION_COOKIE),
			theme: harness_cookie_value(ctx.request().headers(), HARNESS_THEME_COOKIE),
		})
	};
};

const HARNESS_LOGOUT_RESOURCE: harness_app::Resource = harness_app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/logout";
	input: ();
	output: ();

	handler: |ctx| {
		ctx.response().set_cookie(
			vorma::HttpCookie::build((HARNESS_SESSION_COOKIE, ""))
				.path("/")
				.removal()
				.build(),
		);
		Ok(())
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
		resources: harness_app::resources![
			HARNESS_RESOURCE,
			HARNESS_LOGIN_RESOURCE,
			HARNESS_SET_THEME_RESOURCE,
			HARNESS_WHOAMI_RESOURCE,
			HARNESS_LOGOUT_RESOURCE
		],
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

async fn login<'a>(app: &'a TestApp, username: &str) -> vorma::testing::TestSession<'a> {
	let mut session = app.session();
	let response = session
		.request_json(
			vorma::HttpMethod::POST,
			"/api/login",
			&HarnessLoginInput {
				username: username.to_owned(),
			},
		)
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	session
}

async fn whoami(session: &mut vorma::testing::TestSession<'_>) -> HarnessWhoamiOutput {
	let response = session.get("/api/whoami").await;
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	serde_json::from_slice(response.body()).unwrap()
}

#[tokio::test]
async fn set_cookie_headers_reads_the_full_typed_cookie_not_just_a_name_value_pair() {
	use vorma::testing::TestResponseCookies;

	let app = TestApp::from_config(app_config()).unwrap();
	let response = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/login",
			&HarnessLoginInput {
				username: "ada".to_owned(),
			},
		)
		.await;

	let cookies = response.set_cookie_headers();
	assert_eq!(cookies.len(), 1);
	assert_eq!(cookies[0].name(), HARNESS_SESSION_COOKIE);
	assert_eq!(cookies[0].value(), "ada");
	assert_eq!(cookies[0].path(), Some("/"));
}

#[tokio::test]
async fn test_app_itself_never_carries_cookies_across_requests() {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = login(&app, "ada").await;
	// The session absorbed the cookie...
	assert_eq!(whoami(&mut session).await.username.as_deref(), Some("ada"));
	// ...but a bare, non-session request on the SAME app never carries it:
	// TestApp holds no hidden jar of its own.
	let response = app.get("/api/whoami").await;
	let body: HarnessWhoamiOutput = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(body.username, None);
}

#[tokio::test]
async fn two_sessions_against_the_same_app_never_share_a_jar() {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut ada = login(&app, "ada").await;
	let mut lin = login(&app, "lin").await;

	assert_eq!(whoami(&mut ada).await.username.as_deref(), Some("ada"));
	assert_eq!(whoami(&mut lin).await.username.as_deref(), Some("lin"));
}

#[tokio::test]
async fn session_login_then_authorized_request_carries_the_cookie_forward() {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = login(&app, "ada").await;

	let identity = whoami(&mut session).await;
	assert_eq!(identity.username.as_deref(), Some("ada"));
}

#[tokio::test]
async fn session_jar_overwrites_by_name_instead_of_accumulating() {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = app.session();

	let first = session
		.request_json(
			vorma::HttpMethod::POST,
			"/api/login",
			&HarnessLoginInput {
				username: "ada".to_owned(),
			},
		)
		.await;
	assert_eq!(first.status(), vorma::HttpStatusCode::OK);
	assert_eq!(whoami(&mut session).await.username.as_deref(), Some("ada"));

	// Logging in again under the same cookie name must REPLACE the jar
	// entry, not add a second `harness_session` pair alongside it (which
	// would make the merged `Cookie` header ambiguous).
	let second = session
		.request_json(
			vorma::HttpMethod::POST,
			"/api/login",
			&HarnessLoginInput {
				username: "lin".to_owned(),
			},
		)
		.await;
	assert_eq!(second.status(), vorma::HttpStatusCode::OK);
	assert_eq!(whoami(&mut session).await.username.as_deref(), Some("lin"));
}

#[tokio::test]
async fn session_jar_holds_multiple_distinct_cookie_names_at_once() {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = login(&app, "ada").await;

	let theme_response = session
		.request(vorma::HttpMethod::POST, "/api/theme/dark")
		.send()
		.await;
	assert_eq!(theme_response.status(), vorma::HttpStatusCode::OK);

	let identity = whoami(&mut session).await;
	assert_eq!(identity.username.as_deref(), Some("ada"));
	assert_eq!(identity.theme.as_deref(), Some("dark"));
}

#[tokio::test]
async fn session_request_merges_the_jar_with_an_explicit_cookie_into_one_header() {
	/*
	The `Cookie` request header is a single `name=value; ...` line by the
	HTTP spec, and `HeaderMap::get` (what real app code reads through, the
	same accessor board's `session::token_of` uses) only ever returns the
	FIRST header line for a repeated name. A session request that carried
	the jar as one `Cookie:` line and an explicit `.cookie(...)` pair as a
	SECOND `Cookie:` line would silently make one of the two invisible to
	the app under test — this proves both are actually merged into one line
	instead.
	*/
	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = login(&app, "ada").await;

	let response = session
		.request(vorma::HttpMethod::POST, "/api/theme/dark")
		.cookie("harness_extra", "present")
		.send()
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);

	let identity = whoami(&mut session).await;
	assert_eq!(
		identity.username.as_deref(),
		Some("ada"),
		"the jar's own cookie must still have reached the app"
	);
	assert_eq!(
		identity.theme.as_deref(),
		Some("dark"),
		"the theme set by the same request must have reached the app too"
	);
}

#[tokio::test]
async fn session_request_explicit_cookie_overrides_a_same_named_jar_entry() {
	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = login(&app, "ada").await;
	assert_eq!(whoami(&mut session).await.username.as_deref(), Some("ada"));

	// An explicit `.cookie(HARNESS_SESSION_COOKIE, ...)` on one request must
	// win over the jar's own same-named entry for THAT request only.
	let response = session
		.request(vorma::HttpMethod::GET, "/api/whoami")
		.cookie(HARNESS_SESSION_COOKIE, "lin")
		.send()
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let body: HarnessWhoamiOutput = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(body.username.as_deref(), Some("lin"));

	// The jar itself is untouched by a per-request override: the next
	// request without an explicit cookie goes back to "ada".
	assert_eq!(whoami(&mut session).await.username.as_deref(), Some("ada"));
}

#[tokio::test]
async fn session_jar_honors_cookie_clearing_not_just_overwriting() {
	use vorma::testing::TestResponseCookies;

	let app = TestApp::from_config(app_config()).unwrap();
	let mut session = login(&app, "ada").await;
	assert_eq!(whoami(&mut session).await.username.as_deref(), Some("ada"));

	let logout = session
		.request(vorma::HttpMethod::POST, "/api/logout")
		.send()
		.await;
	assert_eq!(logout.status(), vorma::HttpStatusCode::OK);
	// The signal the jar keys off actually arrived on the wire: a real
	// `Max-Age <= 0`, not merely an empty value it happens to also carry.
	let logout_cookies = logout.set_cookie_headers();
	assert_eq!(logout_cookies.len(), 1);
	assert_eq!(logout_cookies[0].name(), HARNESS_SESSION_COOKIE);
	assert!(logout_cookies[0].max_age().unwrap() <= cookie::time::Duration::ZERO);

	// The cleared cookie must be gone from the jar entirely (no longer
	// sent at all), not merely overwritten with an empty value: the
	// resource distinguishes an empty session value from "no cookie" the
	// same way `harness_cookie_value` returning `None` does.
	assert_eq!(whoami(&mut session).await.username, None);
}
