//! C-compatible browser WASM route matcher ABI for Vorma's TypeScript client.

#![deny(missing_docs)]
#![allow(unsafe_code)]
#![deny(unsafe_op_in_unsafe_fn)]

mod abi;
mod encoding;
mod output;
mod registry;

use abi::{STATUS_ERROR, STATUS_MATCH, STATUS_NO_MATCH};
#[cfg(test)]
use encoding::push_len;

#[unsafe(no_mangle)]
/// Allocate WASM memory for a caller-provided byte buffer.
pub extern "C" fn vorma_client_matcher_alloc(len: usize) -> *mut u8 {
	abi::alloc(len)
}

#[unsafe(no_mangle)]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
/// Deallocate WASM memory previously returned by [`vorma_client_matcher_alloc`].
pub extern "C" fn vorma_client_matcher_dealloc(ptr: *mut u8, len: usize) {
	abi::dealloc(ptr, len);
}

#[unsafe(no_mangle)]
/// Create a new client matcher and return its opaque id.
pub extern "C" fn vorma_client_matcher_new() -> u32 {
	registry::new_matcher()
}

#[unsafe(no_mangle)]
/// Free a matcher id returned by [`vorma_client_matcher_new`].
pub extern "C" fn vorma_client_matcher_free(matcher_id: u32) {
	registry::free_matcher(matcher_id);
}

#[unsafe(no_mangle)]
/// Register one UTF-8 pattern buffer with a matcher.
pub extern "C" fn vorma_client_matcher_register_pattern(
	matcher_id: u32,
	ptr: *const u8,
	len: usize,
) -> u32 {
	let Some(pattern) = abi::read_str(ptr, len) else {
		return STATUS_ERROR;
	};
	registry::register_pattern(matcher_id, pattern).map_or(STATUS_ERROR, |_| STATUS_MATCH)
}

#[unsafe(no_mangle)]
/// Find nested matches for one UTF-8 path buffer and publish the encoded output.
pub extern "C" fn vorma_client_matcher_find_nested_matches(
	matcher_id: u32,
	ptr: *const u8,
	len: usize,
) -> u32 {
	let Some(path) = abi::read_str(ptr, len) else {
		output::set_output(Vec::new());
		return STATUS_ERROR;
	};

	match registry::find_nested_matches(matcher_id, path) {
		Ok(Some(out)) => {
			output::set_output(out);
			STATUS_MATCH
		}
		Ok(None) => {
			output::set_output(Vec::new());
			STATUS_NO_MATCH
		}
		Err(()) => {
			output::set_output(Vec::new());
			STATUS_ERROR
		}
	}
}

#[unsafe(no_mangle)]
/// Pointer to the last encoded matcher output buffer.
pub extern "C" fn vorma_client_matcher_output_ptr() -> *const u8 {
	output::output_ptr()
}

#[unsafe(no_mangle)]
/// Length of the last encoded matcher output buffer.
pub extern "C" fn vorma_client_matcher_output_len() -> usize {
	output::output_len()
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
