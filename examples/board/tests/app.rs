//! Request-level tests for Vorma Board on `vorma::testing::TestApp`,
//! each against its own temporary SQLite database file.

use std::sync::atomic::{AtomicU64, Ordering};

use std::sync::Arc;

use vorma::testing::{TEST_CLIENT_BUILD_ID, TestApp, TestResponseCookies, TestSession};
use vorma_board_example::{
	CSRF_ECHO_HEADER, CSRF_HEADER, Db, MARK_ASSET, MAX_STORY_ATTACHMENT_BYTES,
	MAX_STORY_ATTACHMENTS, STORY_TAGS, app_config_with_db,
};

static NEXT_DB: AtomicU64 = AtomicU64::new(0);

fn tmp_db() -> Arc<Db> {
	let unique = format!(
		"vorma-board-test-{}-{}.db",
		std::process::id(),
		NEXT_DB.fetch_add(1, Ordering::Relaxed)
	);
	Db::open(&std::env::temp_dir().join(unique)).expect("tmp sqlite opens")
}

fn app() -> TestApp {
	app_with_db().0
}

fn app_with_db() -> (TestApp, Arc<Db>) {
	let db = tmp_db();
	let app = TestApp::builder(app_config_with_db(Arc::clone(&db)).expect("config builds"))
		.with_public_asset(MARK_ASSET, &include_bytes!("../public/mark.svg")[..])
		.build()
		.expect("board app boots in memory");
	(app, db)
}

fn json(body: &[u8]) -> serde_json::Value {
	serde_json::from_slice(body).expect("json body")
}

fn view_payload_path(path_and_query: &str) -> String {
	let separator = if path_and_query.contains('?') {
		'&'
	} else {
		'?'
	};
	format!(
		"{path_and_query}{separator}{}=_",
		vorma::build_interface::runtime::VORMA_JSON_QUERY_KEY
	)
}

/// Logs in and returns a session that carries the resulting cookie forward
/// on every later request it makes.
async fn login<'a>(app: &'a TestApp, username: &str) -> TestSession<'a> {
	let mut session = app.session();
	let response = session
		.request_json(
			vorma::HttpMethod::POST,
			"/api/session",
			&serde_json::json!({ "username": username }),
		)
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::CREATED);
	session
}

const MULTIPART_BOUNDARY: &str = "board-test-boundary";

/// Builds a `multipart/form-data` body, one call per field.
///
/// Repeated calls with the same `name` are how a client-native repeated form field (a
/// `<input type="file" multiple>` under one name, or several checked boxes sharing a
/// `name`) actually looks on the wire — several parts, same `name`, in request order.
/// Building that shape here is what lets tests exercise `FormData`'s repeated-field
/// accessors (`texts`, `files_named`) the same way a real browser submission would.
#[derive(Default)]
struct MultipartBody {
	body: String,
}

impl MultipartBody {
	fn field(mut self, name: &str, value: &str) -> Self {
		self.body.push_str(&format!(
			"--{MULTIPART_BOUNDARY}\r\nContent-Disposition: form-data; \
			name=\"{name}\"\r\n\r\n{value}\r\n"
		));
		self
	}

	fn file(mut self, name: &str, file_name: &str, content_type: &str, content: &str) -> Self {
		self.body.push_str(&format!(
			"--{MULTIPART_BOUNDARY}\r\nContent-Disposition: form-data; name=\"{name}\"; \
			filename=\"{file_name}\"\r\nContent-Type: {content_type}\r\n\r\n{content}\r\n"
		));
		self
	}

	fn build(mut self) -> Vec<u8> {
		self.body.push_str(&format!("--{MULTIPART_BOUNDARY}--\r\n"));
		self.body.into_bytes()
	}
}

fn multipart_fields(fields: &[(&str, &str)]) -> Vec<u8> {
	fields
		.iter()
		.fold(MultipartBody::default(), |body, (name, value)| {
			body.field(name, value)
		})
		.build()
}

async fn submit_story(session: &mut TestSession<'_>, title: &str, url: &str) -> String {
	let response = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			multipart_fields(&[("title", title), ("url", url)]),
		)
		.send()
		.await;
	assert!(response.status().is_redirection());
	response.headers()["location"]
		.to_str()
		.expect("location is ascii")
		.to_owned()
}

