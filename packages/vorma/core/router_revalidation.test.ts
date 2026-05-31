// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	get_history_key,
	has_route_render_commit,
	json_response,
	last_route_render_commit,
	mock_fetch,
	redirect_response,
	register_ccc_lifecycle,
	route_render_commit_count,
	route_response,
	setup,
	simulate_popstate,
} from "./_test_helpers.ts";
import {
	BUILD_ID_HEADER,
	VERCEL_DPL_QUERY_PARAM_KEY,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";
import {
	MAX_REVALIDATION_RETRIES,
	REVALIDATION_BACKOFF_BASE_MS,
	REVALIDATION_BACKOFF_CAP_MS,
	REVALIDATION_DEBOUNCE_MS,
} from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
});

/////////////////////////////////////////////////////////////////////
/////// Revalidate basics
/////////////////////////////////////////////////////////////////////

describe("revalidate", () => {
	it("commits fresh data on completion", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/"],
				loaders_data: [{ fresh: true }],
			}),
		);
		await expect(rev).resolves.toEqual({ ok: true });

		expect(has_route_render_commit(commit)).toBe(true);
		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("sends Vercel deployment ID on revalidation requests when present", async () => {
		const { core } = await setup({
			payload: { deployment_id: "dpl_test_123" },
		});
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);

		const url = new URL(call(0).url);
		expect(url.searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY)).toBe("dpl_test_123");

		call(0).resolve(route_response());
		await expect(rev).resolves.toEqual({ ok: true });
	});

	it("reports revalidation as a route update without a URL change", async () => {
		const route_updates: any[] = [];
		const { core } = await setup({
			payload: {
				matched_patterns: ["/"],
				loaders_data: [{ fresh: false }],
			},
			clientOptions: {
				onRouteUpdate: (
					route: unknown,
					previous_route: unknown,
					reason: unknown,
				) => {
					route_updates.push({ route, previous_route, reason });
				},
			},
		});
		route_updates.length = 0;
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/"],
				loaders_data: [{ fresh: true }],
			}),
		);
		await rev;

		expect(route_updates).toHaveLength(1);
		expect(route_updates[0]).toMatchObject({
			reason: "revalidation",
			route: {
				href: `${window.location.origin}/`,
			},
			previous_route: {
				href: `${window.location.origin}/`,
			},
		});
	});

	it("does not report route update when revalidation route state is unchanged", async () => {
		const route_updates: any[] = [];
		const { core } = await setup({
			payload: {
				matched_patterns: ["/"],
				loaders_data: [{ fresh: false }],
			},
			clientOptions: {
				onRouteUpdate: (
					route: unknown,
					previous_route: unknown,
					reason: unknown,
				) => {
					route_updates.push({ route, previous_route, reason });
				},
			},
		});
		route_updates.length = 0;
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/"],
				loaders_data: [{ fresh: false }],
			}),
		);
		await rev;

		expect(route_updates).toHaveLength(0);
	});

	it("returns a promise that resolves when freshness is achieved", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);

		let resolved = false;
		let revalidation_result: unknown;
		void rev.then((result) => {
			resolved = true;
			revalidation_result = result;
		});
		await vi.advanceTimersByTimeAsync(0);
		expect(resolved).toBe(false);

		call(0).resolve(route_response());
		await rev;
		expect(resolved).toBe(true);
		expect(revalidation_result).toEqual({ ok: true });
	});

	it("defers revalidation while navigate is in flight", async () => {
		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		await wait_for(1);
		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		await nav;

		await wait_for(2);
		call(1).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		expect(route_render_commit_count(commit)).toBeGreaterThanOrEqual(2);
	});

	it("fires a new revalidation when called again after first is in flight", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		call(0).resolve(route_response());
		await wait_for(2);

		call(1).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		expect(calls).toHaveLength(2);
		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("does not satisfy a newer debounced revalidation with older in-flight data", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const first = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);

		const second = core.revalidate();
		let resolved = false;
		void Promise.all([first, second]).then(() => {
			resolved = true;
		});

		call(0).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		expect(resolved).toBe(false);

		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(2);
		call(1).resolve(route_response());

		await expect(Promise.all([first, second])).resolves.toEqual([
			{ ok: true },
			{ ok: true },
		]);
	});

	it("does not push or replace history on successful revalidation", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const initial_push = push_spy.mock.calls.length;
		const initial_replace = replace_spy.mock.calls.length;
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(route_response());
		await rev;

		expect(push_spy.mock.calls.length).toBe(initial_push);
		expect(replace_spy.mock.calls.length).toBe(initial_replace);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Revalidate redirects
/////////////////////////////////////////////////////////////////////

describe("revalidate redirects", () => {
	it("follows soft redirect by navigating", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/new-page" }));
		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/new-page"],
				loaders_data: [{ navigated: true }],
			}),
		);
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (route_render_commit_count(commit) > 0) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(has_route_render_commit(commit)).toBe(true);
	});

	it("drops stale build skew responses by default", async () => {
		const on_build_skew = vi.fn();
		const { core, hard_redirect } = await setup({
			clientOptions: { onBuildSkewDetected: on_build_skew },
		});
		const { call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_VORMA_BUILD_SKEW]: "1",
			}),
		);
		await vi.advanceTimersByTimeAsync(0);

		await expect(rev).resolves.toEqual({
			ok: false,
			reason: "build_skew",
		});
		expect(hard_redirect).not.toHaveBeenCalled();
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "manual",
					requestedHref: `${window.location.origin}/`,
				}),
			}),
		);
	});

	it("uses replaceState for revalidation redirect", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const initial_replace = replace_spy.mock.calls.length;
		const { call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/new-page" }));
		await wait_for(2);
		call(1).resolve(route_response());

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().navigation === null) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(push_spy).not.toHaveBeenCalled();
		const replace_calls_after = replace_spy.mock.calls.slice(initial_replace);
		expect(replace_calls_after.length).toBeGreaterThanOrEqual(1);
		expect(replace_calls_after[replace_calls_after.length - 1]![2]).toContain(
			"/new-page",
		);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Debounce
/////////////////////////////////////////////////////////////////////

describe("revalidate debounce", () => {
	it("coalesces rapid revalidate calls", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.revalidate();
		void core.revalidate();
		void core.revalidate();

		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);

		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);
	});

	it("all callers' promises resolve from a single coalesced revalidation", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const p1 = core.revalidate();
		const p2 = core.revalidate();
		const p3 = core.revalidate();

		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(route_response());

		await expect(Promise.all([p1, p2, p3])).resolves.toEqual([
			{ ok: true },
			{ ok: true },
			{ ok: true },
		]);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Derived is_revalidating status
/////////////////////////////////////////////////////////////////////

describe("derived isRevalidating status", () => {
	it("is true immediately when revalidate is called", async () => {
		const { core } = await setup();

		void core.revalidate();

		expect(core.getWorkState().revalidation !== null).toBe(true);
	});

	it("reports isRevalidating during revalidation flight", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.revalidate();
		expect(core.getWorkState().revalidation !== null).toBe(true);

		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		expect(core.getWorkState().revalidation !== null).toBe(true);

		call(0).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().revalidation === null) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("is true immediately when non-GET submit requests refresh", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		await vi.advanceTimersByTimeAsync(0);

		expect(core.getWorkState().apiRequests.length > 0).toBe(false);
		expect(core.getWorkState().revalidation !== null).toBe(true);
	});

	it("transitions from submitting to revalidating with no gap", async () => {
		const work_updates: any[] = [];
		const { core } = await setup({
			clientOptions: {
				onWorkUpdate: (work: any) => {
					work_updates.push({ ...work });
				},
			},
		});
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response());
		await result.revalidationPromise;

		// There should be no work update where all work is idle
		// between submitting and revalidating
		const non_final = work_updates.slice(0, -1);
		const had_gap = non_final.some((work) => {
			return (
				work.navigation === null &&
				work.revalidation === null &&
				work.apiRequests.length === 0
			);
		});
		expect(had_gap).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Post-submit freshness system
/////////////////////////////////////////////////////////////////////

describe("post-submit freshness", () => {
	it("automatically revalidates after a settled mutation submit", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response());
		await result.revalidationPromise;

		expect(calls).toHaveLength(2);
		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("satisfies invariant when user navigates after non-GET submit", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const _ = await sub;

		// System revalidation fires
		await wait_for(2);

		// User navigates — aborts system revalidation
		const nav = core.navigate("/other-page");
		await wait_for(3);

		const nav_call = calls.find((c, i) => {
			return i > 1 && !c.signal.aborted;
		});
		expect(nav_call).toBeTruthy();
		nav_call!.resolve(route_response());
		await nav;

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Backoff
/////////////////////////////////////////////////////////////////////

describe("revalidation backoff", () => {
	it("retries with delay after revalidation failure", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		await wait_for(1);
		call(0).resolve(new Response("", { status: 500, statusText: "Error" }));
		await vi.advanceTimersByTimeAsync(0);

		expect(calls).toHaveLength(1);

		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_BASE_MS);
		await wait_for(2);

		call(1).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().revalidation === null) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("increases delay exponentially on repeated failures", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		// First attempt fails
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500 }));
		await vi.advanceTimersByTimeAsync(0);

		// After first backoff (500ms)
		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_BASE_MS);
		await wait_for(2);

		// Second attempt fails
		call(1).resolve(new Response("", { status: 500 }));
		await vi.advanceTimersByTimeAsync(0);

		// Should not retry before doubled delay
		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_BASE_MS);
		expect(calls).toHaveLength(2);

		// After doubled delay
		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_BASE_MS);
		await wait_for(3);

		call(2).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().revalidation === null) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("resets backoff on new non-GET submit", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		// First attempt fails
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500 }));
		await vi.advanceTimersByTimeAsync(0);

		// New non-GET submit resets backoff
		void core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(2);
		call(1).resolve(json_response({ ok: true }));
		await vi.advanceTimersByTimeAsync(0);

		// System revalidation should fire immediately (backoff reset)
		await wait_for(3);
		call(2).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().revalidation === null) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("gives up after max retries", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const rev = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		// First attempt
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500 }));
		await vi.advanceTimersByTimeAsync(0);

		// Remaining attempts with backoff
		for (let i = 1; i < MAX_REVALIDATION_RETRIES; i++) {
			await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_CAP_MS);
			await wait_for(i + 1);
			call(i).resolve(new Response("", { status: 500 }));
			await vi.advanceTimersByTimeAsync(0);
		}

		// No more retries
		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_CAP_MS * 2);
		expect(calls).toHaveLength(MAX_REVALIDATION_RETRIES);
		await expect(rev).resolves.toEqual({
			ok: false,
			reason: "max_retries_exhausted",
		});
		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("can start a new freshness demand after retries are exhausted", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const first = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		for (let i = 0; i < MAX_REVALIDATION_RETRIES; i++) {
			if (i > 0) {
				await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_CAP_MS);
			}
			await wait_for(i + 1);
			call(i).resolve(new Response("", { status: 500 }));
			await vi.advanceTimersByTimeAsync(0);
		}

		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_CAP_MS);
		await expect(first).resolves.toEqual({
			ok: false,
			reason: "max_retries_exhausted",
		});

		const second = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(MAX_REVALIDATION_RETRIES + 1);
		call(MAX_REVALIDATION_RETRIES).resolve(route_response());

		await expect(second).resolves.toEqual({ ok: true });
		expect(calls).toHaveLength(MAX_REVALIDATION_RETRIES + 1);
		expect(core.getWorkState().revalidation !== null).toBe(false);
	});

	it("caps backoff delay at maximum", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		// Burn through attempts to reach capped delay
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500 }));
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 1; i < 7; i++) {
			await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_CAP_MS);
			await wait_for(i + 1);
			call(i).resolve(new Response("", { status: 500 }));
			await vi.advanceTimersByTimeAsync(0);
		}

		// Next attempt: uncapped would exceed cap, should cap
		await vi.advanceTimersByTimeAsync(REVALIDATION_BACKOFF_CAP_MS - 1);
		expect(calls).toHaveLength(7);

		await vi.advanceTimersByTimeAsync(1);
		await wait_for(8);

		call(7).resolve(route_response());
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().revalidation === null) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Revalidation stale-ownership
/////////////////////////////////////////////////////////////////////

