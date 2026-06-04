use std::collections::VecDeque;
use std::fmt;
use std::io;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU32, Ordering};
use std::sync::mpsc::{self, Receiver};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use crate::build_cancel::BuildCancel;
use crate::config::{VormaCfg, to_cfg};
use crate::constants::{
	LIVE_STATE_MODE_ENV_KEY, VITE_PLUGIN_SERVER_PORT_ENV_KEY, VITE_PLUGIN_SERVER_TOKEN_ENV_KEY,
};
use crate::constants::{SUPERVISOR_READY_TIMEOUT, SUPERVISOR_SHUTDOWN_GRACE_PERIOD};
use crate::signals::ChildProcessStopHandles;
use crate::work_queue::DevWorkQueueSender;
use vorma::__private::Config;
use vorma::__private::constants::{ENV_KEY_IS_BUILD, ENV_KEY_IS_DEV};

pub(crate) const SUPERVISOR_APP_SERVER_NAME: &str = "app server";
pub(crate) const SUPERVISOR_VITE_SERVER_NAME: &str = "Vite server";
const DEV_APP_SERVER_DEFAULT_PORT: u16 = 8080;
const DEV_VITE_SERVER_DEFAULT_PORT: u16 = 5173;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct ChildProcessExit {
	pub(crate) name: String,
	pub(crate) err: String,
}

#[derive(Clone, Debug)]
pub(crate) struct ProcessStopHandle {
	process_id: Arc<AtomicU32>,
}

impl ProcessStopHandle {
	fn new() -> Self {
		Self {
			process_id: Arc::new(AtomicU32::new(0)),
		}
	}

	pub(crate) fn request_stop(&self) {
		let process_id = self.process_id.load(Ordering::SeqCst);
		if process_id != 0 {
			request_stop(process_id);
		}
	}

	pub(crate) fn force_kill(&self) {
		let process_id = self.process_id.load(Ordering::SeqCst);
		if process_id != 0 {
			force_kill(process_id);
		}
	}

	fn store(&self, process_id: u32) {
		self.process_id.store(process_id, Ordering::SeqCst);
	}

	fn clear_if_current(&self, process_id: u32) {
		let _ = self
			.process_id
			.compare_exchange(process_id, 0, Ordering::SeqCst, Ordering::SeqCst);
	}
}

#[derive(Debug)]
pub(crate) struct DevProcesses {
	app_server: Supervisor,
	vite_server: Supervisor,
	unexpected_exits: Arc<Mutex<VecDeque<ChildProcessExit>>>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum ProcessStopRequest {
	NotRunning,
	Requested,
}

pub(crate) struct AppServerStartArgs {
	pub(crate) root_dir: PathBuf,
	pub(crate) executable: PathBuf,
	pub(crate) is_dev: bool,
	pub(crate) build_cancel: Arc<BuildCancel>,
	pub(crate) wake_run_loop: DevWorkQueueSender,
}

pub(crate) struct ViteServerStartArgs {
	pub(crate) config: Config,
	pub(crate) dev_mux_port: u16,
	pub(crate) vite_plugin_token: String,
	pub(crate) build_cancel: Arc<BuildCancel>,
	pub(crate) wake_run_loop: DevWorkQueueSender,
}

impl DevProcesses {
	pub(crate) fn new() -> Self {
		Self {
			app_server: Supervisor::new(),
			vite_server: Supervisor::new(),
			unexpected_exits: Arc::new(Mutex::new(VecDeque::new())),
		}
	}

	pub(crate) fn stop_all(&mut self, should_force: bool) {
		let app_server = &mut self.app_server;
		let vite_server = &mut self.vite_server;
		thread::scope(|scope| {
			let app_server_stop = scope.spawn(move || {
				app_server.stop(should_force);
			});
			vite_server.stop(should_force);
			app_server_stop
				.join()
				.expect("app server stop thread panicked");
		});
	}

	pub(crate) fn begin_stop_app_server(&mut self) -> ProcessStopRequest {
		self.app_server.begin_stop()
	}

	pub(crate) fn finish_stop_app_server(&mut self, stop: ProcessStopRequest, should_force: bool) {
		if stop == ProcessStopRequest::Requested {
			self.app_server.finish_stop(should_force);
		}
	}

