//! Request-level tests for the Vorma Notes example, on the public test
//! harness only: `vorma::testing::TestApp` boots the whole app in memory
//! with zero build artifacts or hand-rolled manifests on disk.

use vorma::testing::TestApp;
use vorma_notes_example::app_config;

fn app() -> TestApp {
	TestApp::builder(app_config().expect("app config builds"))
		.with_public_asset("mark.svg", &b"<svg></svg>"[..])
		.with_public_asset("fonts/IoskeleyMono-400.woff2", &b"font"[..])
		.build()
		.expect("notes app boots in memory")
}

fn body_string(response: impl Into<Vec<u8>>) -> String {
	String::from_utf8(response.into()).expect("utf8 body")
}

#[tokio::test]
async fn create_note_returns_created_with_the_note() {
	let app = app();
	let response = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/notes",
			&serde_json::json!({ "body": "  Read the API aloud. #docs  " }),
		)
		.await;

	assert_eq!(response.status(), vorma::HttpStatusCode::CREATED);
	let body: serde_json::Value = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(body["note"]["id"], 3);
	assert_eq!(body["note"]["body"], "Read the API aloud. #docs");
	assert_eq!(body["note"]["tags"], serde_json::json!(["docs"]));
}

#[tokio::test]
async fn create_note_rejection_is_the_json_error_envelope() {
	let app = app();
	let response = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/notes",
			&serde_json::json!({ "body": "   " }),
		)
		.await;

	assert_eq!(response.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		body_string(response.into_body().to_vec()),
		r#"{"error":"note body is required"}"#
	);
}

#[tokio::test]
async fn note_view_nests_under_the_layout() {
	let app = app();
	let response = app.get_view_payload("/notes/1").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();
	assert_eq!(
		payload["matched_patterns"],
		serde_json::json!(["/", "/notes/:note_id"])
	);
	assert_eq!(payload["views_data"][0]["app_name"], "Vorma Notes");
	assert_eq!(payload["views_data"][1]["note"]["id"], 1);
}

#[tokio::test]
async fn missing_note_renders_as_a_normal_page_state() {
	let app = app();
	let response = app.get("/notes/nope").await;

	/*
	Views are a rendering protocol: a missing note is a committed page
	state (HTTP 200) — parents served, not-found treatment rendered.
	*/
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let body = body_string(response.into_body().to_vec());
	assert!(body.contains("Note not found"));
	assert!(body.contains(r#""note":null"#));
}

#[tokio::test]
async fn tags_splat_filters_notes_by_every_selected_tag() {
	let app = app();
	let response = app.get_view_payload("/tags/api/coverage").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let payload: serde_json::Value = serde_json::from_slice(response.body()).unwrap();
	let tags_view = &payload["views_data"][1];
	assert_eq!(
		tags_view["selected"],
		serde_json::json!(["api", "coverage"])
	);
	assert_eq!(tags_view["notes"].as_array().unwrap().len(), 1);
	assert_eq!(
		tags_view["all_tags"],
		serde_json::json!(["api", "coverage"])
	);
}

#[tokio::test]
async fn stats_view_runs_its_subtask_and_segment_errors_render_through() {
	let app = app();

	let ok = app.get_view_payload("/stats").await;
	assert_eq!(ok.status(), vorma::HttpStatusCode::OK);
	let payload: serde_json::Value = serde_json::from_slice(ok.body()).unwrap();
	assert_eq!(payload["views_data"][1]["note_count"], 2);
	assert_eq!(payload["views_data"][1]["tag_count"], 2);

	let failed = app.get_view_payload("/stats?fail=true").await;
	assert_eq!(failed.status(), vorma::HttpStatusCode::OK);
	let payload: serde_json::Value = serde_json::from_slice(failed.body()).unwrap();
	/*
	The layout (index 0) still served; only the explicit client message
	reaches the wire at the failing segment.
	*/
	assert_eq!(
		payload["outermost_server_err"],
		"Stats are unavailable right now."
	);
	assert_eq!(payload["outermost_server_err_idx"], 1);
	assert_eq!(payload["views_data"][0]["app_name"], "Vorma Notes");
}

#[tokio::test]
async fn admin_gate_redirects_anonymous_and_rejects_banned_sessions() {
	let app = app();

	let anonymous = app.get("/admin").await;
	assert!(anonymous.status().is_redirection());
	assert_eq!(anonymous.headers()["location"], "/");

	let banned = app
		.request(vorma::HttpMethod::GET, "/admin")
		.cookie("notes_session", "banned")
		.send()
		.await;
	assert_eq!(banned.status(), vorma::HttpStatusCode::UNAUTHORIZED);

	let signed_in = app
		.request(vorma::HttpMethod::GET, "/admin")
		.cookie("notes_session", "ada")
		.send()
		.await;
	assert_eq!(signed_in.status(), vorma::HttpStatusCode::OK);
	assert!(body_string(signed_in.into_body().to_vec()).contains(r#""session":"ada""#));
}

#[tokio::test]
async fn legacy_view_redirects_home() {
	let app = app();
	let response = app.get("/legacy").await;

	assert!(response.status().is_redirection());
	assert_eq!(response.headers()["location"], "/");
}

#[tokio::test]
async fn login_sets_the_session_cookie_and_logout_clears_it() {
	let app = app();

	let login = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/session",
			&serde_json::json!({ "user": "Ada" }),
		)
		.await;
	assert_eq!(login.status(), vorma::HttpStatusCode::CREATED);
	let set_cookie = login.headers()["set-cookie"].to_str().unwrap();
	assert!(set_cookie.starts_with("notes_session=ada"));
	assert!(set_cookie.contains("HttpOnly"));

	let logout = app
		.request(vorma::HttpMethod::DELETE, "/api/session")
		.send()
		.await;
	assert_eq!(logout.status(), vorma::HttpStatusCode::OK);
	let cleared = logout.headers()["set-cookie"].to_str().unwrap();
	assert!(cleared.starts_with("notes_session="));
	assert!(cleared.contains("Max-Age=0"));
}

#[tokio::test]
async fn import_accepts_a_multipart_notes_file() {
	let app = app();
	let boundary = "notes-test-boundary";
	let body = format!(
		"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; \
		filename=\"notes.txt\"\r\nContent-Type: text/plain\r\n\r\n\
		First imported #import\nSecond imported\n\r\n--{boundary}--\r\n"
	);

	let response = app
		.request(vorma::HttpMethod::POST, "/api/notes/import")
		.body(format!("multipart/form-data; boundary={boundary}"), body)
		.send()
		.await;

	assert_eq!(response.status(), vorma::HttpStatusCode::CREATED);
	assert_eq!(
		body_string(response.into_body().to_vec()),
		r#"{"imported":2}"#
	);
}

#[tokio::test]
async fn duplicate_redirects_at_the_fresh_copy() {
	let app = app();
	let response = app
		.request(vorma::HttpMethod::POST, "/api/notes/1/duplicate")
		.send()
		.await;

	assert!(response.status().is_redirection());
	assert_eq!(response.headers()["location"], "/notes/3");
}

#[tokio::test]
async fn ping_is_a_query_resource_with_its_header() {
	let app = app();
	let response = app.get("/api/ping").await;

	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	assert_eq!(response.headers()["x-notes-api"], "1");
	assert_eq!(response.headers()["x-notes-task-middleware"], "1");
	assert_eq!(body_string(response.into_body().to_vec()), r#"{"ok":true}"#);
}
