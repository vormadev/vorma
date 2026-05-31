use std::sync::atomic::{AtomicBool, Ordering};

use tokio::sync::watch;

#[derive(Debug)]
pub(crate) struct BuildCancel {
	cancelled: AtomicBool,
	cancel_tx: watch::Sender<bool>,
}

impl BuildCancel {
	pub(crate) fn new(cancelled: bool) -> Self {
		let (cancel_tx, _) = watch::channel(cancelled);
		Self {
			cancelled: AtomicBool::new(cancelled),
			cancel_tx,
		}
	}

	pub(crate) fn reset(&self) {
		self.store(false, Ordering::SeqCst);
	}

	pub(crate) fn load(&self, ordering: Ordering) -> bool {
		self.cancelled.load(ordering)
	}

	pub(crate) fn store(&self, cancelled: bool, ordering: Ordering) {
		self.cancelled.store(cancelled, ordering);
		self.cancel_tx.send_replace(cancelled);
	}

	pub(crate) fn subscribe(&self) -> watch::Receiver<bool> {
		self.cancel_tx.subscribe()
	}

	pub(crate) fn is_cancelled(&self) -> bool {
		self.load(Ordering::SeqCst)
	}

	pub(crate) fn check(&self, stage: &str) -> Result<(), String> {
		if self.is_cancelled() {
			return Err(format!("build cancelled during {stage}"));
		}
		Ok(())
	}
}

impl Default for BuildCancel {
	fn default() -> Self {
		Self::new(false)
	}
}
