//! Pins `app!`'s accepted `$state:ty` path forms.
//!
//! `app!` used to compile only when `$state` was a `crate::`-anchored path,
//! because the substituted tokens resolved inside the macro's generated
//! nested module rather than at the call site — a bare local state name, a
//! `use`-imported bare name, and a `self::`-qualified path all failed with
//! `cannot find type ... in this scope`. The anchor-alias fix (a
//! `type __VormaAppState = $state;` emitted beside the generated module,
//! referenced as `super::__VormaAppState` inside it) resolves `$state` at
//! the call site once and reuses that resolution, so every one of these
//! forms compiles alongside the pre-existing `crate::`-anchored form. This
//! file is the from-red-to-green pin for that widening; every existing
//! caller's `crate::`-anchored form is exercised elsewhere (for example
//! `tests/app_declaration_contract.rs`) and the full workspace build is the
//! pin that they stay green.

mod bare_name_form {
	#[derive(Default)]
	struct BareState;

	vorma::app!(mod bare_app for BareState);

	#[test]
	fn bare_local_state_name_compiles_and_resolves() {
		let app = bare_app::App::from_config(super::config_for(BareState));
		assert!(app.is_err());
	}
}

mod use_imported_form {
	mod state_owner {
		#[derive(Default)]
		pub struct ImportedState;
	}

	use state_owner::ImportedState;

	vorma::app!(mod imported_app for ImportedState);

	#[test]
	fn use_imported_bare_state_name_compiles_and_resolves() {
		let app = imported_app::App::from_config(super::config_for(ImportedState));
		assert!(app.is_err());
	}
}

mod self_qualified_form {
	#[derive(Default)]
	struct SelfState;

	vorma::app!(mod self_app for self::SelfState);

	#[test]
	fn self_qualified_state_path_compiles_and_resolves() {
		let app = self_app::App::from_config(super::config_for(SelfState));
		assert!(app.is_err());
	}
}

/// Shared minimal (and intentionally invalid, since only the state-type
/// resolution is under test) [`vorma::AppConfig`] builder, generic over
/// each form's distinct state type. Every field beyond `state` is filler;
/// `dist_dir` is deliberately empty so `App::from_config` fails cheaply
/// without touching the filesystem, keeping this a pure compile/resolve
/// pin rather than a build-behavior test.
fn config_for<State: Send + Sync + 'static>(state: State) -> vorma::AppConfig<State> {
	vorma::AppConfig {
		root_dir: env!("CARGO_MANIFEST_DIR").into(),
		server_target: vorma::ServerTarget {
			cargo_package: "state-path-forms-package".to_owned(),
			cargo_bin: "state-path-forms-server".to_owned(),
		},
		dist_dir: String::new(),
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
		state,
		views: vorma::Views::new(),
		resources: vorma::Resources::new(),
		middlewares: vorma::Middlewares::new(),
		tasks_options: vorma::tasks::TasksOptions::default(),
		document: vorma::DocumentBuilder::new(|_ctx| async move { Ok(vorma::Document::new()) }),
		request_body_limit: 1024 * 1024,
	}
}
