use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use tokio::sync::Notify;

/// Host-agnostic cancellation token for one execution context.
///
/// This is the crate's only cancellation primitive: builds, CLIs, and
/// daemons can construct a root token however their own shutdown signal
/// arrives (Ctrl-C, a build watcher's restart, a request's client
/// disconnect) and hand it to [`Tasks::exec_ctx`](crate::Tasks::exec_ctx);
/// nothing in `vorma-tasks` assumes an HTTP request is the only thing that
/// gets cancelled.
///
/// Cancellation here is **cooperative and preemptive at task boundaries**,
/// not forceful interruption mid-instruction:
///
/// - [`cancel`](Self::cancel) flips this token (and every clone of it) to
///   cancelled and wakes anything awaiting [`cancelled`](Self::cancelled).
/// - A task body that is already running to completion when cancellation
///   arrives is not interrupted mid-poll; the runtime observes cancellation
///   at task-resolution boundaries (before starting a run, and immediately
///   after one completes) and — for a run that actually suspends instead
///   of finishing synchronously — races the body's own future against
///   [`cancelled`](Self::cancelled) so a genuinely long-running body is cut
///   off promptly rather than run to completion regardless.
/// - A run whose *result* raced a cancellation is never treated as having
///   produced a usable value: even a run that returns `Ok` is discarded if
///   cancellation was observed for it, and cancellation itself is never
///   memoized or cached (see the crate root docs for the full retention
///   rules).
///
/// [`child`](Self::child) builds a hierarchy: cancelling a parent cancels
/// every descendant, but cancelling a child never reaches back up to its
/// parent or siblings. [`ExecCtx::child`](crate::ExecCtx::child) and
/// [`ParallelBatch`](crate::ParallelBatch) use this to give nested and
/// sibling work its own cancellation scope while still inheriting
/// shutdown from the caller.
#[derive(Clone)]
pub struct CancelToken {
	inner: Arc<CancelInner>,
	parent: Option<Arc<CancelToken>>,
}

impl CancelToken {
	/// Create a non-cancelled root token.
	///
	/// ```
	/// use vorma_tasks::CancelToken;
	///
	/// let cancel = CancelToken::new();
	/// assert!(!cancel.is_cancelled());
	/// ```
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
	///
	/// Idempotent: cancelling an already-cancelled token is a no-op.
	/// Cancellation only flows downward — cancelling a child token created
	/// by [`child`](Self::child) never cancels its parent.
	///
	/// ```
	/// use vorma_tasks::CancelToken;
	///
	/// let cancel = CancelToken::new();
	/// cancel.cancel();
	/// assert!(cancel.is_cancelled());
	/// ```
	pub fn cancel(&self) {
		if !self.inner.cancelled.swap(true, Ordering::AcqRel) {
			self.inner.notify.notify_waiters();
		}
	}

	/// Whether this token or any parent token has been cancelled.
	///
	/// ```
	/// use vorma_tasks::CancelToken;
	///
	/// let parent = CancelToken::new();
	/// let child = parent.child();
	/// assert!(!child.is_cancelled());
	///
	/// parent.cancel();
	/// assert!(child.is_cancelled(), "cancellation flows down from parent to child");
	/// ```
	pub fn is_cancelled(&self) -> bool {
		self.inner.cancelled.load(Ordering::Acquire)
			|| self
				.parent
				.as_ref()
				.is_some_and(|parent| parent.is_cancelled())
	}

	/// Wait until this token or any parent token is cancelled.
	///
	/// Returns immediately if the token is already cancelled. Task bodies
	/// that perform genuinely long-running work without their own natural
	/// suspension points (a tight CPU loop, a blocking call) should poll
	/// this periodically or race it with `tokio::select!` to stay
	/// cancellation-responsive; bodies built from `.await`ed I/O or nested
	/// [`Task::run`](crate::Task::run) calls are already covered by the
	/// runtime's own cancellation checks at resolution boundaries.
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
	///
	/// The relationship is one-directional: cancelling `self` cancels
	/// every child (transitively, through however many `child()` calls
	/// deep), but cancelling a child never cancels `self` or any sibling.
	/// This is what lets [`ExecCtx::child`](crate::ExecCtx::child) and
	/// [`ParallelBatch`](crate::ParallelBatch) give nested work its own
	/// cancellation scope — a parallel sibling that fails can cancel its
	/// own siblings without reaching back up to unrelated work sharing the
	/// same parent context.
	///
	/// ```
	/// use vorma_tasks::CancelToken;
	///
	/// let parent = CancelToken::new();
	/// let child = parent.child();
	///
	/// child.cancel();
	/// assert!(child.is_cancelled());
	/// assert!(!parent.is_cancelled(), "a child cancelling never reaches its parent");
	/// ```
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