	pub(crate) fn stop_vite_server(&mut self, should_force: bool) {
		self.vite_server.stop(should_force);
	}

	pub(crate) fn vite_server_running(&self) -> bool {
		self.vite_server.is_running()
	}

	pub(crate) fn vite_server_port(&self) -> Option<u16> {
		self.vite_server
			.is_running()
			.then(|| self.vite_server.port())
	}

	pub(crate) fn pop_unexpected_exit(&self) -> Option<ChildProcessExit> {
		self.unexpected_exits
			.lock()
			.expect("child process exits lock poisoned")
			.pop_front()
	}

	pub(crate) fn stop_handles(
		&self,
		build_cancel: Arc<BuildCancel>,
		wake_run_loop: DevWorkQueueSender,
	) -> ChildProcessStopHandles {
		ChildProcessStopHandles {
			app_server_sv: self.app_server.process_stop_handle(),
			vite_server_sv: self.vite_server.process_stop_handle(),
			build_cancel,
			wake_run_loop,
		}
	}

	pub(crate) fn start_app_server(&mut self, args: AppServerStartArgs) -> Result<(), String> {
		if self.app_server.is_running() {
			return Err("app server already running before activation".to_owned());
		}
		let unexpected_exits = Arc::clone(&self.unexpected_exits);
		let build_cancel = Arc::clone(&args.build_cancel);
		let wake_run_loop = args.wake_run_loop;
		self.app_server
			.start(SvStartOpts {
				preferred_port: DEV_APP_SERVER_DEFAULT_PORT,
				build_cancel: Arc::clone(&args.build_cancel),
				make_cmd: move |port| {
					Ok(app_server_cmd(
						&args.root_dir,
						&args.executable,
						args.is_dev,
						port,
					))
				},
				ready_endpoint: "/.vorma/healthz".to_owned(),
				on_unexpected_exit: Some(move |err| {
					build_cancel.store(true, Ordering::SeqCst);
					unexpected_exits
						.lock()
						.expect("child process exits lock poisoned")
						.push_back(ChildProcessExit {
							name: SUPERVISOR_APP_SERVER_NAME.to_owned(),
							err,
						});
					wake_run_loop.wake();
				}),
			})
			.map_err(|err| err.to_string())?;
		eprintln!(
			"App server ready: http://localhost:{}",
			self.app_server.port()
		);
		Ok(())
	}

