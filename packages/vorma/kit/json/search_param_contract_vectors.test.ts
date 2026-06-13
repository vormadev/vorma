import { describe, expect, it } from "vitest";
import vector_file from "./search_param_contract_vectors.json";
import { serializeToSearchParams } from "./search_param_serializer.ts";

/**
 * Golden vectors shared with the Rust server-side query decoder
 * (crates/vorma/src/input_decoder.rs). The Rust suite asserts that decoding
 * `query` against the vector's type contracts yields `expected`; this suite
 * asserts that serializing `expected` yields `query`. Together they pin both
 * sides of the wire format to the same fixtures, so a change to either
 * implementation that shifts the encoding fails one of the suites.
 */
type ContractVector = {
	name: string;
	query: string;
	expected: unknown;
};

const { vectors }: { vectors: Array<ContractVector> } = vector_file;

/**
 * Key order in a query string is not part of the contract (the Rust decoder
 * is order-insensitive and the serializer sorts keys), but value order within
 * one repeated key is (arrays). Compare key -> values[] instead of raw text.
 */
function params_by_key(params: URLSearchParams): Record<string, Array<string>> {
	const out: Record<string, Array<string>> = {};
	for (const key of new Set(params.keys())) {
		out[key] = params.getAll(key);
	}
	return out;
}

describe("search param contract vectors (shared with the Rust decoder)", () => {
	it("loads a non-empty vector set", () => {
		expect(vectors.length).toBeGreaterThan(0);
	});

	for (const vector of vectors) {
		it(`serializes ${vector.name}`, () => {
			const actual = params_by_key(serializeToSearchParams(vector.expected));
			const expected = params_by_key(new URLSearchParams(vector.query));
			expect(actual).toEqual(expected);
		});
	}
});
