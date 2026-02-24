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

	it("passes through ArrayBuffer-backed typed arrays without cloning", () => {
		const bytes = new Uint8Array([1, 2, 3, 4]);
		expect(resolveVormaRequestBody(bytes)).toBe(bytes);
	});

	it("clones SharedArrayBuffer-backed views before returning a body", () => {
		if (typeof SharedArrayBuffer === "undefined") {
			return;
		}

		const sharedBuffer = new SharedArrayBuffer(4);
		const view = new Uint8Array(sharedBuffer);
		view.set([5, 6, 7, 8]);

		const body = resolveVormaRequestBody(view);
		expect(body).toBeInstanceOf(Uint8Array);
		expect(body).not.toBe(view);
		expect(Array.from(body as Uint8Array)).toEqual([5, 6, 7, 8]);
	});
});

const TEST_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

const CUSTOM_RUNE_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: "$",
	actionsSplatRune: "**",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
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

	it("throws when dynamic params required by the pattern are missing", () => {
		expect(() =>
			resolveVormaPath({
				vormaAppConfig: TEST_CONFIG,
				type: "loader",
				props: {
					pattern: "/users/:id",
				},
			}),
		).toThrow('Missing required route params for pattern "/users/:id": id');

		expect(() =>
			resolveVormaPath({
				vormaAppConfig: TEST_CONFIG,
				type: "query",
				props: {
					pattern: "/search/:section/:slug",
					params: {
						section: "guides",
					},
				},
			}),
		).toThrow(
			'Missing required route params for pattern "/search/:section/:slug": slug',
		);
	});

	it("throws when splat values required by the pattern are missing", () => {
		expect(() =>
			resolveVormaPath({
				vormaAppConfig: TEST_CONFIG,
				type: "loader",
				props: {
					pattern: "/files/*",
				},
			}),
		).toThrow('Missing required splat values for pattern "/files/*"');

		expect(() =>
			resolveVormaPath({
				vormaAppConfig: CUSTOM_RUNE_CONFIG,
				type: "query",
				props: {
					pattern: "/users/$id/**",
					params: {
						id: "42",
					},
				},
			}),
		).toThrow('Missing required splat values for pattern "/users/$id/**"');
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

	it("throws when query input root is a primitive value", () => {
		expect(() =>
			buildQueryURL(TEST_CONFIG, {
				pattern: "/search",
				input: 0,
			} as any),
		).toThrow("Query input root must be an object, null, or undefined.");

		expect(() =>
			buildQueryURL(TEST_CONFIG, {
				pattern: "/search",
				input: false,
			} as any),
		).toThrow("Query input root must be an object, null, or undefined.");

		expect(() =>
			buildQueryURL(TEST_CONFIG, {
				pattern: "/search",
				input: "",
			} as any),
		).toThrow("Query input root must be an object, null, or undefined.");
	});

	it("throws when query input root is an array", () => {
		expect(() =>
			buildQueryURL(TEST_CONFIG, {
				pattern: "/search",
				input: ["x"],
			} as any),
		).toThrow("Query input root must be an object, null, or undefined.");
	});

	it("treats null query input as empty query params", () => {
		const url = buildQueryURL(TEST_CONFIG, {
			pattern: "/search",
			input: null,
		} as any);

		expect(url.search).toBe("");
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