#[tokio::test]
async fn login_then_layout_carries_the_session_user() {
	let app = app();
	let mut session = login(&app, "Ada").await;

	let payload = session.get_view_payload("/").await;
	assert_eq!(payload.status(), vorma::HttpStatusCode::OK);
	let body = json(payload.body());
	assert_eq!(body["views_data"][0]["app_name"], "Vorma Board");
	assert_eq!(body["views_data"][0]["current_user"]["username"], "ada");
}

#[tokio::test]
async fn anonymous_layout_has_no_session_user() {
	let app = app();
	let mark_url = app.public_url(MARK_ASSET).expect("mark public url");
	let payload = app.get_view_payload("/").await;

	let body = json(payload.body());
	assert_eq!(body["views_data"][0]["mark_url"], mark_url);
	assert_eq!(
		body["views_data"][0]["current_user"],
		serde_json::Value::Null
	);
	// The shell footer's live totals come from the single_flight stats task.
	assert_eq!(body["views_data"][0]["stats"]["stories"], 0);
	assert_eq!(body["views_data"][0]["stats"]["votes"], 0);

	let mark = app.get(&mark_url).await;
	assert_eq!(mark.status(), vorma::HttpStatusCode::OK);
	assert_eq!(
		mark.body().as_ref(),
		&include_bytes!("../public/mark.svg")[..]
	);
}

#[tokio::test]
async fn submit_requires_a_session() {
	let app = app();
	let response = app
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			multipart_fields(&[("title", "No auth"), ("url", "https://example.com")]),
		)
		.send()
		.await;

	assert_eq!(response.status(), vorma::HttpStatusCode::UNAUTHORIZED);
	assert_eq!(
		json(response.body()),
		serde_json::json!({ "error": "sign in first" })
	);
}

#[tokio::test]
async fn submit_redirects_at_the_new_story_and_the_story_page_renders_it() {
	let app = app();
	let mut session = login(&app, "ada").await;

	let location = submit_story(&mut session, "Vorma ships", "https://vorma.dev").await;
	assert_eq!(location, "/s/1");

	let payload = app.get_view_payload(&location).await;
	let body = json(payload.body());
	assert_eq!(
		body["matched_patterns"],
		serde_json::json!(["/", "/s/:story_id"])
	);
	let story = &body["views_data"][1]["story"];
	assert_eq!(story["title"], "Vorma ships");
	assert_eq!(story["author"], "ada");
	assert_eq!(story["points"], 0);
	assert_eq!(body["views_data"][1]["comments"], serde_json::json!([]));
}

#[tokio::test]
async fn user_routes_redirect_to_canonical_lowercase_paths() {
	let app = app();

	let profile = app.get("/u/Ada").await;
	assert!(profile.status().is_redirection());
	assert_eq!(profile.headers()["location"], "/u/ada");

	let comments = app.get("/u/Ada/comments").await;
	assert!(comments.status().is_redirection());
	assert_eq!(comments.headers()["location"], "/u/ada/comments");
}

#[tokio::test]
async fn method_scoped_middleware_marks_mutating_requests_only() {
	let app = app();

	let post = app
		.request(vorma::HttpMethod::POST, "/api/session")
		.header(CSRF_HEADER, "board-token")
		.body(
			"application/json",
			serde_json::json!({ "username": "ada" })
				.to_string()
				.into_bytes(),
		)
		.send()
		.await;
	assert_eq!(post.status(), vorma::HttpStatusCode::CREATED);
	assert_eq!(post.headers()[CSRF_ECHO_HEADER], "board-token");

	let get = app
		.request(vorma::HttpMethod::GET, &view_payload_path("/"))
		.header(CSRF_HEADER, "board-token")
		.send()
		.await;
	assert_eq!(get.status(), vorma::HttpStatusCode::OK);
	assert!(!get.headers().contains_key(CSRF_ECHO_HEADER));
}

#[tokio::test]
async fn missing_story_renders_as_a_normal_page_state() {
	let app = app();
	let payload = app.get_view_payload("/s/999").await;

	assert_eq!(payload.status(), vorma::HttpStatusCode::OK);
	let body = json(payload.body());
	assert_eq!(body["views_data"][1]["story"], serde_json::Value::Null);
}

