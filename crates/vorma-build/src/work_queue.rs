use std::sync::mpsc::{self, Receiver, SyncSender};
use std::sync::{Arc, Mutex};

#[derive(Debug)]
pub(crate) struct DevWorkQueue {
	pending: Arc<Mutex<PendingDevWork>>,
	sender: DevWorkQueueSender,
	wake_rx: Option<Receiver<()>>,
}

#[derive(Clone, Debug)]
pub(crate) struct DevWorkQueueSender {
	pending: Arc<Mutex<PendingDevWork>>,
	wake_tx: SyncSender<()>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum DevWork {
	RefreshServer,
	StaticBuild { includes_client_revalidate: bool },
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
struct PendingDevWork {
	refresh_server: bool,
	static_build: bool,
	includes_client_revalidate: bool,
}

impl DevWorkQueue {
	pub(crate) fn new() -> Self {
		let (wake_tx, wake_rx) = mpsc::sync_channel(1);
		let pending = Arc::new(Mutex::new(PendingDevWork::default()));
		Self {
			pending: Arc::clone(&pending),
			sender: DevWorkQueueSender { pending, wake_tx },
			wake_rx: Some(wake_rx),
		}
	}

	pub(crate) fn sender(&self) -> DevWorkQueueSender {
		self.sender.clone()
	}

	pub(crate) fn take_receiver(&mut self) -> Receiver<()> {
		self.wake_rx
			.take()
			.expect("dev work queue receiver already taken")
	}

	pub(crate) fn claim_work(&self) -> Option<DevWork> {
		self.pending
			.lock()
			.expect("dev work queue lock poisoned")
			.claim()
	}
}

impl Default for DevWorkQueue {
	fn default() -> Self {
		Self::new()
	}
}

impl DevWorkQueueSender {
	pub(crate) fn queue_server_refresh(&self) {
		self.pending
			.lock()
			.expect("dev work queue lock poisoned")
			.queue_server_refresh();
		self.wake();
	}

	pub(crate) fn queue_static_build(&self, includes_client_revalidate: bool) {
		self.pending
			.lock()
			.expect("dev work queue lock poisoned")
			.queue_static_build(includes_client_revalidate);
		self.wake();
	}

	pub(crate) fn wake(&self) {
		let _ = self.wake_tx.try_send(());
	}
}

impl PendingDevWork {
	fn queue_server_refresh(&mut self) {
		self.refresh_server = true;
	}

	fn queue_static_build(&mut self, includes_client_revalidate: bool) {
		self.static_build = true;
		self.includes_client_revalidate |= includes_client_revalidate;
	}

	fn claim(&mut self) -> Option<DevWork> {
		let claimed = std::mem::take(self);
		if claimed.refresh_server {
			return Some(DevWork::RefreshServer);
		}
		if claimed.static_build {
			return Some(DevWork::StaticBuild {
				includes_client_revalidate: claimed.includes_client_revalidate,
			});
		}
		None
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn server_refresh_wins_over_static_build() {
		let queue = DevWorkQueue::new();
		let sender = queue.sender();

		sender.queue_static_build(true);
		sender.queue_server_refresh();

		assert_eq!(queue.claim_work(), Some(DevWork::RefreshServer));
		assert_eq!(queue.claim_work(), None);
	}

	#[test]
	fn static_build_coalesces_client_revalidate_flag() {
		let queue = DevWorkQueue::new();
		let sender = queue.sender();

		sender.queue_static_build(true);
		sender.queue_static_build(false);

		assert_eq!(
			queue.claim_work(),
			Some(DevWork::StaticBuild {
				includes_client_revalidate: true,
			}),
		);
	}

	#[test]
	fn wake_coalesces_notifications_until_receiver_drains() {
		let mut queue = DevWorkQueue::new();
		let sender = queue.sender();
		let rx = queue.take_receiver();

		sender.queue_static_build(false);
		sender.queue_static_build(true);

		rx.try_recv().unwrap();
		assert!(rx.try_recv().is_err());
	}
}
