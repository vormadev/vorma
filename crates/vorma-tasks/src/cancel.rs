use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use tokio::sync::watch;

/// Host-agnostic cancellation token for one execution context.
#[derive(Clone)]
pub struct CancelToken {
	inner: Arc<CancelInner>,
	parent: Option<Arc<CancelToken>>,
}

impl CancelToken {
	pub fn new() -> Self {
		let (tx, _) = watch::channel(false);
		Self {
			inner: Arc::new(CancelInner {
				cancelled: AtomicBool::new(false),
				tx,
			}),
			parent: None,
		}
	}

	pub fn cancel(&self) {
		if !self.inner.cancelled.swap(true, Ordering::SeqCst) {
			let _ = self.inner.tx.send(true);
		}
	}

	pub fn is_cancelled(&self) -> bool {
		self.inner.cancelled.load(Ordering::SeqCst)
			|| self
				.parent
				.as_ref()
				.is_some_and(|parent| parent.is_cancelled())
	}

	pub async fn cancelled(&self) {
		if self.is_cancelled() {
			return;
		}

		let mut rx = self.inner.tx.subscribe();
		if let Some(parent) = &self.parent {
			let parent_cancelled = Box::pin(parent.cancelled());
			tokio::select! {
				_ = wait_for_cancel(&mut rx) => {}
				_ = parent_cancelled => {}
			}
		} else {
			wait_for_cancel(&mut rx).await;
		}
	}

	pub fn child(&self) -> Self {
		let (tx, _) = watch::channel(false);
		Self {
			inner: Arc::new(CancelInner {
				cancelled: AtomicBool::new(false),
				tx,
			}),
			parent: Some(Arc::new(self.clone())),
		}
	}
}

impl Default for CancelToken {
	fn default() -> Self {
		Self::new()
	}
}

struct CancelInner {
	cancelled: AtomicBool,
	tx: watch::Sender<bool>,
}

async fn wait_for_cancel(rx: &mut watch::Receiver<bool>) {
	while !*rx.borrow_and_update() {
		if rx.changed().await.is_err() {
			return;
		}
	}
}
