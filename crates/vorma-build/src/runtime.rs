use std::collections::{BTreeSet, VecDeque};
use std::path::PathBuf;
use std::sync::mpsc::Receiver;
use std::sync::{Arc, Mutex};

use paranoid::local_lock::ProcessLock;

use crate::build_cancel::BuildCancel;
use crate::constants::{DEV_LOOPBACK_HOST, VITE_PLUGIN_TOKEN_HEADER};
use crate::dev_mux::{DevMuxRuntime, DevMuxServer, start_dev_mux_server};
use crate::dev_watcher::{DevWatcher, DevWatcherStartArgs};
use crate::generation::CommittedGeneration;
use crate::generation::DevMuxGeneration;
use crate::signals::ChildProcessStopHandles;
use crate::supervisor::{
	AppServerStartArgs, DevProcesses, ProcessStopRequest, ViteServerStartArgs,
};
use crate::work_queue::{DevWork, DevWorkQueue};

#[derive(Debug)]
pub(crate) struct DevRuntime {
	dev_lock: Option<ProcessLock>,
	build_cancel: Arc<BuildCancel>,
	dev_work: DevWorkQueue,
	fatal_events: Arc<Mutex<VecDeque<String>>>,
	css_files_to_watch: Arc<Mutex<BTreeSet<PathBuf>>>,
	dev_mux: DevMuxRuntime,
	dev_mux_server: Option<DevMuxServer>,
	processes: DevProcesses,
	watcher: DevWatcher,
}

impl DevRuntime {
	pub(crate) fn new() -> Self {
		Self {
			dev_lock: None,
			build_cancel: Arc::new(BuildCancel::new(false)),
			dev_work: DevWorkQueue::new(),
			fatal_events: Arc::new(Mutex::new(VecDeque::new())),
			css_files_to_watch: Arc::new(Mutex::new(BTreeSet::new())),
			dev_mux: DevMuxRuntime::default(),
			dev_mux_server: None,
			processes: DevProcesses::new(),
			watcher: DevWatcher::default(),
		}
	}

	pub(crate) fn build_cancel(&self) -> Arc<BuildCancel> {
		Arc::clone(&self.build_cancel)
	}

	pub(crate) fn take_dev_work_receiver(&mut self) -> Receiver<()> {
		self.dev_work.take_receiver()
	}

	pub(crate) fn claim_dev_work(&self) -> Option<DevWork> {
		self.dev_work.claim_work()
	}

	pub(crate) fn stop_handles(&self) -> ChildProcessStopHandles {
		self.processes
			.stop_handles(Arc::clone(&self.build_cancel), self.dev_work.sender())
	}

	pub(crate) fn reset_build_cancel(&self) {
		self.build_cancel.reset();
	}

	pub(crate) fn hold_dev_lock(&mut self, lock: ProcessLock) {
		self.dev_lock = Some(lock);
	}

	pub(crate) fn has_dev_lock(&self) -> bool {
		self.dev_lock.is_some()
	}

	pub(crate) fn replace_dev_lock(&mut self, lock: ProcessLock) -> Option<ProcessLock> {
		self.dev_lock.replace(lock)
	}

	pub(crate) fn clear_dev_lock(&mut self) -> Option<ProcessLock> {
		self.dev_lock.take()
	}

	pub(crate) fn publish_css_files_to_watch(&self, files: BTreeSet<PathBuf>) {
		*self
			.css_files_to_watch
			.lock()
			.expect("css_files_to_watch lock poisoned") = files;
	}

	pub(crate) fn pop_fatal_event(&self) -> Option<String> {
		self.fatal_events
			.lock()
			.expect("fatal_events lock poisoned")
			.pop_front()
	}

	pub(crate) fn pop_child_process_exit(&self) -> Option<crate::supervisor::ChildProcessExit> {
		self.processes.pop_unexpected_exit()
	}

	pub(crate) fn prepare_dev_mux(&mut self) -> Result<u16, String> {
		self.dev_mux.prepare()
	}

	pub(crate) fn publish_dev_mux_generation(&self, generation: Option<DevMuxGeneration>) {
		self.dev_mux.publish_generation(generation);
	}

