//! Vite plugin contract projected from the build plan.
//!
//! The framework's own Vite plugin (the TypeScript side, not this crate)
//! talks to this crate's build process over a small loopback RPC (see
//! [`crate::vite_plugin_rpc`]) using the request/response shapes defined
//! here. [`VitePluginConfig`] itself is generated into TypeScript via
//! `#[derive(vorma::TsGen)]` — the same derive app code uses for its own
//! route contracts — so its Rust and TypeScript shapes can never drift.

use std::path::{Path, PathBuf};

use path_slash::PathExt;
use serde::{Deserialize, Serialize};

use crate::build_output::resolve_workspace_path;
use crate::build_plan::BuildProjectionPlan;

/// Environment key carrying the Vite plugin RPC server port.
pub const VITE_PLUGIN_SERVER_PORT_ENV_KEY: &str = "__VORMA_VITE_PLUGIN_SERVER_PORT";
/// Environment key carrying the Vite plugin RPC token.
pub const VITE_PLUGIN_SERVER_TOKEN_ENV_KEY: &str = "__VORMA_VITE_PLUGIN_SERVER_TOKEN";
/// HTTP header carrying the Vite plugin RPC token.
pub const VITE_PLUGIN_TOKEN_HEADER: &str = "x-vorma-vite-plugin-token";
/// Loopback host used by Vite plugin control/RPC traffic.
pub const VITE_PLUGIN_LOOPBACK_HOST: &str = "127.0.0.1";
/// Vite plugin RPC mount prefix.
pub const VITE_PLUGIN_BASE_PATH: &str = "/vite-plugin";
/// Vite plugin RPC path below [`VITE_PLUGIN_BASE_PATH`].
pub const VITE_PLUGIN_RPC_PATH: &str = "/rpc";
/// Vite plugin config-change control path: restarting Vite over this path
/// is reserved for changes to the [`VitePluginConfig`] payload itself
/// (entry module, view-module list, ignored patterns, dedupe list, public
/// static base path) — never
/// for filemap-only changes, and never unconditionally on app rebuilds (see
/// [`VITE_PLUGIN_ASSETS_CHANGED_PATH`] for the targeted alternative).
pub const VITE_PLUGIN_CONFIG_CHANGED_PATH: &str = "/cfg-changed";
/// Vite plugin changed-assets control path: sends changed source keys so
/// the plugin can invalidate exactly the Vite modules that reference them,
/// without restarting Vite. Public-asset saves must always go through this
/// path, never [`VITE_PLUGIN_CONFIG_CHANGED_PATH`].
pub const VITE_PLUGIN_ASSETS_CHANGED_PATH: &str = "/assets-changed";
/// Prefix used by JavaScript/CSS public URL references.
pub const VITE_PLUGIN_PUBLIC_URL_PREFIX: &str = "@public/";

const RUST_SOURCE_IGNORE_PATTERN: &str = "**/*.rs";
const GLOB_ALL_FILES_SUFFIX: &str = "**/*";

/// JSON config response consumed by the TypeScript Vite plugin. Projected
/// once per generation by [`Self::from_build_plan`]; a change to any field
/// here is what [`VITE_PLUGIN_CONFIG_CHANGED_PATH`] signals.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize, vorma::TsGen)]
pub struct VitePluginConfig {
	/// Public static base path for production asset URLs.
	pub public_static_base_path: String,
	/// Frontend app entry module path, relative to the package-manager directory.
	pub entry_module: String,
	/// Route client module paths, relative to the package-manager directory.
	pub view_modules: Vec<String>,
	/// Vite filesystem watch ignore patterns.
	pub ignored_patterns: Vec<String>,
	/// UI dependency names Vite should de-duplicate.
	pub dedupe_list: Vec<String>,
}

impl VitePluginConfig {
	/// Project a Vite plugin config response from the build plan: resolves
	/// the entry module and every view's client module to paths relative to
	/// the JS package-manager directory (where Vite actually runs from —
	/// see [`crate::vite_command`]), and derives the filesystem-watch ignore
	/// patterns (the Rust source glob, the Vorma output directory, and the
	/// generated TypeScript file — none of these should ever trigger a Vite
	/// reaction).
	pub fn from_build_plan(plan: &BuildProjectionPlan) -> Result<Self, VitePluginConfigError> {
		let root_dir = PathBuf::from(plan.workspace().root_dir());
		if root_dir.as_os_str().is_empty() {
			return Err(VitePluginConfigError::EmptyRootDir);
		}
		let js_package_manager_dir =
			resolve_workspace_path(&root_dir, plan.vite_inputs().js_package_manager_dir());
		let entry_module = package_manager_relative_path(
			&root_dir,
			&js_package_manager_dir,
			plan.vite_inputs().entry_file(),
		)?;
		let view_modules = plan
			.vite_inputs()
			.view_client_files()
			.iter()
			.map(|path| package_manager_relative_path(&root_dir, &js_package_manager_dir, path))
			.collect::<Result<Vec<_>, _>>()?;
		let ignored_patterns = vite_ignored_patterns(plan, &root_dir);
		Ok(Self {
			public_static_base_path: plan.vite_inputs().public_static_base().to_owned(),
			entry_module,
			view_modules,
			ignored_patterns,
			dedupe_list: plan
				.vite_inputs()
				.dedupe_list()
				.iter()
				.map(|value| (*value).to_owned())
				.collect(),
		})
	}
}

