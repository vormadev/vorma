// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	deferred,
	expect_revalidation_promise_resolves_ok,
	mock_fetch,
	redirect_response,
	register_ccc_lifecycle,
	route_response,
	seed_payload,
	setup,
	t_opts,
	tick,
} from "./_test_helpers.ts";
import { BUILD_ID_HEADER, X_CLIENT_REDIRECT, X_VORMA_BUILD_SKEW } from "./constants.ts";
import {
	REVALIDATION_DEBOUNCE_MS,
	create_client_core,
	type ClientCore,
	type WorkIndicatorOptions,
} from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

describe("work integration", () => {
	it("onWorkUpdate receives current work state", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		const work_updates: any[] = [];
		await core.boot({
			onWorkUpdate: (work) => {
				return work_updates.push({ ...work });
			},
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(route_response());

		await core.navigate("/page");

		expect(work_updates.length).toBeGreaterThan(0);
		const navigating = work_updates.find((work) => {
			return work.navigation !== null;
		});
		expect(navigating).toBeDefined();
		expect(navigating).toHaveProperty("revalidation");
		expect(navigating).toHaveProperty("apiRequests");
	});

	it("getWorkState returns current snapshot", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		const idle = core.getWorkState();
		expect(idle).toEqual({
			navigation: null,
			revalidation: null,
			prefetch: null,
			apiRequests: [],
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// API submit revalidation settlement
/////////////////////////////////////////////////////////////////////

describe("API submit revalidation settlement", () => {
	it("settles the revalidation promise for cross-origin mutation rejection", async () => {
		const { core } = await setup();

		const result = await core.submit_inner("https://example.com/api", {
			method: "POST",
		});

		expect(result.success).toBe(false);
		await expect_revalidation_promise_resolves_ok(result.revalidationPromise);
	});

	it("settles the revalidation promise for invalid API redirects", async () => {
		const { core } = await setup();
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			redirect_response({
				[X_CLIENT_REDIRECT]: "mailto:not-http",
			}),
		);

		const result = await core.submit_inner("/api/some-resource", {
			method: "POST",
		});

		expect(result.success).toBe(false);
		await expect_revalidation_promise_resolves_ok(result.revalidationPromise);
	});

	it("settles the revalidation promise for hard API redirects", async () => {
		const { core, hard_redirect } = await setup();
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			redirect_response({
				[X_CLIENT_REDIRECT]: "https://example.com/elsewhere",
			}),
		);

		const result = await core.submit_inner("/api/some-resource", {
			method: "POST",
		});

		expect(result.success).toBe(true);
		expect(hard_redirect).toHaveBeenCalledWith("https://example.com/elsewhere");
		await expect_revalidation_promise_resolves_ok(result.revalidationPromise);
	});

	it("reports build skew context for hard API redirects", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			clientOptions: { onBuildSkewDetected: on_build_skew },
			payload: { client_build_id: "build-1" },
		});
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_CLIENT_REDIRECT]: "https://example.com/elsewhere",
			}),
		);

		const result = await core.submit_inner("/api/some-resource", {
			method: "POST",
		});

		expect(result.success).toBe(true);
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					resourceKind: "mutation",
					kind: "resource",
					method: "POST",
					ok: true,
					requestedHref: `${window.location.origin}/api/some-resource`,
					status: 200,
				}),
			}),
		);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Work indicators
/////////////////////////////////////////////////////////////////////

