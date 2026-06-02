// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	deferred,
	get_history_key,
	get_stored_scroll,
	has_route_render_commit,
	last_route_render_commit,
	mock_fetch,
	native_redirect_response,
	non_ok_redirect_response,
	redirect_response,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_render_commit_count,
	route_render_scroll_intent_at,
	route_response,
	set_scroll_position,
	setup,
	simulate_popstate,
	tick,
} from "./_test_helpers.ts";
import {
	BUILD_ID_HEADER,
	HISTORY_KEY_FIELD,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";
import { MAX_REDIRECTS, MAX_SCROLL_ENTRIES } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

/////////////////////////////////////////////////////////////////////
/////// Navigate
/////////////////////////////////////////////////////////////////////

describe("navigate", () => {
	it("fetches and commits state on success", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/about");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/about"],
				views_data: [{ page: "about" }],
			}),
		);
		const result = await nav;

		expect(result.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/about");
		expect(state.entries[0].view_data).toEqual({ page: "about" });
	});

	it("latest navigation wins when earlier is superseded", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const n1 = core.navigate("/first");
		await wait_for(1);

		const n2 = core.navigate("/second");
		await wait_for(2);

		expect(call(0).signal.aborted).toBe(true);

		call(1).resolve(
			route_response({
				matched_patterns: ["/second"],
				views_data: [{ page: "second" }],
			}),
		);
		const [r1, r2] = await Promise.all([n1, n2]);

		expect(r1.didNavigate).toBe(false);
		expect(r2.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ page: "second" });
	});

	it("settles superseded navigation without waiting for ignored abort", async () => {
		const { core, commit } = await setup();
		const first_fetch = deferred<Response>();
		const second_fetch = deferred<Response>();
		const calls: Array<{ url: string; signal: AbortSignal }> = [];

		vi.spyOn(globalThis, "fetch").mockImplementation(
			(input: string | URL | Request, init?: RequestInit) => {
				const url =
					input instanceof URL
						? input.href
						: typeof input === "string"
							? input
							: input.url;
				if (!init?.signal) {
					throw new Error("Expected route fetch to receive a signal");
				}
				calls.push({ url, signal: init.signal });
				if (url.includes("/first")) {
					return first_fetch.promise;
				}
				return second_fetch.promise;
			},
		);

		const first = core.navigate("/first");
		await tick();
		expect(calls).toHaveLength(1);

		const second = core.navigate("/second");
		await tick();
		expect(calls).toHaveLength(2);
		expect(calls[0]!.signal.aborted).toBe(true);
		await expect(first).resolves.toEqual({ didNavigate: false });

		second_fetch.resolve(
			route_response({
				matched_patterns: ["/second"],
				views_data: [{ page: "second" }],
			}),
		);
		await expect(second).resolves.toEqual({ didNavigate: true });

		first_fetch.resolve(
			route_response({
				matched_patterns: ["/first"],
				views_data: [{ page: "first" }],
			}),
		);
		await tick();

		expect(route_render_commit_count(commit)).toBe(1);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].view_data).toEqual({ page: "second" });
	});

	it("aborts in-flight fetch when new navigation starts", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.navigate("/first");
		await wait_for(1);

		void core.navigate("/second");
		await wait_for(2);

		expect(calls[0]!.signal.aborted).toBe(true);
		expect(calls[1]!.url).toContain("/second");

		call(1).resolve(route_response());
		await tick();
	});

	it("shares result when already navigating to same URL", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const n1 = core.navigate("/same");
		await wait_for(1);
		const n2 = core.navigate("/same");

		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		const [r1, r2] = await Promise.all([n1, n2]);

		expect(r1.didNavigate).toBe(true);
		expect(r2.didNavigate).toBe(true);
	});

	it("reuses in-flight fetch when only hash differs", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const n1 = core.navigate("/page#one");
		await wait_for(1);
		const n2 = core.navigate("/page#two");

		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		const [r1, r2] = await Promise.all([n1, n2]);

		expect(r1.didNavigate).toBe(false);
		expect(r2.didNavigate).toBe(true);
	});

	it("commits the latest hash intent when reusing in-flight view data", async () => {
		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const first = core.navigate("/page#one");
		await wait_for(1);
		const second = core.navigate("/page#two");

		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		const [r1, r2] = await Promise.all([first, second]);

		expect(r1.didNavigate).toBe(false);
		expect(r2.didNavigate).toBe(true);
		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ hash: "#two" });
		expect(window.location.hash).toBe("#two");
	});

	it("starts new fetch when search params differ", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.navigate("/page?a=1");
		await wait_for(1);
		void core.navigate("/page?b=2");
		await wait_for(2);

		expect(calls[0]!.signal.aborted).toBe(true);
		expect(calls[1]!.url).toContain("b=2");

		call(1).resolve(route_response());
		await tick();
	});

	it("does not commit on non-ok response", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/fail");
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500, statusText: "Err" }));
		const result = await nav;

		expect(result.didNavigate).toBe(false);
		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("does not commit on 304 response", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/not-modified");
		await wait_for(1);
		call(0).resolve(new Response(null, { status: 304, statusText: "Not Modified" }));
		const result = await nav;

		expect(result.didNavigate).toBe(false);
		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("does not commit on network failure", async () => {
		const { core, commit } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(new Error("network down"));

		const result = await core.navigate("/fail");

		expect(result.didNavigate).toBe(false);
		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("returns failure on JSON parse error", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/bad-json");
		await wait_for(1);
		call(0).resolve(
			new Response("not json", {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const result = await nav;

		expect(result.didNavigate).toBe(false);
		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("does not fetch for same-document no-op navigation", async () => {
		const { core } = await setup();
		const { calls } = mock_fetch();

		const result = await core.navigate("/");

		expect(calls).toHaveLength(0);
		expect(result.didNavigate).toBe(false);
	});

	it("does not fetch when only hash differs from current URL", async () => {
		const { core } = await setup();
		const { calls } = mock_fetch();

		const result = await core.navigate("/#section");

		expect(calls).toHaveLength(0);
		expect(result.didNavigate).toBe(true);
	});

	it("fetches when search params differ from current URL", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.navigate("/?a=1");
		await wait_for(1);
		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		await tick();
	});

	it("superseded navigation does not commit", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const n1 = core.navigate("/aborted");
		await wait_for(1);

		const n2 = core.navigate("/winner");
		await wait_for(2);

		call(1).resolve(
			route_response({
				matched_patterns: ["/winner"],
				views_data: [{ winner: true }],
			}),
		);
		await Promise.all([n1, n2]);

		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ winner: true });
	});

	it("recovers after network failure and allows subsequent navigation", async () => {
		const { core, commit } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(new Error("network down"));
		const r1 = await core.navigate("/fail");
		expect(r1.didNavigate).toBe(false);

		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/recover");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/recover"],
				views_data: [{ recovered: true }],
			}),
		);
		const r2 = await nav;

		expect(r2.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ recovered: true });
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prefetch
/////////////////////////////////////////////////////////////////////

