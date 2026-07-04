//! Exact overlap detection between two matchers.
//!
//! [`find_overlap`] decides whether some concrete request path is accepted
//! by both of two matchers — either type, in either position — and returns
//! the found [`Overlap`] (an example path plus the pattern each side
//! matched) when one exists. `None` means the two matchers are provably
//! disjoint over every possible request path.
//!
//! Acceptance is each matcher's own, exactly as built: a [`FlatMatcher`]
//! accepts the paths `find_best_match` resolves, a [`NestedMatcher`]
//! accepts the paths `find_nested_matches` resolves. Registration context
//! is therefore fully respected — a pattern whose matches are taken by
//! sibling registrations (for example a root catch-all yielding to a
//! covering match) contributes exactly what the real matcher would
//! serve, nothing more.
//!
//! The decision is exact, not heuristic: candidate paths are drawn from a
//! finite family that provably contains a witness whenever any witness
//! exists, and every candidate is evaluated by the two real matchers, so
//! the result can never drift from the matching semantics it describes.
//!
//! # Completeness of the candidate family
//!
//! Suppose some path is accepted by both sides. Each side pins it to one
//! of its registered patterns: the flat side's best match, or the leaf of
//! the nested side's claiming chain. For that pattern pair, the shared
//! path can be canonicalized without losing either side's acceptance:
//!
//! - Segment count: splat tails are unconstrained beyond their one-or-more
//!   minimum, so the path shrinks to at most
//!   `max(fixed_len(a), fixed_len(b)) + 1` real segments.
//! - Trailing slashes: at most one trailing slash ever matches (the
//!   dirty-path rule), and acceptance distinguishes only counts 0 and 1,
//!   so enumerating counts 0, 1, and 2 covers every behavior class with
//!   one count to spare.
//! - Segment values: positions pinned by either pattern's literal keep
//!   that literal; every other position takes one fresh symbol chosen to
//!   equal no literal registered in either matcher. The grammar has no
//!   constraint coupling two positions, so per-position replacement
//!   preserves the pair's acceptance — and because the fresh symbol
//!   matches no static segment of any other registered pattern, the
//!   canonical path competes with at most the patterns the original did,
//!   which only ever widens set-level acceptance, never narrows it.
//!
//! The resulting candidate is in the enumerated family, so evaluating the
//! family against both full matchers decides overlap exactly. The
//! brute-force oracle suite in `tests/overlap.rs` pins this argument
//! against exhaustive path enumeration across all four type pairings.

use crate::matcher::{FlatMatcher, NestedMatcher};
use crate::pattern::Pattern;
use crate::segment::SegmentKind;

/// One found overlap between two matchers, returned by [`find_overlap`]:
/// a concrete shared request path, plus the pattern each side matched
/// it with.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Overlap {
	example_path: String,
	left_pattern: String,
	right_pattern: String,
}

impl Overlap {
	/// One concrete request path accepted by both matchers — an example,
	/// since overlapping matchers generally share infinitely many.
	pub fn example_path(&self) -> &str {
		&self.example_path
	}

	/// Normalized pattern the left matcher resolved the example path to.
	pub fn left_pattern(&self) -> &str {
		&self.left_pattern
	}

	/// Normalized pattern the right matcher resolved the example path to.
	pub fn right_pattern(&self) -> &str {
		&self.right_pattern
	}
}

mod sealed {
	use crate::pattern::Pattern;

	pub trait Sealed {
		fn registered_patterns(&self) -> Vec<&Pattern>;
		fn accepts_path(&self, path: &str) -> bool;
		fn matched_pattern_for(&self, path: &str) -> Option<String>;
	}
}

/// A matcher type [`find_overlap`] can take in either position.
///
/// Implemented by [`FlatMatcher`] and [`NestedMatcher`] — the two sides
/// passed to one [`find_overlap`] call need not be the same matcher
/// type. Sealed against outside implementations: an `OverlapSide` must
/// answer overlap questions using its type's own real matching
/// algorithm, which only this crate's two matcher types can provide.
pub trait OverlapSide: sealed::Sealed {}

impl OverlapSide for FlatMatcher {}
impl OverlapSide for NestedMatcher {}

