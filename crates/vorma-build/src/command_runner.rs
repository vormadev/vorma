use std::io;
use std::process::{Command, Output, Stdio};
use std::sync::Arc;

use tokio::io::{AsyncRead, AsyncReadExt};

use crate::build_cancel::BuildCancel;
use crate::process_wait::{ChildWaitError, current_thread_runtime, wait_child_or_cancel};
use crate::supervisor::{clear_vorma_runtime_env, prepare_child_process};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum CommandStderr {
	Inherit,
	Collect,
}

#[derive(Debug)]
pub(crate) enum CommandRunError {
	CommandStart { source: io::Error },
	CommandWait { source: io::Error },
	CommandFailed { output: Output },
	Cancelled,
}

pub(crate) fn run_command_inheriting_stdio(
	mut command: Command,
	build_cancel: &Arc<BuildCancel>,
) -> Result<(), CommandRunError> {
	clear_vorma_runtime_env(&mut command);
	prepare_child_process(&mut command);
	command.stdout(Stdio::inherit()).stderr(Stdio::inherit());
	let runtime =
		current_thread_runtime().map_err(|source| CommandRunError::CommandWait { source })?;
	runtime.block_on(async {
		let mut command = tokio::process::Command::from(command);
		let mut child = command
			.spawn()
			.map_err(|source| CommandRunError::CommandStart { source })?;
		let status = match wait_child_or_cancel(&mut child, build_cancel).await {
			Ok(status) => status,
			Err(ChildWaitError::Cancelled) => return Err(CommandRunError::Cancelled),
			Err(ChildWaitError::Wait(source)) => {
				return Err(CommandRunError::CommandWait { source });
			}
		};
		if !status.success() {
			return Err(CommandRunError::CommandFailed {
				output: Output {
					status,
					stdout: Vec::new(),
					stderr: Vec::new(),
				},
			});
		}
		Ok(())
	})
}

pub(crate) fn run_command_collecting_output(
	mut command: Command,
	build_cancel: &Arc<BuildCancel>,
	stderr: CommandStderr,
) -> Result<Output, CommandRunError> {
	clear_vorma_runtime_env(&mut command);
	run_command_collecting_output_preserving_env(command, build_cancel, stderr)
}

pub(crate) fn run_command_collecting_output_preserving_env(
	mut command: Command,
	build_cancel: &Arc<BuildCancel>,
	stderr: CommandStderr,
) -> Result<Output, CommandRunError> {
	prepare_child_process(&mut command);
	command.stdout(Stdio::piped());
	match stderr {
		CommandStderr::Inherit => {
			command.stderr(Stdio::inherit());
		}
		CommandStderr::Collect => {
			command.stderr(Stdio::piped());
		}
	}
	let runtime =
		current_thread_runtime().map_err(|source| CommandRunError::CommandWait { source })?;
	runtime.block_on(async {
		let mut command = tokio::process::Command::from(command);
		let mut child = command
			.spawn()
			.map_err(|source| CommandRunError::CommandStart { source })?;
		let stdout = child
			.stdout
			.take()
			.ok_or_else(|| CommandRunError::CommandWait {
				source: io::Error::other("missing command stdout pipe"),
			})?;
		let stderr = match stderr {
			CommandStderr::Inherit => None,
			CommandStderr::Collect => {
				Some(
					child
						.stderr
						.take()
						.ok_or_else(|| CommandRunError::CommandWait {
							source: io::Error::other("missing command stderr pipe"),
						})?,
				)
			}
		};
		let stdout_thread = tokio::spawn(read_pipe(stdout));
		let stderr_thread = stderr.map(|stderr| tokio::spawn(read_pipe(stderr)));
		let status = match wait_child_or_cancel(&mut child, build_cancel).await {
			Ok(status) => status,
			Err(ChildWaitError::Cancelled) => {
				let _ = join_pipe(stdout_thread).await;
				if let Some(stderr_thread) = stderr_thread {
					let _ = join_pipe(stderr_thread).await;
				}
				return Err(CommandRunError::Cancelled);
			}
			Err(ChildWaitError::Wait(source)) => {
				return Err(CommandRunError::CommandWait { source });
			}
		};

		let stdout = join_pipe(stdout_thread).await?;
		let stderr = match stderr_thread {
			Some(stderr_thread) => join_pipe(stderr_thread).await?,
			None => Vec::new(),
		};
		let output = Output {
			status,
			stdout,
			stderr,
		};

		if !output.status.success() {
			return Err(CommandRunError::CommandFailed { output });
		}
		Ok(output)
	})
}

async fn read_pipe<R>(mut reader: R) -> io::Result<Vec<u8>>
where
	R: AsyncRead + Send + Unpin + 'static,
{
	let mut output = Vec::new();
	reader.read_to_end(&mut output).await?;
	Ok(output)
}

async fn join_pipe(
	join_handle: tokio::task::JoinHandle<io::Result<Vec<u8>>>,
) -> Result<Vec<u8>, CommandRunError> {
	join_handle
		.await
		.map_err(|_| CommandRunError::CommandWait {
			source: io::Error::other("command pipe reader panicked"),
		})?
		.map_err(|source| CommandRunError::CommandWait { source })
}

impl std::fmt::Display for CommandRunError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::CommandStart { source } => write!(f, "start command: {source}"),
			Self::CommandWait { source } => write!(f, "wait for command: {source}"),
			Self::CommandFailed { output } => {
				write!(f, "command exited with status {}", output.status)
			}
			Self::Cancelled => write!(f, "command cancelled"),
		}
	}
}

impl std::error::Error for CommandRunError {}
