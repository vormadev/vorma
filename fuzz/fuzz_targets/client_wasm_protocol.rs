#![no_main]

use libfuzzer_sys::fuzz_target;

const MAX_PATTERNS: usize = 32;

fuzz_target!(|data: &[u8]| {
	let matcher_id = vorma_client_wasm::vorma_client_matcher_new();
	let mut chunks = data.split(|byte| *byte == 0);

	for pattern in chunks.by_ref().take(MAX_PATTERNS) {
		call_with_input(pattern, |ptr, len| {
			let _ = vorma_client_wasm::vorma_client_matcher_register_pattern(matcher_id, ptr, len);
		});
	}

	if let Some(path) = chunks.next() {
		call_with_input(path, |ptr, len| {
			let _ =
				vorma_client_wasm::vorma_client_matcher_find_nested_matches(matcher_id, ptr, len);
		});
		let _ = vorma_client_wasm::vorma_client_matcher_output_len();
		let _ = vorma_client_wasm::vorma_client_matcher_output_ptr();
	}

	vorma_client_wasm::vorma_client_matcher_free(matcher_id);
});

fn call_with_input<T>(input: &[u8], call: impl FnOnce(*const u8, usize) -> T) -> T {
	let ptr = vorma_client_wasm::vorma_client_matcher_alloc(input.len());
	if ptr.is_null() {
		return call(ptr, input.len());
	}
	unsafe {
		std::ptr::copy_nonoverlapping(input.as_ptr(), ptr, input.len());
	}
	let result = call(ptr, input.len());
	vorma_client_wasm::vorma_client_matcher_dealloc(ptr, input.len());
	result
}
