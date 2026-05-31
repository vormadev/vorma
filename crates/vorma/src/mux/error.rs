use std::fmt;

use vorma_tasks::Error as TaskError;

#[derive(Debug)]
pub enum Error {
	InvalidMatcherOptions(String),
	InvalidMountRoot(String),
	InvalidPattern(String),
	DuplicatePattern(String),
	Invariant(String),
	RouteNotFound(String),
	TaskJoin(String),
}

impl fmt::Display for Error {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::InvalidMatcherOptions(error) => write!(f, "invalid matcher options: {error}"),
			Self::InvalidMountRoot(error) => write!(f, "invalid mount root: {error}"),
			Self::InvalidPattern(error) => write!(f, "invalid pattern: {error}"),
			Self::DuplicatePattern(pattern) => write!(f, "duplicate pattern: {pattern}"),
			Self::Invariant(message) => write!(f, "mux invariant violated: {message}"),
			Self::RouteNotFound(pattern) => write!(f, "route not found: {pattern}"),
			Self::TaskJoin(error) => write!(f, "task join error: {error}"),
		}
	}
}

impl std::error::Error for Error {}

#[doc(hidden)]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum InputError {
	BadRequest(String),
	Internal(String),
}

impl InputError {
	#[doc(hidden)]
	pub fn bad_request(message: impl Into<String>) -> Self {
		Self::BadRequest(message.into())
	}

	#[doc(hidden)]
	pub fn internal(message: impl Into<String>) -> Self {
		Self::Internal(message.into())
	}

	#[doc(hidden)]
	pub fn is_bad_request(&self) -> bool {
		matches!(self, Self::BadRequest(_))
	}
}

impl fmt::Display for InputError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::BadRequest(message) | Self::Internal(message) => f.write_str(message),
		}
	}
}

impl std::error::Error for InputError {}

#[derive(Debug)]
pub enum RouteExecutionError<E> {
	Input(InputError),
	Task(TaskError<E>),
}

impl<E> Clone for RouteExecutionError<E> {
	fn clone(&self) -> Self {
		match self {
			Self::Input(error) => Self::Input(error.clone()),
			Self::Task(error) => Self::Task(error.clone()),
		}
	}
}

impl<E> RouteExecutionError<E> {
	#[cfg(test)]
	pub fn is_bad_request(&self) -> bool {
		matches!(self, Self::Input(error) if error.is_bad_request())
	}
}

impl<E> From<InputError> for RouteExecutionError<E> {
	fn from(error: InputError) -> Self {
		Self::Input(error)
	}
}

impl<E> From<TaskError<E>> for RouteExecutionError<E> {
	fn from(error: TaskError<E>) -> Self {
		Self::Task(error)
	}
}

impl<E> fmt::Display for RouteExecutionError<E> {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Input(error) => write!(f, "{error}"),
			Self::Task(error) => write!(f, "{error}"),
		}
	}
}

impl<E> std::error::Error for RouteExecutionError<E> where E: fmt::Debug + Send + Sync + 'static {}