describe("work indicators", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	type WorkIndicatorRenderEvent = {
		kind: "start" | "stop";
		was_visible: boolean;
	};

	type WorkIndicatorRenderer = WorkIndicatorOptions & {
		events: WorkIndicatorRenderEvent[];
		force_visible: () => void;
		is_visible: () => boolean;
		reset_events: () => void;
		start: ReturnType<typeof vi.fn>;
		stop: ReturnType<typeof vi.fn>;
	};

	function make_work_indicator_renderer(
		initial_visible = false,
	): WorkIndicatorRenderer {
		let visible = initial_visible;
		const events: WorkIndicatorRenderEvent[] = [];
		const start = vi.fn(() => {
			events.push({ kind: "start", was_visible: visible });
			visible = true;
		});
		const stop = vi.fn(() => {
			events.push({ kind: "stop", was_visible: visible });
			visible = false;
		});
		return {
			events,
			force_visible: () => {
				visible = true;
			},
			is_visible: () => {
				return visible;
			},
			reset_events: () => {
				events.length = 0;
				start.mockClear();
				stop.mockClear();
			},
			start,
			startDelayMs: 1,
			stop,
			stopDelayMs: 1,
		};
	}

	function expect_work_indicator_idle(
		core: ClientCore,
		config: WorkIndicatorRenderer,
	): void {
		expect(core.workIndicator.isActive()).toBe(false);
		expect(core.getWorkState()).toEqual({
			apiRequests: [],
			navigation: null,
			prefetch: null,
			revalidation: null,
		});
		expect(config.is_visible()).toBe(false);
	}

	function expect_stop_after_last_start(config: WorkIndicatorRenderer): void {
		let last_start_index = -1;
		let last_stop_index = -1;
		for (let i = 0; i < config.events.length; i++) {
			const event = config.events[i];
			if (event?.kind === "start") {
				last_start_index = i;
			}
			if (event?.kind === "stop") {
				last_stop_index = i;
			}
		}
		expect(last_start_index).toBeGreaterThanOrEqual(0);
		expect(last_stop_index).toBeGreaterThan(last_start_index);
	}

	async function setup_core(workIndicator: WorkIndicatorOptions): Promise<ClientCore> {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		await core_res.val.boot({ workIndicator });
		return core_res.val;
	}

	it("starts and stops around navigation with delays", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 10,
			stopDelayMs: 10,
		};
		const core = await setup_core(config);

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		void core.navigate("/page");
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).toHaveBeenCalled();

		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();

		expect(config.stop).toHaveBeenCalled();
	});

	it("respects category skips", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			skipApiRequests: true,
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);

		vi.spyOn(globalThis, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await core.submit_inner(
			"/api/some-resource",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips work indicator for opted-out submissions", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);

		vi.spyOn(globalThis, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await core.submit_inner(
			"/api/some-resource",
			{ method: "POST" },
			{
				revalidate: false,
				skipWorkIndicator: true,
			},
		);
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips work indicator for opted-out GET query submissions", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const submit = core.submit_inner("/api/search?q=ada", undefined, {
			resourceKind: "query",
			dedupeKey: "search:debug",
			skipWorkIndicator: true,
		});
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(core.getWorkState().apiRequests).toEqual([
			{
				key: "search:debug",
				method: "GET",
				href: "http://localhost:3000/api/search?q=ada",
			},
		]);
		expect(config.start).not.toHaveBeenCalled();
		expect(core.workIndicator.isActive()).toBe(false);

		fetcher.call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const result = await submit;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(result.success).toBe(true);
		expect(fetcher.calls).toHaveLength(1);
		expect(config.start).not.toHaveBeenCalled();
		expect(config.stop).not.toHaveBeenCalled();
		expect_work_indicator_idle(core, config);
	});

	it("skips work indicator for opted-out mutation revalidation", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const submit = core.submit_inner(
			"/api/some-resource",
			{ method: "POST" },
			{
				skipWorkIndicator: true,
			},
		);
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).not.toHaveBeenCalled();

		fetcher.call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const result = await submit;
		await fetcher.wait_for(2);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).not.toHaveBeenCalled();

		fetcher.call(1).resolve(route_response());
		await result.revalidationPromise;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("shows work indicator for focus-triggered revalidation by default", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;
		const fetcher = mock_fetch();

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 100 },
			workIndicator: config,
		});

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);
		await fetcher.wait_for(1);

		expect(core.getWorkState().revalidation).not.toBeNull();
		expect(config.start).toHaveBeenCalledTimes(1);

		fetcher.call(0).resolve(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("skips work indicator for configured focus-triggered revalidation", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;
		const fetcher = mock_fetch();

		await core.boot({
			revalidateOnWindowFocus: {
				skipWorkIndicator: true,
				staleTimeMs: 100,
			},
			workIndicator: config,
		});

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);
		await fetcher.wait_for(1);

		expect(core.getWorkState().revalidation).not.toBeNull();
		expect(config.start).not.toHaveBeenCalled();

		fetcher.call(0).resolve(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips work indicator for opted-out navigations", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		void core.navigate("/quiet-page", {
			skipWorkIndicator: true,
		});
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();

		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();
	});

	it("overlapping work does not cause start-stop thrash", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);

		let resolve_first!: (r: Response) => void;
		let resolve_second!: (r: Response) => void;
		let call_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			call_count++;
			if (call_count === 1) {
				return new Promise((r) => {
					resolve_first = r;
				});
			}
			return new Promise((r) => {
				resolve_second = r;
			});
		});

		void core.submit_inner("/api/a", { method: "POST" }, { revalidate: false });
		void core.submit_inner("/api/b", { method: "POST" }, { revalidate: false });
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);

		resolve_first(
			new Response(JSON.stringify({}), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.advanceTimersByTimeAsync(0);

		// Still one submit in flight, should not stop
		expect(config.stop).not.toHaveBeenCalled();

		resolve_second(
			new Response(JSON.stringify({}), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("clears pending start timer when work finishes before delay", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 100,
			stopDelayMs: 10,
		};
		const core = await setup_core(config);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(route_response());

		await core.navigate("/fast-page");
		await vi.advanceTimersByTimeAsync(200);

		expect(config.start).not.toHaveBeenCalled();
		expect(config.stop).not.toHaveBeenCalled();
	});

	it("cancels pending stop timer when new work begins", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 100,
		};
		const core = await setup_core(config);

		let resolve_first!: (r: Response) => void;
		let resolve_second!: (r: Response) => void;
		let call_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			call_count++;
			if (call_count === 1) {
				return new Promise((r) => {
					resolve_first = r;
				});
			}
			return new Promise((r) => {
				resolve_second = r;
			});
		});

		void core.navigate("/first");
		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		resolve_first(route_response());
		await vi.advanceTimersByTimeAsync(0);
		await tick();

		void core.navigate("/second");
		await vi.advanceTimersByTimeAsync(100);

		expect(config.stop).not.toHaveBeenCalled();
		expect(config.start).toHaveBeenCalledTimes(1);

		resolve_second(route_response());
		await vi.advanceTimersByTimeAsync(100);
		await tick();

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("tracks app-owned promise work", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);
		const external_work = deferred<number>();

		const tracked = core.workIndicator.track(external_work.promise);

		expect(core.workIndicator.isActive()).toBe(true);
		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		external_work.resolve(42);
		await expect(tracked).resolves.toBe(42);

		expect(core.workIndicator.isActive()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("keeps app-owned work active after Vorma work settles", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMs: 1,
			stopDelayMs: 1,
		};
		const core = await setup_core(config);
		const external_work = deferred<void>();
		const tracked = core.workIndicator.track(external_work.promise);

		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		const nav = core.navigate("/page");
		await vi.advanceTimersByTimeAsync(1);
		resolve_fetch(route_response());
		await nav;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.stop).not.toHaveBeenCalled();

		external_work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("does not stop a visible renderer when boot starts idle", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer(true);

		await setup_core(config);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.stop).not.toHaveBeenCalled();
		expect(config.is_visible()).toBe(true);
	});

	it("does not stop a visible renderer after skipped Vorma work settles", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		await vi.advanceTimersByTimeAsync(1);
		config.reset_events();
		config.force_visible();

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		void core.navigate("/quiet-page", {
			skipWorkIndicator: true,
		});
		await vi.advanceTimersByTimeAsync(1);
		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect(config.stop).not.toHaveBeenCalled();
		expect(config.is_visible()).toBe(true);
	});

	it("stops after Vorma-owned navigation aborts", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(
			new DOMException("Aborted", "AbortError"),
		);

		await core.navigate("/aborted");
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect(config.start).not.toHaveBeenCalled();
		expect_work_indicator_idle(core, config);

		const slow_fetch = mock_fetch();
		const nav = core.navigate("/slow-abort");
		await slow_fetch.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);
		slow_fetch.call(0).reject(new DOMException("Aborted", "AbortError"));
		await nav;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("stops after Vorma-owned API request rejects", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const submit = core.submit_inner(
			"/api/failing-resource",
			{ method: "POST" },
			{ revalidate: false },
		);
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);
		fetcher.call(0).reject(new Error("network down"));
		const result = await submit;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(result.success).toBe(false);
		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("stops after revalidation debounce and fetch settle", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const revalidation = core.revalidate();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(1);
		fetcher.call(0).resolve(route_response());
		await revalidation;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("moves a visible Vorma-owned indicator across option replacement", async () => {
		vi.useFakeTimers();
		const first_config = make_work_indicator_renderer();
		const core = await setup_core(first_config);
		const external_work = deferred<void>();
		const tracked = core.workIndicator.track(external_work.promise);

		await vi.advanceTimersByTimeAsync(1);

		expect(first_config.start).toHaveBeenCalledTimes(1);
		expect(first_config.is_visible()).toBe(true);

		const second_config = make_work_indicator_renderer();
		await core.boot({ workIndicator: second_config });
		await vi.advanceTimersByTimeAsync(1);

		expect(first_config.is_visible()).toBe(false);
		expect(first_config.stop).toHaveBeenCalledTimes(1);
		expect(second_config.start).toHaveBeenCalledTimes(1);
		expect(second_config.is_visible()).toBe(true);

		external_work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, second_config);
		expect_stop_after_last_start(first_config);
		expect_stop_after_last_start(second_config);
	});

	it("keeps the renderer reconciled after varied work ordering", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const tracked_work = [deferred<void>(), deferred<void>(), deferred<void>()];

		const tracked = tracked_work.map((work) => {
			return core.workIndicator.track(work.promise);
		});

		let resolve_first_fetch!: (r: Response) => void;
		let resolve_second_fetch!: (r: Response) => void;
		let fetch_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			fetch_count++;
			if (fetch_count === 1) {
				return new Promise((r) => {
					resolve_first_fetch = r;
				});
			}
			if (fetch_count === 2) {
				return new Promise((r) => {
					resolve_second_fetch = r;
				});
			}
			return Promise.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
		});

		const first_nav = core.navigate("/first");
		await vi.advanceTimersByTimeAsync(1);
		void core.submit_inner(
			"/api/some-resource",
			{ method: "POST" },
			{ revalidate: false },
		);
		tracked_work[1]!.resolve();
		await tracked[1];
		resolve_first_fetch(route_response());
		await first_nav;
		await tick();

		const second_nav = core.navigate("/second");
		await vi.advanceTimersByTimeAsync(1);
		tracked_work[0]!.resolve();
		await tracked[0];
		resolve_second_fetch(route_response());
		await second_nav;
		await tick();

		tracked_work[2]!.resolve();
		await tracked[2];
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("does not orphan after mixed Vorma and app work settle", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();
		const external_work = deferred<void>();
		const tracked = core.workIndicator.track(external_work.promise);

		const nav = core.navigate("/chaos-nav");
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);

		const submit = core.submit_inner(
			"/api/chaos-resource",
			{ method: "POST" },
			{ revalidate: false },
		);
		await fetcher.wait_for(2);
		const revalidation = core.revalidate();

		fetcher.call(0).resolve(route_response());
		await nav;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.is_visible()).toBe(true);

		external_work.resolve();
		await tracked;
		fetcher.call(1).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await submit;
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(3);

		expect(config.is_visible()).toBe(true);

		fetcher.call(2).resolve(route_response());
		await revalidation;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Revalidation timers
