use std::fmt;
use std::sync::Arc;

/// Result returned by task bodies and by [`Task::run`](crate::Task::run).
///
/// A task body written for the `task!` macro returns
/// `Result<Output, ApplicationError>` — the crate's own [`Error<E>`]
/// wrapper is what callers of [`Task::run`](crate::Task::run) actually
/// receive, since resolution can also fail for reasons the task body never
/// raised itself (cancellation, a dependency cycle, a missing test
/// override).
pub type Result<T, E> = std::result::Result<T, Error<E>>;

/// Error returned while resolving a task.
///
/// `E` is the task's declared application-error type — the third type
/// parameter of [`Task<I, O, E>`](crate::Task) — not this enum itself; a
/// task body's `Err(e)` return is wrapped into [`Error::Failed`]
/// automatically through a `From<E>` implementation.
#[derive(Debug)]
pub enum Error<E> {
	/// Task body returned an application error.
	///
	/// Wrapped in an [`Arc`] because a memoized or cached failure may be
	/// handed out to more than one caller: a `memoized` task retains
	/// non-cancellation errors for the rest of the execution context (see
	/// the crate root docs for the full retention rules), and every
	/// concurrent caller waiting on the same in-flight run receives a
	/// clone of the same outcome.
	Failed(Arc<E>),
	/// Task resolution was cancelled before producing a value.
	///
	/// Never memoized or cached — see the crate root docs for the full
	/// retention rules. Produced when the resolving
	/// [`ExecCtx`](crate::ExecCtx)'s [`CancelToken`](crate::CancelToken) was
	/// already cancelled, or became cancelled while the task body was
	/// still running.
	Cancelled,
	/// Task resolution encountered a dependency cycle.
	///
	/// Returned instead of deadlocking: if resolving `task_name` requires
	/// (transitively) resolving `task_name` again on the same call stack,
	/// the second resolution fails immediately rather than waiting forever
	/// for a run that can never complete.
	Cycle {
		/// Declared task name being resolved.
		task_name: &'static str,
	},
	/// Overrides were configured as required and this task had none.
	///
	/// Only reachable when the [`Tasks`](crate::Tasks) runtime was built
	/// with [`TaskOverrides`](crate::TaskOverrides) in
	/// [`TaskOverrideMode::RequireOverride`](crate::TaskOverrideMode::RequireOverride)
	/// and `task_name` has no [`TaskOverrides::replace`](crate::TaskOverrides::replace)
	/// entry — the pattern a dry-run or test task graph uses to guarantee
	/// every reachable task was deliberately stubbed.
	MissingOverride {
		/// Declared task name being resolved.
		task_name: &'static str,
	},
	/// Cached output existed under this task/input key but had the wrong type.
	///
	/// Guards an internal invariant (one task identity maps to one output
	/// type for the life of the process) rather than a condition
	/// application code can trigger through the public API.
	TypeMismatch {
		/// Declared task name being resolved.
		task_name: &'static str,
	},
}

impl<E> Error<E> {
	/// Whether this error is cancellation.
	///
	/// ```
	/// use vorma_tasks::Error;
	///
	/// let error: Error<&str> = Error::Cancelled;
	/// assert!(error.is_cancelled());
	///
	/// let cycle: Error<&str> = Error::Cycle { task_name: "example" };
	/// assert!(!cycle.is_cancelled());
	/// ```
	pub fn is_cancelled(&self) -> bool {
		matches!(self, Self::Cancelled)
	}
}

impl<E> Clone for Error<E> {
	fn clone(&self) -> Self {
		match self {
			Self::Failed(error) => Self::Failed(error.clone()),
			Self::Cancelled => Self::Cancelled,
			Self::Cycle { task_name } => Self::Cycle { task_name },
			Self::MissingOverride { task_name } => Self::MissingOverride { task_name },
			Self::TypeMismatch { task_name } => Self::TypeMismatch { task_name },
		}
	}
}

impl<E> From<E> for Error<E> {
	fn from(error: E) -> Self {
		Self::Failed(Arc::new(error))
	}
}

impl<E> fmt::Display for Error<E> {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Failed(_) => f.write_str("task failed"),
			Self::Cancelled => f.write_str("task cancelled"),
			Self::Cycle { task_name } => {
				write!(f, "task cycle detected while resolving {task_name}")
			}
			Self::MissingOverride { task_name } => {
				write!(f, "task override required before resolving {task_name}")
			}
			Self::TypeMismatch { task_name } => {
				write!(f, "cached task output had the wrong type for {task_name}")
			}
		}
	}
}

impl<E: fmt::Debug + Send + Sync + 'static> std::error::Error for Error<E> {}
