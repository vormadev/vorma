//! Application server process command projection.

use std::path::PathBuf;

use vorma_contract::constants::{ENV_KEY_IS_DEV, ENV_VALUE_ENABLED};

use crate::build_plan::BuildProjectionPlan;
use crate::process_runner::{
	BuildProcessCommand, BuildProcessError, BuildProcessRunner, StartedBuildProcess,
};

/// Environment key carrying the app server bind port.
pub const APP_SERVER_PORT_ENV_KEY: &str = "PORT";

/// Runtime inputs needed by a dev app server process.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AppServerDevInput {
	port: u16,
}

impl AppServerDevInput {
	/// Create dev app server runtime inputs.
	pub fn new(port: u16) -> Self {
		Self { port }
	}

	/// App server bind port.
	pub fn port(&self) -> u16 {
		self.port
	}
}

/// Prebuilt app-server executable paired with the cargo target it was built from.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PrebuiltAppServer {
	cargo_package: String,
	cargo_bin: String,
	executable: PathBuf,
}

impl PrebuiltAppServer {
	/// Record a prebuilt app-server executable for the given cargo target.
	pub fn new(
		cargo_package: impl Into<String>,
		cargo_bin: impl Into<String>,
		executable: impl Into<PathBuf>,
	) -> Self {
		Self {
			cargo_package: cargo_package.into(),
			cargo_bin: cargo_bin.into(),
			executable: executable.into(),
		}
	}

	/// Whether this executable was built from the plan's current server target.
	pub fn matches_plan_server_target(&self, plan: &BuildProjectionPlan) -> bool {
		plan.server_target().cargo_package() == self.cargo_package
			&& plan.server_target().cargo_bin() == self.cargo_bin
	}
}

/*
A live-state update can move the server target itself; the prebuilt executable
is only valid for the target it was compiled from, so a moved target falls
back to `cargo run` for that one activation.
*/
/// Start the dev app server from a prebuilt executable, or via `cargo run` when
/// the plan's server target no longer matches the executable's target.
pub fn start_app_server_dev_with_prebuilt(
	plan: &BuildProjectionPlan,
	input: &AppServerDevInput,
	prebuilt: &PrebuiltAppServer,
	runner: &mut impl BuildProcessRunner,
) -> Result<Box<dyn StartedBuildProcess + Send>, AppServerProcessError> {
	if !prebuilt.matches_plan_server_target(plan) {
		return start_app_server_dev(plan, input, runner);
	}
	let command = app_server_dev_executable_command(plan, input, &prebuilt.executable)?;
	runner
		.start_inheriting_stdio(&command)
		.map_err(|source| AppServerProcessError::Process { source })
}

/// Build the dev app server command for a prebuilt executable without executing it.
pub fn app_server_dev_executable_command(
	plan: &BuildProjectionPlan,
	input: &AppServerDevInput,
	executable: &std::path::Path,
) -> Result<BuildProcessCommand, AppServerProcessError> {
	if input.port() == 0 {
		return Err(AppServerProcessError::InvalidPort);
	}
	let root_dir = PathBuf::from(plan.workspace().root_dir());
	if root_dir.as_os_str().is_empty() {
		return Err(AppServerProcessError::EmptyRootDir);
	}
	let executable =
		executable
			.to_str()
			.ok_or_else(|| AppServerProcessError::NonUtf8ExecutablePath {
				path: executable.display().to_string(),
			})?;
	Ok(BuildProcessCommand::new(executable, Vec::new(), root_dir)
		.with_env(APP_SERVER_PORT_ENV_KEY, input.port().to_string())
		.with_env(ENV_KEY_IS_DEV, ENV_VALUE_ENABLED))
}

/// Start the dev app server without waiting for it to exit.
pub fn start_app_server_dev(
	plan: &BuildProjectionPlan,
	input: &AppServerDevInput,
	runner: &mut impl BuildProcessRunner,
) -> Result<Box<dyn StartedBuildProcess + Send>, AppServerProcessError> {
	let command = app_server_dev_command(plan, input)?;
	runner
		.start_inheriting_stdio(&command)
		.map_err(|source| AppServerProcessError::Process { source })
}