describe("prefetch", () => {
	it("fires fetch but does not commit", async () => {
		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/prefetch");
		await wait_for(1);
		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		await tick();

		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("promotes to navigate on matching URL without refetching", async () => {
		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/page");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/page"],
				views_data: [{ prefetched: true }],
			}),
		);
		await tick();

		const result = await core.navigate("/page");

		expect(calls).toHaveLength(1);
		expect(result.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].view_data).toEqual({ prefetched: true });
	});

	it("does not promote a completed failed prefetch", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/page");
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500, statusText: "Err" }));
		await tick();

		const result = core.navigate("/page");
		await wait_for(2);
		call(1).resolve(route_response());

		await expect(result).resolves.toEqual({ didNavigate: true });
		expect(calls).toHaveLength(2);
	});

	it("promotes when only hash differs", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/page#a");
		await wait_for(1);
		call(0).resolve(route_response());
		await tick();

		const result = await core.navigate("/page#b");

		expect(calls).toHaveLength(1);
		expect(result.didNavigate).toBe(true);
	});

	it("aborts previous prefetch when new one starts", async () => {
		const { core } = await setup();
		const { calls, wait_for } = mock_fetch();

		core.start_prefetch("/first");
		await wait_for(1);
		core.start_prefetch("/second");
		await wait_for(2);

		expect(calls[0]!.signal.aborted).toBe(true);
		expect(calls[1]!.url).toContain("/second");
	});

	it("skips prefetch when URL matches current page", async () => {
		const { core } = await setup();
		const { calls } = mock_fetch();

		core.start_prefetch("/");
		await tick();

		expect(calls).toHaveLength(0);
	});

	it("stop_prefetch aborts in-flight fetch", async () => {
		const { core } = await setup();
		const { calls, wait_for } = mock_fetch();

		core.start_prefetch("/prefetch");
		await wait_for(1);

		expect(calls[0]!.signal.aborted).toBe(false);
		core.stop_prefetch("/prefetch");
		expect(calls[0]!.signal.aborted).toBe(true);
	});

	it("does not start prefetch when already navigating to same URL", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.navigate("/page");
		await wait_for(1);

		core.start_prefetch("/page");
		await tick();

		expect(calls).toHaveLength(1);

		call(0).resolve(route_response());
		await tick();
	});

	it("does not start prefetch for cross-origin URLs", async () => {
		const { core } = await setup();
		const { calls } = mock_fetch();

		core.start_prefetch("https://external.example/page");
		await tick();

		expect(calls).toHaveLength(0);
	});

	it("reuses in-flight prefetch when only hash differs", async () => {
		const { core } = await setup();
		const { calls, wait_for } = mock_fetch();

		core.start_prefetch("/page#first");
		await wait_for(1);

		core.start_prefetch("/page#second");
		await tick();

		const non_aborted = calls.filter((c) => {
			return !c.signal.aborted;
		});
		expect(non_aborted).toHaveLength(1);
	});

	it("starts new prefetch when path differs", async () => {
		const { core } = await setup();
		const { calls, wait_for } = mock_fetch();

		core.start_prefetch("/page-a");
		await wait_for(1);
		core.start_prefetch("/page-b");
		await wait_for(2);

		expect(calls).toHaveLength(2);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Navigate redirects
/////////////////////////////////////////////////////////////////////

describe("navigate redirects", () => {
	it("follows X-Client-Redirect", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/target" }));
		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/target"],
				views_data: [{ redirected: true }],
			}),
		);
		const result = await nav;

		expect(result.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].view_data).toEqual({ redirected: true });
	});

	it("follows redirect even when response is non-ok", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(non_ok_redirect_response({ [X_CLIENT_REDIRECT]: "/target" }));
		await wait_for(2);
		call(1).resolve(route_response());
		const result = await nav;

		expect(result.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
	});

	it("hard reloads navigation when view data reports build skew", async () => {
		const on_build_skew = vi.fn();
		const { core, hard_redirect } = await setup({
			clientOptions: { onBuildSkewDetected: on_build_skew },
		});
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_VORMA_BUILD_SKEW]: "1",
			}),
		);
		await nav;

		expect(hard_redirect).toHaveBeenCalledWith(expect.stringContaining("/start"));
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "navigation",
					requestedHref: `${window.location.origin}/start`,
				}),
			}),
		);
	});

	it("prioritizes build skew over X-Client-Redirect", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_VORMA_BUILD_SKEW]: "1",
				[X_CLIENT_REDIRECT]: "/soft",
			}),
		);
		await nav;

		expect(hard_redirect).toHaveBeenCalledWith(expect.stringContaining("/start"));
	});

	it("follows native browser redirects", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(
			native_redirect_response(`${window.location.origin}/native-target`),
		);
		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/native-target"],
				views_data: [{ native: true }],
			}),
		);
		const result = await nav;

		expect(result.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
	});

	it("hard redirects for cross-origin soft redirect", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "https://external.example/path",
			}),
		);
		await nav;

		expect(hard_redirect).toHaveBeenCalledWith("https://external.example/path");
	});

	it("chains through multiple soft redirects", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/mid" }));
		await wait_for(2);
		call(1).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/final" }));
		await wait_for(3);
		call(2).resolve(
			route_response({
				matched_patterns: ["/final"],
				views_data: [{ final: true }],
			}),
		);
		const result = await nav;

		expect(result.didNavigate).toBe(true);
		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].view_data).toEqual({ final: true });
	});

	it("maintains navigating status through redirect chains", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		expect(core.getWorkState().navigation !== null).toBe(true);

		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/mid" }));
		await wait_for(2);
		expect(core.getWorkState().navigation !== null).toBe(true);

		call(1).resolve(route_response());
		await nav;

		expect(core.getWorkState().navigation !== null).toBe(false);
	});

	it("caps redirect chains and does not commit", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");

		for (let i = 0; i <= MAX_REDIRECTS; i++) {
			await wait_for(i + 1);
			call(i).resolve(
				redirect_response({
					[X_CLIENT_REDIRECT]: `/redirect-${i}`,
				}),
			);
		}

		const result = await nav;

		expect(result.didNavigate).toBe(false);
		expect(has_route_render_commit(commit)).toBe(false);
	});

	it("does not follow stale redirect from superseded navigation", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const n1 = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/redirect-target" }));
		await wait_for(2);

		const n2 = core.navigate("/winner");
		await wait_for(3);
		call(2).resolve(
			route_response({
				matched_patterns: ["/winner"],
				views_data: [{ winner: true }],
			}),
		);
		await Promise.all([n1, n2]);

		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ winner: true });
	});

	it("resolves relative redirect targets against request URL", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.navigate("/base/start");
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "child/final" }));
		await wait_for(2);

		expect(call(1).url).toContain("/base/child/final");
		call(1).resolve(route_response());
		await tick();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Redirect edge cases
