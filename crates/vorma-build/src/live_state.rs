use std::collections::BTreeMap;
use std::fmt;
use std::path::Path;
use std::process::{Command, Output};
use std::sync::Arc;

use serde::{Deserialize, Serialize};
use serde_json::Value;
use vorma::__private::Config;
use vorma::__private::core::Contract;
use vorma::__private::core::contract_for;
use vorma::{DocumentBuildCtx, DocumentBuilder, Resources, Views};

use crate::build_cancel::BuildCancel;
use crate::command_runner::{
	CommandRunError, CommandStderr, run_command_collecting_output_preserving_env,
};
use crate::config::{VormaCfg, to_cfg};
use crate::constants::LIVE_STATE_MODE_ENV_KEY;
use crate::supervisor::clear_vorma_runtime_env;
use crate::ts_gen::{LiveTsResult, to_live_ts_result};
use crate::ts_modules::TsViewModule;
use crate::ts_modules::{get_dev_view_modules_from_contract, validate_dev_view_modules_for_config};
use vorma::__private::constants::ENV_KEY_IS_BUILD;

#[derive(Clone, Debug, Default, Deserialize, PartialEq, Serialize)]
pub(crate) struct LiveState {
	#[serde(default)]
	pub(crate) error: String,
	pub(crate) vorma_config: Config,
	pub(crate) ts_result: LiveTsResult,
	pub(crate) ts_modules: BTreeMap<String, TsViewModule>,
	pub(crate) search_schemas: BTreeMap<String, Value>,
	pub(crate) root_document_hash_source: String,
}

#[derive(Debug)]
pub(crate) enum LiveStateError {
	CommandStart {
		program: String,
		source: std::io::Error,
	},
	CommandWait {
		program: String,
		source: std::io::Error,
	},
	CommandFailed {
		program: String,
		output: Output,
	},
	Cancelled {
		program: String,
	},
	Parse {
		message: String,
	},
	App {
		message: String,
	},
}

pub(crate) fn read_live_state_from_build_entry_executable(
	executable: impl AsRef<Path>,
	root_dir: impl AsRef<Path>,
	build_cancel: &Arc<BuildCancel>,
) -> Result<LiveState, LiveStateError> {
	let executable = executable.as_ref();
	let root_dir = root_dir.as_ref();
	let program = executable.to_string_lossy().into_owned();
	let mut command = Command::new(executable);
	clear_vorma_runtime_env(&mut command);
	command
		.current_dir(root_dir)
		.env(ENV_KEY_IS_BUILD, "1")
		.env(LIVE_STATE_MODE_ENV_KEY, "1");
	let output =
		run_command_collecting_output_preserving_env(command, build_cancel, CommandStderr::Collect)
			.map_err(|err| live_state_error_from_command_error(program.clone(), err))?;
	parse_live_state(&output.stdout)
}

fn live_state_error_from_command_error(program: String, error: CommandRunError) -> LiveStateError {
	match error {
		CommandRunError::CommandStart { source } => {
			LiveStateError::CommandStart { program, source }
		}
		CommandRunError::CommandWait { source } => LiveStateError::CommandWait { program, source },
		CommandRunError::CommandFailed { output } => {
			LiveStateError::CommandFailed { program, output }
		}
		CommandRunError::Cancelled => LiveStateError::Cancelled { program },
	}
}

pub(crate) fn parse_live_state(bytes: &[u8]) -> Result<LiveState, LiveStateError> {
	#[derive(Deserialize)]
	struct ErrorEnvelope {
		#[serde(default)]
		error: String,
	}

	let value: Value = serde_json::from_slice(bytes).map_err(|error| LiveStateError::Parse {
		message: format!("error parsing builder entry output: {error}"),
	})?;
	let error_envelope: ErrorEnvelope =
		serde_json::from_value(value.clone()).map_err(|error| LiveStateError::Parse {
			message: format!("error parsing builder entry output: {error}"),
		})?;
	if !error_envelope.error.is_empty() {
		return Err(LiveStateError::App {
			message: format!("error from builder entry: {}", error_envelope.error),
		});
	}
	let live_state: LiveState =
		serde_json::from_value(value).map_err(|error| LiveStateError::Parse {
			message: format!("error parsing builder entry output: {error}"),
		})?;
	validate_live_state_protocol(&live_state)
		.map_err(|message| LiveStateError::Parse { message })?;
	Ok(live_state)
}

