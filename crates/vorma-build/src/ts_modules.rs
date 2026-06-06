use std::collections::{BTreeMap, BTreeSet};
use std::path::{Path, PathBuf};

use path_clean::PathClean;
use path_slash::PathBufExt;
use serde::{Deserialize, Serialize};
use vorma::__private::core::Contract;

use crate::config::VormaCfg;

#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub(crate) struct TsViewModule {
	pub(crate) pattern: String,
	pub(crate) import_path: String,
	pub(crate) deps: Vec<String>,
}

pub(crate) fn get_dev_view_modules(
	views: &[TsViewModule],
) -> Result<BTreeMap<String, TsViewModule>, String> {
	let mut view_modules = BTreeMap::new();
	let mut seen_mods = BTreeSet::new();

	for view in views {
		let pattern = &view.pattern;
		let import_path = &view.import_path;
		if pattern.is_empty() {
			return Err("TS view pattern cannot be empty".to_owned());
		}
		if import_path.is_empty() {
			return Err(format!(
				"TS view import path cannot be empty for pattern: {pattern}"
			));
		}
		if view_modules.contains_key(pattern.as_str()) {
			return Err(format!("duplicate TS view pattern: {pattern}"));
		}
		if !seen_mods.insert(import_path.to_owned()) {
			return Err(format!("duplicate TS module: {import_path}"));
		}
		view_modules.insert(pattern.to_owned(), view.clone());
	}

	Ok(view_modules)
}

pub(crate) fn get_dev_view_modules_from_contract(
	cfg: &VormaCfg<'_>,
	contract: &Contract,
) -> Result<BTreeMap<String, TsViewModule>, String> {
	let views = contract
		.views()
		.iter()
		.map(|view| TsViewModule {
			pattern: view.pattern.clone(),
			import_path: view.client_file.clone(),
			deps: Vec::new(),
		})
		.collect::<Vec<_>>();
	let modules = get_dev_view_modules(&views)?;
	validate_dev_view_modules_for_config(cfg, &modules)?;
	Ok(modules)
}

pub(crate) fn validate_dev_view_modules_for_config(
	cfg: &VormaCfg<'_>,
	modules: &BTreeMap<String, TsViewModule>,
) -> Result<(), String> {
	let root_dir = PathBuf::from(cfg.root_dir()).clean();
	let mut seen_import_paths = BTreeSet::new();
	for (pattern, module) in modules {
		if pattern != &module.pattern {
			return Err(format!(
				"TS view module map key must match module pattern: {pattern}"
			));
		}
		if !seen_import_paths.insert(module.import_path.clone()) {
			return Err(format!("duplicate TS module: {}", module.import_path));
		}
		validate_view_module_import_path(&root_dir, pattern, &module.import_path)?;
	}
	Ok(())
}

fn validate_view_module_import_path(
	root_dir: &Path,
	pattern: &str,
	import_path: &str,
) -> Result<(), String> {
	if import_path.contains('\\') {
		return Err(format!(
			"TS view import path must use slash separators for pattern: {pattern}"
		));
	}
	let path = PathBuf::from_slash(import_path.trim()).clean();
	if path == Path::new(".") {
		return Err(format!(
			"TS view import path cannot be . for pattern: {pattern}"
		));
	}
	let full_path = if path.is_absolute() {
		path
	} else {
		root_dir.join(path).clean()
	};
	if !full_path.starts_with(root_dir) {
		return Err(format!(
			"TS view import path must be inside root_dir for pattern: {pattern}"
		));
	}
	Ok(())
}

#[cfg(test)]
mod tests {
	use super::*;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	fn config() -> vorma::__private::Config {
		vorma::__private::Config {
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
			..vorma::__private::Config::default()
		}
	}

	#[test]
	fn get_dev_view_modules_rejects_missing_pattern_or_module_and_duplicates() {
		let error = get_dev_view_modules(&[TsViewModule {
			pattern: String::new(),
			import_path: "src/skip.tsx".to_owned(),
			deps: Vec::new(),
		}])
		.unwrap_err();
		assert_eq!(error, "TS view pattern cannot be empty");

		let error = get_dev_view_modules(&[TsViewModule {
			pattern: "/".to_owned(),
			import_path: String::new(),
			deps: Vec::new(),
		}])
		.unwrap_err();
		assert_eq!(error, "TS view import path cannot be empty for pattern: /");

		let error = get_dev_view_modules(&[
			TsViewModule {
				pattern: "/a".to_owned(),
				import_path: "src/a.tsx".to_owned(),
				deps: Vec::new(),
			},
			TsViewModule {
				pattern: "/b".to_owned(),
				import_path: "src/a.tsx".to_owned(),
				deps: Vec::new(),
			},
		])
		.unwrap_err();

		assert_eq!(error, "duplicate TS module: src/a.tsx");
	}

	#[test]
	fn validate_dev_view_modules_rejects_outside_root_import_paths() {
		let config = config();
		let cfg = crate::config::to_cfg(&config).unwrap();
		let modules = BTreeMap::from([(
			"/".to_owned(),
			TsViewModule {
				pattern: "/".to_owned(),
				import_path: "../root.tsx".to_owned(),
				deps: Vec::new(),
			},
		)]);

		let error = validate_dev_view_modules_for_config(&cfg, &modules).unwrap_err();

		assert_eq!(
			error,
			"TS view import path must be inside root_dir for pattern: /"
		);
	}

	#[test]
	fn validate_dev_view_modules_rejects_map_key_pattern_mismatch() {
		let config = config();
		let cfg = crate::config::to_cfg(&config).unwrap();
		let modules = BTreeMap::from([(
			"/actual".to_owned(),
			TsViewModule {
				pattern: "/other".to_owned(),
				import_path: "src/other.tsx".to_owned(),
				deps: Vec::new(),
			},
		)]);

		let error = validate_dev_view_modules_for_config(&cfg, &modules).unwrap_err();

		assert_eq!(
			error,
			"TS view module map key must match module pattern: /actual"
		);
	}
}