/////////////////////////////////////////////////////////////////////

describe("redirect edge cases", () => {
	it("hard reloads build skew responses to the requested URL", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/base/start");
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_VORMA_BUILD_SKEW]: "1",
			}),
		);
		await nav;

		expect(hard_redirect).toHaveBeenCalledWith(
			expect.stringContaining("/base/start"),
		);
	});

	it("treats redirect to current URL as completion without re-fetching", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		// Navigate to /current first
		const first_nav = core.navigate("/current");
		await wait_for(1);
		call(0).resolve(route_response());
		await first_nav;

		// Navigate to /start, get redirected back to /current
		const second_nav = core.navigate("/start");
		await wait_for(2);
		call(1).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/current" }));
		await second_nav;

		// No third fetch — redirect target matches current page
		expect(() => {
			return call(2);
		}).toThrow();
		expect(core.getWorkState().navigation !== null).toBe(false);
	});

	it("returns didNavigate false for non-HTTP redirect schemes", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "mailto:test@example.com",
			}),
		);
		const result = await nav;

		expect(result.didNavigate).toBe(false);
		expect(hard_redirect).not.toHaveBeenCalled();
		expect(core.getWorkState().navigation !== null).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Navigate stress
/////////////////////////////////////////////////////////////////////

describe("navigate stress", () => {
	it.each([11, 29, 47, 83, 131])(
		"last-started wins across rapid navigations (seed=%d)",
		async (seed: number) => {
			const { core } = await setup();
			const { calls, wait_for } = mock_fetch();
			const nav_count = 6;

			const promises = [];
			for (let i = 0; i < nav_count; i++) {
				promises.push(core.navigate(`/stress-${seed}-${i}`));
			}

			// Only last navigation's fetch should be non-aborted
			await wait_for(calls.length);
			const non_aborted = calls.filter((c) => {
				return !c.signal.aborted;
			});
			expect(non_aborted).toHaveLength(1);

			non_aborted[0]!.resolve(
				route_response({
					matched_patterns: [`/stress-${seed}-last`],
					views_data: [{ index: nav_count - 1 }],
				}),
			);

			const results = await Promise.all(promises);
			const last = results[nav_count - 1]!;
			expect(last.didNavigate).toBe(true);

			for (let i = 0; i < nav_count - 1; i++) {
				expect(results[i]!.didNavigate).toBe(false);
			}

			expect(core.getWorkState().navigation !== null).toBe(false);
		},
	);
});

