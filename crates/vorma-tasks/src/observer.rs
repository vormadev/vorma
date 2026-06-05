use std::time::Duration;

use crate::clock::ClockInstant;
use crate::key::TaskId;

/// Passive task-runtime observer.
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

/// Snapshot emitted by the task runtime.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TaskEvent {
	/// Monotonic event timestamp.
	pub at: ClockInstant,
	/// Opaque task identity.
	pub task_id: TaskId,
	/// Rust type name of the task input.
	pub task_input_type: &'static str,
	/// Event kind.
	pub kind: TaskEventKind,
}

/// Task-runtime event kind.
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