/// Build the dev app server command without executing it.
pub fn app_server_dev_command(
	plan: &BuildProjectionPlan,
	input: &AppServerDevInput,
) -> Result<BuildProcessCommand, AppServerProcessError> {
	if input.port() == 0 {
		return Err(AppServerProcessError::InvalidPort);
	}
	if plan.server_target().cargo_package().trim().is_empty() {
		return Err(AppServerProcessError::MissingCargoPackage);
	}
	if plan.server_target().cargo_bin().trim().is_empty() {
		return Err(AppServerProcessError::MissingCargoBin);
	}
	let root_dir = PathBuf::from(plan.workspace().root_dir());
	if root_dir.as_os_str().is_empty() {
		return Err(AppServerProcessError::EmptyRootDir);
	}
	Ok(BuildProcessCommand::new(
		"cargo",
		vec![
			"run".to_owned(),
			"-p".to_owned(),
			plan.server_target().cargo_package().to_owned(),
			"--bin".to_owned(),
			plan.server_target().cargo_bin().to_owned(),
		],
		root_dir,
	)
	.with_env(APP_SERVER_PORT_ENV_KEY, input.port().to_string())
	.with_env(ENV_KEY_IS_DEV, ENV_VALUE_ENABLED))
}

/// App server process command error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AppServerProcessError {
	/// App server port cannot be zero.
	InvalidPort,
	/// Build root directory was empty.
	EmptyRootDir,
	/// Cargo package was empty.
	MissingCargoPackage,
	/// Cargo binary was empty.
	MissingCargoBin,
	/// Prebuilt executable path was not valid UTF-8.
	NonUtf8ExecutablePath {
		/// Rejected executable path.
		path: String,
	},
	/// App server process start failed.
	Process {
		/// Source process execution error.
		source: BuildProcessError,
	},
}

impl std::fmt::Display for AppServerProcessError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidPort => f.write_str("app server port cannot be zero"),
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::MissingCargoPackage => f.write_str("server cargo_package cannot be empty"),
			Self::MissingCargoBin => f.write_str("server cargo_bin cannot be empty"),
			Self::NonUtf8ExecutablePath { path } => {
				write!(f, "app server executable path must be UTF-8: {path}")
			}
			Self::Process { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for AppServerProcessError {}

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
	fn app_server_dev_executable_command_runs_prebuilt_binary_with_dev_env() {
		let command = app_server_dev_executable_command(
			&plan(),
			&AppServerDevInput::new(3000),
			std::path::Path::new("/workspace/app/target/debug/example-server"),
		)
		.unwrap();

		assert_eq!(
			command.program(),
			"/workspace/app/target/debug/example-server"
		);
		assert!(command.args().is_empty());
		assert_eq!(command.env().get(APP_SERVER_PORT_ENV_KEY).unwrap(), "3000");
		assert_eq!(
			command.env().get(ENV_KEY_IS_DEV).unwrap(),
			ENV_VALUE_ENABLED
		);
	}

	#[test]
	fn prebuilt_app_server_matches_only_the_current_plan_server_target() {
		let prebuilt = PrebuiltAppServer::new("example-app", "example-server", "/t/server");
		let moved = PrebuiltAppServer::new("example-app", "renamed-server", "/t/server");

		assert!(prebuilt.matches_plan_server_target(&plan()));
		assert!(!moved.matches_plan_server_target(&plan()));
	}

	#[test]
	fn app_server_dev_command_projects_cargo_target_and_dev_env() {
		let command = app_server_dev_command(&plan(), &AppServerDevInput::new(3000)).unwrap();

		assert_eq!(command.program(), "cargo");
		assert_eq!(
			command.current_dir(),
			std::path::Path::new("/workspace/app")
		);
		assert_eq!(
			command.args(),
			["run", "-p", "example-app", "--bin", "example-server"]
		);
		assert_eq!(command.env().get(APP_SERVER_PORT_ENV_KEY).unwrap(), "3000");
		assert_eq!(
			command.env().get(ENV_KEY_IS_DEV).unwrap(),
			ENV_VALUE_ENABLED
		);
	}
}
