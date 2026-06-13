//! Path pattern validation and matching.

#![deny(missing_docs)]
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

/// Return `path` with a leading `/`.
pub fn ensure_leading_slash(path: &str) -> String {
	if path.starts_with('/') {
		path.to_owned()
	} else {
		format!("/{path}")
	}
}

/// Return `path` with a trailing `/`.
pub fn ensure_trailing_slash(path: &str) -> String {
	if path.ends_with('/') {
		path.to_owned()
	} else {
		format!("{path}/")
	}
}

/// Return `path` without one leading `/`, if present.
pub fn strip_leading_slash(path: &str) -> &str {
	if path.starts_with('/') {
		path.strip_prefix('/').unwrap_or(path)
	} else {
		path
	}
}

/// Return `path` without one trailing `/`, if present.
pub fn strip_trailing_slash(path: &str) -> &str {
	if path.ends_with('/') {
		path.strip_suffix('/').unwrap_or(path)
	} else {
		path
	}
}

/// Return `path` with both leading and trailing `/` separators.
pub fn ensure_leading_and_trailing_slash(path: &str) -> String {
	ensure_leading_slash(&ensure_trailing_slash(path))
}
