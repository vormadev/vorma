use std::sync::Arc;
use std::sync::atomic::Ordering;
use std::sync::mpsc::{self, Receiver};
use std::thread;

use crate::build_cancel::BuildCancel;
use crate::supervisor::ProcessStopHandle;
use crate::work_queue::DevWorkQueueSender;

#[derive(Debug)]
pub(crate) struct SignalThread {
	stop_tx: Option<tokio::sync::watch::Sender<bool>>,
	thread: Option<thread::JoinHandle<Result<(), String>>>,
	shutdown_rx: Receiver<SignalThreadMessage>,
	#[cfg(test)]
	_shutdown_tx_for_test: Option<mpsc::Sender<SignalThreadMessage>>,
	shutdown_requested: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum SignalThreadMessage {
	Shutdown,
	Failed(String),
}

#[derive(Clone, Debug)]
pub(crate) struct ChildProcessStopHandles {
	pub(crate) app_server_sv: ProcessStopHandle,
	pub(crate) vite_server_sv: ProcessStopHandle,
	pub(crate) build_cancel: Arc<BuildCancel>,
	pub(crate) wake_run_loop: DevWorkQueueSender,
}

impl SignalThread {
	#[cfg(test)]
	pub(crate) fn for_test(shutdown_requested: bool) -> Self {
		let (shutdown_tx, shutdown_rx) = mpsc::channel();
		if shutdown_requested {
			let _ = shutdown_tx.send(SignalThreadMessage::Shutdown);
		}
		Self {
			stop_tx: None,
			thread: None,
			shutdown_rx,
			_shutdown_tx_for_test: Some(shutdown_tx),
			shutdown_requested: false,
		}
	}

	pub(crate) fn stop(&mut self) -> Result<(), String> {
		if let Some(stop_tx) = self.stop_tx.take() {
			let _ = stop_tx.send(true);
		}
		let Some(thread) = self.thread.take() else {
			return Ok(());
		};
		thread
			.join()
			.map_err(|_| "signal listener thread panicked".to_owned())?
	}

	pub(crate) fn shutdown_requested(&mut self) -> Result<bool, String> {
		if self.shutdown_requested {
			return Ok(true);
		}
		match self.shutdown_rx.try_recv() {
			Ok(SignalThreadMessage::Shutdown) => {
				self.shutdown_requested = true;
				Ok(true)
			}
			Ok(SignalThreadMessage::Failed(err)) => Err(err),
			Err(mpsc::TryRecvError::Disconnected) => {
				Err("signal listener stopped unexpectedly".to_owned())
			}
			Err(mpsc::TryRecvError::Empty) => Ok(false),
		}
	}
}

impl Drop for SignalThread {
	fn drop(&mut self) {
		let _ = self.stop();
	}
}

impl ChildProcessStopHandles {
	pub(crate) fn request_stop(&self) {
		self.build_cancel.store(true, Ordering::SeqCst);
		self.app_server_sv.request_stop();
		self.vite_server_sv.request_stop();
		self.wake_run_loop.wake();
	}

	pub(crate) fn force_kill(&self) {
		self.build_cancel.store(true, Ordering::SeqCst);
		self.app_server_sv.force_kill();
		self.vite_server_sv.force_kill();
		self.wake_run_loop.wake();
	}
}

pub(crate) fn start_signal_thread(stop_handles: ChildProcessStopHandles) -> SignalThread {
	let (stop_tx, stop_rx) = tokio::sync::watch::channel(false);
	let (shutdown_tx, shutdown_rx) = mpsc::channel();
	let failure_tx = shutdown_tx.clone();
	let thread = thread::spawn(move || {
		let result = (|| -> Result<(), String> {
			let runtime = tokio::runtime::Builder::new_current_thread()
				.enable_all()
				.build()
				.map_err(|err| err.to_string())?;
			runtime.block_on(async move {
				if wait_for_shutdown_signal(stop_rx.clone()).await == ShutdownSignal::Stop {
					return Ok(());
				}

				eprintln!("Shutting down...");
				stop_handles.request_stop();
				let _ = shutdown_tx.send(SignalThreadMessage::Shutdown);

				if wait_for_shutdown_signal(stop_rx).await == ShutdownSignal::Signal {
					eprintln!("Force killing...");
					stop_handles.force_kill();
				}
				Ok(())
			})
		})();
		if let Err(err) = &result {
			let _ = failure_tx.send(SignalThreadMessage::Failed(err.clone()));
		}
		result
	});
	SignalThread {
		stop_tx: Some(stop_tx),
		thread: Some(thread),
		shutdown_rx,
		#[cfg(test)]
		_shutdown_tx_for_test: None,
		shutdown_requested: false,
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum ShutdownSignal {
	Signal,
	Stop,
}

async fn wait_for_shutdown_signal(
	mut stop_rx: tokio::sync::watch::Receiver<bool>,
) -> ShutdownSignal {
	#[cfg(unix)]
	{
		match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
			Ok(mut term) => {
				tokio::select! {
					_ = tokio::signal::ctrl_c() => ShutdownSignal::Signal,
					_ = term.recv() => ShutdownSignal::Signal,
					_ = wait_for_stop_request(&mut stop_rx) => ShutdownSignal::Stop,
				}
			}
			Err(_) => {
				tokio::select! {
					_ = tokio::signal::ctrl_c() => ShutdownSignal::Signal,
					_ = wait_for_stop_request(&mut stop_rx) => ShutdownSignal::Stop,
				}
			}
		}
	}
	#[cfg(not(unix))]
	{
		tokio::select! {
			_ = tokio::signal::ctrl_c() => ShutdownSignal::Signal,
			_ = wait_for_stop_request(&mut stop_rx) => ShutdownSignal::Stop,
		}
	}
}

async fn wait_for_stop_request(stop_rx: &mut tokio::sync::watch::Receiver<bool>) {
	loop {
		if *stop_rx.borrow_and_update() {
			return;
		}
		if stop_rx.changed().await.is_err() {
			return;
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn shutdown_requested_is_latched_after_notification() {
		let mut signal_thread = SignalThread::for_test(true);

		assert!(signal_thread.shutdown_requested().unwrap());
		assert!(signal_thread.shutdown_requested().unwrap());
	}

	#[test]
	fn shutdown_requested_reports_listener_disconnect_as_error() {
		let (_shutdown_tx, shutdown_rx) = mpsc::channel();
		drop(_shutdown_tx);
		let mut signal_thread = SignalThread {
			stop_tx: None,
			thread: None,
			shutdown_rx,
			_shutdown_tx_for_test: None,
			shutdown_requested: false,
		};

		assert_eq!(
			signal_thread.shutdown_requested().unwrap_err(),
			"signal listener stopped unexpectedly"
		);
	}
}
