use std::time::Duration;

use crate::clock::ClockInstant;
use crate::key::TaskId;

/// Passive task-runtime observer.
pub trait TaskObserver: Send + Sync + 'static {
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
	pub at: ClockInstant,
	pub task_id: TaskId,
	pub task_input_type: &'static str,
	pub kind: TaskEventKind,
}

/// Task-runtime event kind.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum TaskEventKind {
	Cancelled,
	ExecCtxMemoHit,
	ExecCtxMemoMiss,
	ExecCtxMemoWait,
	CrossExecCtxCacheHit,
	CrossExecCtxCacheMiss,
	CrossExecCtxInFlightWait,
	CrossExecCtxCacheInserted,
	CrossExecCtxStaleSlotRemoved {
		count: usize,
	},
	RunStarted {
		source: TaskRunSource,
	},
	RunCompleted {
		source: TaskRunSource,
		outcome: TaskEventOutcome,
		duration: Duration,
	},
}

/// Task body source that performed real work.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TaskRunSource {
	ExecCtx,
	CrossExecCtx,
}

/// Coarse task outcome for passive observation.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TaskEventOutcome {
	Success,
	Error,
	Cancelled,
}
