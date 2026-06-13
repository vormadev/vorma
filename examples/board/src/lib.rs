//! Vorma Board: the realistic pressure-test app (census: PRESSURE_TEST_CENSUS.md).

mod document;
pub mod repo;
mod resources;
pub mod session;
pub mod store;
mod views;

pub use repo::{Comment, Story, User};
pub use store::{AppState, Db};

pub(crate) const APP_NAME: &str = "Vorma Board";
pub const REQUEST_BODY_LIMIT: usize = 256 * 1024;

vorma::app!(pub mod app for crate::AppState);

/// Config against the default on-disk database (dev/build/serve).
pub fn app_config() -> vorma::Result<vorma::AppConfig<AppState>> {
	let db_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("board.db");
	app_config_with_db(store::Db::open(&db_path)?)
}

/// Config against a caller-owned database (tests use tmp files).
pub fn app_config_with_db(db: std::sync::Arc<Db>) -> vorma::Result<vorma::AppConfig<AppState>> {
	app_config_with(db, vorma::TasksOptions::default())
}

/// Full-control variant: tests inject task overrides/clocks here.
pub fn app_config_with(
	db: std::sync::Arc<Db>,
	tasks_options: vorma::TasksOptions<vorma::Error>,
) -> vorma::Result<vorma::AppConfig<AppState>> {
	repo::seed_docs(
		&db,
		&std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("seed-data/docs"),
	)?;
	let mut extra_ts = vorma::tsgen::TsDrafter::new();
	extra_ts
		.export_const("frontPageSize", repo::FRONT_PAGE_SIZE)
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
			extra_ts,
			..vorma::TsGenConfig::default()
		},
		dev_watch_config: vorma::DevWatchConfig {
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
			resources::STORY_ATTACHMENT,
		],
		middlewares: app::middlewares![session::current_user_preload(), session::mod_gate()],
		tasks_options,
		document: document::document(),
		request_body_limit: REQUEST_BODY_LIMIT,
	})
}