/////////////////////////////////////////////////////////////////////

describe("revalidation timers", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it("clears replaced refresh debounce timers", async () => {
		vi.useFakeTimers();
		const { core } = await setup();
		const clear_timeout = vi.spyOn(window, "clearTimeout");
		const fetcher = mock_fetch();

		const first = core.revalidate();
		const second = core.revalidate();

		expect(clear_timeout).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(1);
		fetcher.call(0).resolve(route_response());

		await first;
		await second;
	});

	it("keeps same-hash navigation a no-op while revalidation is running", async () => {
		vi.useFakeTimers();
		const { core, scroll_to } = await setup();
		const fetcher = mock_fetch();

		const revalidation = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(1);
		const push_state = vi.spyOn(window.history, "pushState");

		await expect(core.navigate("/")).resolves.toEqual({
			didNavigate: false,
		});

		expect(push_state).not.toHaveBeenCalled();
		expect(scroll_to).toHaveBeenCalledWith(0, 0);

		fetcher.call(0).resolve(route_response());
		await revalidation;
	});

	it("keeps same-hash replace navigation non-navigating while revalidation is running", async () => {
		vi.useFakeTimers();
		const { core, scroll_to } = await setup();
		const fetcher = mock_fetch();

		const revalidation = core.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(1);
		const replace_state = vi.spyOn(window.history, "replaceState");

		await expect(
			core.navigate("/", {
				replace: true,
				state: { source: "replace" },
			}),
		).resolves.toEqual({
			didNavigate: false,
		});

		expect(replace_state).toHaveBeenCalled();
		expect(core.getRouteState().historyState).toEqual({
			source: "replace",
		});
		expect(scroll_to).toHaveBeenCalledWith(0, 0);

		fetcher.call(0).resolve(route_response());
		await revalidation;
	});
});