	pub(crate) fn clear_dev_mux_generation(&self) {
		self.dev_mux.clear_generation();
	}

	pub(crate) fn ensure_dev_mux_server(&mut self) -> Result<(), String> {
		if self.dev_mux_server.is_some() {
			return Ok(());
		}
		let port = self.prepare_dev_mux()?;
		let state = self.dev_mux.state();
		let endpoint = self.dev_mux.dev_refresh_endpoint()?;
		self.dev_mux_server = Some(start_dev_mux_server(state, endpoint, port)?);
		Ok(())
	}

	pub(crate) fn stop_dev_mux_server(&mut self) -> Result<(), String> {
		let Some(mut server) = self.dev_mux_server.take() else {
			return Ok(());
		};
		server.stop()
	}

	pub(crate) fn dev_mux_port(&self) -> Result<u16, String> {
		self.dev_mux.port_u16()
	}

	pub(crate) fn dev_mux_port_i32(&self) -> Result<i32, String> {
		self.dev_mux.port_i32()
	}

	pub(crate) fn dev_refresh_token(&self) -> Result<String, String> {
		Ok(self.dev_mux.dev_refresh_token()?.to_owned())
	}

	pub(crate) fn vite_plugin_token(&self) -> Result<String, String> {
		self.dev_mux.require_vite_plugin_token()
	}

	pub(crate) fn broadcast_refresh(
		&self,
		change_type: crate::browser_sync::ChangeType,
		critical_css: &str,
		build_error: &str,
	) {
		self.dev_mux
			.broadcast_refresh(change_type, critical_css, build_error);
	}

	pub(crate) fn send_vite_plugin_restart(&self) -> Result<(), String> {
		let vite_plugin_token = self.vite_plugin_token()?;
		let ctrl_port = self.dev_mux.vite_plugin_control_port().ok_or_else(|| {
			"Vite plugin control port not set; cannot restart Vite server".to_owned()
		})?;
		let url = format!("http://{DEV_LOOPBACK_HOST}:{ctrl_port}/cfg-changed");
		let response = ureq::post(&url)
			.header(VITE_PLUGIN_TOKEN_HEADER, &vite_plugin_token)
			.config()
			.http_status_as_error(false)
			.build()
			.send_empty()
			.map_err(|err| format!("error sending restart command to Vite plugin: {err}"))?;
		if response.status().as_u16() != 200 {
			return Err(format!(
				"Vite plugin restart request returned status: {}",
				response.status(),
			));
		}
		Ok(())
	}

	pub(crate) fn restart_watcher(
		&mut self,
		committed: &CommittedGeneration,
	) -> Result<(), String> {
		self.publish_css_files_to_watch(committed.static_metadata().css_files_to_watch.clone());
		self.watcher.restart(DevWatcherStartArgs {
			config: committed.config().clone(),
			fatal_events: Arc::clone(&self.fatal_events),
			dev_work: self.dev_work.sender(),
			build_cancel: Arc::clone(&self.build_cancel),
			css_files_to_watch: Arc::clone(&self.css_files_to_watch),
		})
	}

	pub(crate) fn start_vite_server(
		&mut self,
		committed: &CommittedGeneration,
	) -> Result<u16, String> {
		self.ensure_dev_mux_server()?;
		self.processes.start_vite_server(ViteServerStartArgs {
			config: committed.config().clone(),
			dev_mux_port: self.dev_mux_port()?,
			vite_plugin_token: self.vite_plugin_token()?,
			build_cancel: Arc::clone(&self.build_cancel),
			wake_run_loop: self.dev_work.sender(),
		})
	}

	pub(crate) fn vite_server_running(&self) -> bool {
		self.processes.vite_server_running()
	}

	pub(crate) fn vite_server_port(&self) -> Option<u16> {
		self.processes.vite_server_port()
	}

