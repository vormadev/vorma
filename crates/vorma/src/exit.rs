//! Handler early-exit values: the only path to error and redirect outcomes.
//!
//! A view/resource/middleware handler's return type is `Result<Output, Exit>`; the
//! `Exit` type is [`ViewExit`] for view handlers, [`HttpExit`] for resource and
//! middleware handlers. Rust's `?` operator is Vorma's error-handling idiom for
//! handlers: both types implement [`std::error::Error`] and convert from
//! [`crate::Error`], boxed errors, and [`vorma_tasks::Error`], so an ordinary fallible
//! handler body just propagates with `?` instead of hand-building an exit value —
//! reach for the builder methods below ([`err`](ViewExit::err),
//! [`with_client_msg`](ViewExit::with_client_msg), [`with_source`](ViewExit::with_source))
//! only when a handler needs to say more than a plain `?` conversion carries.
//!
//! Views and resources have different exit shapes on purpose: [`ViewExit`] has no HTTP
//! status concept at all, because a view is one segment of a framework-owned rendering
//! protocol, not a standalone HTTP response — only the outermost response ever has a
//! status. [`HttpExit`] (resources and middleware: real HTTP request/response
//! boundaries) carries one, defaulting to 500 when unset.
//!
//! **What the client sees.** Every exit distinguishes a server-side record from
//! client-visible text, and never confuses the two: [`err`](ViewExit::err)'s message is
//! the SERVER-side diagnostic (logs only, never sent to the client), and so is any
//! attached [`with_source`](ViewExit::with_source) error. The *only* text a client ever
//! sees is what [`with_client_msg`](ViewExit::with_client_msg) sets explicitly; without
//! it, the client receives a generic framework message. This means it is always safe to
//! put implementation detail (a SQL error, an internal id, a stack-adjacent detail) into
//! `err`/`with_source` — a handler has to opt in, per exit, to exposing anything to the
//! client.
//!
//! **Redirects are framework-constructed only.** There is no public constructor for a
//! redirect exit; call `ctx.redirect(location)` (or
//! [`redirect_with_status`](crate::ViewCtx::redirect_with_status) for a non-default 3xx
//! status) on a [`ViewCtx`](crate::ViewCtx)/[`ResourceCtx`](crate::ResourceCtx)/
//! [`MiddlewareCtx`](crate::MiddlewareCtx) instead. A redirect must capture request facts
//! (whether the client prefers a client-side vs. browser-native redirect) at the point
//! the ctx has them; a hand-built redirect variant could never make that call correctly.

use std::error::Error as StdError;
use std::fmt;

use http::StatusCode;

use crate::error::BoxError;

/// Early exit from a view handler: a segment error or a redirect.
///
/// See the [crate-root docs](crate#errors-and-early-exits) for the full
/// client-visibility contract this type enforces.
///
/// ```
/// use vorma::ViewExit;
///
/// let exit = ViewExit::err("story 42 not found in the database")
///     .with_client_msg("This story could not be found.");
///
/// // The server-side record is the Display text (goes to logs, never the client).
/// assert_eq!(exit.to_string(), "story 42 not found in the database");
/// ```
#[derive(Debug)]
pub struct ViewExit {
	kind: ExitKind,
}

/// Early exit from a resource or middleware handler: an HTTP error or a redirect.
///
/// See the [crate-root docs](crate#errors-and-early-exits) for the full
/// client-visibility contract this type enforces.
///
/// ```
/// use vorma::{HttpExit, HttpStatusCode};
///
/// let exit = HttpExit::err("unique constraint violated on stories.slug")
///     .with_status(HttpStatusCode::CONFLICT)
///     .with_client_msg("A story with that title already exists.");
///
/// assert_eq!(exit.to_string(), "unique constraint violated on stories.slug");
/// ```
#[derive(Debug)]
pub struct HttpExit {
	kind: ExitKind,
	status: Option<StatusCode>,
}

#[derive(Debug)]
enum ExitKind {
	Err {
		err: String,
		client_msg: Option<String>,
		source: Option<BoxError>,
	},
	Redirect {
		location: String,
	},
}

impl ViewExit {
	/// Exit with an error; `err` is the SERVER-side record (logs/diagnostics).
	pub fn err(err: impl Into<String>) -> Self {
		Self {
			kind: ExitKind::new_err(err),
		}
	}

