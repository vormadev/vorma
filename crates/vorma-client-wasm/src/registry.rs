use std::sync::Mutex;

use vorma_matcher::{MatcherBuilder, NestedMatcher, Options};

use crate::encoding::encode_nested_match;

static MATCHERS: Mutex<Vec<Option<MatcherSlot>>> = Mutex::new(Vec::new());

struct MatcherSlot {
	builder: MatcherBuilder,
	matcher: Option<NestedMatcher>,
}

pub(crate) fn new_matcher() -> u32 {
	let Ok(builder) = MatcherBuilder::new(Options {
		explicit_index_segment_identifier: "_index".to_owned(),
		..Options::default()
	}) else {
		return 0;
	};

	let mut matchers = MATCHERS
		.lock()
		.expect("client matcher registry should not be poisoned");
	{
		for (index, slot) in matchers.iter_mut().enumerate() {
			if slot.is_none() {
				*slot = Some(MatcherSlot::new(builder));
				return match u32::try_from(index + 1) {
					Ok(id) => id,
					Err(_) => {
						*slot = None;
						0
					}
				};
			}
		}

		matchers.push(Some(MatcherSlot::new(builder)));
		match u32::try_from(matchers.len()) {
			Ok(id) => id,
			Err(_) => {
				matchers.pop();
				0
			}
		}
	}
}

pub(crate) fn free_matcher(matcher_id: u32) {
	if matcher_id == 0 {
		return;
	}

	let mut matchers = MATCHERS
		.lock()
		.expect("client matcher registry should not be poisoned");
	let Some(slot) = matchers.get_mut((matcher_id - 1) as usize) else {
		return;
	};
	*slot = None;
}

pub(crate) fn register_pattern(matcher_id: u32, pattern: &str) -> Result<(), ()> {
	if !is_valid_vorma_route_pattern(pattern) {
		return Err(());
	}
	with_matcher_slot_mut(matcher_id, |slot| slot.register_pattern(pattern))
		.ok_or(())?
		.map_err(|_| ())
}

pub(crate) fn find_nested_matches(matcher_id: u32, path: &str) -> Result<Option<Vec<u8>>, ()> {
	with_matcher_slot_mut(matcher_id, |slot| {
		let matcher = slot.matcher();
		let Some(matched) = matcher.find_nested_matches(path) else {
			return Ok(None);
		};
		encode_nested_match(&matched).map(Some)
	})
	.ok_or(())?
}

impl MatcherSlot {
	fn new(builder: MatcherBuilder) -> Self {
		Self {
			builder,
			matcher: None,
		}
	}

	fn register_pattern(&mut self, pattern: &str) -> Result<(), String> {
		self.builder.register_pattern(pattern)?;
		self.matcher = None;
		Ok(())
	}

	fn matcher(&mut self) -> &NestedMatcher {
		if self.matcher.is_none() {
			self.matcher = Some(self.builder.clone().finish_nested());
		}

		self.matcher
			.as_ref()
			.expect("client matcher should be built")
	}
}

fn with_matcher_slot_mut<T>(matcher_id: u32, f: impl FnOnce(&mut MatcherSlot) -> T) -> Option<T> {
	if matcher_id == 0 {
		return None;
	}

	let mut matchers = MATCHERS
		.lock()
		.expect("client matcher registry should not be poisoned");
	let slot = matchers.get_mut((matcher_id - 1) as usize)?;
	let matcher = slot.as_mut()?;
	Some(f(matcher))
}

fn is_valid_vorma_route_pattern(pattern: &str) -> bool {
	!pattern.is_empty() && pattern.starts_with('/')
}
