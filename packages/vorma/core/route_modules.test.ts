// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import type { DecodedRoute } from "./client_core_types.ts";
import { create_route_modules } from "./route_modules.ts";

const TEST_BUILD_ID = "build-1";

function make_unit() {
	return create_route_modules({
		client_build_id: () => TEST_BUILD_ID,
		register_loader: () => {},
	});
}

function route(pattern: string, server_error?: string): DecodedRoute {
	return {
		pattern,
		input: undefined,
		module_url: `/m${pattern}.js`,
		view_data: { from: pattern },
		server_error,
	};
}

function payload(routes: DecodedRoute[]) {
	return {
		routes,
		params: {},
		splat_values: [],
		title: undefined,
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
	};
}

describe("build_route_record", () => {
	it("stamps build id, modules, and hmr versions per match", () => {
		const unit = make_unit();
		const mod = { default: {} };
		const record = unit.build_route_record(
			payload([route("/a")]),
			new Map([["/m/a.js", mod]]),
			[],
		);
		expect(record.client_build_id).toBe(TEST_BUILD_ID);
		expect(record.matches[0]).toMatchObject({
			pattern: "/a",
			module: mod,
			hmr_version: 0,
		});
		expect(record.error).toBeNull();
	});

	it("prefers the server error over a client-loader error", () => {
		const unit = make_unit();
		const record = unit.build_route_record(
			payload([route("/a", "server boom"), route("/a/b")]),
			new Map(),
			[undefined, { error: "loader boom" }],
		);
		expect(record.error).toEqual({
			idx: 0,
			error: "server boom",
			source: "server",
		});
	});

	it("falls back to the outermost client-loader error", () => {
		const unit = make_unit();
		const record = unit.build_route_record(
			payload([route("/a"), route("/a/b")]),
			new Map(),
			[undefined, { error: "loader boom" }],
		);
		expect(record.error).toEqual({
			idx: 1,
			error: "loader boom",
			source: "clientLoader",
		});
	});
});

describe("hmr_update", () => {
	it("normalizes the url and bumps a per-module version", () => {
		const unit = make_unit();
		const first = unit.hmr_update("/m/a.js?t=1", { v: 1 });
		expect(first).toEqual({ url: "/m/a.js", hmr_version: 1 });

		const second = unit.hmr_update("/m/a.js", { v: 2 });
		expect(second.hmr_version).toBe(2);

		const record = unit.build_route_record(payload([route("/a")]), new Map(), []);
		expect(record.matches[0]?.hmr_version).toBe(2);
	});
});
