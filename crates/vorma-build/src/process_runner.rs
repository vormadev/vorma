//! Process command boundary for build orchestration.

use std::collections::BTreeMap;
use std::future;
use std::io;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread;
use std::time::{Duration, Instant};

use tokio::io::{AsyncBufReadExt, AsyncReadExt, BufReader};
use tokio::sync::watch;
use vorma_contract::constants::{ENV_KEY_IS_BUILD, ENV_KEY_IS_DEV};

use crate::live_state::LIVE_BUILD_STATE_ENV_KEY;
use crate::vite_plugin_contract::{
	VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY,
};

const PROCESS_TERMINATION_GRACE: Duration = Duration::from_secs(2);

/// Cancellation signal for one in-flight build process operation.
#[derive(Clone, Debug)]
pub struct BuildProcessCancel {
	inner: Arc<BuildProcessCancelInner>,
}

#[derive(Debug)]
struct BuildProcessCancelInner {
	cancelled: AtomicBool,
	cancel_tx: watch::Sender<bool>,
}

impl BuildProcessCancel {
	/// Create a non-cancelled process cancellation signal.
	pub fn new() -> Self {
		let (cancel_tx, _) = watch::channel(false);
		Self {
			inner: Arc::new(BuildProcessCancelInner {
				cancelled: AtomicBool::new(false),
				cancel_tx,
			}),
		}
	}

	/// Mark this process operation as cancelled.
	pub fn cancel(&self) {
		self.inner.cancelled.store(true, Ordering::SeqCst);
		self.inner.cancel_tx.send_replace(true);
	}

	/// Whether cancellation has been requested.
	pub fn is_cancelled(&self) -> bool {
		self.inner.cancelled.load(Ordering::SeqCst)
	}

	fn subscribe(&self) -> watch::Receiver<bool> {
		self.inner.cancel_tx.subscribe()
	}
}

impl Default for BuildProcessCancel {
	fn default() -> Self {
		Self::new()
	}
}

/// Child process command prepared by build orchestration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuildProcessCommand {
	program: String,
	args: Vec<String>,
	current_dir: PathBuf,
	env: BTreeMap<String, String>,
	clear_vorma_runtime_env: bool,
}

impl BuildProcessCommand {
	/// Create a child process command.
	pub fn new(
		program: impl Into<String>,
		args: Vec<String>,
		current_dir: impl Into<PathBuf>,
	) -> Self {
		Self {
			program: program.into(),
			args,
			current_dir: current_dir.into(),
			env: BTreeMap::new(),
			clear_vorma_runtime_env: true,
		}
	}

	/// Attach an environment variable.
	pub fn with_env(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
		self.env.insert(key.into(), value.into());
		self
	}

	/// Program executable.
	pub fn program(&self) -> &str {
		&self.program
	}

	/// Program arguments.
	pub fn args(&self) -> &[String] {
		&self.args
	}

	/// Child process working directory.
	pub fn current_dir(&self) -> &Path {
		&self.current_dir
	}

	/// Explicit environment variables.
	pub fn env(&self) -> &BTreeMap<String, String> {
		&self.env
	}
}

/// Build process runner abstraction.
pub trait BuildProcessRunner {
	/// Run the command while inheriting stdin/stdout/stderr.
	fn run_inheriting_stdio(
		&mut self,
		command: &BuildProcessCommand,
	) -> Result<(), BuildProcessError>;

	/// Start the command while inheriting stdin/stdout/stderr.
	fn start_inheriting_stdio(
		&mut self,
		command: &BuildProcessCommand,
	) -> Result<Box<dyn StartedBuildProcess + Send>, BuildProcessError> {
		Err(BuildProcessError::StartUnsupported {
			program: command.program().to_owned(),
		})
	}

	/// Run the command to completion while collecting stdout and stderr.
	fn run_collecting_output(
		&mut self,
		command: &BuildProcessCommand,
	) -> Result<BuildProcessOutput, BuildProcessError> {
		Err(BuildProcessError::OutputUnsupported {
			program: command.program().to_owned(),
		})
	}

	/// Run the command to completion while collecting stdout/stderr, terminating the
	/// process group when cancellation is requested.
	fn run_collecting_output_until_cancelled(
		&mut self,
		command: &BuildProcessCommand,
		cancel: &BuildProcessCancel,
	) -> Result<BuildProcessOutput, BuildProcessError> {
		if cancel.is_cancelled() {
			return Err(BuildProcessError::Cancelled {
				program: command.program().to_owned(),
			});
		}
		self.run_collecting_output(command)
	}