impl sealed::Sealed for FlatMatcher {
	fn registered_patterns(&self) -> Vec<&Pattern> {
		self.engine().registered_patterns()
	}

	fn accepts_path(&self, path: &str) -> bool {
		self.find_best_match(path).is_some()
	}

	fn matched_pattern_for(&self, path: &str) -> Option<String> {
		self.find_best_match(path)
			.map(|found| found.pattern.normalized_pattern().to_owned())
	}
}

impl sealed::Sealed for NestedMatcher {
	fn registered_patterns(&self) -> Vec<&Pattern> {
		self.engine().registered_patterns()
	}

	fn accepts_path(&self, path: &str) -> bool {
		self.find_nested_matches(path).is_some()
	}

	fn matched_pattern_for(&self, path: &str) -> Option<String> {
		self.find_nested_matches(path).map(|found| {
			found
				.matches
				.last()
				.expect("a nested match result carries at least one match")
				.pattern
				.normalized_pattern()
				.to_owned()
		})
	}
}

/// Find a concrete request path accepted by both `left` and `right`, or
/// prove there is none.
///
/// `None` means `left` and `right` are disjoint over every possible
/// request path — a caller can rely on that as a proof, not a
/// heuristic best-effort answer (see the module-level docs for why).
/// The result is deterministic: the same two matchers always produce
/// the same answer and the same example path.
///
/// Reach for this when an application builds more than one matcher
/// over the same request space and needs to know whether they could
/// ever both claim one path — for example checking that a public API
/// matcher and an internal admin matcher, registered independently,
/// never silently compete for the same route.
///
/// ```
/// use vorma_matcher::{MatcherBuilder, Options, find_overlap};
///
/// fn flat_matcher(pattern: &str) -> vorma_matcher::FlatMatcher {
///     let mut builder = MatcherBuilder::new(Options::default()).unwrap();
///     builder.register_pattern(pattern).unwrap();
///     builder.finish_flat()
/// }
///
/// let dynamic_side = flat_matcher("/users/:id");
/// let static_side = flat_matcher("/users/export");
/// let overlap = find_overlap(&dynamic_side, &static_side).unwrap();
/// assert_eq!(overlap.example_path(), "/users/export");
/// assert_eq!(overlap.left_pattern(), "/users/:id");
/// assert_eq!(overlap.right_pattern(), "/users/export");
///
/// let disjoint_side = flat_matcher("/posts/:id");
/// assert!(find_overlap(&dynamic_side, &disjoint_side).is_none());
/// ```
pub fn find_overlap(left: &impl OverlapSide, right: &impl OverlapSide) -> Option<Overlap> {
	let fresh = fresh_symbol(left, right);
	for a in left.registered_patterns() {
		for b in right.registered_patterns() {
			for path in candidate_paths(a, b, &fresh) {
				if left.accepts_path(&path) && right.accepts_path(&path) {
					return Some(Overlap {
						left_pattern: left
							.matched_pattern_for(&path)
							.expect("an accepted path resolves to a pattern"),
						right_pattern: right
							.matched_pattern_for(&path)
							.expect("an accepted path resolves to a pattern"),
						example_path: path,
					});
				}
			}
		}
	}
	None
}

// A placeholder segment value equal to no static literal registered in
// either matcher, so canonical candidates never accidentally collide
// with a third pattern's literal.
fn fresh_symbol(left: &impl OverlapSide, right: &impl OverlapSide) -> String {
	let mut symbol = "z".to_owned();
	let collides = |symbol: &str| {
		left.registered_patterns()
			.iter()
			.chain(right.registered_patterns().iter())
			.flat_map(|pattern| pattern.normalized_segments.iter())
			.any(|segment| {
				segment.kind == SegmentKind::Static && segment.normalized_value == symbol
			})
	};
	while collides(&symbol) {
		symbol.push('z');
	}
	symbol
}