	/// Set the client-visible error text (the segment error the UI renders).
	pub fn with_client_msg(mut self, client_msg: impl Into<String>) -> Self {
		self.kind.set_client_msg(client_msg);
		self
	}

	/// Attach a source error for diagnostics chains.
	pub fn with_source(mut self, source: impl Into<BoxError>) -> Self {
		self.kind.set_source(source);
		self
	}

	pub(crate) fn redirected(location: impl Into<String>) -> Self {
		Self {
			kind: ExitKind::Redirect {
				location: location.into(),
			},
		}
	}

	pub(crate) fn is_redirect(&self) -> bool {
		matches!(self.kind, ExitKind::Redirect { .. })
	}

	pub(crate) fn client_msg(&self) -> Option<&str> {
		self.kind.client_msg()
	}
}

impl HttpExit {
	/// Exit with an error; `err` is the SERVER-side record (logs/diagnostics).
	pub fn err(err: impl Into<String>) -> Self {
		Self {
			kind: ExitKind::new_err(err),
			status: None,
		}
	}

	/// Set the response status (defaults to 500 when unset).
	pub fn with_status(mut self, status: StatusCode) -> Self {
		self.status = Some(status);
		self
	}

	/// Set the client-visible error text (the error envelope's message).
	pub fn with_client_msg(mut self, client_msg: impl Into<String>) -> Self {
		self.kind.set_client_msg(client_msg);
		self
	}

	/// Attach a source error for diagnostics chains.
	pub fn with_source(mut self, source: impl Into<BoxError>) -> Self {
		self.kind.set_source(source);
		self
	}

	pub(crate) fn redirected(location: impl Into<String>) -> Self {
		Self {
			kind: ExitKind::Redirect {
				location: location.into(),
			},
			status: None,
		}
	}

	pub(crate) fn is_redirect(&self) -> bool {
		matches!(self.kind, ExitKind::Redirect { .. })
	}

	pub(crate) fn client_msg(&self) -> Option<&str> {
		self.kind.client_msg()
	}

	pub(crate) fn status(&self) -> Option<StatusCode> {
		self.status
	}
}

impl ExitKind {
	fn new_err(err: impl Into<String>) -> Self {
		Self::Err {
			err: err.into(),
			client_msg: None,
			source: None,
		}
	}

	/*
	Builders are no-ops on redirect variants; users cannot construct those
	(framework-only), so the only way to hit the no-op is mutating a value
	returned by ctx.redirect, which is returned immediately by convention.
	*/
	fn set_client_msg(&mut self, value: impl Into<String>) {
		if let Self::Err { client_msg, .. } = self {
			*client_msg = Some(value.into());
		}
	}

	fn set_source(&mut self, value: impl Into<BoxError>) {
		if let Self::Err { source, .. } = self {
			*source = Some(value.into());
		}
	}

	fn client_msg(&self) -> Option<&str> {
		match self {
			Self::Err { client_msg, .. } => client_msg.as_deref(),
			Self::Redirect { .. } => None,
		}
	}

	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Err { err, .. } => f.write_str(err),
			Self::Redirect { location } => write!(f, "redirect to {location}"),
		}
	}

	fn source(&self) -> Option<&(dyn StdError + 'static)> {
		match self {
			Self::Err { source, .. } => source
				.as_ref()
				.map(|source| source.as_ref() as &dyn StdError),
			Self::Redirect { .. } => None,
		}
	}
}

impl fmt::Display for ViewExit {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		self.kind.fmt(f)
	}
}

impl fmt::Display for HttpExit {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		self.kind.fmt(f)
	}
}

impl StdError for ViewExit {
	fn source(&self) -> Option<&(dyn StdError + 'static)> {
		self.kind.source()
	}
}

impl StdError for HttpExit {
	fn source(&self) -> Option<&(dyn StdError + 'static)> {
		self.kind.source()
	}
}

impl From<crate::Error> for ViewExit {
	fn from(error: crate::Error) -> Self {
		let (message, source) = error.into_message_and_source();
		let mut exit = Self::err(message);
		if let Some(source) = source {
			exit = exit.with_source(source);
		}
		exit
	}
}

impl From<crate::Error> for HttpExit {
	fn from(error: crate::Error) -> Self {
		let (message, source) = error.into_message_and_source();
		let mut exit = Self::err(message);
		if let Some(source) = source {
			exit = exit.with_source(source);
		}
		exit
	}
}

