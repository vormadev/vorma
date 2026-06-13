//! Cargo build for the app-server binary plus its live-state read.
/*
The app-server binary is the only app-linked binary: it serves the app AND
answers live-state requests (the runtime emits the contract when the build's
env key is set). One cargo invocation builds it; the same executable is then
run once for live-state and reused as the prebuilt dev server.
*/

use std::path::{Path, PathBuf};

use serde::Deserialize;

use crate::live_state::LiveBuildState;
use crate::live_state_command::LiveStateCommandError;
use crate::process_runner::{
	BuildProcessCancel, BuildProcessCommand, BuildProcessError, BuildProcessRunner,
};

const CARGO_BUILD_MESSAGE_FORMAT_ARG: &str = "--message-format=json-render-diagnostics";
const CARGO_COMPILER_ARTIFACT_REASON: &str = "compiler-artifact";
const CARGO_BIN_TARGET_KIND: &str = "bin";

/// Build the app-server binary, then read live state from the built executable.
pub fn build_app_server_and_read_live_state_until_cancelled(
	root_dir: impl Into<PathBuf>,
	cargo_package: &str,
	cargo_bin: &str,
	runner: &mut impl BuildProcessRunner,
	cancel: &BuildProcessCancel,
	read_live_state: impl FnOnce(
		&Path,
		&BuildProcessCancel,
	) -> Result<LiveBuildState, LiveStateCommandError>,
) -> Result<(PathBuf, LiveBuildState), AppServerBuildError> {
	let command = app_server_build_command(root_dir, cargo_package, cargo_bin)?;
	let mut bin_artifacts: Vec<(String, Option<String>, PathBuf)> = Vec::new();
	let output = runner
		.run_streaming_stdout_lines_until_cancelled(&command, cancel, &mut |line| {
			if let Some(artifact) = bin_artifact_from_cargo_message_line(line) {
				bin_artifacts.push(artifact);
			}
		})
		.map_err(|source| AppServerBuildError::Process { source })?;
	if !output.status_success() {
		return Err(AppServerBuildError::CommandFailed {
			status: output.status().to_owned(),
			output: String::from_utf8_lossy(output.stderr()).into_owned(),
		});
	}
	let executable = executable_for_target(&bin_artifacts, cargo_package, cargo_bin)?;
	let live_state = read_live_state(&executable, cancel)
		.map_err(|source| AppServerBuildError::LiveState { source })?;
	Ok((executable, live_state))
}

/// Build the app-server cargo build command without executing it.
pub fn app_server_build_command(
	root_dir: impl Into<PathBuf>,
	cargo_package: &str,
	cargo_bin: &str,
) -> Result<BuildProcessCommand, AppServerBuildError> {
	let root_dir = root_dir.into();
	if root_dir.as_os_str().is_empty() {
		return Err(AppServerBuildError::EmptyRootDir);
	}
	for (field, value) in [("cargo_package", cargo_package), ("cargo_bin", cargo_bin)] {
		if value.trim().is_empty() {
			return Err(AppServerBuildError::EmptyTargetField { field });
		}
	}
	let args = vec![
		"build".to_owned(),
		"-p".to_owned(),
		cargo_package.to_owned(),
		"--bin".to_owned(),
		cargo_bin.to_owned(),
		CARGO_BUILD_MESSAGE_FORMAT_ARG.to_owned(),
	];
	Ok(BuildProcessCommand::new("cargo", args, root_dir))
}

#[derive(Debug, Deserialize)]
struct CargoBuildMessage {
	reason: String,
	package_id: Option<String>,
	target: Option<CargoBuildMessageTarget>,
	executable: Option<String>,
}

#[derive(Debug, Deserialize)]
struct CargoBuildMessageTarget {
	name: String,
	kind: Vec<String>,
}

