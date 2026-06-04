use std::collections::BTreeMap;
use std::fmt;
use std::io;
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};
use std::sync::Arc;

use serde::Deserialize;
use tokio::io::{AsyncBufReadExt, AsyncRead, BufReader};

use crate::build_cancel::BuildCancel;
use crate::cargo_target::CargoBinTarget;
use crate::command_runner::{CommandRunError, CommandStderr};
use crate::config::VormaCfg;
use crate::process_wait::{ChildWaitError, current_thread_runtime, wait_child_or_cancel};
use crate::supervisor::{clear_vorma_runtime_env, prepare_child_process};

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct CargoBuildArtifacts {
	pub(crate) build_entry_executable: PathBuf,
	pub(crate) app_server_executable: PathBuf,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct CargoBuildEntryArtifact {
	pub(crate) build_entry_executable: PathBuf,
}

#[derive(Debug)]
pub(crate) enum CargoBuildError {
	CommandStart { source: std::io::Error },
	CommandWait { source: std::io::Error },
	CommandFailed { output: Output },
	Cancelled,
	CargoOutput(CargoBuildOutputError),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum CargoBuildOutputError {
	InvalidBuildArgs(String),
	InvalidJson {
		line: String,
		message: String,
	},
	InvalidMetadata {
		message: String,
	},
	UnknownPackageId {
		package_id: String,
	},
	DuplicateExecutable {
		cargo_package: String,
		cargo_bin: String,
	},
	ExecutableUnavailable {
		cargo_package: String,
		cargo_bin: String,
	},
}

#[derive(Deserialize)]
struct CargoMessage {
	reason: String,
	package_id: Option<String>,
	target: Option<CargoMessageTarget>,
	executable: Option<PathBuf>,
}

#[derive(Deserialize)]
struct CargoMessageTarget {
	name: String,
	kind: Vec<String>,
}

#[derive(Deserialize)]
struct CargoMetadata {
	packages: Vec<CargoMetadataPackage>,
}

#[derive(Deserialize)]
struct CargoMetadataPackage {
	id: String,
	name: String,
}

pub(crate) fn compile_dev_targets_with_build_entry_ready<F>(
	cfg: &VormaCfg<'_>,
	build_entry: &CargoBinTarget,
	build_cancel: &Arc<BuildCancel>,
	on_build_entry_ready: F,
) -> Result<CargoBuildArtifacts, CargoBuildError>
where
	F: FnOnce(PathBuf) + Send + 'static,
{
	let args = cfg
		.cargo_build_args(build_entry)
		.map_err(CargoBuildOutputError::InvalidBuildArgs)
		.map_err(CargoBuildError::CargoOutput)?;
	let root_dir = cfg.root_dir();
	run_local_cargo_build(
		root_dir,
		args,
		build_entry,
		&cfg.app_server_cargo_target(),
		build_cancel,
		on_build_entry_ready,
	)
}

pub(crate) fn compile_build_entry(
	cfg: &VormaCfg<'_>,
	build_entry: &CargoBinTarget,
	build_cancel: &Arc<BuildCancel>,
) -> Result<CargoBuildEntryArtifact, CargoBuildError> {
	let args = cfg
		.cargo_build_entry_args(build_entry)
		.map_err(CargoBuildOutputError::InvalidBuildArgs)
		.map_err(CargoBuildError::CargoOutput)?;
	let root_dir = cfg.root_dir();
	let package_names = cargo_package_names(&root_dir, build_cancel)?;
	let output = run_cargo_build_command(&root_dir, args, build_cancel)?;
	let build_entry_executable = cargo_bin_executable(&output.stdout, &package_names, build_entry)
		.map_err(CargoBuildError::CargoOutput)?;

	Ok(CargoBuildEntryArtifact {
		build_entry_executable,
	})
}

fn run_local_cargo_build(
	root_dir: String,
	args: Vec<String>,
	build_entry: &CargoBinTarget,
	app_server: &CargoBinTarget,
	build_cancel: &Arc<BuildCancel>,
	on_build_entry_ready: impl FnOnce(PathBuf) + Send + 'static,
) -> Result<CargoBuildArtifacts, CargoBuildError> {
	let package_names = cargo_package_names(&root_dir, build_cancel)?;
	run_dev_cargo_build_command(
		&root_dir,
		args,
		package_names,
		build_entry.clone(),
		app_server.clone(),
		build_cancel,
		on_build_entry_ready,
	)
}

fn run_cargo_build_command(
	root_dir: &str,
	args: Vec<String>,
	build_cancel: &Arc<BuildCancel>,
) -> Result<Output, CargoBuildError> {
	let mut command = Command::new("cargo");
	command.current_dir(root_dir).args(args);
	run_command_collecting_output(command, build_cancel)
}

fn run_dev_cargo_build_command<F>(
	root_dir: &str,
	args: Vec<String>,
	package_names: BTreeMap<String, String>,
	build_entry: CargoBinTarget,
	app_server: CargoBinTarget,
	build_cancel: &Arc<BuildCancel>,
	on_build_entry_ready: F,
) -> Result<CargoBuildArtifacts, CargoBuildError>
where
	F: FnOnce(PathBuf) + Send + 'static,
{
	let mut command = Command::new("cargo");
	command.current_dir(root_dir).args(args);
	clear_vorma_runtime_env(&mut command);
	prepare_child_process(&mut command);
	command.stdout(Stdio::piped()).stderr(Stdio::inherit());
	let runtime =
		current_thread_runtime().map_err(|source| CargoBuildError::CommandWait { source })?;
	runtime.block_on(async {
		let mut command = tokio::process::Command::from(command);
		let mut child = command
			.spawn()
			.map_err(|source| CargoBuildError::CommandStart { source })?;
		let stdout = child
			.stdout
			.take()
			.ok_or_else(|| CargoBuildError::CommandWait {
				source: io::Error::other("missing command stdout pipe"),
			})?;
		let stdout_thread = tokio::spawn(read_dev_cargo_stdout(
			stdout,
			package_names,
			build_entry,
			app_server,
			on_build_entry_ready,
		));
		let status = match wait_child_or_cancel(&mut child, build_cancel).await {
			Ok(status) => status,
			Err(ChildWaitError::Cancelled) => {
				let _ = join_dev_cargo_stdout(stdout_thread).await;
				return Err(CargoBuildError::Cancelled);
			}
			Err(ChildWaitError::Wait(source)) => {
				return Err(CargoBuildError::CommandWait { source });
			}
		};

		let output = join_dev_cargo_stdout(stdout_thread).await?;
		if !status.success() {
			return Err(CargoBuildError::CommandFailed {
				output: Output {
					status,
					stdout: output.stdout,
					stderr: Vec::new(),
				},
			});
		}
		output.artifacts.map_err(CargoBuildError::CargoOutput)
	})
}

struct DevCargoStdoutOutput {
	stdout: Vec<u8>,
	artifacts: Result<CargoBuildArtifacts, CargoBuildOutputError>,
}

async fn read_dev_cargo_stdout<R, F>(
	reader: R,
	package_names: BTreeMap<String, String>,
	build_entry: CargoBinTarget,
	app_server: CargoBinTarget,
	on_build_entry_ready: F,
) -> io::Result<DevCargoStdoutOutput>
where
	R: AsyncRead + Send + Unpin + 'static,
	F: FnOnce(PathBuf) + Send + 'static,
{
	let mut stdout = Vec::new();
	let mut reader = BufReader::new(reader);
	let mut collector = DevArtifactCollector {
		package_names,
		build_entry,
		app_server,
		build_entry_executable: None,
		app_server_executable: None,
		on_build_entry_ready: Some(on_build_entry_ready),
	};
	let mut line = String::new();
	loop {
		line.clear();
		match reader.read_line(&mut line).await {
			Ok(0) => break,
			Ok(_) => {
				stdout.extend_from_slice(line.as_bytes());
				if let Err(error) = collector.record_line(&line) {
					return Ok(DevCargoStdoutOutput {
						stdout,
						artifacts: Err(error),
					});
				}
			}
			Err(error) => return Err(error),
		}
	}
	Ok(DevCargoStdoutOutput {
		stdout,
		artifacts: collector.finish(),
	})
}

async fn join_dev_cargo_stdout(
	join_handle: tokio::task::JoinHandle<io::Result<DevCargoStdoutOutput>>,
) -> Result<DevCargoStdoutOutput, CargoBuildError> {
	join_handle
		.await
		.map_err(|_| CargoBuildError::CommandWait {
			source: io::Error::other("command stdout reader panicked"),
		})?
		.map_err(|source| CargoBuildError::CommandWait { source })
}

struct DevArtifactCollector<F> {
	package_names: BTreeMap<String, String>,
	build_entry: CargoBinTarget,
	app_server: CargoBinTarget,
	build_entry_executable: Option<PathBuf>,
	app_server_executable: Option<PathBuf>,
	on_build_entry_ready: Option<F>,
}

impl<F> DevArtifactCollector<F>
where
	F: FnOnce(PathBuf),
{
	fn record_line(&mut self, line: &str) -> Result<(), CargoBuildOutputError> {
		let message: CargoMessage =
			serde_json::from_str(line).map_err(|error| CargoBuildOutputError::InvalidJson {
				line: line.to_owned(),
				message: error.to_string(),
			})?;
		if message.reason != "compiler-artifact" {
			return Ok(());
		}
		let Some(target) = message.target else {
			return Ok(());
		};
		if !target.kind.iter().any(|kind| kind == "bin") {
			return Ok(());
		}
		let Some(package_id) = message.package_id else {
			return Ok(());
		};
		let Some(package_name) = self.package_names.get(&package_id) else {
			return Err(CargoBuildOutputError::UnknownPackageId { package_id });
		};
		let cargo_target = CargoBinTarget {
			cargo_package: package_name.clone(),
			cargo_bin: target.name,
		};
		let Some(executable) = message.executable else {
			return Ok(());
		};
		if cargo_target == self.build_entry {
			if self.build_entry_executable.is_some() {
				return Err(CargoBuildOutputError::DuplicateExecutable {
					cargo_package: self.build_entry.cargo_package.clone(),
					cargo_bin: self.build_entry.cargo_bin.clone(),
				});
			}
			self.build_entry_executable = Some(executable.clone());
			if let Some(on_build_entry_ready) = self.on_build_entry_ready.take() {
				on_build_entry_ready(executable);
			}
			return Ok(());
		}
		if cargo_target == self.app_server {
			if self.app_server_executable.is_some() {
				return Err(CargoBuildOutputError::DuplicateExecutable {
					cargo_package: self.app_server.cargo_package.clone(),
					cargo_bin: self.app_server.cargo_bin.clone(),
				});
			}
			self.app_server_executable = Some(executable);
		}
		Ok(())
	}

	fn finish(self) -> Result<CargoBuildArtifacts, CargoBuildOutputError> {
		let build_entry_executable = self.build_entry_executable.ok_or_else(|| {
			CargoBuildOutputError::ExecutableUnavailable {
				cargo_package: self.build_entry.cargo_package.clone(),
				cargo_bin: self.build_entry.cargo_bin.clone(),
			}
		})?;
		let app_server_executable = self.app_server_executable.ok_or_else(|| {
			CargoBuildOutputError::ExecutableUnavailable {
				cargo_package: self.app_server.cargo_package.clone(),
				cargo_bin: self.app_server.cargo_bin.clone(),
			}
		})?;
		Ok(CargoBuildArtifacts {
			build_entry_executable,
			app_server_executable,
		})
	}
}

fn run_cargo_metadata_command(
	root_dir: &str,
	build_cancel: &Arc<BuildCancel>,
) -> Result<Output, CargoBuildError> {
	let mut command = Command::new("cargo");
	command
		.current_dir(root_dir)
		.args(["metadata", "--no-deps", "--format-version=1"]);
	run_command_collecting_output(command, build_cancel)
}

fn run_command_collecting_output(
	command: Command,
	build_cancel: &Arc<BuildCancel>,
) -> Result<Output, CargoBuildError> {
	crate::command_runner::run_command_collecting_output(
		command,
		build_cancel,
		CommandStderr::Inherit,
	)
	.map_err(cargo_build_error_from_command_error)
}

fn cargo_build_error_from_command_error(error: CommandRunError) -> CargoBuildError {
	match error {
		CommandRunError::CommandStart { source } => CargoBuildError::CommandStart { source },
		CommandRunError::CommandWait { source } => CargoBuildError::CommandWait { source },
		CommandRunError::CommandFailed { output } => CargoBuildError::CommandFailed { output },
		CommandRunError::Cancelled => CargoBuildError::Cancelled,
	}
}

fn cargo_package_names(
	root_dir: &str,
	build_cancel: &Arc<BuildCancel>,
) -> Result<BTreeMap<String, String>, CargoBuildError> {
	let output = run_cargo_metadata_command(root_dir, build_cancel)?;
	cargo_package_names_from_metadata(&output.stdout).map_err(CargoBuildError::CargoOutput)
}

fn cargo_package_names_from_metadata(
	stdout: &[u8],
) -> Result<BTreeMap<String, String>, CargoBuildOutputError> {
	let metadata: CargoMetadata =
		serde_json::from_slice(stdout).map_err(|error| CargoBuildOutputError::InvalidMetadata {
			message: error.to_string(),
		})?;
	Ok(metadata
		.packages
		.into_iter()
		.map(|package| (package.id, package.name))
		.collect())
}

#[cfg(test)]
fn cargo_bin_executables(
	stdout: &[u8],
	package_names: &BTreeMap<String, String>,
	cargo_targets: &[&CargoBinTarget],
) -> Result<BTreeMap<CargoBinTarget, PathBuf>, CargoBuildOutputError> {
	let mut executables = BTreeMap::new();
	for cargo_target in cargo_targets {
		executables.insert(
			(*cargo_target).clone(),
			cargo_bin_executable(stdout, package_names, cargo_target)?,
		);
	}
	Ok(executables)
}

fn cargo_bin_executable(
	stdout: &[u8],
	package_names: &BTreeMap<String, String>,
	cargo_target: &CargoBinTarget,
) -> Result<PathBuf, CargoBuildOutputError> {
	let mut executable = None;
	for line in stdout.split(|byte| *byte == b'\n') {
		if line.is_empty() {
			continue;
		}

		let message: CargoMessage =
			serde_json::from_slice(line).map_err(|error| CargoBuildOutputError::InvalidJson {
				line: String::from_utf8_lossy(line).into_owned(),
				message: error.to_string(),
			})?;
		if message.reason != "compiler-artifact" {
			continue;
		}
		let Some(target) = message.target else {
			continue;
		};
		if target.name != cargo_target.cargo_bin || !target.kind.iter().any(|kind| kind == "bin") {
			continue;
		}
		let Some(package_id) = message.package_id else {
			continue;
		};
		let Some(package_name) = package_names.get(&package_id) else {
			return Err(CargoBuildOutputError::UnknownPackageId { package_id });
		};
		if package_name != &cargo_target.cargo_package {
			continue;
		}
		if let Some(next_executable) = message.executable {
			if executable.is_some() {
				return Err(CargoBuildOutputError::DuplicateExecutable {
					cargo_package: cargo_target.cargo_package.clone(),
					cargo_bin: cargo_target.cargo_bin.clone(),
				});
			}
			executable = Some(next_executable);
		}
	}

	executable.ok_or_else(|| CargoBuildOutputError::ExecutableUnavailable {
		cargo_package: cargo_target.cargo_package.clone(),
		cargo_bin: cargo_target.cargo_bin.clone(),
	})
}

impl fmt::Display for CargoBuildError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::CommandStart { source } => write!(f, "start Cargo build command: {source}"),
			Self::CommandWait { source } => write!(f, "wait for Cargo build command: {source}"),
			Self::CommandFailed { output } => {
				write!(
					f,
					"Cargo build command exited with status {}",
					output.status,
				)
			}
			Self::Cancelled => write!(f, "Cargo build command cancelled"),
			Self::CargoOutput(error) => write!(f, "{error}"),
		}
	}
}