/////////////////////////////////////////////////////////////////////
/////// did_navigate return value
/////////////////////////////////////////////////////////////////////

describe("did_navigate return value", () => {
	it("returns didNavigate true on successful navigation", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		const result = await nav;

		expect(result.didNavigate).toBe(true);
	});

	it("returns didNavigate false for same-document no-op", async () => {
		const { core } = await setup();
		mock_fetch();

		const result = await core.navigate("/");

		expect(result.didNavigate).toBe(false);
	});

	it("returns didNavigate true for hash-only change", async () => {
		const { core } = await setup();
		mock_fetch();

		const result = await core.navigate("/#section");

		expect(result.didNavigate).toBe(true);
	});

	it("returns didNavigate false when superseded", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const n1 = core.navigate("/first");
		await wait_for(1);
		const n2 = core.navigate("/second");
		await wait_for(2);

		call(1).resolve(route_response());
		const [r1, r2] = await Promise.all([n1, n2]);

		expect(r1.didNavigate).toBe(false);
		expect(r2.didNavigate).toBe(true);
	});

	it("returns didNavigate true through redirect chain", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/target" }));
		await wait_for(2);
		call(1).resolve(route_response());
		const result = await nav;

		expect(result.didNavigate).toBe(true);
	});
});

/////////////////////////////////////////////////////////////////////
/////// History commits
/////////////////////////////////////////////////////////////////////

