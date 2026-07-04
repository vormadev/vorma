//! Standalone path-pattern parsing and matching: no HTTP types, no I/O,
//! no async — a pure function from registered pattern text and a
//! concrete request path to the pattern (or ordered chain of patterns)
//! that should serve it.
//!
//! # Getting started
//!
//! Build a [`MatcherBuilder`], register every route pattern your
//! application owns, then finish it into an immutable matcher:
//!
//! ```
//! use vorma_matcher::{MatcherBuilder, Options};
//!
//! let mut builder = MatcherBuilder::new(Options::default())?;
//! builder.register_pattern("/users/:user_id")?;
//! builder.register_pattern("/users/:user_id/posts/:post_id")?;
//! let matcher = builder.finish_flat();
//!
//! let found = matcher.find_best_match("/users/42").unwrap();
//! assert_eq!(found.pattern.normalized_pattern(), "/users/:user_id");
//! assert_eq!(&found.params["user_id"], "42");
//! # Ok::<(), String>(())
//! ```
//!
//! # Two matcher shapes, one pattern grammar
//!
//! [`MatcherBuilder::finish_flat`] and [`MatcherBuilder::finish_nested`]
//! share pattern validation, normalization, and the specificity
//! doctrine below, but resolve a path differently:
//!
//! - [`FlatMatcher`] answers "which single pattern serves this path?" —
//!   the shape a plain HTTP router wants.
//! - [`NestedMatcher`] answers "which ordered chain of patterns, from
//!   outermost to innermost, serves this path?" — the shape a
//!   file-system-routed, layout-nesting framework wants (an index
//!   pattern claims its parent path; ancestor layouts ride along).
//!
//! Register the same pattern text with both builders and they parse and
//! normalize it identically; only resolution differs. A pattern created
//! by one is never accepted by the other's finish method — the split is
//! enforced at the type level, not by a runtime mode flag.
//!
//! # Specificity: the one ordering that decides every competition
//!
//! Registered patterns are allowed to overlap — many patterns may be
//! *capable* of matching one path. [`compare_specificity`] is the single
//! total order that decides which one wins, and every part of this
//! crate (matching, registration-time collision checks,
//! [`find_overlap`]) delegates to it rather than reimplementing it:
//!
//! 1. Higher total segment score wins first. Each segment scores by
//!    kind — static and index segments score highest, a dynamic
//!    parameter scores next, a splat scores lowest — and a pattern's
//!    score is the sum across its segments. `/users/export` outranks
//!    `/users/:user_id`, which outranks `/users/*`.
//! 2. A tied score falls to the leftmost segment position where the two
//!    patterns' segment kinds differ; whichever pattern is more specific
//!    at that position wins.
//! 3. A tied ordering still standing loses to any pattern with a
//!    non-splat ending — a trailing splat always tolerates a more exact
//!    finish, at equal rank.
//! 4. A tied ordering standing yet falls to segment count — the longer
//!    pattern wins.
//!
//! `Ordering::Equal` out of [`compare_specificity`] means neither
//! pattern can outrank the other. When two such patterns are also
//! capable of matching a common path, that is a genuine tie the matcher
//! has no principled way to break — [`MatcherBuilder::register_pattern`]
//! rejects registering the second one as a route shape collision, so a
//! live matcher can never hit an unresolved tie at request time.
//!
//! # The dirty-path rule
//!
//! Every matcher applies one uniform rule to path hygiene, regardless of
//! pattern kind:
//!
//! - At most one trailing slash is tolerated as noise: `/a/b` and
//!   `/a/b/` match the same pattern (unless that pattern is an index
//!   pattern, whose canonical spelling *is* the trailing-slash form and
//!   therefore requires it).
//! - An empty path segment never matches anything. A doubled slash —
//!   anywhere in the path, including a doubled trailing slash — spells
//!   an empty segment, so it matches nothing.
//! - Captured parameter and splat values are consequently never empty
//!   strings.
//! - The root catch-all pattern (`/*`) matches the root path itself
//!   (`/`), with no captured splat values — there is no segment there to
//!   capture.
//!
//! # The catch-all cover rule (nested matching)
//!
//! In [`NestedMatcher`], a root catch-all (`/*`) is the natural answer
//! for a custom "not found" view, but a *prefix* hit is never treated as
//! a match on its own: `/*` yields to another registered pattern only
//! when that pattern's own chain actually completes through the
//! requested path. If nothing else covers the path, the catch-all
//! claims it. This is what makes `/*` a complete custom-404 answer
//! rather than a pattern that silently shadows deeper, more specific
//! routes it was never meant to intercept.
//!
//! # Errors are strings, on purpose
//!
//! Pattern validation and registration return `Result<_, String>`. These
//! errors describe malformed pattern *text* supplied by the application
//! author at startup (a splat segment that is not the pattern's final
//! segment, a duplicate parameter name within one pattern, a route
//! shape collision between two patterns) — they are development-time
//! mistakes to fix in source, not values an application branches on at
//! runtime. A string is the right shape for "read this and fix your
//! route table."

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
#![forbid(unsafe_code)]