	/// Run the command like [`Self::run_collecting_output_until_cancelled`], also
	/// invoking `on_stdout_line` for each stdout line as soon as it is read so
	/// callers can react to process output while the process is still running.
	fn run_streaming_stdout_lines_until_cancelled(
		&mut self,
		command: &BuildProcessCommand,
		cancel: &BuildProcessCancel,
		on_stdout_line: &mut dyn FnMut(&str),
	) -> Result<BuildProcessOutput, BuildProcessError> {
		let output = self.run_collecting_output_until_cancelled(command, cancel)?;
		for line in String::from_utf8_lossy(output.stdout()).lines() {
			on_stdout_line(line);
		}
		Ok(output)
	}
}

/// Completed child process output.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BuildProcessOutput {
	status_success: bool,
	status: String,
	stdout: Vec<u8>,
	stderr: Vec<u8>,
}

impl BuildProcessOutput {
	/// Create collected child process output.
	pub fn new(
		status_success: bool,
		status: impl Into<String>,
		stdout: Vec<u8>,
		stderr: Vec<u8>,
	) -> Self {
		Self {
			status_success,
			status: status.into(),
			stdout,
			stderr,
		}
	}

	/// Whether the child exited successfully.
	pub fn status_success(&self) -> bool {
		self.status_success
	}

	/// Displayable child exit status.
	pub fn status(&self) -> &str {
		&self.status
	}

	/// Captured standard output bytes.
	pub fn stdout(&self) -> &[u8] {
		&self.stdout
	}

	/// Captured standard error bytes.
	pub fn stderr(&self) -> &[u8] {
		&self.stderr
	}
}

/// Started child process owned by dev orchestration.
pub trait StartedBuildProcess {
	/// Operating-system process identifier when available.
	#[cfg(test)]
	fn process_id(&self) -> Option<u32>;

	/// Child process exit status if it has already exited.
	fn exit_status(&mut self) -> Result<Option<String>, BuildProcessError> {
		Ok(None)
	}

	/// Terminate the process if it is still running.
	fn terminate(&mut self) -> Result<(), BuildProcessError>;

	/*
	Default no-op keeps test doubles trivial; only the std implementation
	watches real processes (and only on unix — see the impl note).
	*/
	/// Invoke `on_exit` if the process exits before `terminate` is called.
	fn watch_unexpected_exit(&mut self, on_exit: Box<dyn FnOnce() + Send>) {
		let _ = on_exit;
	}
}

/// Standard process runner.
#[derive(Clone, Copy, Debug, Default)]
pub struct StdBuildProcessRunner;

impl BuildProcessRunner for StdBuildProcessRunner {
	fn run_inheriting_stdio(
		&mut self,
		command: &BuildProcessCommand,
	) -> Result<(), BuildProcessError> {
		let status =
			std_command(command, false)
				.status()
				.map_err(|source| BuildProcessError::Start {
					program: command.program().to_owned(),
					message: source.to_string(),
				})?;
		if !status.success() {
			return Err(BuildProcessError::Failed {
				program: command.program().to_owned(),
				status: status.to_string(),
			});
		}
		Ok(())
	}

	fn start_inheriting_stdio(
		&mut self,
		command: &BuildProcessCommand,
	) -> Result<Box<dyn StartedBuildProcess + Send>, BuildProcessError> {
		let child =
			std_command(command, true)
				.spawn()
				.map_err(|source| BuildProcessError::Start {
					program: command.program().to_owned(),
					message: source.to_string(),
				})?;
		Ok(Box::new(StdStartedBuildProcess {
			program: command.program().to_owned(),
			child: Some(child),
			terminating: std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false)),
		}))
	}

	fn run_collecting_output(
		&mut self,
		command: &BuildProcessCommand,
	) -> Result<BuildProcessOutput, BuildProcessError> {
		let output = std_command(command, false)
			.stdout(Stdio::piped())
			.stderr(Stdio::piped())
			.output()
			.map_err(|source| BuildProcessError::Start {
				program: command.program().to_owned(),
				message: source.to_string(),
			})?;
		Ok(BuildProcessOutput::new(
			output.status.success(),
			output.status.to_string(),
			output.stdout,
			output.stderr,
		))
	}

	fn run_collecting_output_until_cancelled(
		&mut self,
		command: &BuildProcessCommand,
		cancel: &BuildProcessCancel,
	) -> Result<BuildProcessOutput, BuildProcessError> {
		if cancel.is_cancelled() {
			return Err(BuildProcessError::Cancelled {
				program: command.program().to_owned(),
			});
		}
		let runtime = tokio::runtime::Builder::new_current_thread()
			.enable_io()
			.enable_time()
			.build()
			.map_err(|source| BuildProcessError::Start {
				program: command.program().to_owned(),
				message: source.to_string(),
			})?;
		runtime.block_on(run_collecting_output_until_cancelled(command, cancel))
	}

	fn run_streaming_stdout_lines_until_cancelled(
		&mut self,
		command: &BuildProcessCommand,
		cancel: &BuildProcessCancel,
		on_stdout_line: &mut dyn FnMut(&str),
	) -> Result<BuildProcessOutput, BuildProcessError> {
		if cancel.is_cancelled() {
			return Err(BuildProcessError::Cancelled {
				program: command.program().to_owned(),
			});
		}
		let runtime = tokio::runtime::Builder::new_current_thread()
			.enable_io()
			.enable_time()
			.build()
			.map_err(|source| BuildProcessError::Start {
				program: command.program().to_owned(),
				message: source.to_string(),
			})?;
		runtime.block_on(run_streaming_stdout_lines_until_cancelled(
			command,
			cancel,
			on_stdout_line,
		))
	}
}

