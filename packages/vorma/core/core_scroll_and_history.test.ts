// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	has_route_render_commit,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_response,
	seed_payload,
	t_opts,
	tick,
} from "./_test_helpers.ts";
import { apply_scroll, create_client_core } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

describe("apply_scroll", () => {
	it("scrolls to element by decoded hash ID", () => {
		const el = document.createElement("div");
		el.id = "my-section";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "#my-section" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("decodes URI-encoded hash", () => {
		const el = document.createElement("div");
		el.id = "hello world";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "#hello%20world" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("falls back to raw ID when decode fails", () => {
		const el = document.createElement("div");
		el.id = "%E0%A4%A";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "#%E0%A4%A" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("handles hash without leading #", () => {
		const el = document.createElement("div");
		el.id = "no-hash";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "no-hash" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("calls scroll_to for coordinate scroll", () => {
		const opts = t_opts();

		apply_scroll({ x: 100, y: 200 }, opts);

		expect(opts.scroll_to).toHaveBeenCalledWith(100, 200);
	});

	it("does nothing for undefined scroll", () => {
		const opts = t_opts();

		apply_scroll(undefined, opts);

		expect(opts.scroll_to).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Boot lifecycle
/////////////////////////////////////////////////////////////////////

describe("history state", () => {
	it("committed RouteState includes history_state from navigate", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/about"] }),
		);

		await core.navigate("/about", { state: { from: "search" } });

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toEqual({ from: "search" });
	});

	it("history_state is undefined when no state is provided", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/page"] }),
		);

		await core.navigate("/page");

		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toBeUndefined();
	});

	it("history_state is undefined on initial load", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toBeUndefined();
	});

	it("history_state is preserved through revalidation", async () => {
		seed_payload();
		const commit = vi.fn();
		const opts = t_opts();
		const core_res = create_client_core({}, commit, opts);
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/stateful"] }),
		);

		await core.navigate("/stateful", {
			state: { modal: "confirm" },
		});

		const nav_state = route_render_commit_at(commit, 0);
		expect(nav_state.history_state).toEqual({ modal: "confirm" });
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/stateful"] }),
		);

		await core.revalidate();
		await tick();

		expect(has_route_render_commit(commit)).toBe(true);
		const reval_state = route_render_commit_at(commit, 0);
		expect(reval_state.history_state).toEqual({ modal: "confirm" });
	});

	it("getRouteState exposes historyState", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/detail"] }),
		);

		await core.navigate("/detail", {
			state: { from: "favorites" },
		});

		const route_state = core.getRouteState();
		expect(route_state.historyState).toEqual({ from: "favorites" });
	});

	it("hash-only navigation commits state to history", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		await core.navigate("/#section", {
			state: { tab: "overview" },
		});

		// State should be written to history even for hash-only nav
		const route_state = core.getRouteState();
		expect(route_state.historyState).toEqual({ tab: "overview" });
	});

	it("state from replace navigation overwrites previous state", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(route_response({ matched_patterns: ["/page"] }))
			.mockResolvedValueOnce(route_response({ matched_patterns: ["/page"] }));

		await core.navigate("/page", { state: { v: 1 } });

		const first_state = route_render_commit_at(commit, 0);
		expect(first_state.history_state).toEqual({ v: 1 });
		commit.mockClear();

		await core.navigate("/page", {
			replace: true,
			state: { v: 2 },
		});

		const second_state = route_render_commit_at(commit, 0);
		expect(second_state.history_state).toEqual({ v: 2 });
	});

	it("complex state objects are preserved", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		const complex_state = {
			selectedContact: { id: "c1", name: "Alice" },
			filters: ["active", "recent"],
			page: 3,
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/compose"] }),
		);

		await core.navigate("/compose", { state: complex_state });

		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toEqual(complex_state);
	});
});