pub(crate) fn validate_live_state_protocol(live_state: &LiveState) -> Result<(), String> {
	let cfg = to_cfg(&live_state.vorma_config)
		.map_err(|err| format!("builder entry output contains invalid config: {err}"))?;
	validate_dev_view_modules_for_config(&cfg, &live_state.ts_modules)
		.map_err(|err| format!("builder entry output contains invalid TS modules: {err}"))?;
	if live_state.root_document_hash_source.trim().is_empty() {
		return Err("builder entry output missing root_document_hash_source".to_owned());
	}
	Ok(())
}

#[cfg(test)]
pub(crate) fn get_live_state(
	config: &Config,
	contract: &Contract,
	root_document_hash_source: impl Into<String>,
) -> Result<LiveState, String> {
	let cfg = to_cfg(config).map_err(|err| format!("error converting config: {err}"))?;
	let mut live_state = get_live_state_from_contract(config, &cfg, contract)?;
	live_state.root_document_hash_source = root_document_hash_source.into();
	Ok(live_state)
}

fn get_live_state_from_contract(
	config: &Config,
	cfg: &VormaCfg<'_>,
	contract: &Contract,
) -> Result<LiveState, String> {
	let ts_result = to_live_ts_result(cfg, contract)
		.map_err(|err| format!("error generating TS types: {err}"))?;
	let ts_modules = get_dev_view_modules_from_contract(cfg, contract)
		.map_err(|err| format!("error getting TS modules: {err}"))?;
	let search_schemas = contract
		.views()
		.iter()
		.map(|view| (view.pattern.clone(), view.search_schema.clone()))
		.collect::<BTreeMap<_, _>>();

	Ok(LiveState {
		vorma_config: config.clone(),
		ts_result,
		ts_modules,
		search_schemas,
		..LiveState::default()
	})
}

pub(crate) fn get_live_state_from_app<S, E>(
	config: &Config,
	views: &Views<S, E>,
	resources: &Resources<S, E>,
	document: &DocumentBuilder,
) -> Result<LiveState, String>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let cfg = to_cfg(config).map_err(|err| format!("error converting config: {err}"))?;
	let contract = contract_for(views, resources)
		.map_err(|err| format!("error collecting app contract: {err}"))?;
	let mut live_state = get_live_state_from_contract(config, &cfg, &contract)?;
	live_state.root_document_hash_source = root_document_hash_source(document)?;
	Ok(live_state)
}

pub(crate) fn root_document_hash_source(document: &DocumentBuilder) -> Result<String, String> {
	let runtime = tokio::runtime::Builder::new_current_thread()
		.enable_all()
		.build()
		.map_err(|err| format!("error preparing document build runtime: {err}"))?;
	let document = runtime
		.block_on(document.build(DocumentBuildCtx::build()))
		.map_err(|err| format!("error building root document shell: {err}"))?;
	crate::document_hash::hash_source(&document)
		.map_err(|err| format!("error serializing root document hash source: {err}"))
}

impl fmt::Display for LiveStateError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::CommandStart { program, source } => {
				write!(
					f,
					"error running builder entry command {program:?}: {source}"
				)
			}
			Self::CommandWait { program, source } => {
				write!(
					f,
					"error waiting for builder entry command {program:?}: {source}"
				)
			}
			Self::CommandFailed { program, output } => {
				let mut combined = String::new();
				combined.push_str(&String::from_utf8_lossy(&output.stdout));
				combined.push_str(&String::from_utf8_lossy(&output.stderr));
				write!(
					f,
					"error running builder entry command {program:?}: status {}; output: {combined}",
					output.status,
				)
			}
			Self::Cancelled { program } => {
				write!(f, "builder entry command {program:?} cancelled")
			}
			Self::Parse { message } => write!(f, "{message}"),
			Self::App { message } => write!(f, "{message}"),
		}
	}
}