#[tokio::test]
async fn catch_all_view_is_a_normal_app_level_fallback() {
	let app = app();
	let payload = app
		.get_view_payload("/missing/board/page?from=docs&from=nav")
		.await;

	assert_eq!(payload.status(), vorma::HttpStatusCode::OK);
	let body = json(payload.body());
	assert_eq!(body["matched_patterns"], serde_json::json!(["/", "/*"]));
	assert_eq!(
		body["views_data"][1]["requested_path"],
		"/missing/board/page"
	);
	assert_eq!(body["views_data"][1]["primary_source"], "docs");
	assert_eq!(
		body["views_data"][1]["source_tags"],
		serde_json::json!(["docs", "nav"])
	);
	assert_eq!(body["views_data"][1]["source_pair_count"], 2);
}

#[tokio::test]
async fn votes_count_once_and_duplicates_get_the_envelope() {
	let app = app();
	let mut session = login(&app, "ada").await;
	submit_story(&mut session, "Voting works", "https://example.com").await;

	let first = session
		.request(vorma::HttpMethod::POST, "/api/stories/1/vote")
		.send()
		.await;
	assert_eq!(first.status(), vorma::HttpStatusCode::OK);
	assert_eq!(json(first.body()), serde_json::json!({ "points": 1 }));

	let duplicate = session
		.request(vorma::HttpMethod::POST, "/api/stories/1/vote")
		.send()
		.await;
	assert_eq!(duplicate.status(), vorma::HttpStatusCode::CONFLICT);
	assert_eq!(
		json(duplicate.body()),
		serde_json::json!({ "error": "you already voted on this story" })
	);
}

#[tokio::test]
async fn front_page_ranks_by_points_then_recency() {
	let app = app();
	let mut ada = login(&app, "ada").await;
	let mut lin = login(&app, "lin").await;
	submit_story(&mut ada, "Older unloved", "https://example.com/a").await;
	submit_story(&mut ada, "Newer unloved", "https://example.com/b").await;
	let third = submit_story(&mut ada, "Loved", "https://example.com/c").await;
	assert_eq!(third, "/s/3");
	for session in [&mut ada, &mut lin] {
		let vote = session
			.request(vorma::HttpMethod::POST, "/api/stories/3/vote")
			.send()
			.await;
		assert_eq!(vote.status(), vorma::HttpStatusCode::OK);
	}

	let payload = app.get_view_payload("/").await;
	let body = json(payload.body());
	let titles = body["views_data"][1]["stories"]
		.as_array()
		.expect("stories array")
		.iter()
		.map(|story| story["title"].as_str().expect("title"))
		.collect::<Vec<_>>();
	assert_eq!(titles[0], "Loved");
	assert_eq!(titles.len(), 3);
}

#[tokio::test]
async fn logout_clears_the_session_cookie_and_the_session_row() {
	/*
	This test deliberately replays a token AFTER the server has invalidated
	it, so it stays on `TestApp`'s stateless, explicit `.cookie(name, value)`
	form rather than `TestSession`: a session's jar would honestly forget the
	cookie the moment it saw the logout response's clearing `Set-Cookie` (the
	same continuation contract exercised elsewhere), which would send no
	cookie at all on the last request below and only prove anonymous access
	is anonymous — already covered by `anonymous_layout_has_no_session_user`.
	The point here is the stronger claim: even a client that still has the
	stale cookie and presents it on purpose gets rejected, because the
	server's own session row is gone.
	*/
	let app = app();
	let login_response = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/session",
			&serde_json::json!({ "username": "ada" }),
		)
		.await;
	assert_eq!(login_response.status(), vorma::HttpStatusCode::CREATED);
	let session_cookie = &login_response.set_cookie_headers()[0];
	let (cookie_name, cookie_value) = (session_cookie.name(), session_cookie.value());

	let logout = app
		.request(vorma::HttpMethod::DELETE, "/api/session")
		.cookie(cookie_name, cookie_value)
		.send()
		.await;
	assert_eq!(logout.status(), vorma::HttpStatusCode::OK);
	let cleared = &logout.set_cookie_headers()[0];
	assert_eq!(cleared.name(), cookie_name);
	assert!(
		!cleared.max_age().unwrap().is_positive(),
		"Max-Age must be <= 0"
	);

	// The old token no longer resolves a user even if replayed.
	let payload = app
		.request(vorma::HttpMethod::GET, &view_payload_path("/"))
		.cookie(cookie_name, cookie_value)
		.send()
		.await;
	assert_eq!(
		json(payload.body())["views_data"][0]["current_user"],
		serde_json::Value::Null
	);
}

