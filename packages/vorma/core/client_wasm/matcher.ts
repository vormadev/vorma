import { init_client_wasm, type RawMatcherExports } from "./load.ts";

const status_no_match = 0;
const status_match = 1;
const max_input_bytes = 64 * 1024;

const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });
const malformed_output_message = "Malformed Vorma client matcher output";

/** A path's match result against a {@link ClientMatcher}'s registered patterns — the full outermost-to-innermost matched chain (`patterns`), plus the union of params/splat values across it. */
export type ClientMatcherNestedMatch = {
	params: Record<string, string>;
	splat_values: string[];
	patterns: string[];
};

/**
 * A WASM-backed route matcher running the same matching engine
 * (`vorma-matcher`) the Rust server uses, exposed for client-side use where
 * the router needs to know which patterns a path would match before a
 * server round-trip — e.g. deciding which client loaders to prestart
 * before the response arrives. Built via `create_client_matcher()`; the
 * router core owns the one instance it needs internally, so an app rarely
 * constructs one directly.
 *
 * `register_pattern` teaches the matcher one view pattern (patterns
 * accumulate; there is no unregister); `find_nested_matches` matches a path
 * against every registered pattern; `free` releases the underlying WASM
 * matcher instance — call it once a matcher is no longer needed, since WASM
 * memory is not garbage-collected by the JS runtime.
 */
export type ClientMatcher = {
	register_pattern: (pattern: string) => void;
	find_nested_matches: (path: string) => ClientMatcherNestedMatch | null;
	free: () => void;
};

function write_input(exports: RawMatcherExports, value: string): [number, number] {
	const bytes = encoder.encode(value);
	if (bytes.length > max_input_bytes) {
		throw new Error("Vorma client matcher input is too large");
	}
	const ptr = exports.vorma_client_matcher_alloc(bytes.length);
	if (ptr === 0) {
		throw new Error("Vorma client matcher input allocation failed");
	}
	new Uint8Array(exports.memory.buffer, ptr, bytes.length).set(bytes);
	return [ptr, bytes.length];
}

function malformed_output(): Error {
	return new Error(malformed_output_message);
}

function read_u32(view: DataView, offset: number): [number, number] {
	if (offset + 4 > view.byteLength) {
		throw malformed_output();
	}
	return [view.getUint32(offset, true), offset + 4];
}

function read_string(bytes: Uint8Array, offset: number): [string, number] {
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	let len: number;
	[len, offset] = read_u32(view, offset);
	if (len > bytes.length - offset) {
		throw malformed_output();
	}
	try {
		return [decoder.decode(bytes.subarray(offset, offset + len)), offset + len];
	} catch {
		throw malformed_output();
	}
}

function read_output(exports: RawMatcherExports): Uint8Array {
	const ptr = exports.vorma_client_matcher_output_ptr();
	const len = exports.vorma_client_matcher_output_len();
	if (len === 0) {
		return new Uint8Array();
	}
	return new Uint8Array(exports.memory.buffer, ptr, len).slice();
}

function decode_match(bytes: Uint8Array): ClientMatcherNestedMatch | null {
	if (bytes.length === 0) {
		return null;
	}
	const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	let offset = 0;
	let param_count: number;
	[param_count, offset] = read_u32(view, offset);
	const params: Record<string, string> = {};
	for (let i = 0; i < param_count; i++) {
		let key: string;
		let value: string;
		[key, offset] = read_string(bytes, offset);
		[value, offset] = read_string(bytes, offset);
		params[key] = value;
	}
	let splat_count: number;
	[splat_count, offset] = read_u32(view, offset);
	const splat_values: string[] = [];
	for (let i = 0; i < splat_count; i++) {
		let value: string;
		[value, offset] = read_string(bytes, offset);
		splat_values.push(value);
	}
	let pattern_count: number;
	[pattern_count, offset] = read_u32(view, offset);
	const patterns: string[] = [];
	for (let i = 0; i < pattern_count; i++) {
		let pattern: string;
		[pattern, offset] = read_string(bytes, offset);
		patterns.push(pattern);
	}
	if (offset !== bytes.length) {
		throw malformed_output();
	}
	return { params, splat_values, patterns };
}

function matcher_error(operation: string, status: number): Error {
	return new Error(`Vorma client matcher ${operation} failed with status ${status}`);
}

export async function create_client_matcher(): Promise<ClientMatcher> {
	const exports = await init_client_wasm();
	const matcher_id = exports.vorma_client_matcher_new();
	if (matcher_id === 0) {
		throw new Error("Failed to create Vorma client matcher");
	}
	let freed = false;
	const with_input = <T>(value: string, fn: (ptr: number, len: number) => T): T => {
		const [ptr, len] = write_input(exports, value);
		try {
			return fn(ptr, len);
		} finally {
			exports.vorma_client_matcher_dealloc(ptr, len);
		}
	};
	return {
		register_pattern: (pattern) => {
			if (freed) {
				return;
			}
			const status = with_input(pattern, (ptr, len) => {
				return exports.vorma_client_matcher_register_pattern(
					matcher_id,
					ptr,
					len,
				);
			});
			if (status !== status_match) {
				throw matcher_error("pattern registration", status);
			}
		},
		find_nested_matches: (path) => {
			if (freed) {
				return null;
			}
			const status = with_input(path, (ptr, len) => {
				return exports.vorma_client_matcher_find_nested_matches(
					matcher_id,
					ptr,
					len,
				);
			});
			if (status === status_no_match) {
				return null;
			}
			if (status !== status_match) {
				throw matcher_error("path match", status);
			}
			return decode_match(read_output(exports));
		},
		free: () => {
			if (freed) {
				return;
			}
			freed = true;
			exports.vorma_client_matcher_free(matcher_id);
		},
	};
}
