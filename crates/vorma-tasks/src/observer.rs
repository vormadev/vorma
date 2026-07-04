use std::time::Duration;

use crate::clock::ClockInstant;
use crate::key::TaskId;

/// Passive task-runtime observer.
///
/// Register one on [`TasksOptions::observer`](crate::TasksOptions::observer)
/// to receive a [`TaskEvent`] for every cache-hit/miss, run start/completion,
/// stale-slot cleanup, and cancellation across every
/// [`ExecCtx`](crate::ExecCtx) sharing that [`Tasks`](crate::Tasks) runtime —
/// the shape a metrics exporter, structured logger, or debugging trace wants.
/// "Passive" means exactly that: `observe` cannot influence resolution in
/// any way, cannot see task inputs or outputs (only their type names), and
/// runs synchronously on the resolving task's own call path, so it must
/// return quickly — anything expensive (a network call, disk I/O) should be
/// handed off to a channel or background task rather than done inline.
///
/// A plain closure or function pointer with signature
/// `Fn(TaskEvent) + Send + Sync + 'static` implements this trait
/// automatically; implement it manually only when the observer needs its
/// own state beyond what a captured closure can hold ergonomically.
///
/// When no observer is configured, event construction is skipped entirely
/// (including the clock reads a duration needs) rather than merely
/// discarded — passive observation costs nothing when nobody is watching.
///
/// ```
/// use std::sync::Arc;
/// use std::sync::atomic::{AtomicUsize, Ordering};
///
/// use vorma_tasks::{TaskEvent, TaskObserver};
///
/// let event_count = Arc::new(AtomicUsize::new(0));
/// let counting_observer = {
///     let event_count = event_count.clone();
///     move |_event: TaskEvent| {
///         event_count.fetch_add(1, Ordering::SeqCst);
///     }
/// };
/// // A plain `Fn(TaskEvent)` closure satisfies `TaskObserver`.
/// fn accepts_observer(_observer: impl TaskObserver) {}
/// accepts_observer(counting_observer);
/// ```
pub trait TaskObserver: Send + Sync + 'static {
	/// Observe one task-runtime event.
	fn observe(&self, event: TaskEvent);
}

impl<F> TaskObserver for F
where
	F: Fn(TaskEvent) + Send + Sync + 'static,
{
	fn observe(&self, event: TaskEvent) {
		self(event);
	}
}

/// One task-runtime event, emitted to every registered [`TaskObserver`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TaskEvent {
	/// Monotonic event timestamp, from the [`Tasks`](crate::Tasks) runtime's
	/// configured [`Clock`](crate::Clock).
	pub at: ClockInstant,
	/// Opaque task identity — the same value [`Task::id`](crate::Task::id)
	/// returns for the task that produced this event.
	pub task_id: TaskId,
	/// Declared task name.
	pub task_name: &'static str,
	/// Rust type name of the task input.
	pub task_input_type: &'static str,
	/// Event kind.
	pub kind: TaskEventKind,
}

/// Task-runtime event kind.
///
/// Every resolution passes through a well-defined sequence of these: an
/// execution-context memo check (`ExecCtxMemo{Hit,Miss,Wait}`) always comes
/// first; a miss on an `extended_cache` task then checks the shared cache
/// (`CrossExecCtx*`); a genuine cache miss brackets the task body with
/// `RunStarted`/`RunCompleted`. Cancellation can interrupt this sequence at
/// any point, always reported as [`TaskEventKind::Cancelled`].
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum TaskEventKind {
	/// Resolution observed cancellation.
	Cancelled,
	/// Value was already memoized in the execution context.
	ExecCtxMemoHit,
	/// Value was not memoized in the execution context.
	ExecCtxMemoMiss,
	/// Resolution waited for an in-flight execution-context run.
	ExecCtxMemoWait,
	/// Value was already cached across execution contexts.
	CrossExecCtxCacheHit,
	/// Value was not cached across execution contexts.
	CrossExecCtxCacheMiss,
	/// Shared caching was bypassed because the cross-execution-context cache is full.
	CrossExecCtxCacheCapacityBypass {
		/// Configured maximum number of cross-execution-context cache entries.
		max_entries: usize,
	},
	/// Resolution waited for an in-flight cross-execution-context run.
	CrossExecCtxInFlightWait,
	/// Successful value was inserted into the cross-execution-context cache.
	CrossExecCtxCacheInserted,
	/// Expired cross-execution-context cache slots were removed.
	CrossExecCtxStaleSlotRemoved {
		/// Number of stale slots removed.
		count: usize,
	},
	/// Task body started running.
	RunStarted {
		/// Cache/memoization layer that ran the task body.
		source: TaskRunSource,
	},
	/// Task body finished running.
	RunCompleted {
		/// Cache/memoization layer that ran the task body.
		source: TaskRunSource,
		/// Coarse task outcome.
		outcome: TaskEventOutcome,
		/// Runtime duration.
		duration: Duration,
	},
}

/// Task body source that performed real work.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TaskRunSource {
	/// Execution-context memoization layer.
	ExecCtx,
	/// Cross-execution-context cache layer.
	CrossExecCtx,
}

/// Coarse task outcome for passive observation.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TaskEventOutcome {
	/// Task body returned a value.
	Success,
	/// Task body returned an error.
	Error,
	/// Task body was cancelled.
	Cancelled,
}