fn bin_artifact_from_cargo_message_line(line: &str) -> Option<(String, Option<String>, PathBuf)> {
	let line = line.trim();
	if line.is_empty() {
		return None;
	}
	let message = serde_json::from_str::<CargoBuildMessage>(line).ok()?;
	if message.reason != CARGO_COMPILER_ARTIFACT_REASON {
		return None;
	}
	let (target, executable) = (message.target?, message.executable?);
	if !target.kind.iter().any(|kind| kind == CARGO_BIN_TARGET_KIND) {
		return None;
	}
	let package_name = message
		.package_id
		.as_deref()
		.and_then(package_name_from_package_id);
	Some((target.name, package_name, PathBuf::from(executable)))
}

/*
A parsed package name that does not match the requested package is never
acceptable; artifacts with unparseable package ids are accepted only when they
leave a single unambiguous candidate.
*/
fn executable_for_target(
	bin_artifacts: &[(String, Option<String>, PathBuf)],
	cargo_package: &str,
	cargo_bin: &str,
) -> Result<PathBuf, AppServerBuildError> {
	let mut named: Vec<&(String, Option<String>, PathBuf)> = bin_artifacts
		.iter()
		.filter(|(name, _, _)| name == cargo_bin)
		.collect();
	named.dedup_by(|a, b| a.2 == b.2);
	let package_matched: Vec<_> = named
		.iter()
		.filter(|(_, package_name, _)| package_name.as_deref() == Some(cargo_package))
		.copied()
		.collect();
	let candidates = if package_matched.is_empty() {
		named
			.into_iter()
			.filter(|(_, package_name, _)| package_name.is_none())
			.collect::<Vec<_>>()
	} else {
		package_matched
	};
	match candidates.as_slice() {
		[(_, _, executable)] => Ok(executable.clone()),
		[] => Err(AppServerBuildError::MissingBinArtifact {
			cargo_package: cargo_package.to_owned(),
			cargo_bin: cargo_bin.to_owned(),
		}),
		_ => Err(AppServerBuildError::AmbiguousBinArtifact {
			cargo_package: cargo_package.to_owned(),
			cargo_bin: cargo_bin.to_owned(),
		}),
	}
}

/*
Package id formats: modern cargo emits PackageId specs such as
`path+file:///workspace/app#example-app@0.1.0` (or `...#0.1.0` with the name
implied by the final URL segment), while pre-spec cargo emitted
`example-app 0.1.0 (path+file:///workspace/app)`.
*/
fn package_name_from_package_id(package_id: &str) -> Option<String> {
	if let Some((url, fragment)) = package_id.split_once('#') {
		if let Some((name, _version)) = fragment.split_once('@') {
			return Some(name.to_owned());
		}
		return url
			.rsplit('/')
			.next()
			.filter(|segment| !segment.is_empty())
			.map(str::to_owned);
	}
	package_id
		.split_whitespace()
		.next()
		.filter(|segment| !segment.is_empty())
		.map(str::to_owned)
}

/// App-server build error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AppServerBuildError {
	/// Build root directory was empty.
	EmptyRootDir,
	/// A cargo target field was empty.
	EmptyTargetField {
		/// Rejected field.
		field: &'static str,
	},
	/// Cargo process execution failed.
	Process {
		/// Source process runner error.
		source: BuildProcessError,
	},
	/// Cargo exited unsuccessfully.
	CommandFailed {
		/// Exit status.
		status: String,
		/// Captured stderr.
		output: String,
	},
	/// Cargo reported no matching bin artifact.
	MissingBinArtifact {
		/// Requested cargo package.
		cargo_package: String,
		/// Requested cargo bin.
		cargo_bin: String,
	},
	/// Cargo reported multiple matching bin artifacts.
	AmbiguousBinArtifact {
		/// Requested cargo package.
		cargo_package: String,
		/// Requested cargo bin.
		cargo_bin: String,
	},
	/// Live-state read from the built executable failed.
	LiveState {
		/// Source live-state command error.
		source: LiveStateCommandError,
	},
}

