use std::path::Path;

use crate::RunMode;
use crate::config::to_cfg;
use crate::generation::GenerationCandidate;
use crate::live_refresh::{prepare_dev_live_generation, prepare_prod_live_generation};
use crate::session::BuildSession;
use crate::static_build::{StaticBuildInput, prepare_static_build, publish_static_outputs};
use crate::utils::create_dir_all_no_symlinks;
use paranoid::local_lock::ProcessLock;

pub(crate) fn prepare_generation_candidate(
	session: &mut BuildSession,
) -> Result<GenerationCandidate, String> {
	let bootstrap_config = session.bootstrap_config_view()?.source().clone();
	let bootstrap_cfg = to_cfg(&bootstrap_config)
		.map_err(|err| format!("error converting bootstrap config: {err}"))?;
	ensure_static_output_dirs(&bootstrap_cfg)?;
	bootstrap_cfg.write_gitignore()?;
	if session.mode() == RunMode::Dev {
		ensure_dev_lock(&bootstrap_cfg, session)?;
	}

	let build_cancel = session.runtime().build_cancel();
	let live = match session.mode() {
		RunMode::Build => {
			prepare_prod_live_generation(&bootstrap_cfg, session.build_entry(), &build_cancel)?
		}
		RunMode::Dev => {
			prepare_dev_live_generation(&bootstrap_cfg, session.build_entry(), &build_cancel)?
		}
	};

	let current_cfg =
		to_cfg(&live.config).map_err(|err| format!("error converting live config: {err}"))?;
	ensure_static_output_dirs(&current_cfg)?;
	current_cfg.write_gitignore()?;

	let previous_static = session
		.committed()
		.map(|generation| generation.static_metadata());
	let static_input = StaticBuildInput {
		config: &live.config,
		previous_static,
		build_cancel: build_cancel.clone(),
		includes_client_revalidate: false,
	};
	let prepared_static = prepare_static_build(&static_input)?;
	let (static_metadata, static_effects) =
		publish_static_outputs(&live.config, &live.live, prepared_static, &build_cancel)?;

	Ok(GenerationCandidate {
		config: live.config,
		live: live.live,
		static_metadata,
		manifest: None,
		artifacts: live.artifacts,
		static_effects,
	})
}

fn ensure_dev_lock(
	cfg: &crate::config::VormaCfg<'_>,
	session: &mut BuildSession,
) -> Result<(), String> {
	if session.runtime().has_dev_lock() {
		return Ok(());
	}
	let mut dev_lock = ProcessLock::new(cfg.dev_lock_out());
	dev_lock
		.acquire()
		.map_err(|err| format!("error acquiring dev lock: {err}"))?;
	session.runtime_mut().hold_dev_lock(dev_lock);
	Ok(())
}

fn ensure_static_output_dirs(cfg: &crate::config::VormaCfg<'_>) -> Result<(), String> {
	create_dir_all_no_symlinks(cfg.vorma_out())
		.map_err(|err| format!("error ensuring Vorma output root: {err}"))?;
	create_dir_all_no_symlinks(cfg.pub_out())
		.map_err(|err| format!("error ensuring public static output root: {err}"))?;
	let ts_gen_out = cfg.ts_gen_out_file();
	let parent = Path::new(&ts_gen_out)
		.parent()
		.ok_or_else(|| "TypeScript output file has no parent directory".to_owned())?;
	create_dir_all_no_symlinks(parent)
		.map_err(|err| format!("error ensuring generated TypeScript output dir: {err}"))?;
	Ok(())
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::config::to_cfg;

	fn temp_root(name: &str) -> std::path::PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-pipeline-{name}-{nonce}"));
		fs::create_dir_all(root.join("public")).unwrap();
		root
	}

	#[test]
	fn ensure_static_output_dirs_creates_framework_owned_dirs_and_gitignore_parent() {
		let root = temp_root("dirs");
		let config = vorma::__private::Config {
			root_dir: root.clone(),
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
			..vorma::__private::Config::default()
		};
		let cfg = to_cfg(&config).unwrap();

		ensure_static_output_dirs(&cfg).unwrap();
		cfg.write_gitignore().unwrap();

		assert!(root.join("dist/.vorma/static/public").is_dir());
		assert_eq!(
			fs::read_to_string(root.join("dist/.vorma/.gitignore")).unwrap(),
			"*\n"
		);

		fs::remove_dir_all(root).unwrap();
	}
}
