//! Public framework configuration and lowering into graph-owned build inputs.
//!
//! These types are the domain-grouped fields of [`AppConfig`](crate::AppConfig): cargo
//! target identity ([`ServerTarget`]), frontend build/static-asset settings
//! ([`FrontendConfig`], [`UiVariant`]), generated TypeScript output
//! ([`TsGenConfig`]), and the dev watcher's include/classification patterns
//! ([`DevWatchConfig`]). An app typically sets these once, in the same function that
//! builds the rest of [`AppConfig`] (see the [crate-root example](crate#getting-started)).

use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use vorma_matcher::ensure_leading_and_trailing_slash;

use crate::framework_graph::{
	BuildInputConfig, DevWatchConfig as GraphDevWatchConfig, FrameworkConfig, FrontendBuildInputs,
	ServerBuildTarget,
};
use crate::tsgen::{TsDrafter, TsExtraType};

/// Cargo target that produces the user app server.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct ServerTarget {
	/// Cargo package name containing the app server binary.
	pub cargo_package: String,
	/// Cargo binary name for the app server.
	pub cargo_bin: String,
}

/// Dev watcher patterns and change classification.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct DevWatchConfig {
	/// Paths/globs watched in dev mode.
	pub watch_patterns: Vec<String>,
	/// Watched paths/globs that require rebuilding the app server.
	pub on_change_recompile_server: Vec<String>,
	/// Watched paths/globs that require client data revalidation.
	pub on_change_client_revalidate: Vec<String>,
}

/// Frontend build and static asset configuration.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct FrontendConfig {
	/// UI adapter emitted into manifests and generated contracts.
	pub ui_variant: UiVariant,
	/// Package-manager command prefix, such as `pnpm`.
	pub js_package_manager_base_cmd: String,
	/// Directory where JavaScript package-manager commands run.
	pub js_package_manager_dir: String,
	/// Vite config file path.
	pub vite_config_file: String,
	/// Client entry module path.
	pub entry_file: String,
	/// Public static source directory.
	pub public_static_src_dir: String,
	/// Critical CSS entry file.
	pub critical_css_file: String,
}

/// Supported UI adapter.
#[derive(Clone, Copy, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum UiVariant {
	/// React UI adapter.
	#[default]
	React,
	/// Preact UI adapter.
	Preact,
	/// Solid UI adapter.
	Solid,
}

impl UiVariant {
	/// Stable lowercase adapter label used in generated manifests.
	pub const fn as_str(self) -> &'static str {
		match self {
			Self::React => "react",
			Self::Preact => "preact",
			Self::Solid => "solid",
		}
	}
}

impl std::fmt::Display for UiVariant {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		f.write_str(self.as_str())
	}
}

/// Generated TypeScript output configuration.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct TsGenConfig {
	/// Generated TypeScript output file.
	pub out_file: String,
	/// Extra Rust types to emit even when routes do not reference them.
	#[serde(default)]
	#[serde(skip)]
	pub extra_types: Vec<TsExtraType>,
	/// Extra structured TypeScript declarations appended after generated types.
	#[serde(default)]
	#[serde(skip)]
	pub extra_ts: TsDrafter,
}

/// Complete public framework configuration.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct Config {
	/*
	Path rule: `root_dir` is the one absolute anchor (PathBuf); every other
	path-ish field in this config is a root-relative String fragment.
	*/
	/// Absolute application root used to resolve all relative config paths.
	pub root_dir: PathBuf,
	/// Build output directory, relative to [`Self::root_dir`] unless absolute.
	pub dist_dir: String,
	/// Cargo package/bin identity of the app server binary.
	pub server_target: ServerTarget,
	/// Public static asset base URL path.
	pub public_static_base: String,
	/// Frontend entry, Vite, package-manager, static, and critical-CSS config.
	pub frontend_config: FrontendConfig,
	/// Generated TypeScript output and supplemental declaration config.
	pub ts_gen_config: TsGenConfig,
	/// Dev watcher include/classification patterns.
	pub dev_watch_config: DevWatchConfig,
}