impl std::fmt::Display for AppServerBuildError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::EmptyRootDir => f.write_str("root_dir cannot be empty"),
			Self::EmptyTargetField { field } => {
				write!(f, "app-server {field} cannot be empty")
			}
			Self::Process { source } => write!(f, "{source}"),
			Self::CommandFailed { status, output } => {
				write!(f, "app-server cargo build exited with {status}: {output}")
			}
			Self::MissingBinArtifact {
				cargo_package,
				cargo_bin,
			} => write!(
				f,
				"cargo reported no bin artifact for {cargo_package}/{cargo_bin}"
			),
			Self::AmbiguousBinArtifact {
				cargo_package,
				cargo_bin,
			} => write!(
				f,
				"cargo reported multiple bin artifacts for {cargo_package}/{cargo_bin}"
			),
			Self::LiveState { source } => write!(f, "{source}"),
		}
	}
}

impl std::error::Error for AppServerBuildError {}

#[cfg(test)]
mod tests {
	use std::path::Path;

	use vorma_contract::framework_graph::{
		FrameworkConfig, FrameworkDeclarations, FrameworkGraph, HandlerId, ViewDeclaration,
	};

	use super::*;
	use crate::process_runner::BuildProcessOutput;
	use crate::test_support::route_type_contract;

	const TEST_ROOT_DIR: &str = "/workspace/app";
	const TEST_HASH_SOURCE: &str = "hash-source";
	const TEST_SERVER_PACKAGE_ID: &str = "path+file:///workspace/app#example-app@0.1.0";