	pub(crate) fn start_vite_server(&mut self, args: ViteServerStartArgs) -> Result<u16, String> {
		if self.vite_server.is_running() {
			return Ok(self.vite_server.port());
		}
		let cfg = to_cfg(&args.config).map_err(|err| format!("error converting config: {err}"))?;
		let unexpected_exits = Arc::clone(&self.unexpected_exits);
		let build_cancel = Arc::clone(&args.build_cancel);
		let wake_run_loop = args.wake_run_loop;
		self.vite_server
			.start(SvStartOpts {
				preferred_port: DEV_VITE_SERVER_DEFAULT_PORT,
				build_cancel: Arc::clone(&args.build_cancel),
				make_cmd: |port| {
					vite_server_cmd(&cfg, port, args.dev_mux_port, &args.vite_plugin_token)
				},
				ready_endpoint: "/@vite/client".to_owned(),
				on_unexpected_exit: Some(move |err| {
					build_cancel.store(true, Ordering::SeqCst);
					unexpected_exits
						.lock()
						.expect("child process exits lock poisoned")
						.push_back(ChildProcessExit {
							name: SUPERVISOR_VITE_SERVER_NAME.to_owned(),
							err,
						});
					wake_run_loop.wake();
				}),
			})
			.map_err(|err| err.to_string())?;
		Ok(self.vite_server.port())
	}
}

impl Default for DevProcesses {
	fn default() -> Self {
		Self::new()
	}
}

fn app_server_cmd(root_dir: &Path, executable: &Path, is_dev: bool, port: u16) -> Command {
	let mut cmd = Command::new(executable);
	clear_vorma_runtime_env(&mut cmd);
	cmd.current_dir(root_dir)
		.stdout(Stdio::inherit())
		.stderr(Stdio::inherit())
		.env("PORT", port.to_string())
		.env(ENV_KEY_IS_DEV, is_dev.to_string());
	cmd
}

fn vite_server_cmd(
	cfg: &VormaCfg<'_>,
	vite_port: u16,
	internal_dev_server_port: u16,
	vite_plugin_token: &str,
) -> Result<Command, String> {
	let args = cfg.vite_server_args(vite_port);
	let mut cmd = Command::new(&args[0]);
	clear_vorma_runtime_env(&mut cmd);
	cmd.args(&args[1..])
		.current_dir(cfg.js_package_manager_dir())
		.stdout(Stdio::inherit())
		.stderr(Stdio::inherit())
		.env(
			VITE_PLUGIN_SERVER_PORT_ENV_KEY,
			internal_dev_server_port.to_string(),
		)
		.env(VITE_PLUGIN_SERVER_TOKEN_ENV_KEY, vite_plugin_token);
	Ok(cmd)
}

#[derive(Debug)]
pub(crate) struct Supervisor {
	state: SupervisorState,
	exit_err: Arc<std::sync::Mutex<Option<String>>>,
	stopping: Arc<AtomicBool>,
	process_stop_handle: ProcessStopHandle,
}

#[derive(Debug)]
enum SupervisorState {
	Stopped,
	Running(RunningSupervisorProcess),
	Stopping(RunningSupervisorProcess),
}

#[derive(Debug)]
struct RunningSupervisorProcess {
	port: u16,
	process_id: u32,
	done: Receiver<()>,
}

pub(crate) struct SvStartOpts<F, G>
where
	F: FnOnce(u16) -> Result<Command, String>,
	G: FnOnce(String) + Send + 'static,
{
	pub(crate) preferred_port: u16,
	pub(crate) build_cancel: Arc<BuildCancel>,
	pub(crate) make_cmd: F,
	pub(crate) ready_endpoint: String,
	pub(crate) on_unexpected_exit: Option<G>,
}

#[derive(Debug)]
pub(crate) enum SupervisorError {
	AlreadyRunning,
	FreePort(String),
	Start(std::io::Error),
	Ready(String),
}

impl Supervisor {
	pub(crate) fn new() -> Self {
		Self {
			state: SupervisorState::Stopped,
			exit_err: Arc::new(std::sync::Mutex::new(None)),
			stopping: Arc::new(AtomicBool::new(false)),
			process_stop_handle: ProcessStopHandle::new(),
		}
	}

	pub(crate) fn is_running(&self) -> bool {
		matches!(
			self.state,
			SupervisorState::Running(_) | SupervisorState::Stopping(_)
		)
	}

	pub(crate) fn port(&self) -> u16 {
		match &self.state {
			SupervisorState::Stopped => 0,
			SupervisorState::Running(process) | SupervisorState::Stopping(process) => process.port,
		}
	}

	pub(crate) fn process_stop_handle(&self) -> ProcessStopHandle {
		self.process_stop_handle.clone()
	}

	pub(crate) fn start<F, G>(&mut self, opts: SvStartOpts<F, G>) -> Result<(), SupervisorError>
	where
		F: FnOnce(u16) -> Result<Command, String>,
		G: FnOnce(String) + Send + 'static,
	{
		if self.is_running() {
			return Err(SupervisorError::AlreadyRunning);
		}

		let port = get_free_port(opts.preferred_port).map_err(SupervisorError::FreePort)?;
		let mut cmd = (opts.make_cmd)(port).map_err(SupervisorError::Ready)?;
		prepare_child_process(&mut cmd);

		let mut child = cmd.spawn().map_err(SupervisorError::Start)?;
		let process_id = child.id();
		let (done_tx, done_rx) = mpsc::channel();
		let exit_err = Arc::clone(&self.exit_err);
		let stopping = Arc::clone(&self.stopping);
		let process_stop_handle = self.process_stop_handle.clone();
		let build_cancel = Arc::clone(&opts.build_cancel);
		let on_unexpected_exit = opts.on_unexpected_exit;

		self.state = SupervisorState::Running(RunningSupervisorProcess {
			port,
			process_id,
			done: done_rx,
		});
		self.process_stop_handle.store(process_id);
		self.stopping.store(false, Ordering::SeqCst);
		*self.exit_err.lock().expect("exit_err lock poisoned") = None;

		thread::spawn(move || {
			let wait_result = child.wait();
			let err = match wait_result {
				Ok(status) if status.success() => "process exited successfully".to_owned(),
				Ok(status) => format!("process exited with status {status}"),
				Err(err) => err.to_string(),
			};
			*exit_err.lock().expect("exit_err lock poisoned") = Some(err.clone());
			if !stopping.load(Ordering::SeqCst)
				&& !build_cancel.load(Ordering::SeqCst)
				&& let Some(on_unexpected_exit) = on_unexpected_exit
			{
				on_unexpected_exit(err);
			}
			process_stop_handle.clear_if_current(process_id);
			let _ = done_tx.send(());
		});

		let ready_url = format!("http://localhost:{}{}", port, opts.ready_endpoint);
		if let Err(err) =
			poll_http_ready_endpoint(|| self.exit_error(), &ready_url, &opts.build_cancel)
		{
			self.stop(true);
			return Err(SupervisorError::Ready(err));
		}

		Ok(())
	}