impl From<BoxError> for ViewExit {
	fn from(source: BoxError) -> Self {
		Self::err(source.to_string()).with_source(source)
	}
}

impl From<BoxError> for HttpExit {
	fn from(source: BoxError) -> Self {
		Self::err(source.to_string()).with_source(source)
	}
}

/*
Concrete, not blanket-generic-over-E: a conservative start that widens only
by a new ruling. `Failed` is the one variant with a real payload, so it is
the one case with a source to preserve; boxing the `Arc<crate::Error>`
itself (rather than cloning out a fresh `crate::Error`) keeps the ENTIRE
original chain walkable — `Arc<T>`'s std `Error` impl forwards `source()`
to `T`, so a caller that did `.with_source(...)` on the application error
is still one more `source()` hop away, exactly as if no task had been
involved. The four runtime variants (`Cancelled`, `Cycle`,
`MissingOverride`, `TypeMismatch`) carry no payload, so their `Display`
text alone is the server record and no source is attached.

`Cancelled` specifically: it is produced only when the resolving
`ExecCtx`'s own cancellation token was already set (see `vorma-tasks`
resolve/parallel-batch entry checks). Inside this engine that token is
cancelled either at whole-request teardown (after every handler has
already returned) or, mid-request, only on invocations already excluded
from the response (a losing same-phase sibling whose output the engine
discards positionally, or a not-yet-started next-phase invocation that
never runs at all) — see `cancel_execution_contexts` in
`execution_engine.rs`. A handler that still gets to return a value the
engine will use can therefore never observe its own context cancelled.
Given that, `Cancelled` gets the plain default-exit treatment rather than
a special case: pinned in `exit.rs`'s test suite so the choice reads as
deliberate, not accidental.
*/
impl From<vorma_tasks::Error<crate::Error>> for ViewExit {
	fn from(error: vorma_tasks::Error<crate::Error>) -> Self {
		match error {
			vorma_tasks::Error::Failed(source) => Self::err(source.to_string()).with_source(source),
			other => Self::err(other.to_string()),
		}
	}
}