describe("history commits", () => {
	it("pushes history on navigate", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(push_spy).toHaveBeenCalledTimes(1);
		expect(push_spy.mock.calls[0]![2]).toBe(`${window.location.origin}/page`);
		const state = push_spy.mock.calls[0]![0] as Record<string, unknown>;
		expect(typeof state[HISTORY_KEY_FIELD]).toBe("string");
	});

	it("replaces history when replace is true", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const initial_replace_count = replace_spy.mock.calls.length;
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page", { replace: true });
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(push_spy).not.toHaveBeenCalled();
		expect(replace_spy.mock.calls.length).toBeGreaterThan(initial_replace_count);
	});

	it("does not push history for same-document no-op", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");

		await core.navigate("/");

		expect(push_spy).not.toHaveBeenCalled();
	});

	it("pushes history through redirect chains to final URL", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start");
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/final" }));
		await wait_for(2);
		call(1).resolve(route_response());
		await nav;

		expect(push_spy).toHaveBeenCalledTimes(1);
		expect(push_spy.mock.calls[0]![2]).toBe(`${window.location.origin}/final`);
	});

	it("generates unique keys for each history entry", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const { call, wait_for } = mock_fetch();

		const n1 = core.navigate("/first");
		await wait_for(1);
		call(0).resolve(route_response());
		await n1;

		const n2 = core.navigate("/second");
		await wait_for(2);
		call(1).resolve(route_response());
		await n2;

		expect(push_spy).toHaveBeenCalledTimes(2);
		const key1 = (push_spy.mock.calls[0]![0] as any)[HISTORY_KEY_FIELD];
		const key2 = (push_spy.mock.calls[1]![0] as any)[HISTORY_KEY_FIELD];
		expect(key1).not.toBe(key2);
	});

	it("propagates replace through redirect chains", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const initial_replace_count = replace_spy.mock.calls.length;
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start", { replace: true });
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/final" }));
		await wait_for(2);
		call(1).resolve(route_response());
		await nav;

		expect(push_spy).not.toHaveBeenCalled();
		const replace_calls_after = replace_spy.mock.calls.slice(initial_replace_count);
		expect(replace_calls_after.length).toBeGreaterThanOrEqual(1);
		const last_replace = replace_calls_after[replace_calls_after.length - 1]!;
		expect(last_replace[2]).toBe(`${window.location.origin}/final`);
	});

	it("does not push history on prefetch", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const { call, wait_for } = mock_fetch();

		core.start_prefetch("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await tick();

		expect(push_spy).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Scroll state
/////////////////////////////////////////////////////////////////////

describe("scroll state", () => {
	it("passes scroll-to-top by default on navigate", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(has_route_render_commit(commit)).toBe(true);
		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ x: 0, y: 0 });
	});

	it("passes hash scroll when URL has hash", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page#section");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(has_route_render_commit(commit)).toBe(true);
		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ hash: "#section" });
	});

	it("passes undefined scroll when scrollToTop is false", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page", { scrollToTop: false });
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(has_route_render_commit(commit)).toBe(true);
		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent).toBeUndefined();
	});

	it("saves current scroll position before committing history", async () => {
		const { core } = await setup();
		const seed_key = get_history_key();
		set_scroll_position(100, 200);
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const saved = get_stored_scroll(seed_key);
		expect(saved).toEqual({ x: 100, y: 200 });
	});

	it("preserves scrollToTop false through redirect chains", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/start", { scrollToTop: false });
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/final" }));
		await wait_for(2);
		call(1).resolve(route_response());
		await nav;

		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Hash-only navigation
/////////////////////////////////////////////////////////////////////

describe("hash-only navigation", () => {
	it("does not fetch and scrolls to hash element", async () => {
		const { core } = await setup();
		const { calls } = mock_fetch();

		const el = document.createElement("div");
		el.id = "two";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		await core.navigate("/#two");

		expect(calls).toHaveLength(0);
		// oxlint-disable-next-line unbound-method
		expect(el.scrollIntoView).toHaveBeenCalled();
	});

	it("pushes history on hash-only navigate", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");

		await core.navigate("/#section");

		expect(push_spy).toHaveBeenCalledTimes(1);
		expect(push_spy.mock.calls[0]![2]).toContain("#section");
	});

	it("replaces history on hash-only navigate with replace: true", async () => {
		const { core } = await setup();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const initial_replace_count = replace_spy.mock.calls.length;

		await core.navigate("/#section", { replace: true });

		expect(push_spy).not.toHaveBeenCalled();
		expect(replace_spy.mock.calls.length).toBeGreaterThan(initial_replace_count);
	});

	it("scrolls to top for truly identical URL", async () => {
		const { core, scroll_to } = await setup();

		const result = await core.navigate("/");

		expect(result.didNavigate).toBe(false);
		expect(scroll_to).toHaveBeenCalledWith(0, 0);
	});

	it("saves current scroll before hash-only commit", async () => {
		const { core } = await setup();
		const seed_key = get_history_key();
		set_scroll_position(10, 20);

		await core.navigate("/#section");

		expect(get_stored_scroll(seed_key)).toEqual({ x: 10, y: 20 });
	});
});

