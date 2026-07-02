//! Vorma Board: a complete example application.
//!
//! This crate shows the usual Vorma shape: one app declaration module, typed views,
//! typed resources, scoped middleware, task-backed domain reads, generated TypeScript,
//! and a document shell shared by HTML and JSON responses.

mod document;
pub mod maintenance;
pub mod repo;
mod resources;
pub mod session;
pub mod store;
mod views;

pub use repo::{Comment, Story, User};
pub use resources::{MAX_STORY_ATTACHMENT_BYTES, MAX_STORY_ATTACHMENTS, STORY_TAGS};
pub use store::{AppState, Db};

pub(crate) const APP_NAME: &str = "Vorma Board";
pub const CSRF_ECHO_HEADER: &str = "x-board-csrf-echo";
pub const CSRF_HEADER: &str = "x-board-csrf-token";
pub const MARK_ASSET: &str = "mark.svg";
pub const REQUEST_BODY_LIMIT: usize = 256 * 1024;

vorma::app!(pub mod app for crate::AppState);

#[derive(Clone, Debug, serde::Serialize, vorma::TsGen)]
pub struct KeyboardShortcut {
	keys: String,
	action: String,
}

/// Build the normal app config used by the dev server and production server.
///
/// Real apps usually keep all Vorma wiring behind one function like this so the build
/// command, server binary, tests, and local tools all assemble the exact same graph. The
/// server binary calls [`db_path`] and [`app_config_with`] directly instead of this
/// function only because it also needs the opened [`Db`] handle itself, to hand the exact
/// same connection pool to its background maintenance worker.
pub fn app_config() -> vorma::Result<vorma::AppConfig<AppState>> {
	app_config_with_db(store::Db::open(&db_path())?)
}

/// Path to the production SQLite database file.
pub fn db_path() -> std::path::PathBuf {
	std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("board.db")
}

/// Build the same app against a caller-owned database.
///
/// `TestApp` uses this to run each test against an isolated SQLite file. The important
/// pattern is that tests should vary application state, not rebuild a different route
/// graph than the one the server uses.
pub fn app_config_with_db(db: std::sync::Arc<Db>) -> vorma::Result<vorma::AppConfig<AppState>> {
	app_config_with(db, vorma::tasks::TasksOptions::default())
}

/// Build the app with explicit task-runtime options.
///
/// Most apps do not need to expose this. Board keeps it as a teaching hook for advanced
/// tests and tools that need to control the task runtime while keeping the same public
/// app declaration.
pub fn app_config_with(
	db: std::sync::Arc<Db>,
	tasks_options: vorma::tasks::TasksOptions<vorma::Error>,
) -> vorma::Result<vorma::AppConfig<AppState>> {
	repo::seed_docs(
		&db,
		&std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("seed-data/docs"),
	)?;
	/*
	`extra_ts` is for app-owned constants and types that should travel through
	the same generated client module as the route/resource contract. Use it for
	values the browser should import from `vorma.gen.ts`, not for ordinary
	application code.
	*/
	let mut extra_ts = vorma::tsgen::TsDrafter::new();
	extra_ts
		.export_const(
			"keyboard_shortcuts",
			[KeyboardShortcut {
				keys: "mod+k".to_owned(),
				action: "focus-search".to_owned(),
			}],
		)
		.map_err(|source| vorma::Error::new(source.to_string()))?
		.export_const("front_page_size", repo::FRONT_PAGE_SIZE)
		.map_err(|source| vorma::Error::new(source.to_string()))?
		.export_const("csrf_header", CSRF_HEADER)
		.map_err(|source| vorma::Error::new(source.to_string()))?
		.export_const("story_tags", resources::STORY_TAGS)
		.map_err(|source| vorma::Error::new(source.to_string()))?
		.export_const("max_story_attachments", resources::MAX_STORY_ATTACHMENTS)
		.map_err(|source| vorma::Error::new(source.to_string()))?
		.export_const(
			"max_story_attachment_bytes",
			resources::MAX_STORY_ATTACHMENT_BYTES,
		)
		.map_err(|source| vorma::Error::new(source.to_string()))?
		.export_type("ModAction", r#""kill" | "restore""#)
		.map_err(|source| vorma::Error::new(source.to_string()))?;
	Ok(vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		dist_dir: ".".to_owned(),
		server_target: vorma::ServerTarget {
			cargo_package: env!("CARGO_PKG_NAME").to_owned(),
			cargo_bin: "board-server".to_owned(),
		},
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
			/*
			`extra_types` registers Rust-owned types that are not already
			reachable from a view or resource contract but should still be
			available to browser code.
			*/
			extra_types: vec![
				vorma::tsgen::TsExtraType::of::<KeyboardShortcut>()
					.map_err(|source| vorma::Error::new(source.to_string()))?,
			],
			extra_ts,
		},
		dev_watch_config: vorma::DevWatchConfig {
			/*
			The dev watcher separates source changes by effect. Rust and schema
			changes rebuild the server; seed-data changes only revalidate the
			client because the graph did not change.
			*/
			watch_patterns: vec!["src/**/*".to_owned(), "public/**/*".to_owned()],
			on_change_recompile_server: vec!["src/**/*.rs".to_owned(), "src/schema.sql".to_owned()],
			on_change_client_revalidate: vec!["seed-data/**/*".to_owned()],
		},
		state: AppState { db },
		views: app::views![
			views::LAYOUT,
			views::FRONT,
			views::STORY,
			views::SUBMIT,
			views::USER,
			views::USER_COMMENTS,
			views::DOCS_INDEX_VIEW,
			views::DOCS_PAGE_VIEW,
			views::SEARCH,
			views::MOD,
			views::MOD_DIAGNOSTICS,
			views::NOT_FOUND,
		],
		resources: app::resources![
			resources::LOGIN,
			resources::LOGOUT,
			resources::SUBMIT_STORY,
			resources::VOTE_STORY,
			resources::CREATE_COMMENT,
			resources::SEARCH,
			resources::KILL_STORY,
			resources::RESTORE_STORY,
			resources::MOD_EXPORT,
			resources::STORY_ATTACHMENT,
		],
		middlewares: app::middlewares![
			session::current_user_preload(),
			session::csrf_header_echo(),
			session::mod_gate(),
		],
		tasks_options,
		document: document::document(),
		request_body_limit: REQUEST_BODY_LIMIT,
	})
}