	pub(crate) fn start_app_server(
		&mut self,
		committed: &CommittedGeneration,
	) -> Result<(), String> {
		let cfg = committed.config_view()?;
		let executable = committed
			.app_server_executable()
			.cloned()
			.ok_or_else(|| "app server executable not available".to_owned())?;
		self.processes.start_app_server(AppServerStartArgs {
			root_dir: cfg.root_dir().to_path_buf(),
			executable,
			is_dev: true,
			build_cancel: Arc::clone(&self.build_cancel),
			wake_run_loop: self.dev_work.sender(),
		})
	}

	pub(crate) fn stop_processes(&mut self, should_force: bool) {
		self.processes.stop_all(should_force);
	}

	pub(crate) fn begin_stop_app_server(&mut self) -> ProcessStopRequest {
		self.processes.begin_stop_app_server()
	}

	pub(crate) fn finish_stop_app_server(&mut self, stop: ProcessStopRequest, should_force: bool) {
		self.processes.finish_stop_app_server(stop, should_force);
	}

	pub(crate) fn stop_vite_server(&mut self, should_force: bool) {
		self.processes.stop_vite_server(should_force);
	}

	pub(crate) fn stop_watcher(&mut self) -> Result<(), String> {
		self.watcher.stop()
	}

	#[cfg(test)]
	pub(crate) fn set_dev_mux_for_test(&mut self, dev_mux: DevMuxRuntime) {
		self.dev_mux = dev_mux;
	}

	#[cfg(test)]
	pub(crate) fn set_vite_plugin_control_port_for_test(&self, port: u16) {
		self.dev_mux.state().set_vite_plugin_control_port(port);
	}

	#[cfg(test)]
	pub(crate) fn add_client_for_test(&self) -> crate::browser_sync::ClientSubscription {
		self.dev_mux.state().add_client()
	}
}

impl Drop for DevRuntime {
	fn drop(&mut self) {
		let _ = self.watcher.stop();
		self.stop_processes(false);
		let _ = self.stop_dev_mux_server();
		if let Some(mut lock) = self.clear_dev_lock() {
			let _ = lock.release();
		}
	}
}

impl Default for DevRuntime {
	fn default() -> Self {
		Self::new()
	}
}

#[cfg(test)]
mod tests {
	use std::io::{Read, Write};
	use std::net::TcpListener;
	use std::thread;

	use super::*;

	fn spawn_control_server(response: &'static str) -> (u16, thread::JoinHandle<String>) {
		let listener = TcpListener::bind((DEV_LOOPBACK_HOST, 0)).unwrap();
		let port = listener.local_addr().unwrap().port();
		let thread = thread::spawn(move || {
			let (mut stream, _) = listener.accept().unwrap();
			let mut buf = [0; 1024];
			let n = stream.read(&mut buf).unwrap();
			stream.write_all(response.as_bytes()).unwrap();
			String::from_utf8_lossy(&buf[..n]).into_owned()
		});
		(port, thread)
	}

	#[test]
	fn vite_plugin_restart_posts_to_control_port() {
		let (port, thread) = spawn_control_server(
			"HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
		);
		let mut runtime = DevRuntime::new();
		runtime.set_dev_mux_for_test(DevMuxRuntime::for_test(3000, "refresh", "secret"));
		runtime.set_vite_plugin_control_port_for_test(port);

		runtime.send_vite_plugin_restart().unwrap();
		let request = thread.join().unwrap();

		assert!(request.starts_with("POST /cfg-changed HTTP/1.1\r\n"));
		assert!(request.contains(&format!("{VITE_PLUGIN_TOKEN_HEADER}: secret")));
	}

	#[test]
	fn vite_plugin_restart_reports_non_ok_status() {
		let (port, thread) = spawn_control_server(
			"HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
		);
		let mut runtime = DevRuntime::new();
		runtime.set_dev_mux_for_test(DevMuxRuntime::for_test(3000, "refresh", "secret"));
		runtime.set_vite_plugin_control_port_for_test(port);

		let error = runtime.send_vite_plugin_restart().unwrap_err();
		let request = thread.join().unwrap();

		assert!(request.starts_with("POST /cfg-changed HTTP/1.1\r\n"));
		assert!(request.contains(&format!("{VITE_PLUGIN_TOKEN_HEADER}: secret")));
		assert!(error.contains("Vite plugin restart request returned status: 503"));
	}
}
