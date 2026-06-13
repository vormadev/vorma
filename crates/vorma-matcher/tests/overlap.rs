//! Overlap detection: exhaustive class-pair pins plus brute-force
//! oracles that check `find_overlap` against direct path enumeration
//! evaluated by the real matchers, across all four type pairings.

use vorma_matcher::{
	FlatMatcher, MatcherBuilder, NestedMatcher, Options, Overlap, Pattern, SegmentKind,
	find_overlap,
};

fn pattern(text: &str) -> Pattern {
	MatcherBuilder::new(Options::default())
		.unwrap()
		.normalize_pattern(text)
		.unwrap()
}

fn flat_matcher(patterns: &[&str]) -> FlatMatcher {
	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	for p in patterns {
		builder.register_pattern(p).unwrap();
	}
	builder.finish_flat()
}

fn nested_matcher(patterns: &[&str]) -> NestedMatcher {
	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	for p in patterns {
		builder.register_pattern(p).unwrap();
	}
	builder.finish_nested()
}

fn assert_witness_replays(
	overlap: &Overlap,
	left_accepts: bool,
	right_accepts: bool,
	left_matched: Option<String>,
	right_matched: Option<String>,
	context: &str,
) {
	assert!(left_accepts, "{context}: left must accept the example path");
	assert!(
		right_accepts,
		"{context}: right must accept the example path"
	);
	assert_eq!(
		left_matched.as_deref(),
		Some(overlap.left_pattern()),
		"{context}: left attribution must match a replay"
	);
	assert_eq!(
		right_matched.as_deref(),
		Some(overlap.right_pattern()),
		"{context}: right attribution must match a replay"
	);
}

#[test]
fn nested_vs_flat_overlap_pins() {
	let cases: &[(&str, &str, Option<&str>)] = &[
		// Under the dirty-path rule a bare trailing slash is noise, so a
		// static view and a same-prefix splat resource share nothing:
		// the view owns the bare path, the splat owns real children.
		("/users", "/users/*", None),
		("/docs", "/docs/*", None),
		// A dynamic nested segment admits any literal a flat pattern pins.
		("/s/:story_id", "/s/export", Some("/s/export")),
		// A nested splat claims every deeper path a flat pattern matches.
		(
			"/docs/*",
			"/docs/manifest.json",
			Some("/docs/manifest.json"),
		),
		("/docs/*", "/docs/a/b/c", Some("/docs/a/b/c")),
		// The root catch-all claims everything.
		("/*", "/api/items", Some("/api/items")),
		("/*", "/", Some("/")),
		// Root family: the empty pattern claims only the root path.
		("", "/", Some("/")),
		("", "/health", None),
		("/", "/health", None),
		// An index pattern claims its parent path on its own shape; no
		// registered sibling is required, so the index alone overlaps
		// the resource at the parent path.
		("/a/:x/", "/a/b", Some("/a/b")),
		// Empty segments never match: a trailing dynamic segment has no
		// path form in common with its index sibling shape.
		("/users/:user", "/users/", None),
		// Disjoint statics, depth mismatches, and literal conflicts.
		("/a", "/b", None),
		("/a/:x", "/a/:y/c", None),
		("/users/export", "/users/import", None),
		("/u/:username", "/api/users/:username", None),
		// Splats never claim their bare root, so the parent path alone
		// does not overlap; only trailing-slash forms do (pinned above).
		("/docs/*", "/docs", None),
	];

	for (view_text, flat_text, expected) in cases {
		let view = nested_matcher(&[view_text]);
		let flat = flat_matcher(&[flat_text]);
		let context = format!("find_overlap(nested[{view_text:?}], flat[{flat_text:?}])");
		let got = find_overlap(&view, &flat);
		assert_eq!(
			got.as_ref().map(Overlap::example_path),
			*expected,
			"{context}"
		);
		if let Some(overlap) = got {
			let path = overlap.example_path();
			assert_witness_replays(
				&overlap,
				view.find_nested_matches(path).is_some(),
				flat.find_best_match(path).is_some(),
				view.find_nested_matches(path).map(|found| {
					found
						.matches
						.last()
						.unwrap()
						.pattern
						.normalized_pattern()
						.to_owned()
				}),
				flat.find_best_match(path)
					.map(|found| found.pattern.normalized_pattern().to_owned()),
				&context,
			);
		}
	}
}

