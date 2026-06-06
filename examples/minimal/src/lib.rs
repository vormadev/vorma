use std::sync::{Mutex, MutexGuard};

use serde::{Deserialize, Serialize};

const APP_NAME: &str = "Minimal Notes";
const APP_DESCRIPTION: &str = "A tiny production-shaped Vorma app.";
pub const REQUEST_BODY_LIMIT: usize = 64 * 1024;

pub struct AppState {
	notes: Mutex<NoteStore>,
}

struct NoteStore {
	next_id: u64,
	notes: Vec<Note>,
}

vorma::app!(pub mod app for crate::AppState);

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct HomeInput {
	#[serde(default)]
	draft: Option<String>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct HomePage {
	app_name: String,
	draft: Option<String>,
	mark_url: String,
	notes: Vec<Note>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct NotePage {
	app_name: String,
	note_id: String,
	note: Option<Note>,
}

#[derive(Clone, Debug, Deserialize, vorma::TsGen)]
pub struct CreateNoteInput {
	body: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct CreateNoteOutput {
	note: Note,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct PingOutput {
	ok: bool,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct Note {
	id: u64,
	body: String,
}

pub const HOME: app::View = app::view! {
	client_file: "src/client/views/home.view.tsx";
	pattern: "/";
	input: HomeInput;
	output: HomePage;
	handler: |ctx| {
		let mark_url = ctx.public_url("mark.svg")?;

		ctx.head()
			.title(APP_NAME)
			.description(APP_DESCRIPTION)
			.icon(mark_url.clone());

		Ok(HomePage {
			app_name: APP_NAME.to_owned(),
			draft: ctx.input().draft.clone(),
			mark_url,
			notes: note_store(ctx.state())?.notes.clone(),
		})
	};
};

pub const NOTE: app::View = app::view! {
	client_file: "src/client/views/note.view.tsx";
	pattern: "/notes/:note_id";
	input: ();
	output: NotePage;

	handler: |ctx| {
		let _ = ctx.request().path();
		let note_id = ctx.param("note_id").to_owned();
		let store = note_store(ctx.state())?;
		let note = note_id.parse::<u64>().ok().and_then(|id| {
			store.notes.iter().find(|note| note.id == id).cloned()
		});

		if let Some(note) = &note {
			ctx.head()
				.title(format!("Note #{}", note.id))
				.description("Read a tiny note.");
		} else {
			ctx.response()
				.set_error_status(vorma::HttpStatusCode::NOT_FOUND, "Note not found");
			ctx.head()
				.title("Note not found")
				.description("The requested note was not found.");
		}

		Ok(NotePage {
			app_name: APP_NAME.to_owned(),
			note_id,
			note,
		})
	};
};

pub const CREATE_NOTE: app::Resource = app::resource! {
	method: vorma::HttpMethod::POST;
	pattern: "/notes";
	input: CreateNoteInput;
	output: CreateNoteOutput;

	handler: |ctx| {
		let body = ctx.input().body.trim();
		if body.is_empty() {
			ctx.response().set_error_status(
				vorma::HttpStatusCode::BAD_REQUEST,
				"note body is required",
			);
			return Err(vorma::Error::runtime("note body is required").into());
		}
		if body.len() > 500 {
			ctx.response().set_error_status(
				vorma::HttpStatusCode::BAD_REQUEST,
				"note body must be at most 500 characters",
			);
			return Err(vorma::Error::runtime(
				"note body must be at most 500 characters",
			)
			.into());
		}

		let mut store = note_store(ctx.state())?;
		let note = Note {
			id: store.next_id,
			body: body.to_owned(),
		};
		store.next_id += 1;
		store.notes.push(note.clone());

		ctx.response().set_status(vorma::HttpStatusCode::CREATED);

		Ok(CreateNoteOutput { note })
	};
};

pub const PING: app::Resource = app::resource! {
	method: vorma::HttpMethod::GET;
	pattern: "/ping";
	input: ();
	output: PingOutput;
	handler: |ctx| {
		let _ = ctx.request().headers();
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-minimal-api"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(PingOutput { ok: true })
	};
};

pub fn app_config() -> vorma::Result<vorma::AppConfig<AppState>> {
	Ok(vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		server_config: vorma::ServerConfig {
			cargo_package: env!("CARGO_PKG_NAME").to_owned(),
			cargo_bin: "minimal-server".to_owned(),
		},
		dist_dir: ".".to_owned(),
		path_config: vorma::PathConfig {
			public_static_base: "/assets/".to_owned(),
			api_base: "/api/".to_owned(),
		},
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
		state: state(),
		views: views(),
		resources: resources(),
		middlewares: middlewares(),
		tasks_options: vorma::TasksOptions::default(),
		document: document(),
		request_body_limit: REQUEST_BODY_LIMIT,
	})
}

fn state() -> AppState {
	AppState {
		notes: Mutex::new(NoteStore {
			next_id: 3,
			notes: vec![
				Note {
					id: 1,
					body: "Keep the Rust API small.".to_owned(),
				},
				Note {
					id: 2,
					body: "Make the minimal app prove the surface.".to_owned(),
				},
			],
		}),
	}
}

fn views() -> app::Views {
	app::views![HOME, NOTE]
}

fn resources() -> app::Resources {
	app::resources![CREATE_NOTE, PING]
}

fn middlewares() -> app::Middlewares {
	app::middlewares![app::Middleware::new(|ctx| async move {
		let _ = ctx.request().path();
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-minimal-task-middleware"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(())
	})]
}

fn document() -> app::DocumentBuilder {
	app::DocumentBuilder::new(|ctx| async move {
		let mark_url = ctx.public_url("mark.svg")?;
		let font_url = ctx.public_url("fonts/IoskeleyMono-400.woff2")?;
		let mut document = vorma::Document::new();
		document.html().lang("en");
		document.body().data("app", "minimal");
		let head = document.head();
		let viewport_name = head.name("viewport").into();
		let viewport_content = head.content("width=device-width, initial-scale=1").into();
		head.meta([viewport_name, viewport_content]);
		let preload_rel = head.rel("preload").into();
		let preload_href = head.href(font_url).into();
		let preload_as = head.r#as("font").into();
		let preload_type = head.r#type("font/woff2").into();
		let preload_cross_origin = head.cross_origin("anonymous").into();
		head.link([
			preload_rel,
			preload_href,
			preload_as,
			preload_type,
			preload_cross_origin,
		]);
		head.meta_charset("utf-8")
			.title(APP_NAME)
			.description(APP_DESCRIPTION)
			.icon(mark_url.clone())
			.meta_property_content("og:title", APP_NAME)
			.meta_property_content("og:description", APP_DESCRIPTION)
			.meta_property_content("og:type", "website")
			.meta_property_content("og:image", mark_url.clone())
			.meta_property_content("og:site_name", APP_NAME)
			.meta_name_content("twitter:card", "summary")
			.meta_name_content("twitter:title", APP_NAME)
			.meta_name_content("twitter:description", APP_DESCRIPTION)
			.meta_name_content("twitter:image", mark_url)
			.meta_name_content("robots", "index,follow");
		Ok(document)
	})
}

fn note_store(state: &AppState) -> vorma::Result<MutexGuard<'_, NoteStore>> {
	state
		.notes
		.lock()
		.map_err(|_| vorma::Error::runtime("note store lock poisoned"))
}

/////////////////////////////////////////////////////////////////////
/////// Tests
/////////////////////////////////////////////////////////////////////

#[cfg(test)]
mod tests {
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::time::{SystemTime, UNIX_EPOCH};

	use axum::body::Body;
	use axum::http::header::CONTENT_TYPE;
	use axum::http::{Request, Response};
	use http_body_util::BodyExt;
	use serde_json::json;
	use tower::ServiceExt;

	use super::*;

	#[tokio::test]
	async fn create_note_api_adds_a_note() {
		let root = test_root("create-note");
		let response = app_request(
			&root,
			Request::post("/api/notes")
				.header(CONTENT_TYPE, "application/json")
				.body(Body::from(r#"{"body":"  Read the API aloud.  "}"#))
				.unwrap(),
		)
		.await;

		assert_eq!(response.status(), vorma::HttpStatusCode::CREATED);
		let body: serde_json::Value = serde_json::from_slice(&response.into_body()).unwrap();
		assert_eq!(body["note"]["id"], 3);
		assert_eq!(body["note"]["body"], "Read the API aloud.");
		fs::remove_dir_all(root).unwrap();
	}

	#[tokio::test]
	async fn create_note_api_validates_input() {
		let root = test_root("validate-note");
		let response = app_request(
			&root,
			Request::post("/api/notes")
				.header(CONTENT_TYPE, "application/json")
				.body(Body::from(r#"{"body":"   "}"#))
				.unwrap(),
		)
		.await;

		assert_eq!(response.status(), vorma::HttpStatusCode::BAD_REQUEST);
		assert_eq!(response.into_body(), b"note body is required\n");
		fs::remove_dir_all(root).unwrap();
	}

	#[tokio::test]
	async fn note_view_reads_route_params() {
		let root = test_root("note-view");
		let response =
			app_request(&root, Request::get("/notes/1").body(Body::empty()).unwrap()).await;

		assert_eq!(response.status(), vorma::HttpStatusCode::OK);
		let body = String::from_utf8(response.into_body()).unwrap();
		assert!(body.contains(r#""note_id":"1""#));
		assert!(body.contains("Keep the Rust API small."));
		fs::remove_dir_all(root).unwrap();
	}

	#[tokio::test]
	async fn note_view_reports_missing_notes_as_not_found() {
		let root = test_root("note-not-found");
		let response = app_request(
			&root,
			Request::get("/notes/nope").body(Body::empty()).unwrap(),
		)
		.await;

		assert_eq!(response.status(), vorma::HttpStatusCode::NOT_FOUND);
		let body = String::from_utf8(response.into_body()).unwrap();
		assert!(body.contains("Note not found"));
		fs::remove_dir_all(root).unwrap();
	}

	async fn app_request(root: &Path, request: Request<Body>) -> Response<Vec<u8>> {
		let mut config = app_config().unwrap();
		config.root_dir = root.to_path_buf();
		config.dist_dir = ".".to_owned();
		let response = vorma::App::from_config(config)
			.unwrap()
			.oneshot(request)
			.await
			.unwrap();
		let (parts, body) = response.into_parts();
		let body = body.collect().await.unwrap().to_bytes().to_vec();
		Response::from_parts(parts, body)
	}

	fn test_root(name: &str) -> PathBuf {
		let now = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root =
			std::env::temp_dir().join(format!("vorma-minimal-{name}-{}-{now}", std::process::id()));
		fs::create_dir_all(root.join(".vorma/static")).unwrap();
		write_manifest(&root);
		root
	}

	fn write_manifest(root: &Path) {
		let manifest = json!({
			"vorma_version": "0.1.0",
			"public_static_base_path": "/assets/",
			"api_mount_root": "/api/",
			"ui_variant": "react",
			"root_document_shell_hash": "test-shell",
			"public_filepaths": [
				"/assets/entry.js",
				"/assets/home.view.js",
				"/assets/mark.svg",
				"/assets/note.view.js",
				"/assets/fonts/IoskeleyMono-400.woff2"
			],
			"public_filemap": {
				"mark.svg": "/assets/mark.svg",
				"fonts/IoskeleyMono-400.woff2": "/assets/fonts/IoskeleyMono-400.woff2"
			},
			"critical_css": "body{margin:0}",
			"search_schemas": {
				"/": {},
				"/notes/:note_id": {}
			},
			"client_entry": {
				"url": "/assets/entry.js",
				"dep_urls": [],
				"css_bundle_urls": []
			},
			"client_views": {
				"/": {
					"url": "/assets/home.view.js",
					"dep_urls": [],
					"css_bundle_urls": []
				},
				"/notes/:note_id": {
					"url": "/assets/note.view.js",
					"dep_urls": [],
					"css_bundle_urls": []
				}
			}
		});
		fs::write(
			root.join(".vorma/static/vorma.manifest.prod.json"),
			serde_json::to_vec(&manifest).unwrap(),
		)
		.unwrap();
	}
}
