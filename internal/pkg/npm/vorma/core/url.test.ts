// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import {
	build_action_url,
	build_typed_link_href,
	resolve_body,
	resolve_path,
} from "./url.ts";

describe("resolve_path", () => {
	describe("loader paths", () => {
		it("resolves static pattern", () => {
			expect(resolve_path("loader", "/users")).toBe("/users");
		});

		it("resolves single dynamic param", () => {
			expect(resolve_path("loader", "/users/:id", { id: "42" })).toBe(
				"/users/42",
			);
		});

		it("resolves multiple dynamic params", () => {
			expect(
				resolve_path("loader", "/users/:id/posts/:postId", {
					id: "42",
					postId: "7",
				}),
			).toBe("/users/42/posts/7");
		});

		it("resolves splat values", () => {
			expect(
				resolve_path("loader", "/docs/*", undefined, [
					"guide",
					"intro",
				]),
			).toBe("/docs/guide/intro");
		});

		it("URL-encodes dynamic param values", () => {
			expect(resolve_path("loader", "/users/:id", { id: "a/b" })).toBe(
				"/users/a%2Fb",
			);
		});

		it("URL-encodes splat values", () => {
			expect(
				resolve_path("loader", "/docs/*", undefined, ["hello world"]),
			).toBe("/docs/hello%20world");
		});

		it("strips _index suffix for loader type", () => {
			expect(resolve_path("loader", "/blog/_index")).toBe("/blog");
		});

		it("resolves root _index to /", () => {
			expect(resolve_path("loader", "/_index")).toBe("/");
		});

		it("normalizes trailing slash", () => {
			expect(resolve_path("loader", "/users/")).toBe("/users");
		});

		it("preserves root path", () => {
			expect(resolve_path("loader", "/")).toBe("/");
		});
	});

	describe("action paths", () => {
		it("does not strip _index suffix", () => {
			expect(resolve_path("action", "/blog/_index")).toBe("/blog/_index");
		});

		it("resolves dynamic params", () => {
			expect(resolve_path("action", "/users/:id", { id: "42" })).toBe(
				"/users/42",
			);
		});
	});
});

describe("build_typed_link_href", () => {
	it("builds href from pattern and params", () => {
		const href = build_typed_link_href("/users/:id", { id: "42" });
		const url = new URL(href);
		expect(url.pathname).toBe("/users/42");
	});

	it("appends search params", () => {
		const href = build_typed_link_href(
			"/users/:id",
			{ id: "42" },
			undefined,
			{ q: "abc" },
		);
		const url = new URL(href);
		expect(url.pathname).toBe("/users/42");
		expect(url.searchParams.get("q")).toBe("abc");
	});

	it("appends hash", () => {
		const href = build_typed_link_href(
			"/users/:id",
			{ id: "42" },
			undefined,
			undefined,
			"#panel",
		);
		const url = new URL(href);
		expect(url.pathname).toBe("/users/42");
		expect(url.hash).toBe("#panel");
	});

	it("builds href with splat values", () => {
		const href = build_typed_link_href("/docs/*", undefined, [
			"guide",
			"intro",
		]);
		const url = new URL(href);
		expect(url.pathname).toBe("/docs/guide/intro");
	});

	it("combines params, search, and hash", () => {
		const href = build_typed_link_href(
			"/users/:id",
			{ id: "42" },
			undefined,
			{ tab: "posts" },
			"#latest",
		);
		const url = new URL(href);
		expect(url.pathname).toBe("/users/42");
		expect(url.searchParams.get("tab")).toBe("posts");
		expect(url.hash).toBe("#latest");
	});

	it("strips _index for loader patterns", () => {
		const href = build_typed_link_href("/blog/_index");
		const url = new URL(href);
		expect(url.pathname).toBe("/blog");
	});
});

describe("build_action_url", () => {
	it("prepends mount root to resolved path", () => {
		const url = build_action_url("/api/", "/users/:id", { id: "42" });
		expect(url.pathname).toBe("/api/users/42");
	});

	it("serializes input as search params", () => {
		const url = build_action_url(
			"/api/",
			"/users/:id",
			{ id: "42" },
			undefined,
			{ include: "posts", page: 2 },
		);
		expect(url.pathname).toBe("/api/users/42");
		expect(url.searchParams.get("include")).toBe("posts");
		expect(url.searchParams.get("page")).toBe("2");
	});

	it("produces no search params when input is undefined", () => {
		const url = build_action_url("/api/", "/users/:id", { id: "42" });
		expect(url.search).toBe("");
	});

	it("produces no search params when input is null", () => {
		const url = build_action_url(
			"/api/",
			"/users/:id",
			{ id: "42" },
			undefined,
			null,
		);
		expect(url.search).toBe("");
	});

	it("URL-encodes dynamic params", () => {
		const url = build_action_url("/api/", "/users/:id", { id: "a/b" });
		expect(url.pathname).toBe("/api/users/a%2Fb");
	});

	it("handles splat values", () => {
		const url = build_action_url("/api/", "/docs/*", undefined, [
			"guide",
			"intro",
		]);
		expect(url.pathname).toBe("/api/docs/guide/intro");
	});

	it("handles root pattern", () => {
		const url = build_action_url("/api/", "/");
		expect(url.pathname).toBe("/api");
	});

	it("normalizes trailing slash on mount root", () => {
		const url = build_action_url("/api", "/users/:id", { id: "42" });
		expect(url.pathname).toBe("/api/users/42");
	});

	it("does not include any search params", () => {
		const url = build_action_url(
			"/api/",
			"/users/:id",
			{ id: "42" },
			undefined,
			undefined,
		);
		expect(url.search).toBe("");
	});
});

describe("resolve_body", () => {
	it("serializes plain objects to JSON", () => {
		expect(resolve_body({ a: 1 })).toBe('{"a":1}');
	});

	it("passes through FormData", () => {
		const form = new FormData();
		form.set("key", "value");
		expect(resolve_body(form)).toBe(form);
	});

	it("passes through URLSearchParams", () => {
		const params = new URLSearchParams({ a: "1" });
		expect(resolve_body(params)).toBe(params);
	});

	it("passes through Blob", () => {
		const blob = new Blob(["data"], { type: "text/plain" });
		expect(resolve_body(blob)).toBe(blob);
	});

	it("passes through ArrayBuffer", () => {
		const buffer = new ArrayBuffer(16);
		expect(resolve_body(buffer)).toBe(buffer);
	});

	it("passes through ArrayBufferView", () => {
		const view = new Uint8Array([1, 2, 3]);
		expect(resolve_body(view)).toBe(view);
	});

	it("passes through null", () => {
		expect(resolve_body(null)).toBeNull();
	});

	it("passes through undefined", () => {
		expect(resolve_body(undefined)).toBeUndefined();
	});

	it("passes through strings", () => {
		expect(resolve_body("raw-string")).toBe("raw-string");
	});

	it("serializes nested objects to JSON", () => {
		const input = { users: [{ id: 1 }, { id: 2 }] };
		expect(resolve_body(input)).toBe(JSON.stringify(input));
	});
});