#[test]
fn flat_vs_flat_overlap_pins() {
	let cases: &[(&str, &str, Option<&str>)] = &[
		// The reserved-prefix space check shape: a splat over a prefix
		// overlaps any concrete path beneath it.
		("/static/*", "/static/app.css", Some("/static/app.css")),
		// The bare prefix itself shares nothing with the splat: splats
		// require a real segment, and a trailing slash is just noise.
		("/static/*", "/static", None),
		// Same-shape dynamics from independent matchers share paths.
		("/users/:id", "/users/:slug", Some("/users/z")),
		// A plain static and its index form share the trailing-slash
		// path: the index matches it exactly, the static by tolerance.
		("/a", "/a/", Some("/a/")),
		// Root family.
		("", "/", Some("/")),
		("/*", "/", Some("/")),
		("/*", "", Some("")),
		// Disjoint.
		("/a", "/b", None),
		("/a/b", "/a", None),
	];

	for (a_text, b_text, expected) in cases {
		let a = flat_matcher(&[a_text]);
		let b = flat_matcher(&[b_text]);
		let context = format!("find_overlap(flat[{a_text:?}], flat[{b_text:?}])");
		let got = find_overlap(&a, &b);
		assert_eq!(
			got.as_ref().map(Overlap::example_path),
			*expected,
			"{context}"
		);
		if let Some(overlap) = got {
			let path = overlap.example_path();
			assert_witness_replays(
				&overlap,
				a.find_best_match(path).is_some(),
				b.find_best_match(path).is_some(),
				a.find_best_match(path)
					.map(|found| found.pattern.normalized_pattern().to_owned()),
				b.find_best_match(path)
					.map(|found| found.pattern.normalized_pattern().to_owned()),
				&context,
			);
		}
	}
}

// Multi-pattern matchers: every pattern pair is covered, and the
// returned attribution names the patterns each full matcher actually
// resolves the example path to.
#[test]
fn set_level_overlap_finds_and_attributes() {
	let flat_a = flat_matcher(&["/api/items", "/api/users/:id"]);
	let flat_b = flat_matcher(&["/health", "/api/users/export"]);
	let overlap = find_overlap(&flat_a, &flat_b).unwrap();
	assert_eq!(overlap.example_path(), "/api/users/export");
	assert_eq!(overlap.left_pattern(), "/api/users/:id");
	assert_eq!(overlap.right_pattern(), "/api/users/export");

	let views = nested_matcher(&["", "/s/:story_id", "/docs", "/docs/*"]);
	let resources = flat_matcher(&["/metrics", "/s/export"]);
	let overlap = find_overlap(&views, &resources).unwrap();
	assert_eq!(overlap.example_path(), "/s/export");
	assert_eq!(overlap.left_pattern(), "/s/:story_id");
	assert_eq!(overlap.right_pattern(), "/s/export");

	let disjoint = flat_matcher(&["/metrics", "/jobs/:id"]);
	assert_eq!(
		find_overlap(&nested_matcher(&["", "/docs"]), &disjoint),
		None
	);
}

// The catch-all yields only to a covering match: a dead prefix is not
// a match, so the catch-all takes the path — and a completing chain
// through the prefix takes it back.
#[test]
fn nested_catch_all_yields_only_to_covering_matches() {
	let resource = flat_matcher(&["/foo/bar"]);

	let dead_prefix = nested_matcher(&["", "/foo", "/*"]);
	let overlap = find_overlap(&dead_prefix, &resource).unwrap();
	assert_eq!(overlap.example_path(), "/foo/bar");
	assert_eq!(overlap.left_pattern(), "/*");

	let completing = nested_matcher(&["", "/foo", "/foo/:id", "/*"]);
	let overlap = find_overlap(&completing, &resource).unwrap();
	assert_eq!(overlap.example_path(), "/foo/bar");
	assert_eq!(overlap.left_pattern(), "/foo/:id");
}

