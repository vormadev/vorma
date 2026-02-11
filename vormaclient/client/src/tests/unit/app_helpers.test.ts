import { afterEach, describe, expect, it, vi } from "vitest";
import {
	__resolvePath,
	buildMutationURL,
	buildQueryURL,
	resolveBody,
	resolveVormaPath,
	resolveVormaRequestBody,
} from "../../app/helpers.ts";

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
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

const CUSTOM_RUNE_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: "$",
	actionsSplatRune: "**",
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

describe("URL and wrapper helper exports", () => {
	it("builds query URLs from mounted action paths and serialized input", () => {
		const url = buildQueryURL(TEST_CONFIG, {
			pattern: "/search/:section",
			params: {
				section: "guides",
			},
			input: {
				q: "vitest",
				page: 2,
			},
		} as any);

		expect(url.origin).toBe("http://localhost:3000");
		expect(url.pathname).toBe("/api/search/guides");
		expect(url.searchParams.get("q")).toBe("vitest");
		expect(url.searchParams.get("page")).toBe("2");
		expect(Array.from(url.searchParams.keys()).sort()).toEqual([
			"page",
			"q",
		]);
	});

	it("builds mutation URLs without encoding input into query params", () => {
		const url = buildMutationURL(TEST_CONFIG, {
			pattern: "/users/:id",
			params: {
				id: "abc-123",
			},
			input: {
				ignoredAtURLLevel: true,
			},
		} as any);

		expect(url.href).toBe("http://localhost:3000/api/users/abc-123");
		expect(url.search).toBe("");
	});

	it("resolves submit bodies via resolveBody wrapper", () => {
		expect(resolveBody({ pattern: "/submit", input: { a: 1 } })).toBe(
			JSON.stringify({ a: 1 }),
		);
		expect(resolveBody({ pattern: "/submit", input: undefined })).toBe(
			undefined,
		);
	});

	it("resolves query and mutation paths using action runes via __resolvePath", () => {
		const queryPath = __resolvePath({
			vormaAppConfig: CUSTOM_RUNE_CONFIG,
			type: "query",
			props: {
				pattern: "/users/$id/**",
				params: {
					id: "a b",
				},
				splatValues: ["docs/reports", "2026"],
			},
		} as any);
		const mutationPath = __resolvePath({
			vormaAppConfig: CUSTOM_RUNE_CONFIG,
			type: "mutation",
			props: {
				pattern: "/users/$id",
				params: {
					id: "mut-1",
				},
			},
		} as any);

		expect(queryPath).toBe("/users/a%20b/docs%2Freports/2026");
		expect(mutationPath).toBe("/users/mut-1");
	});
});
