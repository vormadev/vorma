use std::sync::Mutex;

use vorma_matcher::{Matcher, MatcherBuilder, Options};

const STATUS_NO_MATCH: u32 = 0;
const STATUS_MATCH: u32 = 1;
const STATUS_ERROR: u32 = 2;

static MATCHERS: Mutex<Vec<Option<MatcherSlot>>> = Mutex::new(Vec::new());
static OUTPUT: Mutex<Vec<u8>> = Mutex::new(Vec::new());

struct MatcherSlot {
	builder: MatcherBuilder,
	matcher: Option<Matcher>,
}

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_alloc(len: usize) -> *mut u8 {
	let mut buffer = Vec::<u8>::with_capacity(len);
	let ptr = buffer.as_mut_ptr();
	std::mem::forget(buffer);
	ptr
}

#[unsafe(no_mangle)]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn vorma_client_matcher_dealloc(ptr: *mut u8, len: usize) {
	if ptr.is_null() {
		return;
	}

	unsafe {
		drop(Vec::from_raw_parts(ptr, 0, len));
	}
}

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_new() -> u32 {
	let Ok(builder) = Matcher::builder(Options {
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

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_free(matcher_id: u32) {
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

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_register_pattern(
	matcher_id: u32,
	ptr: *const u8,
	len: usize,
) -> u32 {
	let Some(pattern) = read_str(ptr, len) else {
		return STATUS_ERROR;
	};
	if !is_valid_vorma_route_pattern(pattern) {
		return STATUS_ERROR;
	}

	with_matcher_slot_mut(matcher_id, |slot| {
		if slot.register_pattern(pattern).is_ok() {
			STATUS_MATCH
		} else {
			STATUS_ERROR
		}
	})
	.unwrap_or(STATUS_ERROR)
}

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_find_nested_matches(
	matcher_id: u32,
	ptr: *const u8,
	len: usize,
) -> u32 {
	let Some(path) = read_str(ptr, len) else {
		set_output(Vec::new());
		return STATUS_ERROR;
	};

	with_matcher_slot_mut(matcher_id, |slot| {
		let matcher = slot.matcher();
		let Some(matched) = matcher.find_nested_matches(path) else {
			set_output(Vec::new());
			return STATUS_NO_MATCH;
		};

		match encode_nested_match(&matched) {
			Ok(out) => {
				set_output(out);
				STATUS_MATCH
			}
			Err(()) => {
				set_output(Vec::new());
				STATUS_ERROR
			}
		}
	})
	.unwrap_or_else(|| {
		set_output(Vec::new());
		STATUS_ERROR
	})
}

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_output_ptr() -> *const u8 {
	OUTPUT
		.lock()
		.expect("client matcher output should not be poisoned")
		.as_ptr()
}

#[unsafe(no_mangle)]
pub extern "C" fn vorma_client_matcher_output_len() -> usize {
	OUTPUT
		.lock()
		.expect("client matcher output should not be poisoned")
		.len()
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

	fn matcher(&mut self) -> &Matcher {
		if self.matcher.is_none() {
			self.matcher = Some(self.builder.clone().finish());
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

fn set_output(output: Vec<u8>) {
	*OUTPUT
		.lock()
		.expect("client matcher output should not be poisoned") = output;
}

fn read_str<'a>(ptr: *const u8, len: usize) -> Option<&'a str> {
	if len == 0 {
		return Some("");
	}
	if ptr.is_null() {
		return None;
	}

	let bytes = unsafe { std::slice::from_raw_parts(ptr, len) };
	std::str::from_utf8(bytes).ok()
}

fn is_valid_vorma_route_pattern(pattern: &str) -> bool {
	!pattern.is_empty() && pattern.starts_with('/')
}

fn push_u32(out: &mut Vec<u8>, value: u32) {
	out.extend_from_slice(&value.to_le_bytes());
}

fn push_len(out: &mut Vec<u8>, value: usize) -> Result<(), ()> {
	let value = u32::try_from(value).map_err(|_| ())?;
	push_u32(out, value);
	Ok(())
}

fn push_string(out: &mut Vec<u8>, value: &str) -> Result<(), ()> {
	push_len(out, value.len())?;
	out.extend_from_slice(value.as_bytes());
	Ok(())
}

fn encode_nested_match(matched: &vorma_matcher::NestedMatches) -> Result<Vec<u8>, ()> {
	let mut out = Vec::new();
	let mut params = matched.params.iter().collect::<Vec<_>>();
	params.sort_by(|left, right| left.0.cmp(right.0));

	push_len(&mut out, params.len())?;
	for (key, value) in params {
		push_string(&mut out, key)?;
		push_string(&mut out, value)?;
	}

	push_len(&mut out, matched.splat_values.len())?;
	for value in &matched.splat_values {
		push_string(&mut out, value)?;
	}

	push_len(&mut out, matched.matches.len())?;
	for item in &matched.matches {
		push_string(&mut out, item.pattern.original_pattern())?;
	}

	Ok(out)
}

#[cfg(test)]
mod tests {
	use std::sync::Mutex;

	use super::*;

	static TEST_LOCK: Mutex<()> = Mutex::new(());

	#[derive(Debug, Eq, PartialEq)]
	struct DecodedNestedMatch {
		params: Vec<(String, String)>,
		splat_values: Vec<String>,
		patterns: Vec<String>,
	}

	#[test]
	fn matcher_abi_round_trips_nested_matches() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let matcher_id = vorma_client_matcher_new();

		assert_ne!(matcher_id, 0);
		assert_eq!(register_pattern(matcher_id, "/"), STATUS_MATCH);
		assert_eq!(
			register_pattern(matcher_id, "/stories/:story_id"),
			STATUS_MATCH
		);
		assert_eq!(
			register_pattern(matcher_id, "/stories/:story_id/comments/:comment_id"),
			STATUS_MATCH,
		);

		let status = find_nested_matches(matcher_id, "/stories/42/comments/9");

		assert_eq!(status, STATUS_MATCH);
		assert_eq!(
			decode_nested_match(&output_bytes()),
			DecodedNestedMatch {
				params: vec![
					("comment_id".to_owned(), "9".to_owned()),
					("story_id".to_owned(), "42".to_owned()),
				],
				splat_values: Vec::new(),
				patterns: vec![
					"/".to_owned(),
					"/stories/:story_id".to_owned(),
					"/stories/:story_id/comments/:comment_id".to_owned(),
				],
			},
		);

		vorma_client_matcher_free(matcher_id);
	}

	#[test]
	fn matcher_abi_distinguishes_no_match_from_error() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let matcher_id = vorma_client_matcher_new();

		assert_ne!(matcher_id, 0);
		assert_eq!(register_pattern(matcher_id, "/docs/*"), STATUS_MATCH);
		assert_eq!(
			find_nested_matches(matcher_id, "/stories/42"),
			STATUS_NO_MATCH
		);
		assert_eq!(output_bytes(), Vec::<u8>::new());
		vorma_client_matcher_free(matcher_id);
		assert_eq!(find_nested_matches(matcher_id, "/docs/intro"), STATUS_ERROR);
	}

	#[test]
	fn matcher_abi_rejects_invalid_patterns() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let matcher_id = vorma_client_matcher_new();

		assert_ne!(matcher_id, 0);
		assert_eq!(register_pattern(matcher_id, ""), STATUS_ERROR);
		assert_eq!(register_pattern(matcher_id, "relative"), STATUS_ERROR);
		assert_eq!(register_pattern(matcher_id, "/:"), STATUS_ERROR);
		assert_eq!(register_pattern(matcher_id, "/valid/:id"), STATUS_MATCH);
		assert_eq!(find_nested_matches(matcher_id, "/valid/42"), STATUS_MATCH);

		vorma_client_matcher_free(matcher_id);
	}

	#[test]
	fn abi_length_encoding_rejects_u32_overflow() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let mut out = Vec::new();

		assert_eq!(push_len(&mut out, (u32::MAX as usize) + 1), Err(()));
		assert!(out.is_empty());
	}

	fn register_pattern(matcher_id: u32, pattern: &str) -> u32 {
		with_input(pattern, |ptr, len| {
			vorma_client_matcher_register_pattern(matcher_id, ptr, len)
		})
	}

	fn find_nested_matches(matcher_id: u32, path: &str) -> u32 {
		with_input(path, |ptr, len| {
			vorma_client_matcher_find_nested_matches(matcher_id, ptr, len)
		})
	}

	fn with_input<T>(value: &str, f: impl FnOnce(*const u8, usize) -> T) -> T {
		let bytes = value.as_bytes();
		let ptr = vorma_client_matcher_alloc(bytes.len());
		unsafe {
			std::ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, bytes.len());
		}
		let result = f(ptr, bytes.len());
		vorma_client_matcher_dealloc(ptr, bytes.len());
		result
	}

	fn output_bytes() -> Vec<u8> {
		let ptr = vorma_client_matcher_output_ptr();
		let len = vorma_client_matcher_output_len();
		if len == 0 {
			return Vec::new();
		}

		unsafe { std::slice::from_raw_parts(ptr, len).to_vec() }
	}

	fn decode_nested_match(bytes: &[u8]) -> DecodedNestedMatch {
		let mut offset = 0;
		let param_count = read_u32(bytes, &mut offset);
		let mut params = Vec::new();
		for _ in 0..param_count {
			let key = read_string(bytes, &mut offset);
			let value = read_string(bytes, &mut offset);
			params.push((key, value));
		}

		let splat_count = read_u32(bytes, &mut offset);
		let mut splat_values = Vec::new();
		for _ in 0..splat_count {
			splat_values.push(read_string(bytes, &mut offset));
		}

		let pattern_count = read_u32(bytes, &mut offset);
		let mut patterns = Vec::new();
		for _ in 0..pattern_count {
			patterns.push(read_string(bytes, &mut offset));
		}

		DecodedNestedMatch {
			params,
			splat_values,
			patterns,
		}
	}

	fn read_u32(bytes: &[u8], offset: &mut usize) -> u32 {
		let value = u32::from_le_bytes(bytes[*offset..*offset + 4].try_into().expect("u32 bytes"));
		*offset += 4;
		value
	}

	fn read_string(bytes: &[u8], offset: &mut usize) -> String {
		let len = read_u32(bytes, offset) as usize;
		let value = std::str::from_utf8(&bytes[*offset..*offset + len])
			.expect("utf-8 string")
			.to_owned();
		*offset += len;
		value
	}
}
