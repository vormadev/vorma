//! Vite production build command projection and manifest loading.

use std::path::PathBuf;

use crate::build_output::resolve_workspace_path;
use crate::build_plan::BuildProjectionPlan;
use crate::process_runner::{BuildProcessCommand, BuildProcessError, BuildProcessRunner};
use crate::static_outputs::{PublicStaticOutputError, public_static_output_dir};
use crate::vite_command::{ViteCommandError, ViteCommandParts};
use crate::vite_manifest::{ViteManifest, ViteManifestError};
use crate::vite_plugin_contract::package_manager_relative_fs_path;
use vorma::build_interface::assets::{PUBLIC_OUT_DIR, static_out_dir as runtime_static_out_dir};

/// Vite manifest directory below the temporary production public output.
pub const VITE_PRODUCTION_MANIFEST_TEMP_DIR: &str = "tmp";
/// Vite manifest filename below [`VITE_PRODUCTION_MANIFEST_TEMP_DIR`].
pub const VITE_PRODUCTION_MANIFEST_FILENAME: &str = "vite_manifest.json";

/// Runtime inputs needed by the TypeScript Vite plugin during one
/// production build: the port and auth token for the RPC server this
/// crate's own process exposes back to the plugin (see
/// [`crate::vite_plugin_rpc`]) — production has no dev-server port to
/// project, unlike [`crate::dev_vite::ViteDevServerInput`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViteProductionBuildInput {
	plugin_server_port: u16,
	plugin_server_token: String,
}

impl ViteProductionBuildInput {
	/// Create production Vite build runtime inputs.
	pub fn new(plugin_server_port: u16, plugin_server_token: impl Into<String>) -> Self {
		Self {
			plugin_server_port,
			plugin_server_token: plugin_server_token.into(),
		}
	}

	/// Vite plugin RPC server port.
	pub fn plugin_server_port(&self) -> u16 {
		self.plugin_server_port
	}

	/// Vite plugin RPC token.
	pub fn plugin_server_token(&self) -> &str {
		&self.plugin_server_token
	}
}

/// Run Vite production build and read the produced manifest: runs
/// [`vite_production_build_command`]'s command to completion (inheriting
/// stdio, so build output streams live), then loads the Vite-produced
/// manifest JSON from [`production_vite_manifest_temp_path`].
pub fn run_vite_production_build(
	plan: &BuildProjectionPlan,
	input: &ViteProductionBuildInput,
	runner: &mut impl BuildProcessRunner,
) -> Result<ViteManifest, ProductionViteError> {
	let command = vite_production_build_command(plan, input)?;
	runner
		.run_inheriting_stdio(&command)
		.map_err(|source| ProductionViteError::Process { source })?;
	ViteManifest::read(production_vite_manifest_temp_path(plan)?)
		.map_err(|source| ProductionViteError::Manifest { source })
}

/// Build the Vite production command without executing it: `vite build
/// --outDir <public output dir> --assetsDir . --manifest
/// <VITE_PRODUCTION_MANIFEST_TEMP_DIR>/<VITE_PRODUCTION_MANIFEST_FILENAME>
/// --emptyOutDir false --config <vite_config_file>`. `--emptyOutDir false`
/// is load-bearing: Vite's output directory is the same directory this
/// crate's own [`crate::static_outputs`] publishes public static files and
/// writes the temp manifest into, so letting Vite clear it first would
/// destroy those outputs.
pub fn vite_production_build_command(
	plan: &BuildProjectionPlan,
	input: &ViteProductionBuildInput,
) -> Result<BuildProcessCommand, ProductionViteError> {
	let mut command = ViteCommandParts::from_plan(plan)
		.map_err(|source| ProductionViteError::Command { source })?;
	let public_output_dir = declared_public_output_dir(plan, command.root_dir());
	let out_dir =
		package_manager_relative_fs_path(command.js_package_manager_dir(), &public_output_dir)
			.map_err(|source| ProductionViteError::Command {
				source: ViteCommandError::VitePath { source },
			})?;

	command.args_mut().extend([
		"vite".to_owned(),
		"build".to_owned(),
		"--outDir".to_owned(),
		out_dir,
		"--assetsDir".to_owned(),
		".".to_owned(),
		"--manifest".to_owned(),
		production_vite_manifest_arg(),
		"--emptyOutDir".to_owned(),
		"false".to_owned(),
	]);
	command
		.push_config_arg(plan)
		.map_err(|source| ProductionViteError::Command { source })?;

	Ok(command.into_command(input.plugin_server_port(), input.plugin_server_token()))
}

/// Temporary Vite manifest path produced by the production Vite build.
pub fn production_vite_manifest_temp_path(
	plan: &BuildProjectionPlan,
) -> Result<PathBuf, ProductionViteError> {
	Ok(public_static_output_dir(plan)
		.map_err(|source| ProductionViteError::PublicStaticOutputDir { source })?
		.join(VITE_PRODUCTION_MANIFEST_TEMP_DIR)
		.join(VITE_PRODUCTION_MANIFEST_FILENAME))
}

/// Error from [`run_vite_production_build`], [`vite_production_build_command`],
/// or [`production_vite_manifest_temp_path`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProductionViteError {
	/// Vite command construction failed.
	Command {
		/// Source command projection error.
		source: ViteCommandError,
	},
	/// Runtime public static output directory could not be computed.
	PublicStaticOutputDir {
		/// Source public static output error.
		source: PublicStaticOutputError,
	},
	/// Vite process execution failed.
	Process {
		/// Source process execution error.
		source: BuildProcessError,
	},
	/// Vite manifest loading failed.
	Manifest {
		/// Source Vite manifest error.
		source: ViteManifestError,
	},
}

