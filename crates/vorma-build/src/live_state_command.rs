//! Build-entry live-state child process execution.
//!
//! The live-state protocol (see [`crate::live_state`], re-exported from
//! `vorma-contract`) lets this crate read back an app's compiled framework
//! graph and build-relevant facts from an already-built app-server
//! executable, by running that same executable once with a build-mode
//! environment key set instead of compiling a separate build-entry binary —
//! see [`crate::app_server_build`]'s module doc for why the app-server
//! binary is the only app-linked binary in a Vorma project. This module is
//! the process-execution half; [`crate::live_state`] owns the wire format.

use std::path::{Path, PathBuf};

use vorma_contract::constants::{ENV_KEY_IS_BUILD, ENV_VALUE_ENABLED};

use crate::live_state::{
	LIVE_BUILD_STATE_ENV_KEY, LIVE_BUILD_STATE_ENV_VALUE, LiveBuildState, LiveBuildStateError,
};
use crate::process_runner::{
	BuildProcessCancel, BuildProcessCommand, BuildProcessError, BuildProcessOutput,
	BuildProcessRunner,
};

/// Read live build state by running a prebuilt build-entry executable in
/// live-state mode. The non-cancellable convenience wrapper around
/// [`read_live_build_state_from_executable_until_cancelled`], for tests
/// that have no cancellation token to thread through.
#[cfg(test)]
pub fn read_live_build_state_from_executable(
	executable: &Path,
	root_dir: impl Into<PathBuf>,
	runner: &mut impl BuildProcessRunner,
) -> Result<LiveBuildState, LiveStateCommandError> {
	let cancel = BuildProcessCancel::new();
	read_live_build_state_from_executable_until_cancelled(executable, root_dir, runner, &cancel)
}

/// Read live build state, terminating the child process group if
/// cancellation fires. Runs `executable` once with the build-mode and
/// live-state env keys set (see [`live_state_executable_command`]),
/// collects its output, and parses successful stdout as the live-state
/// JSON protocol (see [`crate::live_state::LiveBuildState::from_json_bytes`]).
pub fn read_live_build_state_from_executable_until_cancelled(
	executable: &Path,
	root_dir: impl Into<PathBuf>,
	runner: &mut impl BuildProcessRunner,
	cancel: &BuildProcessCancel,
) -> Result<LiveBuildState, LiveStateCommandError> {
	let command = live_state_executable_command(executable, root_dir)?;
	let output = runner
		.run_collecting_output_until_cancelled(&command, cancel)
		.map_err(|source| LiveStateCommandError::Process { source })?;
	live_state_from_output(output)
}

/// Build the live-state command for a prebuilt executable without executing
/// it: runs `executable` directly with no arguments, setting the build-mode
/// env key (so the app-server binary knows this run is a build/tooling
/// invocation, not a real serve) and the live-state env key (so it emits
/// the live-state JSON protocol on stdout and exits, instead of serving).
pub fn live_state_executable_command(
	executable: &Path,
	root_dir: impl Into<PathBuf>,
) -> Result<BuildProcessCommand, LiveStateCommandError> {
	let root_dir = root_dir.into();
	if root_dir.as_os_str().is_empty() {
		return Err(LiveStateCommandError::EmptyRootDir);
	}
	if executable.as_os_str().is_empty() {
		return Err(LiveStateCommandError::EmptyExecutablePath);
	}
	let executable =
		executable
			.to_str()
			.ok_or_else(|| LiveStateCommandError::NonUtf8ExecutablePath {
				path: executable.display().to_string(),
			})?;
	Ok(BuildProcessCommand::new(executable, Vec::new(), root_dir)
		.with_env(ENV_KEY_IS_BUILD, ENV_VALUE_ENABLED)
		.with_env(LIVE_BUILD_STATE_ENV_KEY, LIVE_BUILD_STATE_ENV_VALUE))
}

fn live_state_from_output(
	output: BuildProcessOutput,
) -> Result<LiveBuildState, LiveStateCommandError> {
	if !output.status_success() {
		let mut combined = String::new();
		combined.push_str(&String::from_utf8_lossy(output.stdout()));
		combined.push_str(&String::from_utf8_lossy(output.stderr()));
		return Err(LiveStateCommandError::CommandFailed {
			status: output.status().to_owned(),
			output: combined,
		});
	}
	LiveBuildState::from_json_bytes(output.stdout())
		.map_err(|source| LiveStateCommandError::LiveState { source })
}

