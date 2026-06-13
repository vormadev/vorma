//! Specificity ordering: doctrine pins plus an agreement oracle proving
//! `compare_specificity` names the same winner `find_best_match` picks.

use std::cmp::Ordering;

use vorma_matcher::{MatcherBuilder, Options, Pattern, compare_specificity, find_overlap};

fn pattern(text: &str) -> Pattern {
	MatcherBuilder::new(Options::default())
		.unwrap()
		.normalize_pattern(text)
		.unwrap()
}

#[test]
fn specificity_orders_competing_patterns() {
	let cases: &[(&str, &str, Ordering)] = &[
		// Score dominates: static segments outrank dynamics and splats.
		("/s/export", "/s/:story_id", Ordering::Greater),
		("/bob/sally", "/bob/*", Ordering::Greater),
		("/docs/manifest.json", "/docs/*", Ordering::Greater),
		("/users/:id", "/users/*", Ordering::Greater),
		// Equal scores fall to the leftmost differing position rank.
		("/a/:x", "/:y/b", Ordering::Greater),
		// A trailing splat loses to a non-splat ending at equal ranks.
		("/docs", "/docs/*", Ordering::Greater),
		("", "/*", Ordering::Greater),
		// The root catch-all is the floor of the order.
		("/*", "/a", Ordering::Less),
		("/*", "/:x", Ordering::Less),
		("/*", "/a/*", Ordering::Less),
		// Ties: identical shapes have no principled winner.
		("/s/:story_id", "/s/:id", Ordering::Equal),
		("/health", "/health", Ordering::Equal),
		("/a/:x/*", "/a/:y/*", Ordering::Equal),
		// Equal ranks without a shared path also compare Equal; Equal
		// only signals a conflict when the patterns overlap.
		("/a/", "/a/b", Ordering::Equal),
	];

	for (left_text, right_text, expected) in cases {
		let left = pattern(left_text);
		let right = pattern(right_text);
		assert_eq!(
			compare_specificity(&left, &right),
			*expected,
			"compare_specificity({left_text:?}, {right_text:?})"
		);
		assert_eq!(
			compare_specificity(&right, &left),
			expected.reverse(),
			"compare_specificity({right_text:?}, {left_text:?})"
		);
	}
}

const AGREEMENT_PATTERNS: &[&str] = &[
	"", "/", "/*", "/a", "/b", "/:x", "/a/*", "/:x/*", "/a/b", "/a/:y", "/:x/b", "/a/b/*",
	"/a/:y/c", "/a/b/c",
];

// For every catalog pair that legally co-registers and shares a path,
// the flat matcher's pick for that path must be exactly the pattern
// compare_specificity names — the ordering is one implementation, so
// dispatch-by-comparison and matching can never disagree.
#[test]
fn flat_matching_agrees_with_compare_specificity() {
	let mut checked = 0usize;
	for left_text in AGREEMENT_PATTERNS {
		for right_text in AGREEMENT_PATTERNS {
			if left_text == right_text {
				continue;
			}
			let left = pattern(left_text);
			let right = pattern(right_text);

			let mut left_solo = MatcherBuilder::new(Options::default()).unwrap();
			left_solo.register_pattern(left_text).unwrap();
			let mut right_solo = MatcherBuilder::new(Options::default()).unwrap();
			right_solo.register_pattern(right_text).unwrap();
			let Some(overlap) = find_overlap(&left_solo.finish_flat(), &right_solo.finish_flat())
			else {
				continue;
			};

			let mut both = MatcherBuilder::new(Options::default()).unwrap();
			both.register_pattern(left_text).unwrap();
			if both.register_pattern(right_text).is_err() {
				// Same-shape pairs cannot co-register; ties are decided
				// at registration, not at match time.
				assert_eq!(
					compare_specificity(&left, &right),
					Ordering::Equal,
					"only ties may be rejected: ({left_text:?}, {right_text:?})"
				);
				continue;
			}
			let matcher = both.finish_flat();

			let winner = matcher
				.find_best_match(overlap.example_path())
				.expect("the shared path must match the combined matcher")
				.pattern
				.normalized_pattern()
				.to_owned();
			let expected = match compare_specificity(&left, &right) {
				Ordering::Greater => left.normalized_pattern().to_owned(),
				Ordering::Less => right.normalized_pattern().to_owned(),
				Ordering::Equal => panic!(
					"overlapping co-registered patterns must be ordered: ({left_text:?}, {right_text:?})"
				),
			};
			assert_eq!(
				winner,
				expected,
				"path {:?} from ({left_text:?}, {right_text:?})",
				overlap.example_path()
			);
			checked += 1;
		}
	}
	assert!(
		checked >= 20,
		"agreement oracle must exercise a real pair population, got {checked}"
	);
}