impl Config {
	/// Lower public config into graph-owned runtime/build configuration.
	pub fn framework_config(&self) -> Result<FrameworkConfig, ConfigError> {
		let public_static_base = normalize_public_static_base(&self.public_static_base);
		Ok(FrameworkConfig::new(public_static_base).with_build_inputs(
			BuildInputConfig::new(
				ServerBuildTarget::new(
					self.server_target.cargo_package.clone(),
					self.server_target.cargo_bin.clone(),
				),
				utf8_path(&self.root_dir)?,
				self.dist_dir.clone(),
				FrontendBuildInputs::new(
					self.frontend_config.ui_variant.as_str(),
					self.frontend_config.js_package_manager_base_cmd.clone(),
					self.frontend_config.js_package_manager_dir.clone(),
					self.frontend_config.vite_config_file.clone(),
					self.frontend_config.entry_file.clone(),
					self.frontend_config.public_static_src_dir.clone(),
					self.frontend_config.critical_css_file.clone(),
				),
				self.ts_gen_config.out_file.clone(),
				GraphDevWatchConfig::new(
					self.dev_watch_config.watch_patterns.clone(),
					self.dev_watch_config.on_change_recompile_server.clone(),
					self.dev_watch_config.on_change_client_revalidate.clone(),
				),
			)
			.with_generated_typescript_extra_source(self.ts_gen_config.extra_ts.to_string()),
		))
	}
}

impl Default for Config {
	fn default() -> Self {
		Self {
			root_dir: PathBuf::new(),
			dist_dir: String::new(),
			server_target: ServerTarget::default(),
			public_static_base: String::new(),
			frontend_config: FrontendConfig::default(),
			ts_gen_config: TsGenConfig::default(),
			dev_watch_config: DevWatchConfig::default(),
		}
	}
}

/// Public config normalization error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ConfigError {
	/// A path field could not be represented as UTF-8.
	NonUtf8Path {
		/// Field name.
		field: &'static str,
	},
}

impl std::fmt::Display for ConfigError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::NonUtf8Path { field } => write!(f, "{field} must be valid UTF-8"),
		}
	}
}

impl std::error::Error for ConfigError {}

/// Normalize the public static base, allowing empty/root to mean `/`.
#[doc(hidden)]
pub fn normalize_public_static_base(public_static_base: &str) -> String {
	ensure_leading_and_trailing_slash(public_static_base.trim())
}

fn utf8_path(path: &Path) -> Result<String, ConfigError> {
	path.to_str()
		.map(ToOwned::to_owned)
		.ok_or(ConfigError::NonUtf8Path { field: "root_dir" })
}

#[cfg(test)]
mod tests {
	use super::*;

	const TEST_PUBLIC_STATIC_BASE: &str = "static";
	const TEST_ROOT_DIR: &str = "app";

	#[test]
	fn public_config_normalizes_public_static_base_before_graph_lowering() {
		let config = Config {
			root_dir: PathBuf::from(TEST_ROOT_DIR),
			server_target: ServerTarget {
				cargo_package: "server-package".to_owned(),
				cargo_bin: "server-bin".to_owned(),
			},
			dist_dir: "dist".to_owned(),
			public_static_base: TEST_PUBLIC_STATIC_BASE.to_owned(),
			frontend_config: FrontendConfig {
				js_package_manager_base_cmd: "pnpm".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: "src/critical.css".to_owned(),
				ui_variant: UiVariant::React,
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				extra_types: Vec::new(),
				extra_ts: TsDrafter::new(),
			},
			dev_watch_config: DevWatchConfig {
				watch_patterns: vec!["src/**/*.rs".to_owned()],
				on_change_recompile_server: vec!["src/server.rs".to_owned()],
				on_change_client_revalidate: vec!["src/views/**/*.tsx".to_owned()],
			},
		};

		let framework_config = config.framework_config().unwrap();
		let build_inputs = framework_config.build_inputs().unwrap();

		assert_eq!(framework_config.public_static_base(), "/static/");
		assert_eq!(build_inputs.root_dir(), TEST_ROOT_DIR);
		assert_eq!(
			build_inputs.server_target().cargo_package(),
			"server-package"
		);
		assert_eq!(
			build_inputs.frontend_inputs().js_package_manager_base_cmd(),
			"pnpm"
		);
		assert_eq!(build_inputs.frontend_inputs().ui_variant(), "react");
		assert_eq!(
			build_inputs.dev_watch().client_revalidate_patterns(),
			&["src/views/**/*.tsx".to_owned()]
		);
	}
}