impl From<vorma_tasks::Error<crate::Error>> for HttpExit {
	fn from(error: vorma_tasks::Error<crate::Error>) -> Self {
		match error {
			vorma_tasks::Error::Failed(source) => Self::err(source.to_string()).with_source(source),
			other => Self::err(other.to_string()),
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn exits_default_to_server_side_only() {
		let view = ViewExit::err("db lookup failed");
		assert_eq!(view.to_string(), "db lookup failed");
		assert_eq!(view.client_msg(), None);
		assert!(!view.is_redirect());

		let http = HttpExit::err("create rejected");
		assert_eq!(http.status(), None);
		assert_eq!(http.client_msg(), None);
	}

	#[test]
	fn exit_builders_carry_client_facts_and_sources() {
		let http = HttpExit::err("create note rejected: blank body")
			.with_status(StatusCode::BAD_REQUEST)
			.with_client_msg("note body is required")
			.with_source(std::io::Error::other("inner"));

		assert_eq!(http.status(), Some(StatusCode::BAD_REQUEST));
		assert_eq!(http.client_msg(), Some("note body is required"));
		assert_eq!(StdError::source(&http).unwrap().to_string(), "inner");
		assert_eq!(http.to_string(), "create note rejected: blank body");
	}

	#[test]
	fn framework_errors_convert_with_sources_chained() {
		let exit: ViewExit = crate::Error::new("public url missing").into();
		assert_eq!(exit.to_string(), "public url missing");
		assert_eq!(exit.client_msg(), None);

		let boxed: BoxError = Box::new(std::io::Error::other("io broke"));
		let exit: HttpExit = boxed.into();
		assert_eq!(exit.to_string(), "io broke");
		assert!(StdError::source(&exit).is_some());
	}

	#[test]
	fn redirect_variants_display_their_target_and_ignore_err_builders() {
		let exit = ViewExit::redirected("/login").with_client_msg("ignored");
		assert!(exit.is_redirect());
		assert_eq!(exit.client_msg(), None);
		assert_eq!(exit.to_string(), "redirect to /login");
	}

	// A source two hops deep (task -> application error -> io error) proves
	// nothing gets flattened: `Failed` boxes the `Arc<crate::Error>` itself,
	// not a re-stringified copy, so a source attached to the application
	// error is still reachable by walking one hop further.
	fn failed_task_error_with_chained_source() -> vorma_tasks::Error<crate::Error> {
		let application_error =
			crate::Error::new("stats read failed").with_source(std::io::Error::other("disk full"));
		vorma_tasks::Error::from(application_error)
	}

	#[test]
	fn failed_task_errors_convert_into_view_exit_preserving_the_whole_chain() {
		let exit: ViewExit = failed_task_error_with_chained_source().into();

		assert_eq!(exit.to_string(), "stats read failed");
		assert_eq!(exit.client_msg(), None, "safe default, no client text set");

		let hop_one = StdError::source(&exit).expect("application error is the first source hop");
		assert_eq!(hop_one.to_string(), "stats read failed");
		let hop_two = hop_one
			.source()
			.expect("the application error's own source must still be reachable");
		assert_eq!(hop_two.to_string(), "disk full");
		assert!(
			hop_two.source().is_none(),
			"chain ends exactly where the original error's chain ended"
		);
	}

	#[test]
	fn failed_task_errors_convert_into_http_exit_preserving_the_whole_chain() {
		let exit: HttpExit = failed_task_error_with_chained_source().into();

		assert_eq!(exit.to_string(), "stats read failed");
		assert_eq!(exit.status(), None, "framework default status applies");
		assert_eq!(exit.client_msg(), None, "safe default, no client text set");

		let hop_one = StdError::source(&exit).expect("application error is the first source hop");
		assert_eq!(hop_one.to_string(), "stats read failed");
		let hop_two = hop_one
			.source()
			.expect("the application error's own source must still be reachable");
		assert_eq!(hop_two.to_string(), "disk full");
	}

	#[test]
	fn task_runtime_errors_convert_with_display_text_and_no_source() {
		let cycle: vorma_tasks::Error<crate::Error> = vorma_tasks::Error::Cycle {
			task_name: "stats_task",
		};
		let exit: ViewExit = cycle.into();
		assert_eq!(
			exit.to_string(),
			"task cycle detected while resolving stats_task"
		);
		assert!(StdError::source(&exit).is_none());
		assert_eq!(exit.client_msg(), None);

		let missing_override: vorma_tasks::Error<crate::Error> =
			vorma_tasks::Error::MissingOverride {
				task_name: "stats_task",
			};
		let exit: HttpExit = missing_override.into();
		assert_eq!(
			exit.to_string(),
			"task override required before resolving stats_task"
		);
		assert!(StdError::source(&exit).is_none());

		let type_mismatch: vorma_tasks::Error<crate::Error> = vorma_tasks::Error::TypeMismatch {
			task_name: "stats_task",
		};
		let exit: HttpExit = type_mismatch.into();
		assert_eq!(
			exit.to_string(),
			"cached task output had the wrong type for stats_task"
		);
		assert!(StdError::source(&exit).is_none());
	}

	// Pinned, deliberate answer for the one variant a `From` cannot refuse to
	// handle: `Cancelled` gets the plain default-exit treatment, the same as
	// the other payload-free runtime variants, rather than a special case.
	// This is safe because the engine never lets a still-mattering handler
	// observe its own execution context cancelled (see the doc comment on
	// the impls above) — proven independently, on the engine side, by
	// `execution_engine::tests::cancelled_conversion_loses_the_position_race_to_an_earlier_real_error`
	// and its companion `..._wins_the_position_race_when_it_runs_first`
	// (position, not identity, decides which sibling's error is used). This
	// test pins the conversion's own half of that contract: what a
	// `Cancelled` task error becomes if it is ever converted, so the choice
	// is a recorded decision, not an accident.
	#[test]
	fn cancelled_task_errors_convert_to_the_plain_default_exit_form() {
		let cancelled: vorma_tasks::Error<crate::Error> = vorma_tasks::Error::Cancelled;
		let exit: ViewExit = cancelled.into();
		assert_eq!(exit.to_string(), "task cancelled");
		assert!(StdError::source(&exit).is_none());
		assert_eq!(exit.client_msg(), None);
		assert!(!exit.is_redirect());

		let cancelled: vorma_tasks::Error<crate::Error> = vorma_tasks::Error::Cancelled;
		let exit: HttpExit = cancelled.into();
		assert_eq!(exit.to_string(), "task cancelled");
		assert!(StdError::source(&exit).is_none());
		assert_eq!(exit.status(), None);
		assert_eq!(exit.client_msg(), None);
	}
}