/////////////////////////////////////////////////////////////////////
/////// Popstate
/////////////////////////////////////////////////////////////////////

describe("popstate", () => {
	it("navigates on popstate with full URL change", async () => {
		const { core, commit } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;
		commit.mockClear();

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/"],
				views_data: [{ home: true }],
			}),
		);

		for (let i = 0; i < 50; i++) {
			if (route_render_commit_count(commit) > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ home: true });
	});

	it("reuses in-flight popstate data for a later navigation intent", async () => {
		const { core, commit, reload } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		simulate_popstate("pop-intent-one", "/target#one");
		await wait_for(1);

		const nav = core.navigate("/target#two");

		expect(calls).toHaveLength(1);

		call(0).resolve(
			route_response({
				matched_patterns: ["/target"],
				views_data: [{ target: true }],
			}),
		);

		const result = await nav;
		await tick();

		expect(result.didNavigate).toBe(true);
		expect(reload).not.toHaveBeenCalled();
		expect(window.location.hash).toBe("#two");
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ target: true });
	});

	it("does not push history on popstate navigation", async () => {
		const { core } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const push_spy = vi.spyOn(window.history, "pushState");

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(route_response());

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().navigation === null) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(push_spy).not.toHaveBeenCalled();
	});

	it("passes stored destination scroll to full popstate navigation", async () => {
		const { core, commit } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;
		commit.mockClear();

		sessionStorage.setItem(
			SCROLL_STORAGE_KEY,
			JSON.stringify([[seed_key, { x: 11, y: 22 }]]),
		);

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(route_response());

		for (let i = 0; i < 50; i++) {
			if (route_render_commit_count(commit) > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(route_render_scroll_intent_at(commit, 0)?.scroll).toEqual({
			x: 11,
			y: 22,
		});
	});

	it("reloads when full popstate view data cannot commit", async () => {
		const { core, reload } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(new Response("error", { status: 500 }));

		for (let i = 0; i < 50; i++) {
			if (reload.mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(reload).toHaveBeenCalledTimes(1);
	});

	it("reloads when full popstate route preparation fails", async () => {
		const { core, commit, reload } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;
		commit.mockClear();

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/"],
				views_data: [{ home: true }],
				import_urls: ["/missing-popstate-route.js"],
			}),
		);

		for (let i = 0; i < 50; i++) {
			if (reload.mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(reload).toHaveBeenCalledTimes(1);
		expect(route_render_commit_count(commit)).toBe(0);
	});

	it("reloads when full popstate route publication fails", async () => {
		vi.doMock("/reject-popstate-route.js", () => {
			return {
				default: {
					pattern: "/",
					component: () => {
						return null;
					},
					before_route_commit: async () => {
						throw new Error("commit rejected");
					},
				},
			};
		});

		const { core, commit, reload } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;
		commit.mockClear();

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/"],
				views_data: [{ home: true }],
				import_urls: ["/reject-popstate-route.js"],
			}),
		);

		for (let i = 0; i < 50; i++) {
			if (reload.mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(reload).toHaveBeenCalledTimes(1);
		expect(route_render_commit_count(commit)).toBe(0);
	});

	it("handles hash-only popstate without fetching", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const el = document.createElement("div");
		el.id = "two";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		const fetch_count = calls.length;

		simulate_popstate("hash-pop-key", "/page#two");

		for (let i = 0; i < 50; i++) {
			if ((el.scrollIntoView as any).mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(calls.length).toBe(fetch_count);
		// oxlint-disable-next-line unbound-method
		expect(el.scrollIntoView).toHaveBeenCalled();
	});

	it("handles hash-only popstate when entries reuse the same history key", async () => {
		const { scroll_to } = await setup();
		const seed_key = get_history_key();
		const one = document.createElement("div");
		one.id = "one";
		one.scrollIntoView = vi.fn();
		document.body.appendChild(one);
		const two = document.createElement("div");
		two.id = "two";
		two.scrollIntoView = vi.fn();
		document.body.appendChild(two);
		const shared_state = window.history.state;

		window.history.pushState(shared_state, "", "/#one");
		window.history.pushState(shared_state, "", "/#two");

		simulate_popstate(seed_key, "/#one");

		// oxlint-disable-next-line unbound-method
		expect(one.scrollIntoView).toHaveBeenCalledTimes(1);
		// oxlint-disable-next-line unbound-method
		expect(two.scrollIntoView).not.toHaveBeenCalled();

		simulate_popstate(seed_key, "/");

		expect(scroll_to).toHaveBeenCalledWith(0, 0);

		simulate_popstate(seed_key, "/#one");
		simulate_popstate(seed_key, "/#two");

		// oxlint-disable-next-line unbound-method
		expect(one.scrollIntoView).toHaveBeenCalledTimes(2);
		// oxlint-disable-next-line unbound-method
		expect(two.scrollIntoView).toHaveBeenCalledTimes(1);
	});

	it("ignores popstate when history key has not changed", async () => {
		const { commit } = await setup();
		const { calls } = mock_fetch();

		window.dispatchEvent(new PopStateEvent("popstate"));

		for (let i = 0; i < 20; i++) {
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(has_route_render_commit(commit)).toBe(false);
		expect(calls).toHaveLength(0);
	});

	it("aborts in-flight navigation on popstate", async () => {
		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		void core.navigate("/page");
		await wait_for(1);

		simulate_popstate("pop-interrupts", "/previous");

		await wait_for(2);
		expect(calls[0]!.signal.aborted).toBe(true);

		call(1).resolve(
			route_response({
				matched_patterns: ["/previous"],
				views_data: [{ previous: true }],
			}),
		);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().navigation === null) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(has_route_render_commit(commit)).toBe(true);
		const state = last_route_render_commit(commit);
		expect(state.entries[0].view_data).toEqual({ previous: true });
		expect(core.getWorkState().navigation !== null).toBe(false);
	});

	it("saves scroll for leaving page on popstate", async () => {
		const { core } = await setup();
		const seed_key = get_history_key();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const page_key = get_history_key();
		set_scroll_position(42, 99);

		simulate_popstate(seed_key, "/");

		await wait_for(2);
		call(1).resolve(route_response());

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().navigation === null) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(get_stored_scroll(page_key)).toEqual({ x: 42, y: 99 });
	});
});

