//! C-compatible browser WASM route matcher ABI for Vorma's TypeScript client.
//!
//! This crate compiles `vorma-matcher`'s nested route matcher to a `wasm32-unknown-unknown`
//! artifact (`packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm`, built through
//! `make rust-build-client-wasm`) so the generated TypeScript client can resolve the same
//! nested-view route matches in the browser that the Rust server resolves on the backend —
//! client-side navigation and prefetching need to know which view chain a path matches
//! without a round trip. There is no Rust-side consumer of this crate; every function here
//! is a `pub extern "C" fn` called only from `packages/vorma/core/client_wasm/*.ts`
//! (`matcher.ts` wraps the raw calls below into the ergonomic `ClientMatcher` type
//! TypeScript code actually uses — read it alongside this crate for the full round trip).
//!
//! # The ABI protocol
//!
//! Every string crosses the WASM boundary as raw UTF-8 bytes in WASM linear memory, owned
//! by an allocation this crate tracks itself (WASM has no host-callable destructors, so
//! the caller must explicitly free what it explicitly allocated): a caller
//! [`vorma_client_matcher_alloc`]s a buffer, writes UTF-8 bytes into it directly through
//! the WASM instance's exported memory, passes `(ptr, len)` to a function like
//! [`vorma_client_matcher_register_pattern`] or
//! [`vorma_client_matcher_find_nested_matches`], and then
//! [`vorma_client_matcher_dealloc`]s that same `(ptr, len)` pair — a length mismatch is
//! silently ignored (treated as "not the allocation I tracked") rather than corrupting
//! memory. A registered matcher is addressed by an opaque `u32` id from
//! [`vorma_client_matcher_new`], freed with [`vorma_client_matcher_free`]; id `0` is never
//! valid and signals allocation/construction failure. Every matching call returns a status
//! code (0 = no match, 1 = match, 2 = error — see the individual function docs); a match's
//! encoded result (captured params, splat values, and the matched pattern chain) is
//! published to a single shared output buffer, read back through
//! [`vorma_client_matcher_output_ptr`]/[`vorma_client_matcher_output_len`] immediately
//! after the call that produced it — the output buffer is overwritten by the next
//! matching call, so a caller must decode it before calling again.
//!
//! # Limits
//!
//! Both the maximum size of one input buffer and the maximum number of simultaneously
//! live allocations are bounded (see the allocator's internal constants); exceeding
//! either fails the allocation (a null pointer) rather than growing unbounded, since this
//! crate runs inside a browser tab sharing memory with the rest of the page.

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
#![allow(unsafe_code)]
#![deny(unsafe_op_in_unsafe_fn)]

mod abi;
mod encoding;
mod output;
mod registry;

#[cfg(test)]
use abi::{MAX_INPUT_LEN, MAX_LIVE_ALLOCATIONS};
use abi::{STATUS_ERROR, STATUS_MATCH, STATUS_NO_MATCH};
#[cfg(test)]
use encoding::push_len;

#[unsafe(no_mangle)]
/// Allocate `len` bytes of WASM memory the caller writes UTF-8 input into before passing
/// `(ptr, len)` to a matching function. Returns a null pointer when `len` exceeds the
/// crate's per-allocation limit or the crate's live-allocation cap is already reached;
/// every other allocation must be paired with a matching
/// [`vorma_client_matcher_dealloc`] call.
pub extern "C" fn vorma_client_matcher_alloc(len: usize) -> *mut u8 {
	abi::alloc(len)
}

#[unsafe(no_mangle)]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
/// Deallocate WASM memory previously returned by [`vorma_client_matcher_alloc`]. `ptr`
/// and `len` must be the exact pair a prior `alloc` call returned/was called with; any
/// other pair (already freed, or never allocated) is silently ignored rather than
/// corrupting memory.
pub extern "C" fn vorma_client_matcher_dealloc(ptr: *mut u8, len: usize) {
	abi::dealloc(ptr, len);
}

#[unsafe(no_mangle)]
/// Create a new, empty client matcher and return its opaque id. Returns `0` on
/// construction failure — `0` is never a valid matcher id, so a caller should treat it as
/// a hard error rather than passing it to any other function here.
pub extern "C" fn vorma_client_matcher_new() -> u32 {
	registry::new_matcher()
}

