use std::fmt;
use std::sync::Arc;

/// Result returned by task functions and task execution.
pub type Result<T, E> = std::result::Result<T, Error<E>>;

/// Error returned while resolving a task.
#[derive(Debug)]
pub enum Error<E> {
	Failed(Arc<E>),
	Cancelled,
	Cycle { task: &'static str },
	MissingOverride { task: &'static str },
	TypeMismatch { task: &'static str },
}

impl<E> Error<E> {
	pub fn is_cancelled(&self) -> bool {
		matches!(self, Self::Cancelled)
	}
}

impl<E> Clone for Error<E> {
	fn clone(&self) -> Self {
		match self {
			Self::Failed(error) => Self::Failed(error.clone()),
			Self::Cancelled => Self::Cancelled,
			Self::Cycle { task } => Self::Cycle { task },
			Self::MissingOverride { task } => Self::MissingOverride { task },
			Self::TypeMismatch { task } => Self::TypeMismatch { task },
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
			Self::Cycle { task } => write!(f, "task cycle detected while resolving {task}"),
			Self::MissingOverride { task } => {
				write!(f, "task override required before resolving {task}")
			}
			Self::TypeMismatch { task } => {
				write!(f, "cached task output had the wrong type for {task}")
			}
		}
	}
}

impl<E: fmt::Debug + Send + Sync + 'static> std::error::Error for Error<E> {}