impl std::error::Error for CargoBuildError {}

impl fmt::Display for CargoBuildOutputError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::InvalidBuildArgs(message) => write!(f, "{message}"),
			Self::InvalidJson { line, message } => {
				write!(f, "invalid Cargo JSON message {line:?}: {message}")
			}
			Self::InvalidMetadata { message } => {
				write!(f, "invalid Cargo metadata JSON: {message}")
			}
			Self::UnknownPackageId { package_id } => {
				write!(
					f,
					"Cargo reported compiler artifact for unknown package id {package_id:?}"
				)
			}
			Self::DuplicateExecutable {
				cargo_package,
				cargo_bin,
			} => {
				write!(
					f,
					"Cargo reported multiple executables for package {cargo_package:?} bin {cargo_bin:?}"
				)
			}
			Self::ExecutableUnavailable {
				cargo_package,
				cargo_bin,
			} => {
				write!(
					f,
					"Cargo did not report an executable for package {cargo_package:?} bin {cargo_bin:?}"
				)
			}
		}
	}
}

impl std::error::Error for CargoBuildOutputError {}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;
	use std::process::Command;
	use std::sync::Arc;
	use std::sync::atomic::{AtomicBool, Ordering};

	use crate::build_cancel::BuildCancel;
	use crate::cargo_target::CargoBinTarget;

	use super::{
		CargoBuildOutputError, DevArtifactCollector, cargo_bin_executable, cargo_bin_executables,
		cargo_package_names_from_metadata, run_command_collecting_output,
	};

	#[test]
	fn cargo_bin_executable_extracts_matching_bin_executable() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#app@0.1.0","target":{"kind":["lib"],"name":"app"},"executable":null}
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/server"}
"#;
		let cargo_target = cargo_target("example-app", "server");
		let package_names =
			package_names(&[("path+file:///tmp/example#example-app@0.1.0", "example-app")]);

		let executable = cargo_bin_executable(stdout, &package_names, &cargo_target).unwrap();

		assert_eq!(executable, std::path::PathBuf::from("/tmp/server"));
	}

	#[test]
	fn cargo_bin_executable_uses_metadata_for_path_package_ids_without_names() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/sample-app#0.1.0","target":{"kind":["bin"],"name":"sample-app-build"},"executable":"/tmp/sample-app-build"}