	pub(crate) fn stop(&mut self, force: bool) {
		if self.begin_stop() == ProcessStopRequest::Requested {
			self.finish_stop(force);
		}
	}

	fn begin_stop(&mut self) -> ProcessStopRequest {
		let state = std::mem::replace(&mut self.state, SupervisorState::Stopped);
		let process = match state {
			SupervisorState::Stopped => {
				self.state = SupervisorState::Stopped;
				return ProcessStopRequest::NotRunning;
			}
			SupervisorState::Running(process) => process,
			SupervisorState::Stopping(process) => {
				self.state = SupervisorState::Stopping(process);
				return ProcessStopRequest::Requested;
			}
		};
		self.stopping.store(true, Ordering::SeqCst);
		request_stop(process.process_id);
		self.state = SupervisorState::Stopping(process);
		ProcessStopRequest::Requested
	}

	fn finish_stop(&mut self, force: bool) {
		let state = std::mem::replace(&mut self.state, SupervisorState::Stopped);
		let process = match state {
			SupervisorState::Stopped => {
				self.state = SupervisorState::Stopped;
				return;
			}
			SupervisorState::Running(process) | SupervisorState::Stopping(process) => process,
		};
		let stopped_without_force =
			!force && wait_until_done_timeout(&process.done, SUPERVISOR_SHUTDOWN_GRACE_PERIOD);
		if !stopped_without_force {
			force_kill(process.process_id);
			wait_until_done(&process.done);
		}

		self.state = SupervisorState::Stopped;
		self.process_stop_handle
			.clear_if_current(process.process_id);
		*self.exit_err.lock().expect("exit_err lock poisoned") = None;
		self.stopping.store(false, Ordering::SeqCst);
	}

	fn exit_error(&self) -> Option<String> {
		self.exit_err
			.lock()
			.expect("exit_err lock poisoned")
			.clone()
	}
}

fn wait_until_done(done: &Receiver<()>) {
	let _ = done.recv();
}

fn wait_until_done_timeout(done: &Receiver<()>, timeout: Duration) -> bool {
	done.recv_timeout(timeout).is_ok()
}

impl Default for Supervisor {
	fn default() -> Self {
		Self::new()
	}
}

impl Drop for Supervisor {
	fn drop(&mut self) {
		self.stop(false);
	}
}

impl fmt::Display for SupervisorError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::AlreadyRunning => write!(f, "process already running"),
			Self::FreePort(err) => write!(f, "error getting free port: {err}"),
			Self::Start(err) => write!(f, "error starting process: {err}"),
			Self::Ready(err) => write!(f, "{err}"),
		}
	}
}

impl std::error::Error for SupervisorError {}

