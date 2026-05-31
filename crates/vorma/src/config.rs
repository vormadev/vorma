use std::path::PathBuf;

use serde::{Deserialize, Serialize};
use vorma_matcher::ensure_leading_and_trailing_slash;

use crate::tsgen::{TsDrafter, TsExtraType};

/// Cargo target that produces the user app server.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct ServerConfig {
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
	/// UI variant label emitted into manifests and generated contracts.
	pub ui_variant: String,
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

/// Public URL and API mount path configuration.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct PathConfig {
	/// Public static asset base path.
	pub public_static_base: String,
	/// API-route mount root. Empty/root values are invalid after normalization.
	pub api_base: String,
}

impl Default for PathConfig {
	fn default() -> Self {
		Self {
			public_static_base: String::new(),
			api_base: "/api/".to_owned(),
		}
	}
}

#[doc(hidden)]
pub fn normalize_api_mount_root(api_base: &str) -> Result<String, String> {
	let api_base = api_base.trim();
	if api_base.is_empty() || api_base == "/" {
		return Err("api_base must be a non-root path prefix such as /api/".to_owned());
	}
	let normalized = ensure_leading_and_trailing_slash(api_base);
	if normalized == "/" {
		return Err("api_base must be a non-root path prefix such as /api/".to_owned());
	}
	Ok(normalized)
}

#[doc(hidden)]
pub fn normalize_public_static_base(public_static_base: &str) -> String {
	ensure_leading_and_trailing_slash(public_static_base.trim())
}

#[doc(hidden)]
pub fn validate_public_static_base_against_api_mount(
	public_static_base: &str,
	api_base: &str,
) -> Result<(String, String), String> {
	let public_static_base = normalize_public_static_base(public_static_base);
	let api_base = normalize_api_mount_root(api_base)?;
	if public_static_base != "/" && public_static_base.starts_with(&api_base) {
		return Err(format!(
			"public_static_base {public_static_base:?} must not be under api_base {api_base:?}"
		));
	}
	Ok((public_static_base, api_base))
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[doc(hidden)]
pub struct Config {
	pub root_dir: PathBuf,
	pub server_config: ServerConfig,
	pub dist_dir: String,
	pub path_config: PathConfig,
	pub frontend_config: FrontendConfig,
	pub ts_gen_config: TsGenConfig,
	pub dev_watch_config: DevWatchConfig,
}

impl Default for Config {
	fn default() -> Self {
		Self {
			root_dir: PathBuf::new(),
			server_config: ServerConfig::default(),
			dist_dir: String::new(),
			path_config: PathConfig::default(),
			frontend_config: FrontendConfig::default(),
			ts_gen_config: TsGenConfig::default(),
			dev_watch_config: DevWatchConfig::default(),
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn path_config_defaults_api_mount_under_api() {
		let path_config = PathConfig::default();

		assert_eq!(path_config.api_base, "/api/");
	}

	#[test]
	fn config_default_uses_safe_api_mount_root() {
		let config = Config::default();

		assert_eq!(config.path_config.api_base, "/api/");
	}

	#[test]
	fn api_mount_root_must_not_be_root() {
		assert_eq!(normalize_api_mount_root("api").unwrap(), "/api/");
		assert_eq!(
			normalize_api_mount_root("").unwrap_err(),
			"api_base must be a non-root path prefix such as /api/"
		);
		assert_eq!(
			normalize_api_mount_root("/").unwrap_err(),
			"api_base must be a non-root path prefix such as /api/"
		);
	}

	#[test]
	fn public_static_base_must_not_overlap_api_mount_root() {
		assert_eq!(
			validate_public_static_base_against_api_mount("static", "api").unwrap(),
			("/static/".to_owned(), "/api/".to_owned())
		);
		assert_eq!(
			validate_public_static_base_against_api_mount("", "api").unwrap(),
			("/".to_owned(), "/api/".to_owned())
		);
		assert_eq!(
			validate_public_static_base_against_api_mount("api/assets", "api").unwrap_err(),
			"public_static_base \"/api/assets/\" must not be under api_base \"/api/\""
		);
		assert!(validate_public_static_base_against_api_mount("api-static", "api").is_ok());
		assert_eq!(
			validate_public_static_base_against_api_mount("api/v1/assets", "api/v1").unwrap_err(),
			"public_static_base \"/api/v1/assets/\" must not be under api_base \"/api/v1/\""
		);
		assert!(validate_public_static_base_against_api_mount("api", "api/v1").is_ok());
	}
}