/// Error from this module's live-state command-building and execution
/// functions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LiveStateCommandError {
	/// Build root directory was empty.
	EmptyRootDir,
	/// Build-entry executable path was empty.
	EmptyExecutablePath,
	/// Build-entry executable path was not valid UTF-8.
	NonUtf8ExecutablePath {
		/// Rejected executable path.
		path: String,
	},
	/// Build-entry process execution failed.
	Process {
		/// Source process runner error.
		source: BuildProcessError,
	},
	/// Build-entry process exited unsuccessfully.
	CommandFailed {
		/// Exit status.
		status: String,
		/// Combined stdout and stderr.
		output: String,
	},
	/// Live-state protocol parsing or validation failed.
	LiveState {
		/// Source live-state error.
		source: LiveBuildStateError,
	},
}

impl std::fmt::Display for LiveStateCommandError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::EmptyExecutablePath => f.write_str("build-entry executable path cannot be empty"),
			Self::NonUtf8ExecutablePath { path } => {
				write!(f, "build-entry executable path must be UTF-8: {path}")
			}
			Self::Process { source } => write!(f, "{source}"),
			Self::CommandFailed { status, output } => {
				write!(
					f,
					"build-entry live-state command exited with {status}: {output}"
				)
			}
			Self::LiveState { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for LiveStateCommandError {}

#[cfg(test)]
mod tests {
	use std::path::Path;

	use super::*;

	const TEST_ROOT_DIR: &str = "/workspace/app";
	const TEST_BUILD_ENTRY_EXECUTABLE: &str = "/workspace/app/target/debug/build";
	const TEST_LIVE_JSON: &[u8] = br#"{"error":"boom"}"#;

	#[derive(Default)]
	struct FakeRunner {
		command: Option<BuildProcessCommand>,
		output: Option<BuildProcessOutput>,
	}

	impl BuildProcessRunner for FakeRunner {
		fn run_inheriting_stdio(
			&mut self,
			_command: &BuildProcessCommand,
		) -> Result<(), BuildProcessError> {
			unreachable!("live-state command must collect output")
		}

		fn run_collecting_output(
			&mut self,
			command: &BuildProcessCommand,
		) -> Result<BuildProcessOutput, BuildProcessError> {
			self.command = Some(command.clone());
			Ok(self.output.take().unwrap())
		}
	}

	#[test]
	fn live_state_command_projects_build_entry_target_and_env() {
		let command =
			live_state_executable_command(Path::new(TEST_BUILD_ENTRY_EXECUTABLE), TEST_ROOT_DIR)
				.unwrap();

		assert_eq!(command.program(), TEST_BUILD_ENTRY_EXECUTABLE);
		assert_eq!(command.current_dir(), Path::new(TEST_ROOT_DIR));
		assert!(command.args().is_empty());
		assert_eq!(
			command.env().get(ENV_KEY_IS_BUILD).unwrap(),
			ENV_VALUE_ENABLED
		);
		assert_eq!(
			command.env().get(LIVE_BUILD_STATE_ENV_KEY).unwrap(),
			LIVE_BUILD_STATE_ENV_VALUE
		);
	}

	#[test]
	fn live_state_command_surfaces_protocol_error_from_stdout() {
		let mut runner = FakeRunner {
			output: Some(BuildProcessOutput::new(
				true,
				"exit status: 0",
				TEST_LIVE_JSON.to_vec(),
				Vec::new(),
			)),
			..FakeRunner::default()
		};
		let error = read_live_build_state_from_executable(
			Path::new(TEST_BUILD_ENTRY_EXECUTABLE),
			TEST_ROOT_DIR,
			&mut runner,
		)
		.unwrap_err();

		assert_eq!(
			runner
				.command
				.unwrap()
				.env()
				.get(LIVE_BUILD_STATE_ENV_KEY)
				.unwrap(),
			LIVE_BUILD_STATE_ENV_VALUE
		);
		assert_eq!(
			error,
			LiveStateCommandError::LiveState {
				source: LiveBuildStateError::App {
					message: "boom".to_owned()
				}
			}
		);
	}

	#[test]
	fn live_state_command_surfaces_failed_status_output() {
		let mut runner = FakeRunner {
			output: Some(BuildProcessOutput::new(
				false,
				"exit status: 1",
				b"stdout".to_vec(),
				b"stderr".to_vec(),
			)),
			..FakeRunner::default()
		};
		let error = read_live_build_state_from_executable(
			Path::new(TEST_BUILD_ENTRY_EXECUTABLE),
			TEST_ROOT_DIR,
			&mut runner,
		)
		.unwrap_err();

		assert_eq!(
			error,
			LiveStateCommandError::CommandFailed {
				status: "exit status: 1".to_owned(),
				output: "stdoutstderr".to_owned()
			}
		);
	}
}