/////////////////////////////////////////////////////////////////////
/////// Scroll persistence
/////////////////////////////////////////////////////////////////////

describe("scroll persistence", () => {
	it("caps stored entries at maximum", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		for (let i = 0; i < MAX_SCROLL_ENTRIES + 5; i++) {
			set_scroll_position(0, i);
			const nav = core.navigate(`/page-${i}`);
			await wait_for(i + 1);
			call(i).resolve(route_response());
			await nav;
		}

		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		const entries = JSON.parse(raw!);
		expect(entries.length).toBeLessThanOrEqual(MAX_SCROLL_ENTRIES);
	});

	it("saves scroll for correct key on successive navigations", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const seed_key = get_history_key();
		set_scroll_position(10, 20);

		const n1 = core.navigate("/first");
		await wait_for(1);
		call(0).resolve(route_response());
		await n1;
		expect(get_stored_scroll(seed_key)).toEqual({ x: 10, y: 20 });

		const first_key = get_history_key();
		set_scroll_position(30, 40);

		const n2 = core.navigate("/second");
		await wait_for(2);
		call(1).resolve(route_response());
		await n2;
		expect(get_stored_scroll(first_key)).toEqual({ x: 30, y: 40 });

		// Original entry preserved
		expect(get_stored_scroll(seed_key)).toEqual({ x: 10, y: 20 });
	});
});

/////////////////////////////////////////////////////////////////////
/////// Refresh scroll
/////////////////////////////////////////////////////////////////////