#[tokio::test]
async fn submit_validation_speaks_the_envelope() {
	let app = app();
	let mut session = login(&app, "ada").await;

	let both = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			multipart_fields(&[
				("title", "Both provided"),
				("url", "https://example.com"),
				("body", "and text"),
			]),
		)
		.send()
		.await;
	assert_eq!(both.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		json(both.body()),
		serde_json::json!({ "error": "provide a link or text, not both" })
	);

	let bad_url = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			multipart_fields(&[("title", "Bad link"), ("url", "ftp://example.com")]),
		)
		.send()
		.await;
	assert_eq!(bad_url.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		json(bad_url.body()),
		serde_json::json!({ "error": "links must start with http:// or https://" })
	);
}

#[tokio::test]
async fn comments_post_and_render_on_the_story_page() {
	let app = app();
	let mut session = login(&app, "ada").await;
	submit_story(&mut session, "Discuss", "https://example.com").await;

	let anonymous = app
		.request_json(
			vorma::HttpMethod::POST,
			"/api/stories/1/comments",
			&serde_json::json!({ "body": "hi" }),
		)
		.await;
	assert_eq!(anonymous.status(), vorma::HttpStatusCode::UNAUTHORIZED);

	let created = session
		.request(vorma::HttpMethod::POST, "/api/stories/1/comments")
		.body(
			"application/json",
			serde_json::json!({ "body": "First!" })
				.to_string()
				.into_bytes(),
		)
		.send()
		.await;
	assert_eq!(created.status(), vorma::HttpStatusCode::CREATED);
	assert_eq!(json(created.body()), serde_json::json!({ "comment_id": 1 }));

	let reply = session
		.request(vorma::HttpMethod::POST, "/api/stories/1/comments")
		.body(
			"application/json",
			serde_json::json!({ "body": "Replying", "parent_id": 1 })
				.to_string()
				.into_bytes(),
		)
		.send()
		.await;
	assert_eq!(reply.status(), vorma::HttpStatusCode::CREATED);

	let payload = app.get_view_payload("/s/1").await;
	let comments = &json(payload.body())["views_data"][1]["comments"];
	assert_eq!(comments.as_array().expect("comments").len(), 2);
	assert_eq!(comments[1]["parent_id"], 1);

	let missing = session
		.request(vorma::HttpMethod::POST, "/api/stories/99/comments")
		.body(
			"application/json",
			serde_json::json!({ "body": "ghost" })
				.to_string()
				.into_bytes(),
		)
		.send()
		.await;
	assert_eq!(missing.status(), vorma::HttpStatusCode::NOT_FOUND);
	assert_eq!(
		json(missing.body()),
		serde_json::json!({ "error": "that story no longer exists" })
	);
}

#[tokio::test]
async fn search_resource_finds_titles_and_pages() {
	let app = app();
	let mut session = login(&app, "ada").await;
	submit_story(&mut session, "Rust ships generics", "https://example.com/a").await;
	submit_story(&mut session, "Cooking tips", "https://example.com/b").await;

	let hits = app.get("/api/search?q=rust").await;
	assert_eq!(hits.status(), vorma::HttpStatusCode::OK);
	let body = json(hits.body());
	assert_eq!(body["stories"].as_array().expect("hits").len(), 1);
	assert_eq!(body["stories"][0]["title"], "Rust ships generics");
	assert_eq!(body["has_more"], false);

	let empty = app.get("/api/search").await;
	assert_eq!(
		json(empty.body()),
		serde_json::json!({ "stories": [], "has_more": false })
	);
}

