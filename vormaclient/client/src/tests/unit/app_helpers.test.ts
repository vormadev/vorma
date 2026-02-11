import { afterEach, describe, expect, it, vi } from "vitest";
import { resolveVormaPath, resolveVormaRequestBody } from "../../app/helpers.ts";

describe("resolveVormaRequestBody", () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it("serializes plain objects when ReadableStream is unavailable", () => {
		vi.stubGlobal("ReadableStream", undefined as any);

		expect(resolveVormaRequestBody({ key: "value" })).toBe(
			JSON.stringify({ key: "value" }),
		);
	});

	it("passes through ReadableStream bodies when available", () => {
		if (typeof ReadableStream === "undefined") {
			return;
		}

		const stream = new ReadableStream();
		expect(resolveVormaRequestBody(stream)).toBe(stream);
	});

	it("does not throw when body constructors are unavailable", () => {
		vi.stubGlobal("ReadableStream", undefined as any);
		vi.stubGlobal("Blob", undefined as any);
		vi.stubGlobal("FormData", undefined as any);
		vi.stubGlobal("URLSearchParams", undefined as any);
		vi.stubGlobal("ArrayBuffer", undefined as any);

		expect(resolveVormaRequestBody({ key: "value" })).toBe(
			JSON.stringify({ key: "value" }),
		);
	});
});

const TEST_CONFIG = {
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

describe("resolveVormaPath", () => {
	it("URL-encodes dynamic parameter values and replaces repeated exact tokens", () => {
		const path = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/users/:id/:id2/:id",
				params: {
					id: "a/b c",
					id2: "second",
				},
			},
		});

		expect(path).toBe("/users/a%2Fb%20c/second/a%2Fb%20c");
	});

	it("URL-encodes splat segments individually", () => {
		const path = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/files/*",
				splatValues: ["docs and notes", "q1/q2"],
			},
		});

		expect(path).toBe("/files/docs%20and%20notes/q1%2Fq2");
	});

	it("strips explicit loader index segments after path resolution", () => {
		const rootPath = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/_index",
			},
		});

		const nestedPath = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/users/_index",
			},
		});

		expect(rootPath).toBe("/");
		expect(nestedPath).toBe("/users");
	});
});
