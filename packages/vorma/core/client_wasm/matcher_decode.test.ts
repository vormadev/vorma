import { describe, expect, it, vi } from "vitest";
import type { RawMatcherExports } from "./load.ts";

const output_ptr = 1024;
const input_ptr = 2048;

function u32(value: number): number[] {
	return [
		value & 0xff,
		(value >> 8) & 0xff,
		(value >> 16) & 0xff,
		(value >> 24) & 0xff,
	];
}

async function create_matcher_with_output(output: Uint8Array) {
	vi.resetModules();
	const memory = new WebAssembly.Memory({ initial: 1 });
	new Uint8Array(memory.buffer).set(output, output_ptr);
	const exports: RawMatcherExports = {
		memory,
		vorma_client_matcher_alloc: () => {
			return input_ptr;
		},
		vorma_client_matcher_dealloc: () => {},
		vorma_client_matcher_find_nested_matches: () => {
			return 1;
		},
		vorma_client_matcher_free: () => {},
		vorma_client_matcher_new: () => {
			return 1;
		},
		vorma_client_matcher_output_len: () => {
			return output.length;
		},
		vorma_client_matcher_output_ptr: () => {
			return output_ptr;
		},
		vorma_client_matcher_register_pattern: () => {
			return 1;
		},
	};
	vi.doMock("./load.ts", () => {
		return {
			init_client_wasm: async () => {
				return exports;
			},
		};
	});
	const { create_client_matcher } = await import("./matcher.ts");
	return create_client_matcher();
}

describe("client matcher output decoding", () => {
	it("rejects truncated u32 output", async () => {
		const matcher = await create_matcher_with_output(new Uint8Array([1, 0, 0]));

		expect(() => {
			matcher.find_nested_matches("/x");
		}).toThrow();
	});

	it("rejects truncated string output", async () => {
		const matcher = await create_matcher_with_output(
			new Uint8Array([...u32(1), ...u32(2), 0x61]),
		);

		expect(() => {
			matcher.find_nested_matches("/x");
		}).toThrow();
	});

	it("rejects invalid utf8 output", async () => {
		const matcher = await create_matcher_with_output(
			new Uint8Array([...u32(1), ...u32(1), 0xff, ...u32(0), ...u32(0), ...u32(0)]),
		);

		expect(() => {
			matcher.find_nested_matches("/x");
		}).toThrow();
	});

	it("rejects trailing bytes after a complete output record", async () => {
		const matcher = await create_matcher_with_output(
			new Uint8Array([...u32(0), ...u32(0), ...u32(0), 0xff]),
		);

		expect(() => {
			matcher.find_nested_matches("/x");
		}).toThrow();
	});
});
