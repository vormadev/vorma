import { describe, expect, it } from "vitest";
import type { ClientLoaderPrefetch, DecodedRoute } from "./client_core_types.ts";
import { create_client_loader_orchestrator } from "./client_loaders.ts";

const TEST_BUILD_ID = "build-1";

function make_orchestrator() {
	return create_client_loader_orchestrator({
		client_build_id: () => TEST_BUILD_ID,
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

type FakePrefetch = ClientLoaderPrefetch & {
	aborted: boolean;
	seeded: unknown[];
};

function fake_prefetch(pattern: string): FakePrefetch {
	const p: FakePrefetch = {
		pattern,
		aborted: false,
		seeded: [],
		resolve_server_state: (v) => {
			p.seeded.push(v);
		},
		abort: () => {
			p.aborted = true;
		},
		result_promise: Promise.resolve(undefined),
	};
	return p;
}

describe("reconcile", () => {
	it("retains matching prefetches, seeds server state, aborts unclaimed ones", () => {
		const o = make_orchestrator();
		const kept = fake_prefetch("/a");
		const orphan = fake_prefetch("/gone");
		const seen: Array<[string, boolean, boolean]> = [];

		o.reconcile(
			[route("/a"), route("/a/b")],
			[kept, orphan],
			(r, _i, retained, suppressed) => {
				seen.push([r.pattern, retained !== undefined, suppressed]);
			},
		);

		expect(seen).toEqual([
			["/a", true, false],
			["/a/b", false, false],
		]);
		expect(kept.aborted).toBe(false);
		expect(kept.seeded).toHaveLength(1);
		expect(orphan.aborted).toBe(true);
	});

	it("suppresses and aborts everything at or after the outermost server error", () => {
		const o = make_orchestrator();
		const at_error = fake_prefetch("/a/b");
		const after_error = fake_prefetch("/a/b/c");
		const before_error = fake_prefetch("/a");
		const seen: Array<[string, boolean]> = [];

		o.reconcile(
			[route("/a"), route("/a/b", "boom"), route("/a/b/c")],
			[before_error, at_error, after_error],
			(r, _i, _retained, suppressed) => {
				seen.push([r.pattern, suppressed]);
			},
		);

		expect(seen).toEqual([
			["/a", false],
			["/a/b", true],
			["/a/b/c", true],
		]);
		expect(before_error.aborted).toBe(false);
		expect(at_error.aborted).toBe(true);
		expect(after_error.aborted).toBe(true);
	});
});

describe("build_server_state", () => {
	it("stamps the build id, maps matches, and points at the outermost error", () => {
		const o = make_orchestrator();
		const state = o.build_server_state([route("/a"), route("/a/b", "boom")], 0);

		expect(state.clientBuildId).toBe(TEST_BUILD_ID);
		expect(state.matches.map((m) => m.pattern)).toEqual(["/a", "/a/b"]);
		expect(state.outermostServerError).toEqual({ idx: 1, error: "boom" });
		expect(state.viewData).toEqual({ from: "/a" });
	});

	it("reports null when no route errored", () => {
		const o = make_orchestrator();
		expect(o.build_server_state([route("/a")], 0).outermostServerError).toBeNull();
	});
});

describe("loader registry", () => {
	it("registers, fetches, and unregisters loaders by pattern", () => {
		const o = make_orchestrator();
		const loader = async () => "data";
		expect(o.get("/a")).toBeUndefined();

		o.register("/a", loader);
		expect(o.get("/a")).toBe(loader);

		o.unregister("/a");
		expect(o.get("/a")).toBeUndefined();
	});
});