#[derive(Debug)]
struct StdStartedBuildProcess {
	program: String,
	child: Option<Child>,
	terminating: std::sync::Arc<std::sync::atomic::AtomicBool>,
}

impl StartedBuildProcess for StdStartedBuildProcess {
	#[cfg(test)]
	fn process_id(&self) -> Option<u32> {
		self.child.as_ref().map(Child::id)
	}

	fn exit_status(&mut self) -> Result<Option<String>, BuildProcessError> {
		let Some(child) = self.child.as_mut() else {
			return Ok(None);
		};
		child
			.try_wait()
			.map(|status| status.map(|status| status.to_string()))
			.map_err(|source| BuildProcessError::Wait {
				program: self.program.clone(),
				message: source.to_string(),
			})
	}

	fn watch_unexpected_exit(&mut self, on_exit: Box<dyn FnOnce() + Send>) {
		self.watch_unexpected_exit_impl(on_exit);
	}

	fn terminate(&mut self) -> Result<(), BuildProcessError> {
		/*
		Mark intent before any kill so a concurrently-firing exit watcher
		never reports an intentional termination as unexpected.
		*/
		self.terminating
			.store(true, std::sync::atomic::Ordering::SeqCst);
		let Some(child) = self.child.as_mut() else {
			return Ok(());
		};
		if child
			.try_wait()
			.map_err(|source| BuildProcessError::Terminate {
				program: self.program.clone(),
				message: source.to_string(),
			})?
			.is_some()
		{
			self.child = None;
			return Ok(());
		}
		terminate_started_child(&self.program, child)?;
		self.child = None;
		Ok(())
	}
}

impl StdStartedBuildProcess {
	/*
	The watcher peeks at exit via `waitid(..., WNOWAIT)` so the child is
	NOT reaped here: `terminate`/Drop keep sole reaping responsibility, and
	an unexpected exit leaves the zombie for process teardown to collect.
	Windows has no watcher yet (needs a SYNCHRONIZE-handle wait; tracked in
	GO_PARITY_AUDIT finding 6) — dev there behaves as before this monitor.
	*/
	#[cfg(unix)]
	fn watch_unexpected_exit_impl(&mut self, on_exit: Box<dyn FnOnce() + Send>) {
		let Some(child) = self.child.as_ref() else {
			return;
		};
		let pid = child.id();
		let terminating = std::sync::Arc::clone(&self.terminating);
		std::thread::spawn(move || {
			wait_for_unix_process_exit(pid);
			if !terminating.load(std::sync::atomic::Ordering::SeqCst) {
				on_exit();
			}
		});
	}

	#[cfg(windows)]
	fn watch_unexpected_exit_impl(&mut self, _on_exit: Box<dyn FnOnce() + Send>) {}
}

/*
Both arms observe exit WITHOUT reaping, so `terminate`/Drop keep sole
reaping responsibility and the pid stays valid for their kill path.
*/
#[cfg(target_os = "linux")]
fn wait_for_unix_process_exit(pid: u32) {
	use nix::sys::wait::{Id, WaitPidFlag, waitid};

	let id = Id::Pid(nix::unistd::Pid::from_raw(pid as i32));
	loop {
		match waitid(id, WaitPidFlag::WEXITED | WaitPidFlag::WNOWAIT) {
			Ok(_) => return,
			Err(nix::errno::Errno::EINTR) => continue,
			/*
			ECHILD and any other failure mean the process is already gone
			or unobservable; either way it is no longer running normally.
			*/
			Err(_) => return,
		}
	}
}

