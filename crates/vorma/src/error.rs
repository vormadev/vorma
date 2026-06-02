use std::error::Error as StdError;
use std::fmt;
use std::sync::Arc;

/// Boxed application error type used by the default Vorma error parameter.
pub type BoxError = Box<dyn StdError + Send + Sync + 'static>;

/// Simple framework error for app setup and runtime helper failures.
#[derive(Debug)]
pub struct Error {
	message: String,
}

impl Error {
	/// Create a runtime/setup error with a display message.
	pub fn runtime(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}
}

impl fmt::Display for Error {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.write_str(&self.message)
	}
}

impl StdError for Error {}

impl From<crate::tsgen::Error> for Error {
	fn from(error: crate::tsgen::Error) -> Self {
		Self::runtime(error.to_string())
	}
}

impl From<Error> for vorma_tasks::Error<BoxError> {
	fn from(error: Error) -> Self {
		Self::Failed(Arc::new(Box::new(error) as BoxError))
	}
}

/// View error with an optional client-safe message.
#[derive(Debug)]
pub struct ViewError {
	/// Message that may be sent to the browser/client.
	pub client_msg: String,
	/// Server-side source error.
	pub err: Option<BoxError>,
}

impl fmt::Display for ViewError {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		if let Some(err) = &self.err {
			return write!(f, "{err}");
		}
		if !self.client_msg.is_empty() {
			return write!(f, "{}", self.client_msg);
		}
		write!(f, "unknown view error")
	}
}

impl StdError for ViewError {
	fn source(&self) -> Option<&(dyn StdError + 'static)> {
		self.err.as_ref().map(|err| err.as_ref() as &dyn StdError)
	}
}

/// Provides an optional client-safe message for handler errors.
pub trait ViewErrorClientMsg {
	/// Return a client-safe error message, if this error type carries one.
	fn view_error_client_msg(&self) -> Option<&str> {
		None
	}
}

impl ViewErrorClientMsg for ViewError {
	fn view_error_client_msg(&self) -> Option<&str> {
		if self.client_msg.is_empty() {
			return None;
		}
		Some(&self.client_msg)
	}
}

impl ViewErrorClientMsg for Error {}

impl ViewErrorClientMsg for BoxError {
	fn view_error_client_msg(&self) -> Option<&str> {
		self.downcast_ref::<ViewError>()
			.and_then(ViewErrorClientMsg::view_error_client_msg)
	}
}

impl ViewErrorClientMsg for String {}

impl ViewErrorClientMsg for &'static str {}

#[cfg(test)]
mod tests {
	use super::ViewError;

	#[test]
	fn view_error_display_matches_fallback_order() {
		let err = ViewError {
			client_msg: "client".to_owned(),
			err: Some("server".into()),
		};
		assert_eq!(err.to_string(), "server");

		let err = ViewError {
			client_msg: "client".to_owned(),
			err: None,
		};
		assert_eq!(err.to_string(), "client");

		let err = ViewError {
			client_msg: String::new(),
			err: None,
		};
		assert_eq!(err.to_string(), "unknown view error");
	}
}
