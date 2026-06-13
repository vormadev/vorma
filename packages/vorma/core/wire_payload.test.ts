// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import type { ViewPayload } from "./wire_contracts.gen.ts";
import { decode_payload } from "./wire_payload.ts";

const URL_BASE = new URL("https://x.example/users/42?q=hello");

function decode(raw: Partial<ViewPayload>) {
	const registered: Array<[string, unknown]> = [];
	const payload = decode_payload(raw, URL_BASE, (pattern, schema) => {
		registered.push([pattern, schema]);
	});
	return { payload, registered };
}

describe("decode_payload", () => {
	it("defaults every omitted wire field", () => {
		const { payload } = decode({});
		expect(payload).toEqual({
			routes: [],
			params: {},
			splat_values: [],
			title: undefined,
			meta_head_els: [],
			rest_head_els: [],
			css_bundles: [],
			deps: [],
		});
	});

	it("zips patterns with schemas, modules, and view data by index", () => {
		const { payload, registered } = decode({
			matched_patterns: ["/users", "/users/:id"],
			search_schemas: [undefined, { q: "string" }] as unknown[],
			import_urls: ["/m/users.js", "/m/user.js"],
			views_data: [{ list: true }, { id: 42 }],
		});

		expect(payload.routes).toHaveLength(2);
		expect(payload.routes[1]).toMatchObject({
			pattern: "/users/:id",
			module_url: "/m/user.js",
			view_data: { id: 42 },
		});
		expect(registered).toEqual([
			["/users", undefined],
			["/users/:id", { q: "string" }],
		]);
	});

	it("attaches the outermost server error to exactly its route index", () => {
		const { payload } = decode({
			matched_patterns: ["/a", "/a/b", "/a/b/c"],
			outermost_server_err: "boom",
			outermost_server_err_idx: 1,
		});
		expect(payload.routes.map((r) => r.server_error)).toEqual([
			undefined,
			"boom",
			undefined,
		]);
	});

	it("decodes HTML entities in the prepared title", () => {
		const { payload } = decode({
			title: {
				tag: "title",
				dangerous_inner_html: "Fish &amp; Chips",
			},
		});
		expect(payload.title).toBe("Fish & Chips");
	});
});
