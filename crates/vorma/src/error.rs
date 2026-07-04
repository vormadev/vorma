//! Public framework error type.
/*
The plain framework/setup error: config validation, helper failures
(`bind_addr`, `public_url`), build-time faults. Handler EXITS are a
different concept with different types (`ViewExit`/`HttpExit` in
`exit.rs`); both convert from this type via `?`. Nothing here is ever
client-visible.
*/

use std::error::Error as StdError;
use std::fmt;

/// Boxed error type for chained error sources.
pub type BoxError = Box<dyn StdError + Send + Sync + 'static>;

/// Plain framework/setup error carrying a display message and optional source.
///
/// For config validation and helper failures (like [`bind_addr`](crate::bind_addr))
/// outside a request — distinct from [`ViewExit`](crate::ViewExit)/
/// [`HttpExit`](crate::HttpExit), the handler early-exit types with their own
/// client-visibility rules. `Error` converts into both via `?`, so it works as a plain
/// return type for setup code that only ever runs outside a request (an app's `main`, a
/// document builder) and still composes with `?` inside a handler when needed.
#[derive(Debug)]
pub struct Error {
	message: String,
	source: Option<BoxError>,
}

impl Error {
	/// Create an error from a display message.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
			source: None,
		}
	}

	/// Attach a source error for diagnostics chains.
	pub fn with_source(mut self, source: impl Into<BoxError>) -> Self {
		self.source = Some(source.into());
		self
	}

	pub(crate) fn into_message_and_source(self) -> (String, Option<BoxError>) {
		(self.message, self.source)
	}
}

impl fmt::Display for Error {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.write_str(&self.message)
	}
}

impl StdError for Error {
	fn source(&self) -> Option<&(dyn StdError + 'static)> {
		self.source
			.as_ref()
			.map(|source| source.as_ref() as &dyn StdError)
	}
}

/*
Boxed errors convert with the box retained as source, so `?` on arbitrary
library errors inside handlers keeps the diagnostics chain.
*/
impl From<BoxError> for Error {
	fn from(source: BoxError) -> Self {
		let message = source.to_string();
		Self {
			message,
			source: Some(source),
		}
	}
}

impl From<crate::tsgen::Error> for Error {
	fn from(error: crate::tsgen::Error) -> Self {
		Self::new(error.to_string())
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn error_carries_message_and_optional_source() {
		let error = Error::new("db connection refused");
		assert_eq!(error.to_string(), "db connection refused");
		assert!(StdError::source(&error).is_none());

		let error = Error::new("outer").with_source(std::io::Error::other("inner"));
		assert_eq!(StdError::source(&error).unwrap().to_string(), "inner");
	}
}