impl std::fmt::Display for ProductionViteError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::Command { source } => write!(f, "{source}"),
			Self::PublicStaticOutputDir { source } => write!(f, "{source}"),
			Self::Process { source } => write!(f, "{source}"),
			Self::Manifest { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for ProductionViteError {}

fn declared_public_output_dir(plan: &BuildProjectionPlan, root_dir: &std::path::Path) -> PathBuf {
	runtime_static_out_dir(resolve_workspace_path(
		root_dir,
		plan.workspace().dist_dir(),
	))
	.join(PUBLIC_OUT_DIR)
}

fn production_vite_manifest_arg() -> String {
	format!("{VITE_PRODUCTION_MANIFEST_TEMP_DIR}/{VITE_PRODUCTION_MANIFEST_FILENAME}")
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::fs;
	use std::path::PathBuf;

	use http::Method;
	use vorma_contract::framework_graph::{
		BuildInputConfig, DevWatchConfig, FrameworkConfig, FrameworkDeclarations, FrameworkGraph,
		FrontendBuildInputs, HandlerId, ResourceDeclaration, ServerBuildTarget, ViewDeclaration,
	};

	use super::*;
	use crate::projection_compiler::ProjectionBundle;
	use crate::test_support::route_type_contract;
	use crate::test_support::unique_temp_root_with_dirs;
	use crate::vite_manifest::{CLIENT_CORE_WASM_SOURCE_FILENAME, ViteManifestChunk};
	use crate::vite_plugin_contract::{
		VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY,
	};

	fn temp_root() -> PathBuf {
		unique_temp_root_with_dirs("vorma-build-vite", &["public", "web"])
	}

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn plan(root_dir: PathBuf) -> BuildProjectionPlan {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "example-server"),
			root_dir.display().to_string(),
			"dist",
			FrontendBuildInputs::new(
				"react",
				"pnpm exec",
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

	#[derive(Debug)]
	struct FakeRunner {
		commands: Vec<BuildProcessCommand>,
		manifest_path: PathBuf,
	}

	impl BuildProcessRunner for FakeRunner {
		fn run_inheriting_stdio(
			&mut self,
			command: &BuildProcessCommand,
		) -> Result<(), BuildProcessError> {
			self.commands.push(command.clone());
			fs::create_dir_all(self.manifest_path.parent().unwrap()).unwrap();
			let wasm_file = format!("assets/{CLIENT_CORE_WASM_SOURCE_FILENAME}");
			let manifest = BTreeMap::from([
				(
					"../src/entry.tsx".to_owned(),
					ViteManifestChunk {
						file: "assets/entry.js".to_owned(),
						..ViteManifestChunk::default()
					},
				),
				(
					"src/wasm-wrapper.ts".to_owned(),
					ViteManifestChunk {
						file: "assets/wasm-wrapper.js".to_owned(),
						assets: vec![wasm_file.clone()],
						..ViteManifestChunk::default()
					},
				),
				(
					"pkg/vorma_client_wasm_bg.wasm".to_owned(),
					ViteManifestChunk {
						src: "pkg/vorma_client_wasm_bg.wasm".to_owned(),
						file: wasm_file,
						..ViteManifestChunk::default()
					},
				),
				(
					"../src/views/root.tsx".to_owned(),
					ViteManifestChunk {
						file: "assets/root.js".to_owned(),
						..ViteManifestChunk::default()
					},
				),
			]);
			fs::write(&self.manifest_path, serde_json::to_vec(&manifest).unwrap()).unwrap();
			Ok(())
		}
	}

	#[test]
	fn vite_production_command_projects_paths_env_and_manifest_location() {
		let root_dir = temp_root();
		let plan = plan(root_dir.clone());
		let input = ViteProductionBuildInput::new(4173, "token");

		let command = vite_production_build_command(&plan, &input).unwrap();

		assert_eq!(command.program(), "pnpm");
		assert_eq!(command.current_dir(), root_dir.join("web"));
		assert_eq!(
			command.args(),
			[
				"exec",
				"vite",
				"build",
				"--outDir",
				"../dist/.vorma/static/public",
				"--assetsDir",
				".",
				"--manifest",
				"tmp/vite_manifest.json",
				"--emptyOutDir",
				"false",
				"--config",
				"../vite.config.ts"
			]
		);
		assert_eq!(
			command.env().get(VITE_PLUGIN_SERVER_PORT_ENV_KEY).unwrap(),
			"4173"
		);
		assert_eq!(
			command.env().get(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY).unwrap(),
			"token"
		);
		assert_eq!(
			production_vite_manifest_temp_path(&plan).unwrap(),
			fs::canonicalize(&root_dir)
				.unwrap()
				.join("dist")
				.join(".vorma")
				.join("static")
				.join("public")
				.join("tmp")
				.join("vite_manifest.json")
		);

		fs::remove_dir_all(root_dir).unwrap();
	}

	#[test]
	fn vite_production_build_runs_command_and_reads_manifest() {
		let root_dir = temp_root();
		let plan = plan(root_dir.clone());
		let manifest_path = production_vite_manifest_temp_path(&plan).unwrap();
		let mut runner = FakeRunner {
			commands: Vec::new(),
			manifest_path,
		};

		let manifest = run_vite_production_build(
			&plan,
			&ViteProductionBuildInput::new(4173, "token"),
			&mut runner,
		)
		.unwrap();

		assert_eq!(runner.commands.len(), 1);
		assert!(manifest.contains_key("../src/entry.tsx"));
		assert!(manifest.contains_key("../src/views/root.tsx"));

		fs::remove_dir_all(root_dir).unwrap();
	}
}