/*
macOS has the waitid(2) syscall but nix gates its wrapper to other
platforms; kqueue's EVFILT_PROC/NOTE_EXIT is the canonical macOS way to
observe process exit and never touches wait-family reaping at all.
*/
#[cfg(not(target_os = "linux"))]
fn wait_for_unix_process_exit(pid: u32) {
	use nix::sys::event::{EventFilter, EventFlag, FilterFlag, KEvent, Kqueue};

	let Ok(kqueue) = Kqueue::new() else {
		return;
	};
	let registration = KEvent::new(
		pid as usize,
		EventFilter::EVFILT_PROC,
		EventFlag::EV_ADD | EventFlag::EV_ONESHOT,
		FilterFlag::NOTE_EXIT,
		0,
		0,
	);
	let mut events = [KEvent::new(
		0,
		EventFilter::EVFILT_PROC,
		EventFlag::empty(),
		FilterFlag::empty(),
		0,
		0,
	)];
	loop {
		match kqueue.kevent(&[registration], &mut events, None) {
			/*
			A NOTE_EXIT delivery or a registration error (EV_ERROR with
			ESRCH: the process is already gone) both mean it is no longer
			running.
			*/
			Ok(_) => return,
			Err(nix::errno::Errno::EINTR) => continue,
			Err(_) => return,
		}
	}
}

impl Drop for StdStartedBuildProcess {
	fn drop(&mut self) {
		let _ = self.terminate();
	}
}

/// Build process execution error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum BuildProcessError {
	/// Child process could not be started.
	Start {
		/// Program executable.
		program: String,
		/// Filesystem/process error message.
		message: String,
	},
	/// Child process returned a failing exit status.
	Failed {
		/// Program executable.
		program: String,
		/// Exit status.
		status: String,
	},
	/// Runner cannot start long-lived processes.
	StartUnsupported {
		/// Program executable.
		program: String,
	},
	/// Runner cannot collect output from short-lived processes.
	OutputUnsupported {
		/// Program executable.
		program: String,
	},
	/// Process execution was cancelled.
	Cancelled {
		/// Program executable.
		program: String,
	},
	/// Child process could not be terminated.
	Wait {
		/// Program executable.
		program: String,
		/// Filesystem/process error message.
		message: String,
	},
	/// Child process could not be terminated.
	Terminate {
		/// Program executable.
		program: String,
		/// Filesystem/process error message.
		message: String,
	},
}

impl std::fmt::Display for BuildProcessError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::Start { program, message } => {
				write!(f, "start command {program:?}: {message}")
			}
			Self::Failed { program, status } => {
				write!(f, "command {program:?} exited with {status}")
			}
			Self::StartUnsupported { program } => {
				write!(f, "runner cannot start long-lived command {program:?}")
			}
			Self::OutputUnsupported { program } => {
				write!(f, "runner cannot collect output from command {program:?}")
			}
			Self::Cancelled { program } => {
				write!(f, "command {program:?} cancelled")
			}
			Self::Wait { program, message } => {
				write!(f, "wait for command {program:?}: {message}")
			}
			Self::Terminate { program, message } => {
				write!(f, "terminate command {program:?}: {message}")
			}
		}
	}
}

impl std::error::Error for BuildProcessError {}

pub(crate) fn command_base_parts(command: &str) -> Option<Vec<String>> {
	let parts = command
		.split_whitespace()
		.map(str::to_owned)
		.collect::<Vec<_>>();
	if parts.is_empty() {
		return None;
	}
	Some(parts)
}

fn std_command(command: &BuildProcessCommand, new_process_group: bool) -> Command {
	let mut child = Command::new(command.program());
	child
		.args(command.args())
		.current_dir(command.current_dir())
		.stdin(Stdio::inherit())
		.stdout(Stdio::inherit())
		.stderr(Stdio::inherit());
	if command.clear_vorma_runtime_env {
		child
			.env_remove(ENV_KEY_IS_BUILD)
			.env_remove(ENV_KEY_IS_DEV)
			.env_remove(LIVE_BUILD_STATE_ENV_KEY)
			.env_remove(VITE_PLUGIN_SERVER_PORT_ENV_KEY)
			.env_remove(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY);
	}
	for (key, value) in command.env() {
		child.env(key, value);
	}
	configure_process_group(&mut child, new_process_group);
	child
}

async fn run_collecting_output_until_cancelled(
	command: &BuildProcessCommand,
	cancel: &BuildProcessCancel,
) -> Result<BuildProcessOutput, BuildProcessError> {
	let program = command.program().to_owned();
	let mut std_command = std_command(command, true);
	std_command.stdout(Stdio::piped()).stderr(Stdio::piped());
	let mut child = tokio::process::Command::from(std_command)
		.spawn()
		.map_err(|source| BuildProcessError::Start {
			program: program.clone(),
			message: source.to_string(),
		})?;
	let stdout = child
		.stdout
		.take()
		.ok_or_else(|| BuildProcessError::Start {
			program: program.clone(),
			message: "stdout pipe was not captured".to_owned(),
		})?;
	let stderr = child
		.stderr
		.take()
		.ok_or_else(|| BuildProcessError::Start {
			program: program.clone(),
			message: "stderr pipe was not captured".to_owned(),
		})?;
	let stdout_task = tokio::spawn(read_async_pipe(stdout));
	let stderr_task = tokio::spawn(read_async_pipe(stderr));
	let mut cancel_rx = cancel.subscribe();
	tokio::select! {
		status = child.wait() => {
			let status = status.map_err(|source| BuildProcessError::Wait {
				program: program.clone(),
				message: source.to_string(),
			})?;
			let stdout = join_async_pipe(&program, stdout_task).await?;
			let stderr = join_async_pipe(&program, stderr_task).await?;
			Ok(BuildProcessOutput::new(
				status.success(),
				status.to_string(),
				stdout,
				stderr,
			))
		}
		_ = wait_for_process_cancel(&mut cancel_rx) => {
			cancel_collecting_child_process(&program, &mut child).await?;
			let _ = stdout_task.await;
			let _ = stderr_task.await;
			Err(BuildProcessError::Cancelled { program })
		}
	}
}