mod builder;
mod match_result;
mod matcher;
mod options;
mod overlap;
mod parse;
mod pattern;
mod segment;
mod tree;

pub use builder::MatcherBuilder;
pub use match_result::{Match, NestedMatch, NestedMatches, Params, SplatValues};
pub use matcher::{FlatMatcher, NestedMatcher};
pub use options::Options;
pub use overlap::{Overlap, OverlapSide, find_overlap};
pub use parse::parse_segments;
pub use pattern::{Pattern, compare_specificity};
pub use segment::{Segment, SegmentKind};

/// Return `path` with exactly one leading `/` added, if it did not
/// already have one.
///
/// [`MatcherBuilder::register_pattern`] and [`FlatMatcher::find_best_match`]
/// / [`NestedMatcher::find_nested_matches`] all require an absolute path
/// already. Reach for this helper when path text arrives from a source
/// that does not guarantee that — a config file, a CLI argument, a
/// pattern authored by an application user — and needs to become
/// absolute before it is handed to the matcher.
///
/// ```
/// use vorma_matcher::ensure_leading_slash;
///
/// assert_eq!(ensure_leading_slash("users"), "/users");
/// assert_eq!(ensure_leading_slash("/users"), "/users");
/// ```
pub fn ensure_leading_slash(path: &str) -> String {
	if path.starts_with('/') {
		path.to_owned()
	} else {
		format!("/{path}")
	}
}

/// Return `path` with exactly one trailing `/` added, if it did not
/// already have one.
///
/// Reach for this when composing a path known to be the parent of a
/// nested path — for example building `{parent}{child}` and needing a
/// guaranteed separator between the two — without caring whether the
/// parent text already carried one.
///
/// ```
/// use vorma_matcher::ensure_trailing_slash;
///
/// assert_eq!(ensure_trailing_slash("/users"), "/users/");
/// assert_eq!(ensure_trailing_slash("/users/"), "/users/");
/// ```
pub fn ensure_trailing_slash(path: &str) -> String {
	if path.ends_with('/') {
		path.to_owned()
	} else {
		format!("{path}/")
	}
}

/// Return `path` with one leading `/` removed, if present.
///
/// Removes at most one slash — `"//users"` becomes `"/users"`, not
/// `"users"` — matching the crate's dirty-path rule that only a single
/// separator is ever noise. Reach for this when comparing or joining
/// path text outside the matcher's own algorithms, where a leading
/// slash would otherwise have to be special-cased at every call site.
///
/// ```
/// use vorma_matcher::strip_leading_slash;
///
/// assert_eq!(strip_leading_slash("/users"), "users");
/// assert_eq!(strip_leading_slash("users"), "users");
/// ```
pub fn strip_leading_slash(path: &str) -> &str {
	if path.starts_with('/') {
		path.strip_prefix('/').unwrap_or(path)
	} else {
		path
	}
}

/// Return `path` with one trailing `/` removed, if present.
///
/// Removes at most one slash — `"users//"` becomes `"users/"`, not
/// `"users"` — matching the crate's dirty-path rule that only a single
/// separator is ever noise.
///
/// ```
/// use vorma_matcher::strip_trailing_slash;
///
/// assert_eq!(strip_trailing_slash("/users/"), "/users");
/// assert_eq!(strip_trailing_slash("/users"), "/users");
/// ```
pub fn strip_trailing_slash(path: &str) -> &str {
	if path.ends_with('/') {
		path.strip_suffix('/').unwrap_or(path)
	} else {
		path
	}
}

/// Return `path` with both a leading and a trailing `/` added, if
/// either was missing. Equivalent to [`ensure_leading_slash`] composed
/// with [`ensure_trailing_slash`].
///
/// Reach for this when normalizing a path segment into the
/// "surrounded by slashes" form some path-joining logic wants as a
/// uniform building block, regardless of what separators the input
/// text happened to already carry.
///
/// ```
/// use vorma_matcher::ensure_leading_and_trailing_slash;
///
/// assert_eq!(ensure_leading_and_trailing_slash("users"), "/users/");
/// assert_eq!(ensure_leading_and_trailing_slash("/users/"), "/users/");
/// ```
pub fn ensure_leading_and_trailing_slash(path: &str) -> String {
	ensure_leading_slash(&ensure_trailing_slash(path))
}