/// Vite plugin RPC request shape: every request the TypeScript Vite plugin
/// can send this crate's RPC server (see [`crate::vite_plugin_rpc`]),
/// tagged by `method` on the wire.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "method", rename_all = "snake_case")]
pub enum VitePluginRpcRequest {
	/// Request the current Vite config response.
	Cfg,
	/// Resolve one public static source path to its generated public URL.
	Hash {
		/// Public static source path below the configured source directory.
		src_path: String,
	},
	/// Publish Vite's dev control server port.
	SetPort {
		/// Loopback control port.
		port: u16,
	},
}

/// Error from [`VitePluginConfig::from_build_plan`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum VitePluginConfigError {
	/// Build root directory was empty.
	EmptyRootDir,
	/// A Vite input path could not be made relative to the package-manager directory.
	InputPath {
		/// Rejected path.
		path: String,
	},
}

impl std::fmt::Display for VitePluginConfigError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::InputPath { path } => {
				write!(f, "could not project Vite input path {path:?}")
			}
		}
	}
}

impl std::error::Error for VitePluginConfigError {}

fn vite_ignored_patterns(plan: &BuildProjectionPlan, root_dir: &Path) -> Vec<String> {
	let vorma_out_dir =
		resolve_workspace_path(root_dir, plan.workspace().dist_dir()).join(".vorma");
	let generated_typescript_file =
		resolve_workspace_path(root_dir, plan.generated_typescript().output_file());
	vec![
		RUST_SOURCE_IGNORE_PATTERN.to_owned(),
		join_glob(&vorma_out_dir.to_slash_lossy(), GLOB_ALL_FILES_SUFFIX),
		generated_typescript_file.to_slash_lossy().into_owned(),
	]
}

pub(crate) fn package_manager_relative_path(
	root_dir: &Path,
	js_package_manager_dir: &Path,
	path: &str,
) -> Result<String, VitePluginConfigError> {
	let path = resolve_workspace_path(root_dir, path);
	package_manager_relative_fs_path(js_package_manager_dir, &path)
}

pub(crate) fn package_manager_relative_fs_path(
	js_package_manager_dir: &Path,
	path: &Path,
) -> Result<String, VitePluginConfigError> {
	let relative = pathdiff::diff_paths(path, js_package_manager_dir).ok_or_else(|| {
		VitePluginConfigError::InputPath {
			path: path.display().to_string(),
		}
	})?;
	Ok(relative.to_slash_lossy().into_owned())
}

fn join_glob(path: &str, glob_suffix: &str) -> String {
	if path.is_empty() || path == "." {
		return glob_suffix.to_owned();
	}
	format!("{path}/{glob_suffix}")
}

#[cfg(test)]
mod tests {
	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget, ViewDeclaration,
	};

	use super::*;
	use crate::projection_compiler::ProjectionBundle;
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn plan() -> BuildProjectionPlan {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "example-server"),
			"/workspace/app",
			"dist",
			FrontendBuildInputs::new(
				"preact",
				"pnpm",
				"web",
				"vite.config.ts",
				"src/entry.tsx",
				"public",
				"src/critical.css",
			),
			"src/vorma.gen.ts",
			DevWatchConfig::default(),
		));
		let mut declarations = FrameworkDeclarations::new(config);
		declarations.add_view(ViewDeclaration::new(
			"/",
			"src/views/root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("ping"),
		));
		BuildProjectionPlan::compile(&ProjectionBundle::compile(
			&FrameworkGraph::compile(declarations).unwrap(),
		))
		.unwrap()
	}

	#[test]
	fn vite_plugin_config_projects_package_relative_inputs_and_contract_fields() {
		let config = VitePluginConfig::from_build_plan(&plan()).unwrap();
		let value = serde_json::to_value(&config).unwrap();

		assert_eq!(config.public_static_base_path, "/static/");
		assert_eq!(config.entry_module, "../src/entry.tsx");
		assert_eq!(config.view_modules, ["../src/views/root.tsx"]);
		assert_eq!(
			config.ignored_patterns,
			[
				"**/*.rs",
				"/workspace/app/dist/.vorma/**/*",
				"/workspace/app/src/vorma.gen.ts"
			]
		);
		assert_eq!(
			config.dedupe_list,
			[
				"preact",
				"preact/hooks",
				"@preact/signals",
				"preact/jsx-runtime",
				"preact/compat",
				"preact/test-utils"
			]
		);
		assert_eq!(value["public_static_base_path"], "/static/");
		assert_eq!(value["entry_module"], "../src/entry.tsx");
		assert_eq!(value["view_modules"][0], "../src/views/root.tsx");
	}

	#[test]
	fn vite_plugin_rpc_request_uses_fixed_snake_case_method_tags() {
		let request = VitePluginRpcRequest::Hash {
			src_path: "img/logo.svg".to_owned(),
		};

		assert_eq!(
			serde_json::to_value(request).unwrap(),
			serde_json::json!({"method": "hash", "src_path": "img/logo.svg"})
		);
	}
}