/*
Stdout is consumed line-by-line so callers can react to process output (cargo
artifact messages) while the process is still running; stderr stays on a
background task so neither pipe can block the other.
*/
async fn run_streaming_stdout_lines_until_cancelled(
	command: &BuildProcessCommand,
	cancel: &BuildProcessCancel,
	on_stdout_line: &mut dyn FnMut(&str),
) -> Result<BuildProcessOutput, BuildProcessError> {
	let program = command.program().to_owned();
	let mut std_command = std_command(command, true);
	std_command.stdout(Stdio::piped()).stderr(Stdio::piped());
	let mut child = tokio::process::Command::from(std_command)
		.spawn()
		.map_err(|source| BuildProcessError::Start {
			program: program.clone(),
			message: source.to_string(),
		})?;
	let stdout = child
		.stdout
		.take()
		.ok_or_else(|| BuildProcessError::Start {
			program: program.clone(),
			message: "stdout pipe was not captured".to_owned(),
		})?;
	let stderr = child
		.stderr
		.take()
		.ok_or_else(|| BuildProcessError::Start {
			program: program.clone(),
			message: "stderr pipe was not captured".to_owned(),
		})?;
	let stderr_task = tokio::spawn(read_async_pipe(stderr));
	let mut cancel_rx = cancel.subscribe();
	let mut stdout_reader = BufReader::new(stdout);
	let mut stdout_bytes = Vec::new();
	let mut line_bytes = Vec::new();
	loop {
		line_bytes.clear();
		let read = tokio::select! {
			read = stdout_reader.read_until(b'\n', &mut line_bytes) => {
				read.map_err(|source| BuildProcessError::Wait {
					program: program.clone(),
					message: source.to_string(),
				})?
			}
			_ = wait_for_process_cancel(&mut cancel_rx) => {
				cancel_collecting_child_process(&program, &mut child).await?;
				let _ = stderr_task.await;
				return Err(BuildProcessError::Cancelled { program });
			}
		};
		if read == 0 {
			break;
		}
		stdout_bytes.extend_from_slice(&line_bytes);
		let line = String::from_utf8_lossy(&line_bytes);
		on_stdout_line(line.trim_end_matches(['\r', '\n']));
	}
	let status = tokio::select! {
		status = child.wait() => {
			status.map_err(|source| BuildProcessError::Wait {
				program: program.clone(),
				message: source.to_string(),
			})?
		}
		_ = wait_for_process_cancel(&mut cancel_rx) => {
			cancel_collecting_child_process(&program, &mut child).await?;
			let _ = stderr_task.await;
			return Err(BuildProcessError::Cancelled { program });
		}
	};
	let stderr = join_async_pipe(&program, stderr_task).await?;
	Ok(BuildProcessOutput::new(
		status.success(),
		status.to_string(),
		stdout_bytes,
		stderr,
	))
}

async fn read_async_pipe<R>(mut pipe: R) -> Result<Vec<u8>, io::Error>
where
	R: tokio::io::AsyncRead + Unpin,
{
	let mut bytes = Vec::new();
	pipe.read_to_end(&mut bytes).await?;
	Ok(bytes)
}

async fn join_async_pipe(
	program: &str,
	task: tokio::task::JoinHandle<Result<Vec<u8>, io::Error>>,
) -> Result<Vec<u8>, BuildProcessError> {
	task.await
		.map_err(|source| BuildProcessError::Wait {
			program: program.to_owned(),
			message: source.to_string(),
		})?
		.map_err(|source| BuildProcessError::Wait {
			program: program.to_owned(),
			message: source.to_string(),
		})
}

async fn wait_for_process_cancel(cancel_rx: &mut watch::Receiver<bool>) {
	loop {
		if *cancel_rx.borrow_and_update() {
			return;
		}
		if cancel_rx.changed().await.is_err() {
			future::pending::<()>().await;
		}
	}
}