describe("refresh scroll", () => {
	it("saves scroll on beforeunload", async () => {
		await setup();
		set_scroll_position(15, 25);

		window.dispatchEvent(new Event("beforeunload"));

		const raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
		expect(raw).toBeTruthy();
		const parsed = JSON.parse(raw!);
		expect(parsed.x).toBe(15);
		expect(parsed.y).toBe(25);
		expect(typeof parsed.unix).toBe("number");
		expect(parsed.href).toBe(window.location.href);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Status
/////////////////////////////////////////////////////////////////////

describe("status", () => {
	it("reports isNavigating during navigation", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		expect(core.getWorkState().navigation !== null).toBe(true);

		call(0).resolve(route_response());
		await nav;

		expect(core.getWorkState().navigation !== null).toBe(false);
	});

	it("reports isSubmitting during submission", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.submit_inner(
			"/api/resource",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		expect(core.getWorkState().apiRequests.length > 0).toBe(true);

		call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		for (let i = 0; i < 50; i++) {
			if (core.getWorkState().apiRequests.length === 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(core.getWorkState().apiRequests.length > 0).toBe(false);
	});

	it("emits work notifications when navigation target changes", async () => {
		const work_updates: any[] = [];
		const { core } = await setup({
			clientOptions: {
				onWorkUpdate: (work: any) => {
					work_updates.push({ ...work });
				},
			},
		});
		const { call, wait_for } = mock_fetch();

		void core.navigate("/first");
		await wait_for(1);
		void core.navigate("/second");
		await wait_for(2);

		expect(work_updates).toHaveLength(2);
		expect(work_updates.map((work) => work.navigation?.href)).toEqual([
			`${window.location.origin}/first`,
			`${window.location.origin}/second`,
		]);

		call(1).resolve(route_response());
		await tick();
	});

	it("idle after all operations complete", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(core.getWorkState()).toEqual({
			navigation: null,
			revalidation: null,
			prefetch: null,
			apiRequests: [],
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// X-Accepts-Client-Redirect header
/////////////////////////////////////////////////////////////////////

describe("X-Accepts-Client-Redirect header", () => {
	it("is sent on navigate fetches", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(route_response());

		await core.navigate("/page");

		const headers = new Headers(
			(fetch_spy.mock.calls[0]![1] as RequestInit).headers as HeadersInit,
		);
		expect(headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe("1");
	});

	it("is sent on submit fetches", async () => {
		const { core } = await setup();
		const fetch_spy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await core.submit_inner(
			"/api/resource",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		const headers = new Headers(
			(fetch_spy.mock.calls[0]![1] as RequestInit).headers as HeadersInit,
		);
		expect(headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe("1");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Cross-origin navigation
/////////////////////////////////////////////////////////////////////

describe("cross-origin navigation", () => {
	it("hard redirects for cross-origin navigate without fetching", async () => {
		const { core, hard_redirect } = await setup();
		const { calls } = mock_fetch();

		const result = await core.navigate("https://external.example/path");

		expect(calls).toHaveLength(0);
		expect(hard_redirect).toHaveBeenCalledWith("https://external.example/path");
		expect(result.didNavigate).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// vorma_json search param
/////////////////////////////////////////////////////////////////////

describe("vorma_json search param", () => {
	it("adds vorma_json param with build ID to navigate fetch URL", async () => {
		const { core } = await setup({
			payload: { client_build_id: "b-42" },
		});
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(route_response());

		await core.navigate("/page");

		const fetched_url = new URL(fetch_spy.mock.calls[0]![0] as string);
		expect(fetched_url.searchParams.get(VORMA_JSON_KEY)).toBe("b-42");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Scroll state storage validation
/////////////////////////////////////////////////////////////////////

describe("scroll state storage validation", () => {
	it("filters out entries with non-finite coordinates", async () => {
		const { core } = await setup();

		sessionStorage.setItem(
			SCROLL_STORAGE_KEY,
			JSON.stringify([
				["good", { x: 10, y: 20 }],
				["nan-x", { x: NaN, y: 30 }],
				["nan-y", { x: 10, y: NaN }],
				["inf", { x: Infinity, y: 0 }],
				["neg-inf", { x: 0, y: -Infinity }],
			]),
		);

		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		const entries = JSON.parse(raw!);
		const keys = entries.map((e: any) => {
			return e[0];
		});
		expect(keys).toContain("good");
		expect(keys).not.toContain("nan-x");
		expect(keys).not.toContain("nan-y");
		expect(keys).not.toContain("inf");
		expect(keys).not.toContain("neg-inf");
	});

	it("filters out entries with wrong shapes", async () => {
		const { core } = await setup();

		sessionStorage.setItem(
			SCROLL_STORAGE_KEY,
			JSON.stringify([
				["good", { x: 10, y: 20 }],
				["no-obj", "not-an-object"],
				["null-val", null],
				["missing-y", { x: 10 }],
				["missing-x", { y: 20 }],
				["string-coords", { x: "10", y: "20" }],
				42,
				null,
				"garbage",
			]),
		);

		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		const entries = JSON.parse(raw!);
		const keys = entries.map((e: any) => {
			return e[0];
		});
		expect(keys).toContain("good");
		expect(keys).not.toContain("no-obj");
		expect(keys).not.toContain("null-val");
	});

	it("handles corrupted JSON gracefully", async () => {
		const { core } = await setup();

		sessionStorage.setItem(SCROLL_STORAGE_KEY, "not json at all{{{");

		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(core.getWorkState().navigation !== null).toBe(false);
		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		const entries = JSON.parse(raw!);
		expect(entries.length).toBeGreaterThanOrEqual(1);
	});

	it("handles non-array JSON gracefully", async () => {
		const { core } = await setup();

		sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify({ not: "array" }));

		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		expect(core.getWorkState().navigation !== null).toBe(false);
	});
});