"#;
		let cargo_target = cargo_target("sample-app", "sample-app-build");
		let package_names = package_names(&[("path+file:///tmp/sample-app#0.1.0", "sample-app")]);

		let executable = cargo_bin_executable(stdout, &package_names, &cargo_target).unwrap();

		assert_eq!(
			executable,
			std::path::PathBuf::from("/tmp/sample-app-build")
		);
	}

	#[test]
	fn cargo_bin_executable_reports_missing_bin() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"other"},"executable":"/tmp/other"}
"#;
		let cargo_target = cargo_target("example-app", "server");
		let package_names =
			package_names(&[("path+file:///tmp/example#example-app@0.1.0", "example-app")]);

		let error = cargo_bin_executable(stdout, &package_names, &cargo_target).unwrap_err();

		assert_eq!(
			error,
			CargoBuildOutputError::ExecutableUnavailable {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "server".to_owned(),
			},
		);
	}

	#[test]
	fn cargo_bin_executable_rejects_invalid_json() {
		let package_names = BTreeMap::new();
		let error = cargo_bin_executable(
			b"not json\n",
			&package_names,
			&cargo_target("example-app", "server"),
		)
		.unwrap_err();

		assert!(matches!(error, CargoBuildOutputError::InvalidJson { .. }));
	}

	#[test]
	fn cargo_bin_executable_rejects_unknown_package_ids() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/server"}