async fn cancel_collecting_child_process(
	program: &str,
	child: &mut tokio::process::Child,
) -> Result<(), BuildProcessError> {
	let Some(process_id) = child.id() else {
		return Ok(());
	};
	if let Err(error) = request_process_group_termination(program, process_id) {
		// The child may have exited between spawn and the signal.
		return match child.try_wait() {
			Ok(Some(_)) => Ok(()),
			_ => Err(error),
		};
	}
	match tokio::time::timeout(PROCESS_TERMINATION_GRACE, child.wait()).await {
		Ok(Ok(_)) => Ok(()),
		Ok(Err(source)) => Err(BuildProcessError::Wait {
			program: program.to_owned(),
			message: source.to_string(),
		}),
		Err(_) => {
			if let Err(error) = force_process_group_termination(program, process_id)
				&& !matches!(child.try_wait(), Ok(Some(_)))
			{
				return Err(error);
			}
			child
				.wait()
				.await
				.map_err(|source| BuildProcessError::Wait {
					program: program.to_owned(),
					message: source.to_string(),
				})?;
			Ok(())
		}
	}
}

#[cfg(unix)]
fn configure_process_group(command: &mut Command, new_process_group: bool) {
	if new_process_group {
		use std::os::unix::process::CommandExt;

		command.process_group(0);
	}
}

#[cfg(windows)]
fn configure_process_group(command: &mut Command, new_process_group: bool) {
	if new_process_group {
		use std::os::windows::process::CommandExt;

		const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;
		command.creation_flags(CREATE_NEW_PROCESS_GROUP);
	}
}

#[cfg(not(any(unix, windows)))]
fn configure_process_group(_command: &mut Command, _new_process_group: bool) {}

fn terminate_started_child(program: &str, child: &mut Child) -> Result<(), BuildProcessError> {
	terminate_started_child_platform(program, child)?;
	wait_for_child_exit(program, child, PROCESS_TERMINATION_GRACE)
}

#[cfg(unix)]
fn terminate_started_child_platform(
	program: &str,
	child: &mut Child,
) -> Result<(), BuildProcessError> {
	if child_already_exited(child) {
		return Ok(());
	}
	if let Err(error) = request_process_group_termination(program, child.id()) {
		// The child may have exited between the liveness check and the signal.
		return if child_already_exited(child) {
			Ok(())
		} else {
			Err(error)
		};
	}
	if wait_for_child_exit(program, child, PROCESS_TERMINATION_GRACE).is_ok() {
		return Ok(());
	}
	if let Err(error) = force_process_group_termination(program, child.id()) {
		return if child_already_exited(child) {
			Ok(())
		} else {
			Err(error)
		};
	}
	Ok(())
}

#[cfg(any(unix, windows))]
fn child_already_exited(child: &mut Child) -> bool {
	matches!(child.try_wait(), Ok(Some(_)))
}

#[cfg(unix)]
fn request_process_group_termination(
	program: &str,
	process_group_id: u32,
) -> Result<(), BuildProcessError> {
	send_unix_signal_to_process_group(program, nix::sys::signal::Signal::SIGTERM, process_group_id)
}

#[cfg(unix)]
fn force_process_group_termination(
	program: &str,
	process_group_id: u32,
) -> Result<(), BuildProcessError> {
	send_unix_signal_to_process_group(program, nix::sys::signal::Signal::SIGKILL, process_group_id)
}

#[cfg(unix)]
fn send_unix_signal_to_process_group(
	program: &str,
	signal: nix::sys::signal::Signal,
	process_group_id: u32,
) -> Result<(), BuildProcessError> {
	let process_group_id =
		i32::try_from(process_group_id).map_err(|_| BuildProcessError::Terminate {
			program: program.to_owned(),
			message: format!("process group id {process_group_id} exceeds pid_t"),
		})?;
	nix::sys::signal::killpg(nix::unistd::Pid::from_raw(process_group_id), signal).map_err(
		|errno| BuildProcessError::Terminate {
			program: program.to_owned(),
			message: format!("killpg({process_group_id}, {signal}): {errno}"),
		},
	)
}

#[cfg(windows)]
fn terminate_started_child_platform(
	program: &str,
	child: &mut Child,
) -> Result<(), BuildProcessError> {
	if child_already_exited(child) {
		return Ok(());
	}
	if let Err(error) = request_process_group_termination(program, child.id()) {
		// The child may have exited between the liveness check and the signal.
		return if child_already_exited(child) {
			Ok(())
		} else {
			Err(error)
		};
	}
	Ok(())
}