// Acceptance is the nested matcher's own, dynamic-index edition: an
// index pattern claims its parent path on its own shape, with or
// without its non-index sibling registered alongside it.
#[test]
fn dynamic_index_overlap_needs_no_registered_sibling() {
	let resource = flat_matcher(&["/a/b"]);

	for patterns in [&["/a/:x/"][..], &["/a/:x/", "/a/:x"][..]] {
		let view = nested_matcher(patterns);
		let overlap = find_overlap(&view, &resource).unwrap();
		assert_eq!(overlap.example_path(), "/a/b", "patterns = {patterns:?}");
		assert_eq!(
			overlap.left_pattern(),
			"/a/:x/",
			"the index leaf claims the parent path (patterns = {patterns:?})"
		);
	}
}

// The placeholder segment must avoid every literal registered in either
// matcher: with "z" registered as a view, a naive "z" placeholder would
// be suppressed by the "/z" prefix and the real overlap (any other first
// segment) would be missed.
#[test]
fn fresh_placeholder_avoids_registered_literals() {
	let views = nested_matcher(&["", "/z", "/*"]);
	let resource = flat_matcher(&["/:x/y"]);
	let overlap = find_overlap(&views, &resource).unwrap();
	assert_eq!(overlap.example_path(), "/zz/y");
	assert_eq!(overlap.left_pattern(), "/*");
	assert_eq!(overlap.right_pattern(), "/:x/y");

	// The registered literal is a dead prefix for this path, so the
	// catch-all claims it too.
	let pinned = flat_matcher(&["/z/y"]);
	let overlap = find_overlap(&views, &pinned).unwrap();
	assert_eq!(overlap.example_path(), "/z/y");
	assert_eq!(overlap.left_pattern(), "/*");
}

enum Side {
	Flat(FlatMatcher),
	Nested(NestedMatcher),
}

const MODES: [&str; 2] = ["flat", "nested"];

impl Side {
	fn build(mode: &str, patterns: &[&str]) -> Side {
		match mode {
			"flat" => Side::Flat(flat_matcher(patterns)),
			"nested" => Side::Nested(nested_matcher(patterns)),
			other => panic!("unknown mode {other:?}"),
		}
	}

	fn accepts(&self, path: &str) -> bool {
		match self {
			Side::Flat(matcher) => matcher.find_best_match(path).is_some(),
			Side::Nested(matcher) => matcher.find_nested_matches(path).is_some(),
		}
	}

	fn matched_pattern(&self, path: &str) -> Option<String> {
		match self {
			Side::Flat(matcher) => matcher
				.find_best_match(path)
				.map(|found| found.pattern.normalized_pattern().to_owned()),
			Side::Nested(matcher) => matcher.find_nested_matches(path).map(|found| {
				found
					.matches
					.last()
					.unwrap()
					.pattern
					.normalized_pattern()
					.to_owned()
			}),
		}
	}

	fn overlap_with(&self, right: &Side) -> Option<Overlap> {
		match (self, right) {
			(Side::Flat(l), Side::Flat(r)) => find_overlap(l, r),
			(Side::Flat(l), Side::Nested(r)) => find_overlap(l, r),
			(Side::Nested(l), Side::Flat(r)) => find_overlap(l, r),
			(Side::Nested(l), Side::Nested(r)) => find_overlap(l, r),
		}
	}
}

fn fixed_literals(patterns: &[&str]) -> Vec<Vec<Option<String>>> {
	patterns
		.iter()
		.map(|text| {
			pattern(text)
				.normalized_segments()
				.iter()
				.filter_map(|segment| match segment.kind {
					SegmentKind::Static => Some(Some(segment.normalized_value.clone())),
					SegmentKind::Dynamic => Some(None),
					SegmentKind::Splat | SegmentKind::Index => None,
				})
				.collect()
		})
		.collect()
}

