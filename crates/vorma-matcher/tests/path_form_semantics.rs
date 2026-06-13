//! Pins for path-form rules: trailing slashes, doubled slashes, and
//! empty segments, across both matchers.
//!
//! The maintainer-ratified rule: at most one trailing slash is tolerated
//! as noise, and an empty segment never matches anything — so a doubled
//! slash anywhere means no match, and params and splat values never
//! contain empty strings.

use vorma_matcher::{FlatMatcher, MatcherBuilder, NestedMatcher, Options};

fn flat(patterns: &[&str]) -> FlatMatcher {
	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	for p in patterns {
		builder.register_pattern(p).unwrap();
	}
	builder.finish_flat()
}

fn nested(patterns: &[&str]) -> NestedMatcher {
	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	for p in patterns {
		builder.register_pattern(p).unwrap();
	}
	builder.finish_nested()
}

#[test]
fn flat_static_accepts_at_most_one_trailing_slash() {
	let m = flat(&["/a/b"]);
	assert!(m.find_best_match("/a/b").is_some());
	assert!(m.find_best_match("/a/b/").is_some());
	assert!(m.find_best_match("/a/b//").is_none());
	assert!(m.find_best_match("/a/b///").is_none());
}

// Index patterns are the trailing-slash spelling: they require their
// one slash and tolerate nothing further.
#[test]
fn flat_index_requires_exactly_its_trailing_slash() {
	let m = flat(&["/a/b/"]);
	assert!(m.find_best_match("/a/b").is_none());
	assert!(m.find_best_match("/a/b/").is_some());
	assert!(m.find_best_match("/a/b//").is_none());
}

// Dynamic index patterns match the same spelling, capturing the real
// segments only.
#[test]
fn flat_dynamic_index_matches_the_trailing_slash_spelling() {
	let m = flat(&["/a/:x/"]);
	assert!(m.find_best_match("/a/b").is_none());
	let matched = m.find_best_match("/a/b/").unwrap();
	assert_eq!(matched.params["x"], "b");
	assert!(m.find_best_match("/a/b//").is_none());
}

#[test]
fn flat_dynamic_accepts_at_most_one_trailing_slash() {
	let m = flat(&["/a/:x"]);
	assert!(m.find_best_match("/a/b").is_some());
	assert!(m.find_best_match("/a/b/").is_some());
	assert!(m.find_best_match("/a/b//").is_none());
	assert!(m.find_best_match("/a/b///").is_none());
}

// Splats capture real segments only: the bare root does not match
// (one-or-more), its trailing-slash form is the same path and does not
// match either, and splat values never contain empty strings.
#[test]
fn flat_splat_requires_a_real_segment() {
	let m = flat(&["/users/*"]);
	assert!(m.find_best_match("/users").is_none());
	assert!(m.find_best_match("/users/").is_none());
	let matched = m.find_best_match("/users/a").unwrap();
	assert_eq!(matched.splat_values, vec!["a".to_owned()]);
	let matched = m.find_best_match("/users/a/").unwrap();
	assert_eq!(matched.splat_values, vec!["a".to_owned()]);
}

// A doubled slash spells an empty segment, and empty segments match
// nothing — for every pattern kind, in both matchers.
#[test]
fn doubled_slashes_match_nothing() {
	let statics = flat(&["/a/b"]);
	assert!(statics.find_best_match("/a//b").is_none());

	let dynamics = flat(&["/a/:x"]);
	assert!(dynamics.find_best_match("/a//b").is_none());
	assert!(dynamics.find_best_match("/a/b//").is_none());

	let m = nested(&["", "/a", "/a/:x"]);
	assert!(m.find_nested_matches("/a//b").is_none());
	assert!(m.find_nested_matches("/a/b//").is_none());

	let dynamic_leaf = nested(&["", "/users/:user"]);
	assert!(dynamic_leaf.find_nested_matches("/users//").is_none());
	assert!(dynamic_leaf.find_nested_matches("/users/").is_none());
}

#[test]
fn nested_static_strips_exactly_one_trailing_slash() {
	let m = nested(&["", "/a/b"]);
	assert!(m.find_nested_matches("/a/b").is_some());
	assert!(m.find_nested_matches("/a/b/").is_some());
	assert!(m.find_nested_matches("/a/b//").is_none());
}

// Index patterns in the nested matcher claim their parent path, with or
// without its trailing slash.
#[test]
fn nested_index_claims_its_parent_path() {
	let m = nested(&["", "/docs/"]);
	assert!(m.find_nested_matches("/docs").is_some());
	assert!(m.find_nested_matches("/docs/").is_some());
	assert!(m.find_nested_matches("/docs/extra").is_none());
}

// An index pattern claims its parent path on its own; the parent
// pattern need not be registered. The static and dynamic spellings
// behave identically — registering only the index under a dynamic
// directory (no layout sibling) still matches.
#[test]
fn nested_index_claims_its_parent_path_without_the_parent_registered() {
	let static_index = nested(&["", "/docs/"]);
	assert!(static_index.find_nested_matches("/docs").is_some());

	let dynamic_index = nested(&["", "/a/:x/"]);
	let found = dynamic_index.find_nested_matches("/a/b").unwrap();
	let chain: Vec<&str> = found
		.matches
		.iter()
		.map(|m| m.pattern.normalized_pattern())
		.collect();
	assert_eq!(chain, vec!["", "/a/:x/"]);
	assert_eq!(found.params["x"], "b");
	assert!(dynamic_index.find_nested_matches("/a/b/").is_some());
}

// Root-family pins: the empty pattern claims only the root; the root
// catch-all claims the root (with no splat values — there is no segment
// to capture) and everything else.
#[test]
fn root_family_claims() {
	let layout_only = nested(&[""]);
	assert!(layout_only.find_nested_matches("/").is_some());
	assert!(layout_only.find_nested_matches("/x").is_none());

	let catch_all = nested(&["/*"]);
	assert!(catch_all.find_nested_matches("/").is_some());
	assert!(catch_all.find_nested_matches("/x").is_some());
	assert!(catch_all.find_nested_matches("/x/y/z").is_some());

	let flat_catch_all = flat(&["/*"]);
	let matched = flat_catch_all.find_best_match("/").unwrap();
	assert!(matched.splat_values.is_empty());
	let matched = flat_catch_all.find_best_match("/x/y").unwrap();
	assert_eq!(matched.splat_values, vec!["x".to_owned(), "y".to_owned()]);
}