#[cfg(windows)]
fn request_process_group_termination(
	program: &str,
	process_id: u32,
) -> Result<(), BuildProcessError> {
	let status = Command::new("taskkill")
		.args(["/PID", &process_id.to_string(), "/T", "/F"])
		.status()
		.map_err(|source| BuildProcessError::Terminate {
			program: program.to_owned(),
			message: source.to_string(),
		})?;
	if !status.success() {
		return Err(BuildProcessError::Terminate {
			program: program.to_owned(),
			message: format!("taskkill exited with {status}"),
		});
	}
	Ok(())
}

#[cfg(windows)]
fn force_process_group_termination(
	program: &str,
	process_id: u32,
) -> Result<(), BuildProcessError> {
	request_process_group_termination(program, process_id)
}

#[cfg(not(any(unix, windows)))]
fn terminate_started_child_platform(
	program: &str,
	child: &mut Child,
) -> Result<(), BuildProcessError> {
	child.kill().map_err(|source| BuildProcessError::Terminate {
		program: program.to_owned(),
		message: source.to_string(),
	})
}

#[cfg(not(any(unix, windows)))]
fn request_process_group_termination(
	program: &str,
	_process_id: u32,
) -> Result<(), BuildProcessError> {
	Err(BuildProcessError::Terminate {
		program: program.to_owned(),
		message: "process-group termination is unsupported on this platform".to_owned(),
	})
}

#[cfg(not(any(unix, windows)))]
fn force_process_group_termination(
	program: &str,
	process_id: u32,
) -> Result<(), BuildProcessError> {
	request_process_group_termination(program, process_id)
}

