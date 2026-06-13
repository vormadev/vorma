//! Development shutdown signal listener.

use std::sync::mpsc::{self, Receiver};
use std::thread;

#[derive(Debug)]
pub(crate) struct DevShutdownSignalListener {
	stop_tx: Option<tokio::sync::watch::Sender<bool>>,
	thread: Option<thread::JoinHandle<Result<(), String>>>,
	shutdown_rx: Receiver<DevShutdownSignalMessage>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum DevShutdownSignalMessage {
	Shutdown,
	Failed(String),
}

impl DevShutdownSignalListener {
	#[cfg(test)]
	fn for_test(message: Option<DevShutdownSignalMessage>) -> Self {
		let (shutdown_tx, shutdown_rx) = mpsc::channel();
		if let Some(message) = message {
			let _ = shutdown_tx.send(message);
		}
		Self {
			stop_tx: None,
			thread: None,
			shutdown_rx,
		}
	}

	pub(crate) fn recv(self) -> Result<(), String> {
		match self.shutdown_rx.recv() {
			Ok(DevShutdownSignalMessage::Shutdown) => Ok(()),
			Ok(DevShutdownSignalMessage::Failed(message)) => Err(message),
			Err(mpsc::RecvError) => {
				Err("dev shutdown signal listener stopped unexpectedly".to_owned())
			}
		}
	}

	fn stop(&mut self) -> Result<(), String> {
		if let Some(stop_tx) = self.stop_tx.take() {
			let _ = stop_tx.send(true);
		}
		let Some(thread) = self.thread.take() else {
			return Ok(());
		};
		thread
			.join()
			.map_err(|_| "dev shutdown signal listener thread panicked".to_owned())?
	}
}

impl Drop for DevShutdownSignalListener {
	fn drop(&mut self) {
		let _ = self.stop();
	}
}

pub(crate) fn start_dev_shutdown_signal_listener() -> DevShutdownSignalListener {
	let (stop_tx, stop_rx) = tokio::sync::watch::channel(false);
	let (shutdown_tx, shutdown_rx) = mpsc::channel();
	let failure_tx = shutdown_tx.clone();
	let thread = thread::spawn(move || {
		let result = (|| -> Result<(), String> {
			let runtime = tokio::runtime::Builder::new_current_thread()
				.enable_io()
				.build()
				.map_err(|source| source.to_string())?;
			runtime.block_on(async move {
				match wait_for_shutdown_signal(stop_rx).await {
					DevShutdownSignal::Stop => Ok(()),
					DevShutdownSignal::Signal => {
						let _ = shutdown_tx.send(DevShutdownSignalMessage::Shutdown);
						Ok(())
					}
				}
			})
		})();
		if let Err(message) = &result {
			let _ = failure_tx.send(DevShutdownSignalMessage::Failed(message.clone()));
		}
		result
	});
	DevShutdownSignalListener {
		stop_tx: Some(stop_tx),
		thread: Some(thread),
		shutdown_rx,
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum DevShutdownSignal {
	Signal,
	Stop,
}

async fn wait_for_shutdown_signal(
	mut stop_rx: tokio::sync::watch::Receiver<bool>,
) -> DevShutdownSignal {
	#[cfg(unix)]
	{
		match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
			Ok(mut terminate) => {
				tokio::select! {
					_ = tokio::signal::ctrl_c() => DevShutdownSignal::Signal,
					_ = terminate.recv() => DevShutdownSignal::Signal,
					_ = wait_for_stop_request(&mut stop_rx) => DevShutdownSignal::Stop,
				}
			}
			Err(_) => {
				tokio::select! {
					_ = tokio::signal::ctrl_c() => DevShutdownSignal::Signal,
					_ = wait_for_stop_request(&mut stop_rx) => DevShutdownSignal::Stop,
				}
			}
		}
	}
	#[cfg(not(unix))]
	{
		tokio::select! {
			_ = tokio::signal::ctrl_c() => DevShutdownSignal::Signal,
			_ = wait_for_stop_request(&mut stop_rx) => DevShutdownSignal::Stop,
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
	fn recv_returns_after_shutdown_notification() {
		let listener =
			DevShutdownSignalListener::for_test(Some(DevShutdownSignalMessage::Shutdown));

		listener.recv().unwrap();
	}

	#[test]
	fn recv_reports_listener_failure() {
		let listener = DevShutdownSignalListener::for_test(Some(DevShutdownSignalMessage::Failed(
			"signal failed".to_owned(),
		)));

		assert_eq!(listener.recv().unwrap_err(), "signal failed");
	}

	#[test]
	fn recv_reports_listener_disconnect_as_error() {
		let (_shutdown_tx, shutdown_rx) = mpsc::channel();
		drop(_shutdown_tx);
		let listener = DevShutdownSignalListener {
			stop_tx: None,
			thread: None,
			shutdown_rx,
		};

		assert_eq!(
			listener.recv().unwrap_err(),
			"dev shutdown signal listener stopped unexpectedly"
		);
	}
}
