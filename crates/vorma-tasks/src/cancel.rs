use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use tokio::sync::Notify;

/// Host-agnostic cancellation token for one execution context.
#[derive(Clone)]
pub struct CancelToken {
	inner: Arc<CancelInner>,
	parent: Option<Arc<CancelToken>>,
}

impl CancelToken {
	/// Create a non-cancelled root token.
	pub fn new() -> Self {
		Self {
			inner: Arc::new(CancelInner {
				cancelled: AtomicBool::new(false),
				notify: Notify::new(),
			}),
			parent: None,
		}
	}

	/// Cancel this token and notify waiters.
	pub fn cancel(&self) {
		if !self.inner.cancelled.swap(true, Ordering::AcqRel) {
			self.inner.notify.notify_waiters();
		}
	}

	/// Whether this token or any parent token has been cancelled.
	pub fn is_cancelled(&self) -> bool {
		self.inner.cancelled.load(Ordering::Acquire)
			|| self
				.parent
				.as_ref()
				.is_some_and(|parent| parent.is_cancelled())
	}

	/// Wait until this token or any parent token is cancelled.
	pub async fn cancelled(&self) {
		loop {
			if self.inner.cancelled.load(Ordering::Acquire) {
				return;
			}

			/*
			Race-free wait: register interest first, re-check the flag,
			then await. cancel() sets the flag before notifying, so a
			cancellation between the check and the await still wakes
			this registration.
			*/
			let notified = self.inner.notify.notified();
			tokio::pin!(notified);
			notified.as_mut().enable();
			if self.inner.cancelled.load(Ordering::Acquire) {
				return;
			}

			match &self.parent {
				Some(parent) => {
					let parent_cancelled = Box::pin(parent.cancelled());
					tokio::select! {
						_ = &mut notified => {}
						_ = parent_cancelled => return,
					}
				}
				None => notified.await,
			}
		}
	}

	/// Create a child token that is cancelled when this token is cancelled.
	pub fn child(&self) -> Self {
		Self {
			inner: Arc::new(CancelInner {
				cancelled: AtomicBool::new(false),
				notify: Notify::new(),
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
	notify: Notify,
}