fn wait_for_child_exit(
	program: &str,
	child: &mut Child,
	timeout: Duration,
) -> Result<(), BuildProcessError> {
	let deadline = Instant::now() + timeout;
	loop {
		if child
			.try_wait()
			.map_err(|source| BuildProcessError::Wait {
				program: program.to_owned(),
				message: source.to_string(),
			})?
			.is_some()
		{
			return Ok(());
		}
		if Instant::now() >= deadline {
			return Err(BuildProcessError::Terminate {
				program: program.to_owned(),
				message: "child did not exit before termination grace elapsed".to_owned(),
			});
		}
		thread::sleep(Duration::from_millis(10));
	}
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::sync::mpsc;
	use std::time::{Duration, SystemTime, UNIX_EPOCH};

	use super::*;

	#[test]
	fn command_base_parts_rejects_empty_commands() {
		assert_eq!(command_base_parts("   "), None);
		assert_eq!(
			command_base_parts("pnpm exec vite").unwrap(),
			["pnpm", "exec", "vite"]
		);
	}

	#[cfg(unix)]
	#[test]
	fn std_runner_streams_stdout_lines_and_collects_full_output() {
		let command = BuildProcessCommand::new(
			"sh",
			vec![
				"-c".to_owned(),
				"printf 'one\\ntwo\\n'; printf 'err' 1>&2".to_owned(),
			],
			".",
		);
		let mut lines = Vec::new();

		let output = StdBuildProcessRunner
			.run_streaming_stdout_lines_until_cancelled(
				&command,
				&BuildProcessCancel::new(),
				&mut |line| lines.push(line.to_owned()),
			)
			.unwrap();

		assert_eq!(lines, ["one", "two"]);
		assert!(output.status_success());
		assert_eq!(output.stdout(), b"one\ntwo\n");
		assert_eq!(output.stderr(), b"err");
	}

	#[test]
	fn std_command_clears_vorma_runtime_env_then_applies_explicit_env() {
		let command = BuildProcessCommand::new("example", Vec::new(), ".")
			.with_env(LIVE_BUILD_STATE_ENV_KEY, "1");

		let child = std_command(&command, false);

		let mut removed = Vec::new();
		let mut explicit = Vec::new();
		for (key, value) in child.get_envs() {
			match value {
				None => removed.push(key.to_os_string()),
				Some(value) => explicit.push((key.to_os_string(), value.to_os_string())),
			}
		}
		for cleared_key in [
			ENV_KEY_IS_BUILD,
			ENV_KEY_IS_DEV,
			VITE_PLUGIN_SERVER_PORT_ENV_KEY,
			VITE_PLUGIN_SERVER_TOKEN_ENV_KEY,
		] {
			assert!(removed.iter().any(|key| key == cleared_key));
		}
		assert!(
			explicit
				.iter()
				.any(|(key, value)| key == LIVE_BUILD_STATE_ENV_KEY && value == "1")
		);
	}

	#[cfg(unix)]
	#[test]
	fn exit_watcher_fires_for_unexpected_exit_and_not_for_termination() {
		let mut runner = StdBuildProcessRunner;

		/*
		Natural exit: the watcher must fire exactly once after the short
		command finishes on its own.
		*/
		let (exit_tx, exit_rx) = mpsc::channel();
		let mut short_lived = runner
			.start_inheriting_stdio(&BuildProcessCommand::new(
				"sh",
				vec!["-c".to_owned(), "exit 0".to_owned()],
				std::env::temp_dir(),
			))
			.unwrap();
		short_lived.watch_unexpected_exit(Box::new(move || {
			let _ = exit_tx.send(());
		}));
		exit_rx
			.recv_timeout(Duration::from_secs(10))
			.expect("watcher must report a natural exit");

		/*
		Intentional termination: the terminate-intent flag must suppress
		the callback even though the process exits.
		*/
		let (term_tx, term_rx) = mpsc::channel::<()>();
		let mut long_lived = runner
			.start_inheriting_stdio(&BuildProcessCommand::new(
				"sh",
				vec!["-c".to_owned(), "sleep 30".to_owned()],
				std::env::temp_dir(),
			))
			.unwrap();
		long_lived.watch_unexpected_exit(Box::new(move || {
			let _ = term_tx.send(());
		}));
		long_lived.terminate().unwrap();
		/*
		Either outcome proves suppression: Timeout (watcher still parked) or
		Disconnected (watcher observed the exit, saw the terminate flag, and
		dropped the callback unfired). Only an Ok(()) message is a failure.
		*/
		assert!(
			term_rx.recv_timeout(Duration::from_millis(500)).is_err(),
			"terminated processes must not be reported as unexpected exits"
		);
	}

	#[cfg(unix)]
	#[test]
	fn started_process_termination_kills_process_group_grandchildren() {
		let pid_file = std::env::temp_dir().join(format!(
			"vorma-process-runner-grandchild-{}",
			SystemTime::now()
				.duration_since(UNIX_EPOCH)
				.unwrap()
				.as_nanos()
		));
		let script = format!(
			"sleep 30 & echo $! > {}; wait",
			shell_quote(pid_file.to_str().unwrap())
		);
		let command =
			BuildProcessCommand::new("sh", vec!["-c".to_owned(), script], std::env::temp_dir());
		let mut runner = StdBuildProcessRunner;
		let mut process = runner.start_inheriting_stdio(&command).unwrap();
		let grandchild_pid = wait_for_pid_file(&pid_file);

		process.terminate().unwrap();

		let deadline = Instant::now() + Duration::from_secs(2);
		while process_exists(grandchild_pid) && Instant::now() < deadline {
			thread::sleep(Duration::from_millis(10));
		}
		assert!(!process_exists(grandchild_pid));
		let _ = fs::remove_file(pid_file);
	}

	#[cfg(unix)]
	#[test]
	fn collecting_output_cancellation_kills_process_group_grandchildren() {
		let pid_file = std::env::temp_dir().join(format!(
			"vorma-process-runner-cancelled-grandchild-{}",
			SystemTime::now()
				.duration_since(UNIX_EPOCH)
				.unwrap()
				.as_nanos()
		));
		let script = format!(
			"sleep 30 & echo $! > {}; wait",
			shell_quote(pid_file.to_str().unwrap())
		);
		let command =
			BuildProcessCommand::new("sh", vec!["-c".to_owned(), script], std::env::temp_dir());
		let cancel = BuildProcessCancel::new();
		let run_cancel = cancel.clone();
		let output_thread = thread::spawn(move || {
			let mut runner = StdBuildProcessRunner;
			runner.run_collecting_output_until_cancelled(&command, &run_cancel)
		});
		let grandchild_pid = wait_for_pid_file(&pid_file);

		cancel.cancel();
		let error = output_thread.join().unwrap().unwrap_err();

		assert_eq!(
			error,
			BuildProcessError::Cancelled {
				program: "sh".to_owned()
			}
		);
		let deadline = Instant::now() + Duration::from_secs(2);
		while process_exists(grandchild_pid) && Instant::now() < deadline {
			thread::sleep(Duration::from_millis(10));
		}
		assert!(!process_exists(grandchild_pid));
		let _ = fs::remove_file(pid_file);
	}

	#[cfg(unix)]
	fn wait_for_pid_file(path: &Path) -> u32 {
		let deadline = Instant::now() + Duration::from_secs(2);
		loop {
			if let Ok(value) = fs::read_to_string(path)
				&& let Ok(pid) = value.trim().parse::<u32>()
			{
				return pid;
			}
			assert!(
				Instant::now() < deadline,
				"grandchild pid file was not written"
			);
			thread::sleep(Duration::from_millis(10));
		}
	}

	#[cfg(unix)]
	fn process_exists(pid: u32) -> bool {
		Command::new("kill")
			.args(["-0", &pid.to_string()])
			.stderr(Stdio::null())
			.status()
			.is_ok_and(|status| status.success())
	}

	#[cfg(unix)]
	fn shell_quote(value: &str) -> String {
		format!("'{}'", value.replace('\'', "'\\''"))
	}
}