#[unsafe(no_mangle)]
/// Free a matcher id returned by [`vorma_client_matcher_new`]. A `matcher_id` of `0` or
/// one already freed is silently ignored.
pub extern "C" fn vorma_client_matcher_free(matcher_id: u32) {
	registry::free_matcher(matcher_id);
}

#[unsafe(no_mangle)]
/// Register one UTF-8 pattern buffer (written via [`vorma_client_matcher_alloc`]) with a
/// matcher, in the same pattern grammar the server-side matcher accepts (`:name` dynamic
/// segments, `*` splats). Returns `1` (match/success) on success, `2` (error) if
/// `matcher_id` is invalid, the buffer is not valid UTF-8, or the pattern itself is
/// rejected (empty, not absolute, or an otherwise invalid pattern).
pub extern "C" fn vorma_client_matcher_register_pattern(
	matcher_id: u32,
	ptr: *const u8,
	len: usize,
) -> u32 {
	let Some(pattern) = abi::read_str(ptr, len) else {
		return STATUS_ERROR;
	};
	registry::register_pattern(matcher_id, &pattern).map_or(STATUS_ERROR, |_| STATUS_MATCH)
}

#[unsafe(no_mangle)]
/// Find the nested nested-view match chain for one UTF-8 path buffer (written via
/// [`vorma_client_matcher_alloc`]) and publish the encoded result to the shared output
/// buffer (read back through [`vorma_client_matcher_output_ptr`]/
/// [`vorma_client_matcher_output_len`] immediately after this call, before calling
/// again). Returns `0` (no match — the output buffer is empty), `1` (match — the output
/// buffer holds the encoded params/splat-values/pattern-chain), or `2` (error —
/// `matcher_id` is invalid or the buffer is not valid UTF-8; the output buffer is empty).
pub extern "C" fn vorma_client_matcher_find_nested_matches(
	matcher_id: u32,
	ptr: *const u8,
	len: usize,
) -> u32 {
	let Some(path) = abi::read_str(ptr, len) else {
		output::set_output(Vec::new());
		return STATUS_ERROR;
	};

	match registry::find_nested_matches(matcher_id, &path) {
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
/// Pointer to the output buffer [`vorma_client_matcher_find_nested_matches`] most
/// recently published (valid until the next call to that function).
pub extern "C" fn vorma_client_matcher_output_ptr() -> *const u8 {
	output::output_ptr()
}

#[unsafe(no_mangle)]
/// Length of the output buffer [`vorma_client_matcher_find_nested_matches`] most
/// recently published (valid until the next call to that function); `0` when the most
/// recent call found no match or errored.
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

	#[test]
	fn matcher_abi_rejects_oversized_input() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let matcher_id = vorma_client_matcher_new();
		let ptr = vorma_client_matcher_alloc(MAX_INPUT_LEN + 1);

		assert!(ptr.is_null());
		assert_eq!(
			vorma_client_matcher_register_pattern(
				matcher_id,
				std::ptr::NonNull::<u8>::dangling().as_ptr(),
				MAX_INPUT_LEN + 1,
			),
			STATUS_ERROR,
		);

		vorma_client_matcher_free(matcher_id);
	}

	#[test]
	fn matcher_abi_rejects_unallocated_input_pointer() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let matcher_id = vorma_client_matcher_new();

		assert_eq!(
			vorma_client_matcher_register_pattern(
				matcher_id,
				std::ptr::NonNull::<u8>::dangling().as_ptr(),
				1,
			),
			STATUS_ERROR,
		);

		vorma_client_matcher_free(matcher_id);
	}

	#[test]
	fn matcher_abi_dealloc_ignores_wrong_lengths() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let ptr = vorma_client_matcher_alloc(8);

		assert!(!ptr.is_null());
		vorma_client_matcher_dealloc(ptr, 7);
		vorma_client_matcher_dealloc(ptr, 8);
	}

	#[test]
	fn matcher_abi_limits_live_allocations() {
		let _guard = TEST_LOCK.lock().expect("test lock poisoned");
		let mut ptrs = Vec::new();
		for _ in 0..MAX_LIVE_ALLOCATIONS {
			let ptr = vorma_client_matcher_alloc(1);
			assert!(!ptr.is_null());
			ptrs.push(ptr);
		}

		assert!(vorma_client_matcher_alloc(1).is_null());

		for ptr in ptrs {
			vorma_client_matcher_dealloc(ptr, 1);
		}
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