describe("revalidation stale-ownership", () => {
	it("discards revalidation result when URL changed during flight", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);

		// Navigate away while revalidation is in flight
		const nav = core.navigate("/different-page");
		await wait_for(2);
		call(1).resolve(route_response());
		await nav;
		commit.mockClear();

		// Stale revalidation resolves
		call(0).resolve(route_response({ matched_patterns: ["/stale"] }));
		await vi.advanceTimersByTimeAsync(0);

		// Stale result should not commit
		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("applies revalidation result when only hash changed during flight", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		// Navigate to /page first
		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;
		commit.mockClear();

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(2);

		// Hash changes during flight — pathname stays the same
		await core.navigate("/page#changed");

		call(1).resolve(
			route_response({
				matched_patterns: ["/page"],
				loaders_data: [{ still_valid: true }],
			}),
		);
		await vi.advanceTimersByTimeAsync(0);

		for (let i = 0; i < 50; i++) {
			if (route_render_commit_count(commit) > 0) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].loader_data).toEqual({ still_valid: true });
	});
});

/////////////////////////////////////////////////////////////////////
/////// Popstate interaction
/////////////////////////////////////////////////////////////////////

describe("revalidation and popstate", () => {
	it("aborts in-flight revalidation on popstate", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		// Navigate to /page first
		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;
		commit.mockClear();

		const _ = get_history_key();

		// Start revalidation
		void core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(2);

		// Popstate back
		simulate_popstate("popstate-interrupts-rev", "/previous");

		await wait_for(3);
		call(2).resolve(
			route_response({
				matched_patterns: ["/previous"],
				loaders_data: [{ previous: true }],
			}),
		);

		for (let i = 0; i < 50; i++) {
			if (route_render_commit_count(commit) > 0) {
				break;
			}
			await vi.advanceTimersByTimeAsync(0);
		}

		expect(has_route_render_commit(commit)).toBe(true);
		expect(core.getWorkState().revalidation !== null).toBe(false);
		expect(core.getWorkState().navigation !== null).toBe(false);
	});
});