	fn canned_live_state() -> LiveBuildState {
		let mut declarations = FrameworkDeclarations::new(FrameworkConfig::new("/static"));
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			HandlerId::new("root").unwrap(),
		));
		LiveBuildState::from_graph(
			FrameworkGraph::compile(declarations).unwrap(),
			TEST_HASH_SOURCE,
		)
	}

	fn artifact_line(package_id: &str, bin_name: &str, executable: &str) -> String {
		format!(
			"{{\"reason\":\"compiler-artifact\",\"package_id\":{package_id:?},\
			 \"target\":{{\"name\":{bin_name:?},\"kind\":[\"bin\"]}},\
			 \"executable\":{executable:?}}}"
		)
	}

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
			unreachable!("app server build must collect output")
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
	fn app_server_build_command_targets_one_binary_with_json_messages() {
		let command =
			app_server_build_command(TEST_ROOT_DIR, "example-app", "example-server").unwrap();

		assert_eq!(command.program(), "cargo");
		assert_eq!(
			command.args(),
			&[
				"build",
				"-p",
				"example-app",
				"--bin",
				"example-server",
				"--message-format=json-render-diagnostics",
			]
		);
		assert_eq!(command.current_dir(), Path::new(TEST_ROOT_DIR));
	}

	#[test]
	fn app_server_build_extracts_executable_and_reads_live_state() {
		let stdout = [
			r#"{"reason":"build-script-executed","package_id":"path+file:///dep#1.0.0"}"#
				.to_owned(),
			artifact_line(
				TEST_SERVER_PACKAGE_ID,
				"example-server",
				"/target/debug/example-server",
			),
			r#"{"reason":"build-finished","success":true}"#.to_owned(),
		]
		.join("\n");
		let mut runner = FakeRunner {
			output: Some(BuildProcessOutput::new(
				true,
				"exit status: 0".to_owned(),
				stdout.into_bytes(),
				Vec::new(),
			)),
			..FakeRunner::default()
		};

		let (executable, live_state) = build_app_server_and_read_live_state_until_cancelled(
			TEST_ROOT_DIR,
			"example-app",
			"example-server",
			&mut runner,
			&BuildProcessCancel::new(),
			|executable, _| {
				assert_eq!(executable, Path::new("/target/debug/example-server"));
				Ok(canned_live_state())
			},
		)
		.unwrap();

		assert_eq!(executable, Path::new("/target/debug/example-server"));
		assert_eq!(live_state.root_document_hash_source(), TEST_HASH_SOURCE);
	}

	#[test]
	fn app_server_build_disambiguates_same_bin_name_across_packages() {
		let lines = [
			artifact_line(
				"path+file:///workspace/other#other-pkg@0.1.0",
				"main",
				"/t/other-main",
			),
			artifact_line(
				"path+file:///workspace/server#server-pkg@0.1.0",
				"main",
				"/t/server-main",
			),
		];
		let bin_artifacts = lines
			.iter()
			.filter_map(|line| bin_artifact_from_cargo_message_line(line))
			.collect::<Vec<_>>();

		assert_eq!(
			executable_for_target(&bin_artifacts, "server-pkg", "main").unwrap(),
			Path::new("/t/server-main")
		);
	}

	#[test]
	fn app_server_build_rejects_bin_artifacts_from_other_packages() {
		let line = artifact_line(
			"path+file:///workspace/other#other-pkg@0.1.0",
			"example-server",
			"/t/b",
		);
		let bin_artifacts = [line]
			.iter()
			.filter_map(|line| bin_artifact_from_cargo_message_line(line))
			.collect::<Vec<_>>();

		assert_eq!(
			executable_for_target(&bin_artifacts, "example-app", "example-server").unwrap_err(),
			AppServerBuildError::MissingBinArtifact {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			}
		);
	}

	#[test]
	fn app_server_build_reports_missing_bin_artifacts_without_reading_live_state() {
		let mut runner = FakeRunner {
			output: Some(BuildProcessOutput::new(
				true,
				"exit status: 0".to_owned(),
				Vec::new(),
				Vec::new(),
			)),
			..FakeRunner::default()
		};

		let error = build_app_server_and_read_live_state_until_cancelled(
			TEST_ROOT_DIR,
			"example-app",
			"example-server",
			&mut runner,
			&BuildProcessCancel::new(),
			|_, _| unreachable!("live state must not be read without a server artifact"),
		)
		.unwrap_err();

		assert_eq!(
			error,
			AppServerBuildError::MissingBinArtifact {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			}
		);
	}

	#[test]
	fn app_server_build_surfaces_live_state_errors() {
		let stdout = artifact_line(
			TEST_SERVER_PACKAGE_ID,
			"example-server",
			"/target/debug/example-server",
		);
		let mut runner = FakeRunner {
			output: Some(BuildProcessOutput::new(
				true,
				"exit status: 0".to_owned(),
				stdout.into_bytes(),
				Vec::new(),
			)),
			..FakeRunner::default()
		};

		let error = build_app_server_and_read_live_state_until_cancelled(
			TEST_ROOT_DIR,
			"example-app",
			"example-server",
			&mut runner,
			&BuildProcessCancel::new(),
			|_, _| {
				Err(LiveStateCommandError::CommandFailed {
					status: "exit status: 1".to_owned(),
					output: "boom".to_owned(),
				})
			},
		)
		.unwrap_err();

		assert!(matches!(error, AppServerBuildError::LiveState { .. }));
	}

	#[test]
	fn app_server_build_surfaces_failed_cargo_status_with_stderr() {
		let mut runner = FakeRunner {
			output: Some(BuildProcessOutput::new(
				false,
				"exit status: 101".to_owned(),
				Vec::new(),
				b"compile error".to_vec(),
			)),
			..FakeRunner::default()
		};

		let error = build_app_server_and_read_live_state_until_cancelled(
			TEST_ROOT_DIR,
			"example-app",
			"example-server",
			&mut runner,
			&BuildProcessCancel::new(),
			|_, _| unreachable!("live state must not be read after a failed build"),
		)
		.unwrap_err();

		assert_eq!(
			error,
			AppServerBuildError::CommandFailed {
				status: "exit status: 101".to_owned(),
				output: "compile error".to_owned(),
			}
		);
	}

	#[test]
	fn package_id_name_parsing_handles_spec_and_legacy_formats() {
		assert_eq!(
			package_name_from_package_id("path+file:///workspace/app#example-app@0.1.0"),
			Some("example-app".to_owned())
		);
		assert_eq!(
			package_name_from_package_id("path+file:///workspace/example-app#0.1.0"),
			Some("example-app".to_owned())
		);
		assert_eq!(
			package_name_from_package_id("example-app 0.1.0 (path+file:///workspace/app)"),
			Some("example-app".to_owned())
		);
	}
}