fn poll_http_ready_endpoint(
	exit_err: impl Fn() -> Option<String>,
	url: &str,
	build_cancel: &BuildCancel,
) -> Result<(), String> {
	let deadline = Instant::now() + SUPERVISOR_READY_TIMEOUT;
	let mut current_delay = Duration::from_millis(0);
	let delay_inc_amt = Duration::from_millis(20);
	let max_delay = Duration::from_millis(200);

	while Instant::now() < deadline {
		if build_cancel.load(Ordering::SeqCst) {
			return Err("build cancelled before process became ready".to_owned());
		}
		if let Some(err) = exit_err() {
			return Err(format!("process exited before becoming ready: {err}",));
		}
		if http_get_ok(url) {
			return Ok(());
		}

		current_delay = std::cmp::min(current_delay + delay_inc_amt, max_delay);
		thread::sleep(current_delay);
	}

	Err("process did not become ready in time".to_owned())
}

fn http_get_ok(url: &str) -> bool {
	let Ok(response) = ureq::get(url)
		.config()
		.http_status_as_error(false)
		.timeout_global(Some(Duration::from_secs(1)))
		.build()
		.call()
	else {
		return false;
	};
	response.status().as_u16() == 200
}

fn get_free_port(default_port: u16) -> Result<u16, String> {
	get_free_port_with(default_port, check_availability, get_random_free_port)
}

fn get_free_port_with(
	default_port: u16,
	check_availability: impl Fn(u16) -> bool,
	get_random_free_port: impl Fn() -> Result<u16, String>,
) -> Result<u16, String> {
	let default_port = if default_port == 0 {
		8080
	} else {
		default_port
	};
	if check_availability(default_port) {
		return Ok(default_port);
	}

	for offset in 1..=1024 {
		let Some(port) = default_port.checked_add(offset) else {
			break;
		};
		if check_availability(port) {
			return Ok(port);
		}
	}

	get_random_free_port()
}

fn check_availability(port: u16) -> bool {
	let mut successful_probe = false;
	for host in ["0.0.0.0", "::", "localhost", "127.0.0.1", "::1"] {
		match TcpListener::bind((host, port)) {
			Ok(listener) => {
				drop(listener);
				successful_probe = true;
			}
			Err(err) if can_ignore_listen_error(&err) => {}
			Err(_) => return false,
		}
	}
	successful_probe
}

pub(crate) fn get_random_free_port() -> Result<u16, String> {
	let listener = TcpListener::bind(("127.0.0.1", 0))
		.or_else(|_| TcpListener::bind(("localhost", 0)))
		.map_err(|err| err.to_string())?;
	listener
		.local_addr()
		.map(|addr| addr.port())
		.map_err(|err| err.to_string())
}

fn can_ignore_listen_error(err: &io::Error) -> bool {
	let Some(code) = err.raw_os_error() else {
		return false;
	};
	code == libc::EAFNOSUPPORT || code == libc::EPROTONOSUPPORT || code == libc::EADDRNOTAVAIL
}

#[cfg(unix)]
pub(crate) fn prepare_child_process(cmd: &mut Command) {
	use std::os::unix::process::CommandExt;

	cmd.process_group(0);
}

#[cfg(windows)]
pub(crate) fn prepare_child_process(cmd: &mut Command) {
	use std::os::windows::process::CommandExt;

	const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;
	cmd.creation_flags(CREATE_NEW_PROCESS_GROUP);
}

#[cfg(not(any(unix, windows)))]
pub(crate) fn prepare_child_process(_cmd: &mut Command) {}

pub(crate) fn clear_vorma_runtime_env(cmd: &mut Command) {
	cmd.env_remove(ENV_KEY_IS_BUILD)
		.env_remove(ENV_KEY_IS_DEV)
		.env_remove(LIVE_STATE_MODE_ENV_KEY);
}

#[cfg(unix)]
fn request_stop(pid: u32) {
	unsafe {
		libc::kill(-(pid as i32), libc::SIGTERM);
	}
}

#[cfg(windows)]
fn request_stop(pid: u32) {
	let _ = Command::new("taskkill")
		.args(["/T", "/PID", &pid.to_string()])
		.status();
}

#[cfg(not(any(unix, windows)))]
fn request_stop(_pid: u32) {}

#[cfg(unix)]
pub(crate) fn force_kill(pid: u32) {
	unsafe {
		libc::kill(-(pid as i32), libc::SIGKILL);
		libc::kill(pid as i32, libc::SIGKILL);
	}
}