// Fixed (non-splat, non-index) segments of a pattern: the positions that
// constrain a witness path's real segments.
fn fixed_segments(pattern: &Pattern) -> Vec<FixedConstraint> {
	pattern
		.normalized_segments
		.iter()
		.filter_map(|segment| match segment.kind {
			SegmentKind::Static => Some(FixedConstraint::Literal(segment.normalized_value.clone())),
			SegmentKind::Dynamic => Some(FixedConstraint::AnyNonEmpty),
			SegmentKind::Splat | SegmentKind::Index => None,
		})
		.collect()
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum FixedConstraint {
	Literal(String),
	AnyNonEmpty,
}

// Every candidate witness path for the pair, friendliest first: fewer
// segments before more, fewer trailing slashes before more.
fn candidate_paths(a: &Pattern, b: &Pattern, fresh: &str) -> Vec<String> {
	let fixed_a = fixed_segments(a);
	let fixed_b = fixed_segments(b);
	let max_real_segments = fixed_a.len().max(fixed_b.len()) + 1;

	let mut paths = Vec::new();
	for real_segments in 0..=max_real_segments {
		let Some(values) = canonical_segment_values(&fixed_a, &fixed_b, real_segments, fresh)
		else {
			continue;
		};
		for trailing_slashes in 0..=2usize {
			paths.push(assemble_path(&values, trailing_slashes));
		}
	}
	paths
}

// The one canonical value assignment for a candidate of `real_segments`
// segments, or `None` when two required literals conflict at a position.
fn canonical_segment_values(
	fixed_a: &[FixedConstraint],
	fixed_b: &[FixedConstraint],
	real_segments: usize,
	fresh: &str,
) -> Option<Vec<String>> {
	let mut values = Vec::with_capacity(real_segments);
	for position in 0..real_segments {
		let literal_a = match fixed_a.get(position) {
			Some(FixedConstraint::Literal(value)) => Some(value.as_str()),
			_ => None,
		};
		let literal_b = match fixed_b.get(position) {
			Some(FixedConstraint::Literal(value)) => Some(value.as_str()),
			_ => None,
		};
		let value = match (literal_a, literal_b) {
			(Some(left), Some(right)) if left != right => return None,
			(Some(left), _) => left.to_owned(),
			(None, Some(right)) => right.to_owned(),
			(None, None) => fresh.to_owned(),
		};
		values.push(value);
	}
	Some(values)
}

fn assemble_path(values: &[String], trailing_slashes: usize) -> String {
	if values.is_empty() {
		return "/".repeat(trailing_slashes);
	}
	let mut path = String::new();
	for value in values {
		path.push('/');
		path.push_str(value);
	}
	path.push_str(&"/".repeat(trailing_slashes));
	path
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::builder::MatcherBuilder;
	use crate::options::Options;

	fn pattern(text: &str) -> Pattern {
		MatcherBuilder::new(Options::default())
			.unwrap()
			.normalize_pattern(text)
			.unwrap()
	}

	#[test]
	fn normalized_patterns_reregister_to_themselves_under_default_options() {
		for text in [
			"", "/", "/*", "/a", "/a/", "/:x", "/:x/", "/a/*", "/a/b", "/a/:y", "/a/:y/", "/a/b/",
			"/a/b/*",
		] {
			let first = pattern(text);
			let second = pattern(first.normalized_pattern());
			assert_eq!(
				first.normalized_pattern(),
				second.normalized_pattern(),
				"pattern {text:?} must round-trip through normalization"
			);
			assert_eq!(
				first.normalized_segments(),
				second.normalized_segments(),
				"pattern {text:?} segments must round-trip through normalization"
			);
		}
	}

	#[test]
	fn candidate_paths_skip_conflicting_literal_positions() {
		let a = pattern("/users/export");
		let b = pattern("/users/import");
		let candidates = candidate_paths(&a, &b, "z");
		assert!(
			candidates
				.iter()
				.all(|path| !path.contains("export") || !path.contains("import")),
			"conflicting literals must not co-occur: {candidates:?}"
		);
	}

	#[test]
	fn fresh_symbol_avoids_registered_literals() {
		let mut builder = MatcherBuilder::new(Options::default()).unwrap();
		builder.register_pattern("/z").unwrap();
		builder.register_pattern("/zz/a").unwrap();
		let left = builder.finish_flat();
		let mut builder = MatcherBuilder::new(Options::default()).unwrap();
		builder.register_pattern("/q").unwrap();
		let right = builder.finish_flat();
		assert_eq!(fresh_symbol(&left, &right), "zzz");
	}
}
