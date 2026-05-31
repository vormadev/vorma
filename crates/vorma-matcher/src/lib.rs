//! Route pattern validation and matching.

#![forbid(unsafe_code)]

mod builder;
mod match_result;
mod matcher;
mod options;
mod parse;
mod pattern;
mod segment;
mod tree;

pub use builder::MatcherBuilder;
pub use match_result::{Match, NestedMatch, NestedMatches, Params};
pub use matcher::Matcher;
pub use options::Options;
pub use parse::parse_segments;
pub use pattern::Pattern;
pub use segment::{Segment, SegmentKind};

pub fn has_leading_slash(p: &str) -> bool {
	p.starts_with('/')
}

pub fn has_trailing_slash(p: &str) -> bool {
	p.ends_with('/')
}

pub fn ensure_leading_slash(p: &str) -> String {
	if has_leading_slash(p) {
		p.to_owned()
	} else {
		format!("/{p}")
	}
}

pub fn ensure_trailing_slash(p: &str) -> String {
	if has_trailing_slash(p) {
		p.to_owned()
	} else {
		format!("{p}/")
	}
}

pub fn strip_leading_slash(p: &str) -> &str {
	if has_leading_slash(p) { &p[1..] } else { p }
}

pub fn strip_trailing_slash(p: &str) -> &str {
	if has_trailing_slash(p) {
		&p[..p.len() - 1]
	} else {
		p
	}
}

pub fn ensure_leading_and_trailing_slash(p: &str) -> String {
	ensure_leading_slash(&ensure_trailing_slash(p))
}