#[tokio::test]
async fn mod_area_is_gated_and_kill_restore_round_trips() {
	let (app, db) = app_with_db();
	let mut session = login(&app, "ada").await;
	submit_story(&mut session, "Spam", "https://example.com").await;

	// Anonymous documents are redirected away from the mod area.
	let anonymous = app.get("/mod").await;
	assert!(anonymous.status().is_redirection());
	assert_eq!(anonymous.headers()["location"], "/");

	// The scoped middleware covers mod RESOURCES identically.
	let anonymous_kill = app
		.request(vorma::HttpMethod::POST, "/api/mod/stories/1/kill")
		.send()
		.await;
	assert!(anonymous_kill.status().is_redirection());

	let kill = session
		.request(vorma::HttpMethod::POST, "/api/mod/stories/1/kill")
		.send()
		.await;
	assert_eq!(kill.status(), vorma::HttpStatusCode::OK);

	// Killed stories vanish from the front page and the story page is a
	// soft not-found.
	let front = app.get_view_payload("/").await;
	assert_eq!(
		json(front.body())["views_data"][1]["stories"],
		serde_json::json!([])
	);
	let story = app.get_view_payload("/s/1").await;
	assert_eq!(
		json(story.body())["views_data"][1]["story"],
		serde_json::Value::Null
	);

	// The mod queue lists it, with the action logged.
	let queue = session.get_view_payload("/mod").await;
	let queue_body = json(queue.body());
	assert_eq!(queue_body["views_data"][1]["killed"][0]["title"], "Spam");
	assert_eq!(queue_body["views_data"][1]["log"][0]["action"], "kill");

	let restore = session
		.request(vorma::HttpMethod::POST, "/api/mod/stories/1/restore")
		.send()
		.await;
	assert_eq!(restore.status(), vorma::HttpStatusCode::OK);
	let front = app.get_view_payload("/").await;
	assert_eq!(
		json(front.body())["views_data"][1]["stories"][0]["title"],
		"Spam"
	);

	// Banned sessions get the real 401 envelope.
	db.with(|conn| conn.execute("UPDATE users SET banned = 1 WHERE username = 'ada'", []))
		.expect("ban write");
	let banned = session
		.request(vorma::HttpMethod::POST, "/api/mod/stories/1/kill")
		.send()
		.await;
	assert_eq!(banned.status(), vorma::HttpStatusCode::UNAUTHORIZED);
	assert_eq!(
		json(banned.body()),
		serde_json::json!({ "error": "This account cannot access moderation." })
	);
}

#[tokio::test]
async fn mod_export_is_gated_and_digests_every_story() {
	let app = app();

	// Same scoped middleware as the kill/restore endpoints protects export.
	let anonymous = app
		.request(vorma::HttpMethod::POST, "/api/mod/export")
		.send()
		.await;
	assert!(anonymous.status().is_redirection());

	let mut session = login(&app, "ada").await;
	submit_story(&mut session, "First", "https://example.com/1").await;
	submit_story(&mut session, "Second", "https://example.com/2").await;
	let comment = session
		.request(vorma::HttpMethod::POST, "/api/stories/1/comments")
		.body(
			"application/json",
			serde_json::json!({ "body": "nice find" })
				.to_string()
				.into_bytes(),
		)
		.send()
		.await;
	assert_eq!(comment.status(), vorma::HttpStatusCode::CREATED);

	let export = session
		.request(vorma::HttpMethod::POST, "/api/mod/export")
		.send()
		.await;
	assert_eq!(export.status(), vorma::HttpStatusCode::OK);
	let digest = json(export.body());
	assert_eq!(digest["complete"], true);
	let rows = digest["rows"].as_array().expect("export rows");
	assert_eq!(rows.len(), 2);
	assert_eq!(rows[0]["title"], "First");
	assert_eq!(rows[0]["comment_count"], 1);
	assert_eq!(rows[1]["title"], "Second");
	assert_eq!(rows[1]["comment_count"], 0);
}