impl std::error::Error for LiveStateError {}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::build_cancel::BuildCancel;
	use std::sync::Arc;
	use std::sync::atomic::{AtomicBool, Ordering};
	use vorma::__private::Config;
	use vorma::__private::core::contract_for;
	use vorma::{
		Document, DocumentBuilder, FrontendConfig, PathConfig, Resources, ServerConfig,
		TsGenConfig, Views,
	};

	vorma::app!(mod live_state_app for ());

	fn valid_config() -> Config {
		Config {
			root_dir: crate::test_support::root_dir(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				js_package_manager_base_cmd: "pnpm exec".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/client/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				..FrontendConfig::default()
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/client/vorma.gen.ts".to_owned(),
				..TsGenConfig::default()
			},
			..Config::default()
		}
	}

	#[test]
	fn parse_live_state_reads_direct_json() {
		let live_state = LiveState {
			vorma_config: valid_config(),
			ts_result: LiveTsResult {
				routes_section: "routes".to_owned(),
				..LiveTsResult::default()
			},
			ts_modules: BTreeMap::from([(
				"/".to_owned(),
				TsViewModule {
					pattern: "/".to_owned(),
					import_path: "src/root.tsx".to_owned(),
					deps: Vec::new(),
				},
			)]),
			search_schemas: BTreeMap::new(),
			root_document_hash_source: "document-hash-source".to_owned(),
			..LiveState::default()
		};
		let json = serde_json::to_vec(&live_state).unwrap();

		let got = parse_live_state(&json).unwrap();

		assert_eq!(got.ts_result.routes_section, "routes");
		assert_eq!(got.ts_modules["/"].import_path, "src/root.tsx");
	}

	#[test]
	fn parse_live_state_reads_root_document_hash_source() {
		let json = serde_json::to_vec(&LiveState {
			vorma_config: valid_config(),
			ts_result: LiveTsResult::default(),
			ts_modules: BTreeMap::new(),
			search_schemas: BTreeMap::new(),
			root_document_hash_source: "document-hash-source".to_owned(),
			..LiveState::default()
		})
		.unwrap();

		let got = parse_live_state(&json).unwrap();

		assert_eq!(got.root_document_hash_source, "document-hash-source");
	}

	#[test]
	fn parse_live_state_rejects_partial_success_json() {
		let error = parse_live_state(br#"{}"#).unwrap_err();

		assert!(
			error
				.to_string()
				.contains("error parsing builder entry output: missing field"),
			"{error}",
		);
	}

	#[test]
	fn parse_live_state_rejects_empty_document_hash_source() {
		let json = serde_json::to_vec(&LiveState {
			vorma_config: valid_config(),
			ts_result: LiveTsResult::default(),
			ts_modules: BTreeMap::new(),
			search_schemas: BTreeMap::new(),
			root_document_hash_source: String::new(),
			..LiveState::default()
		})
		.unwrap();

		let error = parse_live_state(&json).unwrap_err();

		assert_eq!(
			error.to_string(),
			"builder entry output missing root_document_hash_source",
		);
	}

	#[test]
	fn parse_live_state_rejects_view_module_outside_root_dir() {
		let json = serde_json::to_vec(&LiveState {
			vorma_config: valid_config(),
			ts_result: LiveTsResult::default(),
			ts_modules: BTreeMap::from([(
				"/".to_owned(),
				TsViewModule {
					pattern: "/".to_owned(),
					import_path: "../root.tsx".to_owned(),
					deps: Vec::new(),
				},
			)]),
			search_schemas: BTreeMap::new(),
			root_document_hash_source: "document-hash-source".to_owned(),
			..LiveState::default()
		})
		.unwrap();

		let error = parse_live_state(&json).unwrap_err();

		assert_eq!(
			error.to_string(),
			"builder entry output contains invalid TS modules: TS view import path must be inside root_dir for pattern: /"
		);
	}

	#[test]
	fn parse_live_state_reports_app_error() {
		let json = br#"{"error":"broken"}"#;

		let error = parse_live_state(json).unwrap_err();

		assert_eq!(error.to_string(), "error from builder entry: broken");
	}

	#[cfg(unix)]
	#[test]
	fn read_live_state_from_build_entry_executable_drains_stderr_while_waiting() {
		use std::os::unix::fs::PermissionsExt;

		let path = std::env::temp_dir().join(format!(
			"vorma-live-state-drain-{}-{}.sh",
			std::process::id(),
			crate::utils::random_id(8).unwrap()
		));
		std::fs::write(
            &path,
            r#"#!/bin/sh
if [ "$__VORMA_IS_BUILD" != "1" ] || [ "$__VORMA_LIVE_STATE_MODE" != "1" ]; then
    cat <<'JSON'
{"error":"missing live state env"}
JSON
    exit 0
fi
i=0
while [ "$i" -lt 20000 ]; do
    printf 'stderr-line-%05d\n' "$i" >&2
    i=$((i + 1))
done
cat <<'JSON'
{"vorma_config":{"root_dir":"/tmp","server_config":{"cargo_package":"example-app","cargo_bin":"example-server"},"dist_dir":"dist","path_config":{"public_static_base":"/static/","api_base":"/api/"},"frontend_config":{"ui_variant":"react","js_package_manager_base_cmd":"pnpm exec","js_package_manager_dir":".","vite_config_file":"","entry_file":"src/client/entry.tsx","public_static_src_dir":"public","critical_css_file":""},"ts_gen_config":{"out_file":"src/client/vorma.gen.ts"},"dev_watch_config":{"watch_patterns":[],"on_change_recompile_server":[],"on_change_client_revalidate":[]}},"ts_result":{"routes_section":"","core_types_section":"","baked_app_types_section":"","user_ts_section":""},"ts_modules":{},"search_schemas":{},"root_document_hash_source":"document-hash-source"}
JSON
"#,
        )
        .unwrap();
		let mut perms = std::fs::metadata(&path).unwrap().permissions();
		perms.set_mode(0o755);
		std::fs::set_permissions(&path, perms).unwrap();
		let build_cancel = Arc::new(BuildCancel::new(false));

		let live_state =
			read_live_state_from_build_entry_executable(&path, ".", &build_cancel).unwrap();

		assert!(live_state.error.is_empty());
		let _ = std::fs::remove_file(path);
	}

	#[test]
	fn get_live_state_builds_ts_modules_and_search_schemas_from_contract() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: vorma::FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let views = live_state_app::views![live_state_app::view! {
			client_file: "src/root.tsx";
			pattern: "/";
			input: ();
			output: ();

			handler: |ctx| {
				let _: live_state_app::ViewCtx = ctx;
				Ok(())
			};
		}];
		let resources = live_state_app::resources![live_state_app::resource! {
			method: vorma::HttpMethod::GET;
			pattern: "/ping";
			input: ();
			output: ();

			handler: |ctx| {
				let _: live_state_app::ResourceCtx = ctx;
				Ok(())
			};
		}];
		let contract = contract_for(&views, &resources).unwrap();

		let got = get_live_state(&config, &contract, "<html></html>").unwrap();

		assert_eq!(got.vorma_config, config);
		assert!(
			got.ts_result
				.routes_section
				.contains("const __vorma_resources = [")
		);
		assert_eq!(got.ts_modules["/"].import_path, "src/root.tsx");
		assert_eq!(got.search_schemas["/"], serde_json::json!({}));
		assert_eq!(got.root_document_hash_source, "<html></html>");
	}

	#[test]
	fn get_live_state_from_app_builds_contract_and_document_hash_source() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let views = live_state_app::views![live_state_app::view! {
			client_file: "src/root.tsx";
			pattern: "/";
			input: ();
			output: ();

			handler: |ctx| {
				let _: live_state_app::ViewCtx = ctx;
				Ok(())
			};
		}];
		let resources = live_state_app::resources![];
		let document = DocumentBuilder::new(|ctx| async move {
			let mut document = Document::new();
			document.head().title("Example");
			let stylesheet = ctx.public_url("app.css")?;
			let head = document.head();
			head.link([head.rel("stylesheet").into(), head.href(stylesheet).into()]);
			Ok(document)
		});

		let got = get_live_state_from_app(&config, &views, &resources, &document).unwrap();
		let root_document = root_document_hash_source(&got);

		assert!(got.ts_modules.contains_key("/"));
		assert!(
			root_document_head_defaults(&root_document)
				.iter()
				.any(|el| { el["tag"] == "title" && el["text_content"] == "Example" })
		);
		assert!(
			root_document_head_defaults(&root_document)
				.iter()
				.any(|el| { el["tag"] == "link" && el["attributes"]["href"] == "/app.css" })
		);
	}

	#[test]
	fn get_live_state_from_app_validates_config_before_document_build() {
		let document_ran = Arc::new(AtomicBool::new(false));
		let document_ran_for_builder = Arc::clone(&document_ran);
		let config = Config::default();
		let views: Views<(), &'static str> = Views::new();
		let resources: Resources<(), &'static str> = Resources::new();
		let document = DocumentBuilder::new(move |_| {
			document_ran_for_builder.store(true, Ordering::SeqCst);
			async move { Err("document should not run before config validation".to_owned()) }
		});

		let error = get_live_state_from_app(&config, &views, &resources, &document).unwrap_err();

		assert!(error.starts_with("error converting config:"));
		assert!(!document_ran.load(Ordering::SeqCst));
	}

	#[test]
	fn get_live_state_from_app_validates_routes_before_document_build() {
		let document_ran = Arc::new(AtomicBool::new(false));
		let document_ran_for_builder = Arc::clone(&document_ran);
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		fn noop_view_handler(
			_: vorma::__private::ErasedRequestCtx<(), &'static str>,
		) -> vorma::__private::ErasedRouteFuture<&'static str> {
			Box::pin(async { Ok(serde_json::Value::Null) })
		}

		let mut views: Views<(), &'static str> = Views::new();
		views.push(vorma::View::from_static(
			"",
			"bad.view.tsx",
			vorma::__private::type_resolver::<()>,
			vorma::__private::type_resolver::<()>,
			vorma::__private::search_schema_resolver::<()>,
			noop_view_handler,
		));
		let resources: Resources<(), &'static str> = Resources::new();
		let document = DocumentBuilder::new(move |_| {
			document_ran_for_builder.store(true, Ordering::SeqCst);
			async move { Err("document should not run before route validation".to_owned()) }
		});

		let error = get_live_state_from_app(&config, &views, &resources, &document).unwrap_err();

		assert!(error.starts_with("error collecting app contract:"));
		assert!(error.contains("pattern must not be empty"));
		assert!(!document_ran.load(Ordering::SeqCst));
	}

	#[test]
	fn get_live_state_from_app_builds_document_hash_source_with_logical_public_urls() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let views: Views<(), &'static str> = Views::new();
		let resources: Resources<(), &'static str> = Resources::new();
		let document = DocumentBuilder::new(|ctx| async move {
			let mut document = Document::new();
			let app_css = ctx.public_url(" app.css ")?;
			let head = document.head();
			head.link([head.rel("stylesheet").into(), head.href(app_css).into()]);
			Ok(document)
		});

		let got = get_live_state_from_app(&config, &views, &resources, &document).unwrap();
		let root_document = root_document_hash_source(&got);

		assert!(
			root_document_head_defaults(&root_document)
				.iter()
				.any(|el| { el["tag"] == "link" && el["attributes"]["href"] == "/app.css" })
		);
	}

	#[test]
	fn get_live_state_from_app_builds_document_hash_source_with_synthetic_request() {
		let config = Config {
			root_dir: crate::test_support::root_dir(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: crate::test_support::path_config(),
			frontend_config: FrontendConfig {
				ui_variant: vorma::UiVariant::React,
				..crate::test_support::frontend_config()
			},
			ts_gen_config: crate::test_support::ts_gen_config(),
			..Config::default()
		};
		let views: Views<(), &'static str> = Views::new();
		let resources: Resources<(), &'static str> = Resources::new();
		let document = DocumentBuilder::new(|ctx| async move {
			let mut document = Document::new();
			document
				.head()
				.meta_property_content("og:url", ctx.request().path());
			Ok(document)
		});

		let got = get_live_state_from_app(&config, &views, &resources, &document).unwrap();
		let root_document = root_document_hash_source(&got);

		assert!(
			root_document_head_defaults(&root_document)
				.iter()
				.any(|el| {
					el["tag"] == "meta"
						&& el["attributes"]["property"] == "og:url"
						&& el["attributes"]["content"] == "/"
				})
		);
	}

	fn root_document_hash_source(live_state: &LiveState) -> Value {
		serde_json::from_str(&live_state.root_document_hash_source).unwrap()
	}

	fn root_document_head_defaults(root_document: &Value) -> &[Value] {
		root_document["head_defaults"].as_array().unwrap()
	}
}