// Every path over the pairing's literal alphabet plus a fresh symbol,
// for every length in the relevant band and trailing-slash counts 0..=3.
// The extra length and extra slash deliberately exceed the bounds
// find_overlap relies on, so this enumeration also stresses its
// finite-model argument.
fn oracle_paths(left_patterns: &[&str], right_patterns: &[&str]) -> Vec<String> {
	let all_fixed: Vec<Vec<Option<String>>> = fixed_literals(left_patterns)
		.into_iter()
		.chain(fixed_literals(right_patterns))
		.collect();
	let max_len = all_fixed
		.iter()
		.map(Vec::len)
		.max()
		.unwrap_or(0)
		.saturating_add(2);

	let mut paths = Vec::new();
	for len in 0..=max_len {
		let mut alphabets = Vec::with_capacity(len);
		for position in 0..len {
			let mut alphabet: Vec<String> = Vec::new();
			for fixed in &all_fixed {
				if let Some(Some(lit)) = fixed.get(position)
					&& !alphabet.contains(lit)
				{
					alphabet.push(lit.clone());
				}
			}
			alphabet.push("q".to_owned());
			alphabets.push(alphabet);
		}
		let mut indexes = vec![0usize; len];
		loop {
			let mut path = String::new();
			for (position, index) in indexes.iter().enumerate() {
				path.push('/');
				path.push_str(&alphabets[position][*index]);
			}
			for trailing in 0..=3usize {
				if path.is_empty() {
					paths.push("/".repeat(trailing));
				} else {
					paths.push(format!("{}{}", path, "/".repeat(trailing)));
				}
			}
			let mut position = len;
			loop {
				if position == 0 {
					break;
				}
				position -= 1;
				indexes[position] += 1;
				if indexes[position] < alphabets[position].len() {
					break;
				}
				indexes[position] = 0;
			}
			if indexes.iter().all(|index| *index == 0) {
				break;
			}
		}
	}
	paths.sort();
	paths.dedup();
	paths
}

fn assert_overlap_agrees_with_brute_force(
	left_patterns: &[&str],
	right_patterns: &[&str],
	left_mode: &str,
	right_mode: &str,
) {
	let left = Side::build(left_mode, left_patterns);
	let right = Side::build(right_mode, right_patterns);

	let brute_force = oracle_paths(left_patterns, right_patterns)
		.into_iter()
		.find(|path| left.accepts(path) && right.accepts(path));
	let got = left.overlap_with(&right);

	let context = format!(
		"{left_mode}[{left_patterns:?}] vs {right_mode}[{right_patterns:?}]: function = {got:?}, brute force = {brute_force:?}"
	);
	assert_eq!(got.is_some(), brute_force.is_some(), "{context}");
	if let Some(overlap) = got {
		let path = overlap.example_path();
		assert!(
			left.accepts(path) && right.accepts(path),
			"{context}: witness must replay"
		);
		assert_eq!(
			left.matched_pattern(path).as_deref(),
			Some(overlap.left_pattern()),
			"{context}: left attribution"
		);
		assert_eq!(
			right.matched_pattern(path).as_deref(),
			Some(overlap.right_pattern()),
			"{context}: right attribution"
		);
	}
}

const ORACLE_PATTERNS: &[&str] = &[
	"", "/", "/*", "/a", "/a/", "/b", "/:x", "/:x/", "/a/*", "/:x/*", "/a/b", "/a/:y", "/a/:y/",
	"/a/b/", "/a/b/*", "/a/:y/c",
];

#[test]
fn single_pattern_overlap_agrees_with_brute_force_in_all_four_pairings() {
	for left_text in ORACLE_PATTERNS {
		for right_text in ORACLE_PATTERNS {
			for left_mode in MODES {
				for right_mode in MODES {
					assert_overlap_agrees_with_brute_force(
						&[left_text],
						&[right_text],
						left_mode,
						right_mode,
					);
				}
			}
		}
	}
}

const ORACLE_SET_PAIRS: &[(&[&str], &[&str])] = &[
	(&["", "/foo", "/*"], &["/foo/bar"]),
	(&["", "/*"], &["/foo/bar"]),
	(&["/a/:x/", "/a/:x"], &["/a/b"]),
	(&["/a/:x/"], &["/a/b"]),
	(&["", "/z", "/*"], &["/:x/y"]),
	(&["", "/docs", "/docs/*"], &["/docs/manifest", "/metrics"]),
	(&["/users", "/users/"], &["/users/*"]),
	(&["", "/s/:story_id"], &["/s/export", "/api/s/:id"]),
];

#[test]
fn multi_pattern_overlap_agrees_with_brute_force_in_all_four_pairings() {
	for (left_patterns, right_patterns) in ORACLE_SET_PAIRS {
		for left_mode in MODES {
			for right_mode in MODES {
				assert_overlap_agrees_with_brute_force(
					left_patterns,
					right_patterns,
					left_mode,
					right_mode,
				);
			}
		}
	}
}