#[tokio::test]
async fn docs_splat_serves_seeded_pages_at_any_depth() {
	let app = app();

	let index = app.get_view_payload("/docs").await;
	let index_body = json(index.body());
	assert_eq!(
		index_body["matched_patterns"],
		serde_json::json!(["/", "/docs"])
	);
	assert!(
		index_body["views_data"][1]["index"]
			.as_array()
			.expect("index")
			.len() >= 3
	);

	let nested = app.get_view_payload("/docs/moderation/kill-policy").await;
	let nested_body = json(nested.body());
	assert_eq!(
		nested_body["matched_patterns"],
		serde_json::json!(["/", "/docs", "/docs/*"])
	);
	assert_eq!(nested_body["views_data"][2]["page"]["title"], "Kill policy");

	let missing = app.get_view_payload("/docs/nope").await;
	assert_eq!(
		json(missing.body())["views_data"][2]["page"],
		serde_json::Value::Null
	);
}

#[tokio::test]
async fn diagnostics_segment_fails_alone_while_parents_render() {
	let app = app();
	let mut session = login(&app, "ada").await;

	let payload = session.get_view_payload("/mod/diagnostics").await;
	assert_eq!(payload.status(), vorma::HttpStatusCode::OK);
	let body = json(payload.body());
	assert_eq!(
		body["matched_patterns"],
		serde_json::json!(["/", "/mod", "/mod/diagnostics"])
	);
	assert_eq!(
		body["outermost_server_err"],
		"Diagnostics are not available in the demo."
	);
	assert_eq!(body["outermost_server_err_idx"], 2);
	// The mod queue (parent) still served its data.
	assert!(body["views_data"][1]["killed"].is_array());
}

#[tokio::test]
async fn attachments_upload_with_the_story_and_download_back() {
	let app = app();
	let mut session = login(&app, "ada").await;

	/*
	Two files under the same `attachment` name is exactly what
	`<input type="file" multiple>` posts on the wire: one part per selected file,
	all sharing one field name. The server reads this back with `files_named`, the
	repeated-value counterpart to the single-file `file()` the pre-multi-attachment
	form used.
	*/
	let submitted = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			MultipartBody::default()
				.field("title", "With files")
				.field("body", "see attachments")
				.file("attachment", "notes.txt", "text/plain", "hello attachment")
				.file("attachment", "second.txt", "text/plain", "a second file")
				.build(),
		)
		.send()
		.await;
	assert!(submitted.status().is_redirection());

	let payload = app.get_view_payload("/s/1").await;
	let attachments = &json(payload.body())["views_data"][1]["attachments"];
	let attachments = attachments.as_array().expect("attachments array");
	assert_eq!(attachments.len(), 2);
	assert_eq!(attachments[0]["file_name"], "notes.txt");
	assert_eq!(attachments[1]["file_name"], "second.txt");
	let first_id = attachments[0]["id"].as_i64().expect("first attachment id");
	let second_id = attachments[1]["id"].as_i64().expect("second attachment id");

	let download = app
		.get(&format!("/api/stories/1/attachments/{first_id}"))
		.await;
	assert_eq!(download.status(), vorma::HttpStatusCode::OK);
	assert_eq!(download.headers()["content-type"], "text/plain");
	assert!(
		download.headers()["content-disposition"]
			.to_str()
			.expect("ascii")
			.contains("notes.txt")
	);
	assert_eq!(download.body().as_ref(), b"hello attachment");

	let second_download = app
		.get(&format!("/api/stories/1/attachments/{second_id}"))
		.await;
	assert_eq!(second_download.status(), vorma::HttpStatusCode::OK);
	assert_eq!(second_download.body().as_ref(), b"a second file");

	let head = app
		.request(
			vorma::HttpMethod::HEAD,
			&format!("/api/stories/1/attachments/{first_id}"),
		)
		.send()
		.await;
	assert_eq!(head.status(), vorma::HttpStatusCode::OK);
	assert_eq!(head.headers()["content-type"], "text/plain");
	assert_eq!(head.headers()["content-length"], "16");
	assert!(head.body().is_empty());

	// A story with no attachments serves a 404 for any attachment id.
	submit_story(&mut session, "No attachments", "https://example.com").await;
	let none = app
		.get(&format!("/api/stories/2/attachments/{first_id}"))
		.await;
	assert_eq!(none.status(), vorma::HttpStatusCode::NOT_FOUND);

	/*
	`first_id` is a real attachment id, just not one that belongs to story 2: the
	route must still 404 rather than serve story 1's file under story 2's URL.
	*/
	let cross_story = app
		.get(&format!("/api/stories/2/attachments/{first_id}"))
		.await;
	assert_eq!(cross_story.status(), vorma::HttpStatusCode::NOT_FOUND);
}

