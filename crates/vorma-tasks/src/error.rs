use std::fmt;
use std::sync::Arc;

/// Result returned by task functions and task execution.
pub type Result<T, E> = std::result::Result<T, Error<E>>;

/// Error returned while resolving a task.
#[derive(Debug)]
pub enum Error<E> {
	/// Task body returned an application error.
	Failed(Arc<E>),
	/// Task resolution was cancelled before producing a value.
	Cancelled,
	/// Task resolution encountered a dependency cycle.
	Cycle {
		/// Declared task name being resolved.
		task_name: &'static str,
	},
	/// Overrides were configured as required and this task had none.
	MissingOverride {
		/// Declared task name being resolved.
		task_name: &'static str,
	},
	/// Cached output existed under this task/input key but had the wrong type.
	TypeMismatch {
		/// Declared task name being resolved.
		task_name: &'static str,
	},
}

impl<E> Error<E> {
	/// Whether this error is cancellation.
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