"#;
		let package_names = BTreeMap::new();

		let error = cargo_bin_executable(
			stdout,
			&package_names,
			&cargo_target("example-app", "server"),
		)
		.unwrap_err();

		assert_eq!(
			error,
			CargoBuildOutputError::UnknownPackageId {
				package_id: "path+file:///tmp/example#example-app@0.1.0".to_owned(),
			},
		);
	}

	#[test]
	fn cargo_bin_executables_extracts_build_entry_and_app_server() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"build"},"executable":"/tmp/build"}
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/server"}
"#;
		let build = cargo_target("example-app", "build");
		let server = cargo_target("example-app", "server");
		let package_names =
			package_names(&[("path+file:///tmp/example#example-app@0.1.0", "example-app")]);

		let executables =
			cargo_bin_executables(stdout, &package_names, &[&build, &server]).unwrap();

		assert_eq!(executables[&build], std::path::PathBuf::from("/tmp/build"));
		assert_eq!(
			executables[&server],
			std::path::PathBuf::from("/tmp/server"),
		);
	}

	#[test]
	fn cargo_bin_executables_disambiguates_same_bin_name_across_packages() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/build#example-build@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/build-server"}
{"reason":"compiler-artifact","package_id":"path+file:///tmp/app#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/app-server"}
"#;
		let build = cargo_target("example-build", "server");
		let server = cargo_target("example-app", "server");
		let package_names = package_names(&[
			(
				"path+file:///tmp/build#example-build@0.1.0",
				"example-build",
			),
			("path+file:///tmp/app#example-app@0.1.0", "example-app"),
		]);

		let executables =
			cargo_bin_executables(stdout, &package_names, &[&build, &server]).unwrap();

		assert_eq!(
			executables[&build],
			std::path::PathBuf::from("/tmp/build-server")
		);
		assert_eq!(
			executables[&server],
			std::path::PathBuf::from("/tmp/app-server")
		);
	}

	#[test]
	fn dev_artifact_collector_reports_build_entry_before_app_server_finishes() {
		let called = Arc::new(AtomicBool::new(false));
		let called_for_callback = Arc::clone(&called);
		let build = cargo_target("example-build", "server");
		let server = cargo_target("example-app", "server");
		let mut collector = DevArtifactCollector {
			package_names: package_names(&[
				(
					"path+file:///tmp/build#example-build@0.1.0",
					"example-build",
				),
				("path+file:///tmp/app#example-app@0.1.0", "example-app"),
			]),
			build_entry: build,
			app_server: server,
			build_entry_executable: None,
			app_server_executable: None,
			on_build_entry_ready: Some(move |executable| {
				assert_eq!(executable, std::path::PathBuf::from("/tmp/build-server"));
				called_for_callback.store(true, Ordering::SeqCst);
			}),
		};

		collector
            .record_line(
                r#"{"reason":"compiler-artifact","package_id":"path+file:///tmp/build#example-build@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/build-server"}"#,
            )
            .unwrap();

		assert!(called.load(Ordering::SeqCst));
		assert!(collector.app_server_executable.is_none());

		collector
            .record_line(
                r#"{"reason":"compiler-artifact","package_id":"path+file:///tmp/app#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/app-server"}"#,
            )
            .unwrap();
		let artifacts = collector.finish().unwrap();

		assert_eq!(
			artifacts.build_entry_executable,
			std::path::PathBuf::from("/tmp/build-server")
		);
		assert_eq!(
			artifacts.app_server_executable,
			std::path::PathBuf::from("/tmp/app-server")
		);
	}

	#[test]
	fn cargo_bin_executable_rejects_duplicate_bin_artifacts() {
		let stdout = br#"
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/one"}
{"reason":"compiler-artifact","package_id":"path+file:///tmp/example#example-app@0.1.0","target":{"kind":["bin"],"name":"server"},"executable":"/tmp/two"}
"#;
		let cargo_target = cargo_target("example-app", "server");
		let package_names =
			package_names(&[("path+file:///tmp/example#example-app@0.1.0", "example-app")]);

		let error = cargo_bin_executable(stdout, &package_names, &cargo_target).unwrap_err();

		assert_eq!(
			error,
			CargoBuildOutputError::DuplicateExecutable {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "server".to_owned(),
			},
		);
	}

	#[test]
	fn cargo_package_names_from_metadata_maps_exact_package_ids_to_names() {
		let stdout = br#"
{
  "packages": [
    {
      "id": "path+file:///tmp/sample-app#0.1.0",
      "name": "sample-app"
    },
    {
      "id": "path+file:///tmp/other#other-app@0.1.0",
      "name": "other-app"
    }
  ]
}
"#;

		let package_names = cargo_package_names_from_metadata(stdout).unwrap();

		assert_eq!(
			package_names["path+file:///tmp/sample-app#0.1.0"],
			"sample-app"
		);
		assert_eq!(
			package_names["path+file:///tmp/other#other-app@0.1.0"],
			"other-app"
		);
	}

	#[cfg(unix)]
	#[test]
	fn run_command_collecting_output_drains_stdout_while_waiting() {
		let mut command = Command::new("/bin/sh");
		command.arg("-c").arg(
			r#"i=0
while [ "$i" -lt 20000 ]; do
    printf 'stdout-line-%05d\n' "$i"
    i=$((i + 1))
done"#,
		);
		let build_cancel = Arc::new(BuildCancel::new(false));

		let output = run_command_collecting_output(command, &build_cancel).unwrap();

		assert!(output.status.success());
		assert!(output.stdout.len() > 100_000);
		assert!(output.stderr.is_empty());
	}

	fn cargo_target(cargo_package: &str, cargo_bin: &str) -> CargoBinTarget {
		CargoBinTarget {
			cargo_package: cargo_package.to_owned(),
			cargo_bin: cargo_bin.to_owned(),
		}
	}

	fn package_names(ids: &[(&str, &str)]) -> BTreeMap<String, String> {
		ids.iter()
			.map(|(package_id, package_name)| {
				((*package_id).to_owned(), (*package_name).to_owned())
			})
			.collect()
	}
}