#[cfg(windows)]
pub(crate) fn force_kill(pid: u32) {
	let _ = Command::new("taskkill")
		.args(["/T", "/F", "/PID", &pid.to_string()])
		.status();
}

#[cfg(not(any(unix, windows)))]
pub(crate) fn force_kill(_pid: u32) {}

#[cfg(test)]
mod tests {
	use std::ffi::OsStr;
	use std::fs;
	use std::io::{Read, Write};
	use std::net::TcpListener;
	use std::path::{Path, PathBuf};
	use std::time::{SystemTime, UNIX_EPOCH};

	use super::*;
	use crate::config::to_cfg;
	use vorma::__private::Config;
	use vorma::{FrontendConfig, PathConfig, ServerConfig, TsGenConfig};

	#[test]
	fn get_free_port_prefers_available_default() {
		let got = get_free_port_with(49152, |port| port == 49152, || Ok(60000)).unwrap();

		assert_eq!(got, 49152);
	}

	#[test]
	fn get_free_port_falls_forward_when_default_is_occupied() {
		let got = get_free_port_with(49152, |port| port == 49154, || Ok(60000)).unwrap();

		assert_eq!(got, 49154);
	}

	#[test]
	fn poll_http_ready_endpoint_returns_when_endpoint_is_ok() {
		let Ok(listener) = TcpListener::bind(("127.0.0.1", 0)) else {
			return;
		};
		let port = listener.local_addr().unwrap().port();

		thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut buf = [0; 256];
			let _ = stream.read(&mut buf);
			stream
				.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
				.unwrap();
		});

		poll_http_ready_endpoint(
			|| None,
			&format!("http://localhost:{port}/.vorma/healthz"),
			&BuildCancel::new(false),
		)
		.unwrap();
	}

	#[test]
	fn poll_http_ready_endpoint_reports_early_exit() {
		let error = poll_http_ready_endpoint(
			|| Some("process exited with status 1".to_owned()),
			"http://localhost:1/.vorma/healthz",
			&BuildCancel::new(false),
		)
		.unwrap_err();

		assert_eq!(
			error,
			"process exited before becoming ready: process exited with status 1"
		);
	}

	#[test]
	fn poll_http_ready_endpoint_reports_build_cancel() {
		let build_cancel = BuildCancel::new(true);

		let error =
			poll_http_ready_endpoint(|| None, "http://localhost:1/.vorma/healthz", &build_cancel)
				.unwrap_err();

		assert_eq!(error, "build cancelled before process became ready");
	}

	#[test]
	fn supervisor_reports_clean_exit_before_ready_as_unexpected() {
		let mut sv = Supervisor::new();

		let err = sv
			.start(SvStartOpts {
				preferred_port: 0,
				build_cancel: Arc::new(BuildCancel::new(false)),
				make_cmd: |_| {
					let mut cmd = Command::new(std::env::current_exe().unwrap());
					cmd.args(["--exact", "__vorma_no_such_test__"]);
					Ok(cmd)
				},
				ready_endpoint: "/.vorma/healthz".to_owned(),
				on_unexpected_exit: Option::<fn(String)>::None,
			})
			.unwrap_err();

		assert!(err.to_string().contains("process exited successfully"));
	}

	#[test]
	fn supervisor_start_rejects_active_process() {
		let mut sv = Supervisor::new();
		let (_done_tx, done_rx) = mpsc::channel();
		sv.state = SupervisorState::Stopping(RunningSupervisorProcess {
			port: 49152,
			process_id: 123,
			done: done_rx,
		});

		let err = sv
			.start(SvStartOpts {
				preferred_port: 0,
				build_cancel: Arc::new(BuildCancel::new(false)),
				make_cmd: |_| Ok(Command::new(std::env::current_exe().unwrap())),
				ready_endpoint: "/.vorma/healthz".to_owned(),
				on_unexpected_exit: Option::<fn(String)>::None,
			})
			.unwrap_err();

		assert_eq!(err.to_string(), "process already running");
	}

	#[test]
	fn app_server_cmd_sets_runtime_dev_env() {
		let cmd = app_server_cmd(Path::new("."), Path::new("/tmp/server"), true, 8765);

		assert_eq!(cmd.get_program(), OsStr::new("/tmp/server"));
		assert_eq!(cmd.get_current_dir(), Some(Path::new(".")));
		assert_eq!(
			cmd.get_envs().find(|(k, _)| *k == "PORT").unwrap().1,
			Some(OsStr::new("8765"))
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == ENV_KEY_IS_DEV)
				.unwrap()
				.1,
			Some(OsStr::new("true")),
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == ENV_KEY_IS_BUILD)
				.unwrap()
				.1,
			None,
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == LIVE_STATE_MODE_ENV_KEY)
				.unwrap()
				.1,
			None,
		);
	}

	#[test]
	fn vite_server_cmd_uses_args_dir_and_internal_server_env() {
		let root = temp_dir("vite-server-cmd");
		fs::create_dir_all(root.join("frontend")).unwrap();
		let mut config = config(&root);
		config.frontend_config.js_package_manager_base_cmd = "pnpm exec".to_owned();
		config.frontend_config.js_package_manager_dir =
			root.join("frontend").to_string_lossy().into_owned();
		config.frontend_config.vite_config_file = root
			.join("frontend")
			.join("vite.config.ts")
			.to_string_lossy()
			.into_owned();
		let cfg = to_cfg(&config).unwrap();

		let cmd = vite_server_cmd(&cfg, 5173, 4321, "secret").unwrap();

		assert_eq!(cmd.get_program(), OsStr::new("pnpm"));
		assert_eq!(
			cmd.get_args().collect::<Vec<_>>(),
			vec![
				OsStr::new("exec"),
				OsStr::new("vite"),
				OsStr::new("--host"),
				OsStr::new("127.0.0.1"),
				OsStr::new("--port"),
				OsStr::new("5173"),
				OsStr::new("--clearScreen"),
				OsStr::new("false"),
				OsStr::new("--strictPort"),
				OsStr::new("true"),
				OsStr::new("--config"),
				OsStr::new("vite.config.ts"),
			],
		);
		assert_eq!(
			cmd.get_current_dir(),
			Some(Path::new(&config.frontend_config.js_package_manager_dir)),
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == VITE_PLUGIN_SERVER_PORT_ENV_KEY)
				.unwrap()
				.1,
			Some(OsStr::new("4321")),
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == VITE_PLUGIN_SERVER_TOKEN_ENV_KEY)
				.unwrap()
				.1,
			Some(OsStr::new("secret")),
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == ENV_KEY_IS_BUILD)
				.unwrap()
				.1,
			None,
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == ENV_KEY_IS_DEV)
				.unwrap()
				.1,
			None,
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == LIVE_STATE_MODE_ENV_KEY)
				.unwrap()
				.1,
			None,
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn clear_vorma_runtime_env_removes_internal_mode_flags() {
		let mut cmd = Command::new("example");

		clear_vorma_runtime_env(&mut cmd);

		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == ENV_KEY_IS_BUILD)
				.unwrap()
				.1,
			None,
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == ENV_KEY_IS_DEV)
				.unwrap()
				.1,
			None,
		);
		assert_eq!(
			cmd.get_envs()
				.find(|(k, _)| *k == LIVE_STATE_MODE_ENV_KEY)
				.unwrap()
				.1,
			None,
		);
	}

	fn config(root: &Path) -> Config {
		fs::create_dir_all(root).unwrap();
		Config {
			root_dir: root.to_path_buf(),
			dist_dir: root.join("dist").to_string_lossy().into_owned(),
			server_config: ServerConfig {
				cargo_package: "example-app".to_owned(),
				cargo_bin: "example-server".to_owned(),
			},
			path_config: PathConfig {
				public_static_base: "/static/".to_owned(),
				api_base: "/api/".to_owned(),
			},
			frontend_config: FrontendConfig {
				ui_variant: "react".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: root.join("public").to_string_lossy().into_owned(),
				critical_css_file: ".".to_owned(),
				..crate::test_support::frontend_config()
			},
			ts_gen_config: TsGenConfig {
				out_file: root.join("vorma.gen.ts").to_string_lossy().into_owned(),
				..crate::test_support::ts_gen_config()
			},
			..Config::default()
		}
	}

	fn temp_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		std::env::temp_dir().join(format!("vorma-supervisor-{name}-{nonce}"))
	}
}