/////////////////////////////////////////////////////////////////////
/////// Focus-triggered revalidation
/////////////////////////////////////////////////////////////////////

describe("focus-triggered revalidation", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it("fires revalidate after stale time elapsed", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 100 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect(globalThis.fetch).toHaveBeenCalled();
	});

	it("reports window focus as the build skew revalidation reason", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const on_build_skew = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			onBuildSkewDetected: on_build_skew,
			revalidateOnWindowFocus: { staleTimeMs: 100 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(
			new Response("", {
				headers: {
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: "1",
				},
			}),
		);

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100 + REVALIDATION_DEBOUNCE_MS);
		await vi.advanceTimersByTimeAsync(0);

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "windowFocus",
				}),
			}),
		);
	});

	it("does not fire when stale time has not elapsed", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 5000 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(0);

		expect(globalThis.fetch).not.toHaveBeenCalled();
	});

	it("does not listen when disabled", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({ revalidateOnWindowFocus: false });

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(200);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect(globalThis.fetch).not.toHaveBeenCalled();
	});

	it("stale time resets after successful navigation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 1000 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		// Navigate resets last_activity_ts
		await core.navigate("/page");

		// Advance less than stale time after navigation
		await vi.advanceTimersByTimeAsync(500);
		const fetch_count_before = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		// No additional fetch for revalidation (only the navigate fetch)
		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count_before);
	});

	it("does not fire during active navigation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 0 },
		});

		let resolve_nav!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_nav = r;
			});
		});

		void core.navigate("/in-flight");
		await vi.advanceTimersByTimeAsync(200);

		const fetch_count = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count);

		resolve_nav(route_response());
		await vi.advanceTimersByTimeAsync(100);
	});

	it("does not fire during active submission", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 0 },
		});

		let resolve_submit!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_submit = r;
			});
		});

		void core.submit_inner(
			"/api/some-resource",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.advanceTimersByTimeAsync(200);

		const fetch_count = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count);

		resolve_submit(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.advanceTimersByTimeAsync(100);
	});

	it("does not fire during active revalidation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 0 },
		});

		let resolve_rev!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_rev = r;
			});
		});

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(200);

		const fetch_count = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count);

		resolve_rev(route_response());
		await vi.advanceTimersByTimeAsync(100);
	});

	it("does not advance stale-time timestamp for aborted navigations", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 100 },
		});

		vi.spyOn(globalThis, "fetch")
			.mockRejectedValueOnce(new DOMException("Aborted", "AbortError"))
			.mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(90);
		await core.navigate("/aborted");

		await vi.advanceTimersByTimeAsync(20);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(2);
	});

	it("does not reset stale-time for hash-only navigations", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMs: 100 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(90);
		await core.navigate("/#section");
		await vi.advanceTimersByTimeAsync(0);

		await vi.advanceTimersByTimeAsync(20);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(1);
	});
});

/////////////////////////////////////////////////////////////////////
/////// HMR
/////////////////////////////////////////////////////////////////////