#[tokio::test]
async fn submit_tags_round_trip_through_the_checkbox_group() {
	let app = app();
	let mut session = login(&app, "ada").await;

	/*
	A checkbox group posts one `tag` field per checked box, all sharing the
	`tag` name — the same repeated-field shape as the multi-file case above, just
	with plain text values instead of files. The server reads every value back
	with `texts("tag")`.
	*/
	let submitted = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			MultipartBody::default()
				.field("title", "Tagged story")
				.field("url", "https://example.com")
				.field("tag", "tech")
				.field("tag", "show")
				.build(),
		)
		.send()
		.await;
	assert!(submitted.status().is_redirection());

	let payload = app.get_view_payload("/s/1").await;
	assert_eq!(
		json(payload.body())["views_data"][1]["tags"],
		serde_json::json!(["show", "tech"])
	);
}

#[tokio::test]
async fn submit_rejects_a_tag_outside_the_known_vocabulary() {
	let app = app();
	let mut session = login(&app, "ada").await;

	/*
	`texts("tag")` yields every submitted value regardless of whether the
	server recognizes it; the vocabulary filter in the handler is what keeps a
	tampered request from persisting an arbitrary tag string.
	*/
	let submitted = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			MultipartBody::default()
				.field("title", "Sneaky tag")
				.field("url", "https://example.com")
				.field("tag", "tech")
				.field("tag", "not-a-real-tag")
				.build(),
		)
		.send()
		.await;
	assert!(submitted.status().is_redirection());

	let payload = app.get_view_payload("/s/1").await;
	assert_eq!(
		json(payload.body())["views_data"][1]["tags"],
		serde_json::json!(["tech"])
	);
}

#[tokio::test]
async fn submit_rejects_more_tag_fields_than_the_vocabulary_has() {
	let app = app();
	let mut session = login(&app, "ada").await;

	// One more `tag` field than checkboxes actually exist: `fields_named("tag")`
	// counts the group's shape independently of what values it carries.
	let mut form = MultipartBody::default()
		.field("title", "Too many tags")
		.field("url", "https://example.com");
	for _ in 0..=STORY_TAGS.len() {
		form = form.field("tag", "tech");
	}
	let response = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			form.build(),
		)
		.send()
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		json(response.body()),
		serde_json::json!({ "error": "that tag selection is not valid" })
	);
}

#[tokio::test]
async fn submit_rejects_too_many_attachments() {
	let app = app();
	let mut session = login(&app, "ada").await;

	let mut form = MultipartBody::default()
		.field("title", "Too many files")
		.field("url", "https://example.com");
	// One more than the server's own attachment-count limit.
	for index in 0..=MAX_STORY_ATTACHMENTS {
		form = form.file(
			"attachment",
			&format!("file-{index}.txt"),
			"text/plain",
			"x",
		);
	}
	let response = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			form.build(),
		)
		.send()
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		json(response.body()),
		serde_json::json!({
			"error": format!("attach at most {MAX_STORY_ATTACHMENTS} files")
		})
	);
}

#[tokio::test]
async fn submit_rejects_a_field_value_that_is_too_long() {
	let app = app();
	let mut session = login(&app, "ada").await;

	// Longer than the server's blanket per-field byte cap, enforced over every
	// submitted field via `fields()` regardless of which field name it is.
	let oversized_body = "x".repeat(5_000);
	let response = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			MultipartBody::default()
				.field("title", "Oversized field")
				.field("body", &oversized_body)
				.build(),
		)
		.send()
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		json(response.body()),
		serde_json::json!({ "error": "one of the submitted fields is too long" })
	);
}

