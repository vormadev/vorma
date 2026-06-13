//! Shared Vite process command projection.

use std::path::PathBuf;

use crate::build_output::resolve_workspace_path;
use crate::build_plan::BuildProjectionPlan;
use crate::process_runner::{BuildProcessCommand, command_base_parts};
use crate::vite_plugin_contract::{
	VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, VitePluginConfigError,
	package_manager_relative_path,
};

pub(crate) struct ViteCommandParts {
	program: String,
	args: Vec<String>,
	root_dir: PathBuf,
	js_package_manager_dir: PathBuf,
}

impl ViteCommandParts {
	pub(crate) fn from_plan(plan: &BuildProjectionPlan) -> Result<Self, ViteCommandError> {
		let mut args = command_base_parts(plan.vite_inputs().js_package_manager_base_cmd())
			.ok_or(ViteCommandError::EmptyCommandBase)?;
		let program = args.remove(0);
		let root_dir = PathBuf::from(plan.workspace().root_dir());
		if root_dir.as_os_str().is_empty() {
			return Err(ViteCommandError::EmptyRootDir);
		}
		let js_package_manager_dir =
			resolve_workspace_path(&root_dir, plan.vite_inputs().js_package_manager_dir());
		Ok(Self {
			program,
			args,
			root_dir,
			js_package_manager_dir,
		})
	}

	pub(crate) fn args_mut(&mut self) -> &mut Vec<String> {
		&mut self.args
	}

	pub(crate) fn root_dir(&self) -> &PathBuf {
		&self.root_dir
	}

	pub(crate) fn js_package_manager_dir(&self) -> &PathBuf {
		&self.js_package_manager_dir
	}

	pub(crate) fn push_config_arg(
		&mut self,
		plan: &BuildProjectionPlan,
	) -> Result<(), ViteCommandError> {
		let vite_config_file = plan.vite_inputs().vite_config_file().trim();
		if vite_config_file.is_empty() {
			return Ok(());
		}
		self.args.extend([
			"--config".to_owned(),
			package_manager_relative_path(
				&self.root_dir,
				&self.js_package_manager_dir,
				vite_config_file,
			)
			.map_err(|source| ViteCommandError::VitePath { source })?,
		]);
		Ok(())
	}

	pub(crate) fn into_command(
		self,
		plugin_server_port: u16,
		plugin_server_token: &str,
	) -> BuildProcessCommand {
		BuildProcessCommand::new(self.program, self.args, self.js_package_manager_dir)
			.with_env(
				VITE_PLUGIN_SERVER_PORT_ENV_KEY,
				plugin_server_port.to_string(),
			)
			.with_env(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, plugin_server_token)
	}
}

/// Shared Vite command construction error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ViteCommandError {
	/// JavaScript package-manager command prefix was empty.
	EmptyCommandBase,
	/// Build root directory was empty.
	EmptyRootDir,
	/// A Vite path could not be projected.
	VitePath {
		/// Source Vite path projection error.
		source: VitePluginConfigError,
	},
}

impl std::fmt::Display for ViteCommandError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyCommandBase => {
				f.write_str("frontend_config.js_package_manager_base_cmd cannot be empty")
			}
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::VitePath { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for ViteCommandError {}
