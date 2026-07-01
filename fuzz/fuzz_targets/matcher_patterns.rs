#![no_main]

use libfuzzer_sys::fuzz_target;
use vorma_matcher::{MatcherBuilder, Options};

const MAX_PATTERNS: usize = 32;
const MAX_PATHS: usize = 32;

fuzz_target!(|data: &[u8]| {
	let Ok(input) = std::str::from_utf8(data) else {
		return;
	};

	let Ok(mut builder) = MatcherBuilder::new(Options::default()) else {
		return;
	};

	let mut lines = input.lines();
	for pattern in lines.by_ref().take(MAX_PATTERNS) {
		let _ = builder.register_pattern(pattern);
	}

	let flat_matcher = builder.clone().finish_flat();
	let nested_matcher = builder.finish_nested();
	for path in lines.take(MAX_PATHS) {
		let _ = flat_matcher.find_best_match(path);
		let _ = nested_matcher.find_nested_matches(path);
	}
});
