//! Vorma Notes: a production-shaped app that exercises the full surface.

mod document;
mod resources;
mod session;
mod store;
mod views;

pub use store::AppState;

pub(crate) const APP_NAME: &str = "Vorma Notes";
pub(crate) const APP_DESCRIPTION: &str =
	"A production-shaped Vorma app that exercises the full surface.";
pub const REQUEST_BODY_LIMIT: usize = 64 * 1024;

vorma::app!(pub mod app for crate::AppState);

/*
Emitted into the generated TypeScript even though no route references it:
the client-side command palette consumes it directly.
*/
#[derive(Clone, Debug, serde::Serialize, vorma::TsGen)]
pub struct KeyboardShortcut {
	keys: String,
	action: String,
}

pub fn app_config() -> vorma::Result<vorma::AppConfig<AppState>> {
	let mut extra_ts = vorma::tsgen::TsDrafter::new();
	extra_ts
		.export_const(
			"keyboardShortcuts",
			[KeyboardShortcut {
				keys: "mod+k".to_owned(),
				action: "focus-composer".to_owned(),
			}],
		)
		.map_err(|source| vorma::Error::new(source.to_string()))?;

	Ok(vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		dist_dir: ".".to_owned(),
		server_target: vorma::ServerTarget {
			cargo_package: env!("CARGO_PKG_NAME").to_owned(),
			cargo_bin: "notes-server".to_owned(),
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
			extra_types: vec![
				vorma::tsgen::TsExtraType::of::<KeyboardShortcut>()
					.map_err(|source| vorma::Error::new(source.to_string()))?,
			],
			extra_ts,
		},
		dev_watch_config: vorma::DevWatchConfig {
			watch_patterns: vec!["src/**/*".to_owned(), "public/**/*".to_owned()],
			on_change_recompile_server: vec!["src/**/*.rs".to_owned()],
			on_change_client_revalidate: vec!["seed-data/**/*".to_owned()],
		},
		state: store::seed_state(),
		views: app::views![
			views::LAYOUT,
			views::HOME,
			views::NOTE,
			views::TAGS,
			views::STATS,
			views::ADMIN,
			views::LEGACY,
		],
		resources: app::resources![
			resources::CREATE_NOTE,
			resources::DELETE_NOTE,
			resources::IMPORT_NOTES,
			resources::DUPLICATE_NOTE,
			resources::LOGIN,
			resources::LOGOUT,
			resources::PING,
		],
		middlewares: app::middlewares![session::session_gate(), request_tag_middleware()],
		tasks_options: vorma::TasksOptions::default(),
		document: document::document(),
		request_body_limit: REQUEST_BODY_LIMIT,
	})
}

/// Stamp every Vorma response so proxies can attribute it.
fn request_tag_middleware() -> app::Middleware {
	app::Middleware::new(|ctx| async move {
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-notes-task-middleware"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(())
	})
}
