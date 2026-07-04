//! Vite development server command projection.

use crate::build_plan::BuildProjectionPlan;
use crate::process_runner::{
	BuildProcessCommand, BuildProcessError, BuildProcessRunner, StartedBuildProcess,
};
use crate::vite_command::{ViteCommandError, ViteCommandParts};
use crate::vite_plugin_contract::VITE_PLUGIN_LOOPBACK_HOST;

/// Runtime inputs needed by the TypeScript Vite plugin during dev: the port
/// Vite's own dev HTTP server should bind, plus the port and auth token for
/// the RPC server this crate's own process exposes back to the plugin (see
/// [`crate::vite_plugin_rpc`]).
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViteDevServerInput {
	vite_server_port: u16,
	plugin_server_port: u16,
	plugin_server_token: String,
}

impl ViteDevServerInput {
	/// Create dev Vite server runtime inputs.
	pub fn new(
		vite_server_port: u16,
		plugin_server_port: u16,
		plugin_server_token: impl Into<String>,
	) -> Self {
		Self {
			vite_server_port,
			plugin_server_port,
			plugin_server_token: plugin_server_token.into(),
		}
	}

	/// Vite dev HTTP server port.
	pub fn vite_server_port(&self) -> u16 {
		self.vite_server_port
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

/// Start the Vite development server without waiting for it to exit. Runs
/// [`vite_dev_server_command`]'s command via `runner`.
pub fn start_vite_dev_server(
	plan: &BuildProjectionPlan,
	input: &ViteDevServerInput,
	runner: &mut impl BuildProcessRunner,
) -> Result<Box<dyn StartedBuildProcess + Send>, DevViteError> {
	let command = vite_dev_server_command(plan, input)?;
	runner
		.start_inheriting_stdio(&command)
		.map_err(|source| DevViteError::Process { source })
}

/// Build the Vite development server command without executing it: `vite
/// --host 127.0.0.1 --port <vite_server_port> --strictPort --clearScreen
/// false --config <vite_config_file>`, with the plugin RPC port/token set
/// as env vars the framework's Vite plugin reads at Vite startup.
/// `--strictPort` means Vite fails fast on a port collision instead of
/// silently picking a different port — this crate already reserved
/// `vite_server_port` as a free port before this command runs (see
/// [`crate::dev_build`]'s loopback port allocation), so a collision here
/// means something else raced onto that port between reservation and Vite's
/// own bind.
pub fn vite_dev_server_command(
	plan: &BuildProjectionPlan,
	input: &ViteDevServerInput,
) -> Result<BuildProcessCommand, DevViteError> {
	let mut command =
		ViteCommandParts::from_plan(plan).map_err(|source| DevViteError::Command { source })?;
	command.args_mut().extend([
		"vite".to_owned(),
		"--host".to_owned(),
		VITE_PLUGIN_LOOPBACK_HOST.to_owned(),
		"--port".to_owned(),
		input.vite_server_port().to_string(),
		"--strictPort".to_owned(),
		"--clearScreen".to_owned(),
		"false".to_owned(),
	]);
	command
		.push_config_arg(plan)
		.map_err(|source| DevViteError::Command { source })?;

	Ok(command.into_command(input.plugin_server_port(), input.plugin_server_token()))
}

/// Error from [`start_vite_dev_server`] or [`vite_dev_server_command`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevViteError {
	/// Vite command construction failed.
	Command {
		/// Source command projection error.
		source: ViteCommandError,
	},
	/// Vite process start failed.
	Process {
		/// Source process execution error.
		source: BuildProcessError,
	},
}

impl std::fmt::Display for DevViteError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::Command { source } => write!(f, "{source}"),
			Self::Process { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for DevViteError {}

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
	use crate::vite_plugin_contract::{
		VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY,
	};

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn plan() -> BuildProjectionPlan {
		let config = FrameworkConfig::new("/static").with_build_inputs(BuildInputConfig::new(
			ServerBuildTarget::new("example-app", "example-server"),
			"/workspace/app",
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

	#[test]
	fn vite_dev_server_command_projects_paths_and_plugin_env() {
		let command =
			vite_dev_server_command(&plan(), &ViteDevServerInput::new(5173, 4173, "token"))
				.unwrap();

		assert_eq!(command.program(), "pnpm");
		assert_eq!(
			command.current_dir(),
			std::path::Path::new("/workspace/app/web")
		);
		assert_eq!(
			command.args(),
			[
				"exec",
				"vite",
				"--host",
				"127.0.0.1",
				"--port",
				"5173",
				"--strictPort",
				"--clearScreen",
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
	}
}