#[tokio::test]
async fn submit_rejects_attachments_that_together_exceed_the_byte_cap() {
	let app = app();
	let mut session = login(&app, "ada").await;

	// One file already over the combined cap: `files()` sums every attachment's
	// body regardless of name, so a single oversized file is enough to trip it.
	let oversized_file = "x".repeat(MAX_STORY_ATTACHMENT_BYTES + 1);
	let response = session
		.request(vorma::HttpMethod::POST, "/api/stories")
		.body(
			format!("multipart/form-data; boundary={MULTIPART_BOUNDARY}"),
			MultipartBody::default()
				.field("title", "Oversized attachment")
				.field("url", "https://example.com")
				.file(
					"attachment",
					"big.bin",
					"application/octet-stream",
					&oversized_file,
				)
				.build(),
		)
		.send()
		.await;
	assert_eq!(response.status(), vorma::HttpStatusCode::BAD_REQUEST);
	assert_eq!(
		json(response.body()),
		serde_json::json!({ "error": "attachments are too large altogether" })
	);
}

#[tokio::test]
async fn user_pages_nest_profile_and_comment_tabs() {
	let app = app();
	let mut session = login(&app, "ada").await;
	submit_story(&mut session, "Mine", "https://example.com").await;
	let comment = session
		.request(vorma::HttpMethod::POST, "/api/stories/1/comments")
		.body(
			"application/json",
			serde_json::json!({ "body": "self reply" })
				.to_string()
				.into_bytes(),
		)
		.send()
		.await;
	assert_eq!(comment.status(), vorma::HttpStatusCode::CREATED);

	let profile = app.get_view_payload("/u/ada").await;
	let profile_body = json(profile.body());
	assert_eq!(
		profile_body["matched_patterns"],
		serde_json::json!(["/", "/u/:username"])
	);
	assert_eq!(profile_body["views_data"][1]["profile"]["story_count"], 1);
	assert_eq!(profile_body["views_data"][1]["stories"][0]["title"], "Mine");

	let comments_tab = app.get_view_payload("/u/ada/comments").await;
	let comments_body = json(comments_tab.body());
	assert_eq!(
		comments_body["matched_patterns"],
		serde_json::json!(["/", "/u/:username", "/u/:username/comments"])
	);
	assert_eq!(
		comments_body["views_data"][2]["comments"][0]["body"],
		"self reply"
	);

	let unknown = app.get_view_payload("/u/nobody").await;
	assert_eq!(
		json(unknown.body())["views_data"][1]["profile"],
		serde_json::Value::Null
	);
}

#[tokio::test]
async fn app_booted_from_config_serves_the_build_id_header() {
	/*
	`TestApp::from_config` is the no-frills constructor for apps that do not
	need synthesized public assets in the test. It boots the same graph the
	server uses. Every Vorma response carries the client build id header so the
	browser can detect a stale bundle; `client_build_id()` exposes the value the
	harness committed, which for a test app is `TEST_CLIENT_BUILD_ID`.
	*/
	let app = TestApp::from_config(app_config_with_db(tmp_db()).expect("config builds"))
		.expect("board app boots in memory");
	assert_eq!(app.client_build_id(), TEST_CLIENT_BUILD_ID);

	let response = app.get("/api/search?q=nothing").await;
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	assert_eq!(
		response.headers()[vorma::CLIENT_BUILD_ID_HEADER_KEY],
		TEST_CLIENT_BUILD_ID
	);
}

#[tokio::test]
async fn document_shell_renders_the_boolean_and_known_safe_body_attributes() {
	/*
	Every other test in this file reads typed view/resource data, never raw HTML —
	the honest way to prove a document-builder attribute actually reaches the wire
	is to fetch the real page and check the rendered `<body>` tag byte-for-byte,
	the same proof `crates/vorma/tests/public_api.rs`'s usability test uses at the
	builder level. `app.get("/")` (a plain page fetch, not `get_view_payload`) is
	what returns the full HTML document instead of the JSON view-data envelope.
	*/
	let app = app();
	let response = app.get("/").await;
	assert_eq!(response.status(), vorma::HttpStatusCode::OK);
	let html = String::from_utf8(response.body().to_vec()).expect("document is utf-8");

	// `boolean_attribute` renders bare: present, with no `="..."` at all.
	assert!(html.contains(" data-server-rendered "));
	assert!(!html.contains("data-server-rendered=\""));

	// `known_safe_attribute` skips escaping: the ampersand survives raw, not as `&amp;`.
	assert!(html.contains("data-built-with=\"Vorma Board & Rust\""));
	assert!(!html.contains("Vorma Board &amp; Rust"));
}
