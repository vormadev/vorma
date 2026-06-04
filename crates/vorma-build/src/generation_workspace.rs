use std::path::Path;

use crate::config::VormaCfg;
use crate::constants::GITIGNORE_CONTENT;
use crate::utils::{create_dir_all_no_symlinks, write_str_to_file};

pub(crate) fn prepare_generation_output_layout(cfg: &VormaCfg<'_>) -> Result<(), String> {
	ensure_framework_output_dirs(cfg)?;
	write_str_to_file(GITIGNORE_CONTENT, cfg.gitignore_out())
		.map_err(|err| format!("error writing .gitignore: {err}"))?;
	Ok(())
}

fn ensure_framework_output_dirs(cfg: &VormaCfg<'_>) -> Result<(), String> {
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
	use std::path::PathBuf;
	use std::time::{SystemTime, UNIX_EPOCH};

	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	use super::*;
	use crate::config::to_cfg;

	fn temp_root(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		let root = std::env::temp_dir().join(format!("vorma-generation-workspace-{name}-{nonce}"));
		fs::create_dir_all(root.join("public")).unwrap();
		root
	}

	#[test]
	fn prepare_generation_output_layout_creates_framework_owned_dirs_and_gitignore() {
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

		prepare_generation_output_layout(&cfg).unwrap();

		assert!(root.join("dist/.vorma/static/public").is_dir());
		assert_eq!(
			fs::read_to_string(root.join("dist/.vorma/.gitignore")).unwrap(),
			"*\n"
		);

		fs::remove_dir_all(root).unwrap();
	}
}
