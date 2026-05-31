use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use vorma::__private::Config;

use crate::build_cancel::BuildCancel;
use crate::config::{CargoBinTarget, VormaCfg};
use crate::generation::{BuildArtifactMode, BuildArtifacts, LiveMetadata};
use crate::live_state::{
	LiveState, read_live_state_from_build_entry_executable, validate_live_state_protocol,
};
use crate::local_cargo::{
	CargoBuildArtifacts, compile_build_entry, compile_dev_targets_with_build_entry_ready,
};

#[derive(Clone, Debug, PartialEq)]
pub(crate) struct PreparedLiveGeneration {
	pub(crate) config: Config,
	pub(crate) live: LiveMetadata,
	pub(crate) artifacts: BuildArtifacts,
}

impl PreparedLiveGeneration {
	pub(crate) fn from_prod(build_entry_executable: PathBuf, live_state: LiveState) -> Self {
		Self {
			config: live_state.vorma_config.clone(),
			live: LiveMetadata::from(live_state),
			artifacts: BuildArtifacts {
				build_entry_executable,
				mode: BuildArtifactMode::Prod,
			},
		}
	}

	pub(crate) fn from_dev(
		build_entry_executable: PathBuf,
		artifacts: CargoBuildArtifacts,
		live_state: LiveState,
	) -> Self {
		Self {
			config: live_state.vorma_config.clone(),
			live: LiveMetadata::from(live_state),
			artifacts: BuildArtifacts {
				build_entry_executable,
				mode: BuildArtifactMode::Dev {
					app_server_executable: artifacts.app_server_executable,
				},
			},
		}
	}
}

impl From<LiveState> for LiveMetadata {
	fn from(live_state: LiveState) -> Self {
		Self {
			generated_ts: live_state.ts_result,
			route_modules: live_state.ts_modules,
			search_schemas: live_state.search_schemas,
			root_document_hash_source: live_state.root_document_hash_source,
		}
	}
}

pub(crate) fn prepare_prod_live_generation(
	cfg: &VormaCfg<'_>,
	build_entry: &CargoBinTarget,
	build_cancel: &Arc<BuildCancel>,
) -> Result<PreparedLiveGeneration, String> {
	let artifact = compile_build_entry(cfg, build_entry, build_cancel)
		.map_err(|err| format!("error compiling build entry: {err}"))?
		.build_entry_executable;
	let live_state =
		read_live_state_from_build_entry_executable(&artifact, cfg.root_dir(), build_cancel)
			.map_err(|err| format!("error reading live state: {err}"))?;
	validate_live_state_protocol(&live_state)
		.map_err(|err| format!("invalid live state from build entry: {err}"))?;
	Ok(PreparedLiveGeneration::from_prod(artifact, live_state))
}

pub(crate) fn prepare_dev_live_generation(
	cfg: &VormaCfg<'_>,
	build_entry: &CargoBinTarget,
	build_cancel: &Arc<BuildCancel>,
) -> Result<PreparedLiveGeneration, String> {
	let root_dir = cfg.root_dir();
	let live_state_handle = Arc::new(Mutex::new(None));
	let live_state_handle_for_callback = Arc::clone(&live_state_handle);
	let build_cancel_for_live_state = Arc::clone(build_cancel);
	let root_dir_for_live_state = root_dir.clone();
	let artifacts = match compile_dev_targets_with_build_entry_ready(
		cfg,
		build_entry,
		build_cancel,
		move |build_entry_executable| {
			let root_dir = root_dir_for_live_state;
			let build_cancel = build_cancel_for_live_state;
			let executable_for_result = build_entry_executable.clone();
			let handle = std::thread::spawn(move || {
				let live_state = read_live_state_from_build_entry_executable(
					&build_entry_executable,
					&root_dir,
					&build_cancel,
				)
				.map_err(|err| err.to_string())?;
				Ok::<_, String>((executable_for_result, live_state))
			});
			*live_state_handle_for_callback
				.lock()
				.expect("live state handle lock poisoned") = Some(handle);
		},
	) {
		Ok(artifacts) => artifacts,
		Err(err) => {
			if let Some(handle) = live_state_handle
				.lock()
				.expect("live state handle lock poisoned")
				.take()
			{
				let _ = handle.join();
			}
			return Err(err.to_string());
		}
	};
	let live_state_handle = live_state_handle
		.lock()
		.expect("live state handle lock poisoned")
		.take()
		.ok_or_else(|| "Cargo did not report the build entry executable".to_owned())?;
	let (build_entry_executable, live_state) = live_state_handle
		.join()
		.map_err(|_| "live state thread panicked".to_owned())??;
	validate_live_state_protocol(&live_state)
		.map_err(|err| format!("invalid live state from dev build: {err}"))?;
	Ok(PreparedLiveGeneration::from_dev(
		build_entry_executable,
		artifacts,
		live_state,
	))
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::live_state::LiveState;
	use crate::ts_gen::LiveTsResult;
	use crate::ts_modules::TsRoute;
	use std::collections::BTreeMap;
	use std::path::PathBuf;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	fn config() -> Config {
		Config {
			root_dir: std::env::current_dir().unwrap(),
			dist_dir: "dist".to_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
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
	fn live_state_maps_to_generation_metadata_without_session_state() {
		let live_state = LiveState {
			vorma_config: config(),
			ts_result: LiveTsResult {
				routes_section: "routes".to_owned(),
				..LiveTsResult::default()
			},
			ts_modules: BTreeMap::from([(
				"/".to_owned(),
				TsRoute {
					pattern: "/".to_owned(),
					import_path: "src/root.tsx".to_owned(),
					deps: Vec::new(),
				},
			)]),
			root_document_hash_source: "doc".to_owned(),
			..LiveState::default()
		};

		let metadata = LiveMetadata::from(live_state);

		assert_eq!(metadata.generated_ts.routes_section, "routes");
		assert_eq!(metadata.route_modules["/"].import_path, "src/root.tsx");
		assert_eq!(metadata.root_document_hash_source, "doc");
	}

	#[test]
	fn dev_live_generation_records_build_entry_and_app_server_artifacts() {
		let live_state = LiveState {
			vorma_config: config(),
			root_document_hash_source: "doc".to_owned(),
			..LiveState::default()
		};

		let prepared = PreparedLiveGeneration::from_dev(
			PathBuf::from("/tmp/example-build"),
			CargoBuildArtifacts {
				build_entry_executable: PathBuf::from("/tmp/example-build"),
				app_server_executable: PathBuf::from("/tmp/example-server"),
			},
			live_state,
		);

		assert_eq!(
			prepared.artifacts.build_entry_executable,
			PathBuf::from("/tmp/example-build")
		);
		assert_eq!(
			prepared.artifacts.mode,
			BuildArtifactMode::Dev {
				app_server_executable: PathBuf::from("/tmp/example-server"),
			},
		);
	}

	#[test]
	fn prod_live_generation_records_only_build_entry_artifact() {
		let live_state = LiveState {
			vorma_config: config(),
			root_document_hash_source: "doc".to_owned(),
			..LiveState::default()
		};

		let prepared =
			PreparedLiveGeneration::from_prod(PathBuf::from("/tmp/example-build"), live_state);

		assert_eq!(
			prepared.artifacts.build_entry_executable,
			PathBuf::from("/tmp/example-build")
		);
		assert_eq!(prepared.artifacts.mode, BuildArtifactMode::Prod);
	}
}
