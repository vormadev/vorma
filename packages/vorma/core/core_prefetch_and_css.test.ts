// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	has_route_render_commit,
	mock_fetch,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_render_commit_count,
	route_response,
	seed_payload,
	setup,
	t_opts,
	tick,
	wait_until,
} from "./_test_helpers.ts";
import { create_client_core } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

describe("prefetch integration", () => {
	it("start_prefetch forwards to router with correct URL", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise(() => {});
		});

		core.start_prefetch("/prefetch-target");
		await tick();

		expect(globalThis.fetch).toHaveBeenCalled();
		const fetched_url = (globalThis.fetch as any).mock.calls[0][0];
		expect(fetched_url.toString()).toContain("/prefetch-target");
	});

	it("reports build skew from a successful prefetch response", async () => {
		seed_payload({ client_build_id: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		const on_build_skew = vi.fn();
		await core.boot({ onBuildSkewDetected: on_build_skew });

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-2"),
		);

		core.start_prefetch("/prefetch-target");
		await tick(10);

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					ok: true,
					requestedHref: `${window.location.origin}/prefetch-target`,
					status: 200,
					trigger: "prefetch",
				}),
			}),
		);
	});

	it("stop_prefetch cancels in-flight prefetch", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		let signal: AbortSignal | undefined;
		vi.spyOn(globalThis, "fetch").mockImplementation((_url: any, init?: any) => {
			signal = init?.signal;
			return new Promise(() => {});
		});

		core.start_prefetch("/prefetch-target");
		await tick();

		expect(signal).toBeDefined();
		expect(signal!.aborted).toBe(false);

		core.stop_prefetch("/prefetch-target");

		expect(signal!.aborted).toBe(true);
	});

	it("resolves prestarted client loader server data during prefetch", async () => {
		let client_loader_call_count = 0;
		let seen_server_data: unknown;

		vi.doMock("/known-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/known-prefetch",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						client_loader_call_count++;
						seen_server_data = await serverPromise;
						return { client: true };
					},
				},
			};
		});

		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const first_nav = core.navigate("/known-prefetch");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/known-prefetch"],
				views_data: [{ initial: true }],
				import_urls: ["/known-prefetch-module.js"],
			}),
		);
		await first_nav;

		const away_nav = core.navigate("/other");
		await wait_for(2);
		call(1).resolve(route_response());
		await away_nav;

		client_loader_call_count = 0;
		seen_server_data = undefined;

		core.start_prefetch("/known-prefetch");
		await wait_for(3);

		/*
		Prestart defers on async matcher readiness (WASM init), so the call
		count is awaited rather than asserted synchronously. The server
		response is still unresolved here, which is what proves the loader
		started BEFORE its server data existed.
		*/
		await wait_until(() => {
			return client_loader_call_count === 1;
		}, "client loader must prestart during prefetch");
		expect(client_loader_call_count).toBe(1);
		expect(seen_server_data).toBeUndefined();

		call(2).resolve(
			route_response({
				matched_patterns: ["/known-prefetch"],
				views_data: [{ prefetched: true }],
				import_urls: ["/known-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return seen_server_data !== undefined;
		}, "expected prefetch server data to resolve");

		expect(calls).toHaveLength(3);
		expect(seen_server_data).toEqual({
			clientBuildId: "build-1",
			matches: [
				{
					pattern: "/known-prefetch",
					input: {},
					viewData: { prefetched: true },
				},
			],
			outermostServerError: null,
			viewData: { prefetched: true },
		});
	});

	it("runs first-time route client loader during prefetch and reuses it on navigation", async () => {
		let client_loader_call_count = 0;
		let seen_server_data: unknown;

		vi.doMock("/first-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/first-prefetch",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						client_loader_call_count++;
						seen_server_data = await serverPromise;
						return { from_client: true };
					},
				},
			};
		});

		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/first-prefetch");
		await wait_for(1);

		call(0).resolve(
			route_response({
				matched_patterns: ["/first-prefetch"],
				views_data: [{ from_server: true }],
				import_urls: ["/first-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return seen_server_data !== undefined;
		}, "expected first-time prefetch client loader to run");

		expect(has_route_render_commit(commit)).toBe(false);
		expect(client_loader_call_count).toBe(1);
		expect(seen_server_data).toEqual({
			clientBuildId: "build-1",
			matches: [
				{
					pattern: "/first-prefetch",
					input: {},
					viewData: { from_server: true },
				},
			],
			outermostServerError: null,
			viewData: { from_server: true },
		});

		const result = await core.navigate("/first-prefetch");

		expect(result.didNavigate).toBe(true);
		expect(calls).toHaveLength(1);
		expect(client_loader_call_count).toBe(1);
		expect(route_render_commit_count(commit)).toBe(1);

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].client_loader_data).toEqual({
			from_client: true,
		});
	});

	it("clears prepared prefetch from work state while retaining it for navigation", async () => {
		vi.doMock("/work-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/work-prefetch",
					component: () => {
						return null;
					},
				},
			};
		});

		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/work-prefetch");
		await wait_for(1);

		expect(core.getWorkState().prefetch?.href).toContain("/work-prefetch");

		call(0).resolve(
			route_response({
				matched_patterns: ["/work-prefetch"],
				views_data: [{ from_server: true }],
				import_urls: ["/work-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return core.getWorkState().prefetch === null;
		}, "expected prepared prefetch to leave work state");

		const result = await core.navigate("/work-prefetch");

		expect(result.didNavigate).toBe(true);
		expect(calls).toHaveLength(1);
		expect(route_render_commit_count(commit)).toBe(1);
	});

	it("aborts first-time route client loader when prefetch is stopped", async () => {
		let captured_signal: AbortSignal | null = null;

		vi.doMock("/abort-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/abort-prefetch",
					component: () => {
						return null;
					},
					client_loader: async ({ signal }: any) => {
						captured_signal = signal;
						return new Promise(() => {});
					},
				},
			};
		});

		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		core.start_prefetch("/abort-prefetch");
		await wait_for(1);

		call(0).resolve(
			route_response({
				matched_patterns: ["/abort-prefetch"],
				views_data: [{ value: 1 }],
				import_urls: ["/abort-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return captured_signal !== null;
		}, "expected first-time prefetch client loader signal");

		expect(captured_signal!.aborted).toBe(false);

		core.stop_prefetch("/abort-prefetch");

		expect(captured_signal!.aborted).toBe(true);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loader cancellation
/////////////////////////////////////////////////////////////////////

describe("CSS preload gating", () => {
	it("blocks navigation commit until CSS preloads settle", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		const boot_commit_count = route_render_commit_count(commit);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ css_bundles: ["/blocking.css"] }),
		);

		const nav = core.navigate("/css-blocked");

		// Poll until the preload link appears
		let preload: Element | null = null;
		for (let i = 0; i < 200; i++) {
			preload = document.head.querySelector(
				'link[data-vorma-css-preload="/blocking.css"]',
			);
			if (preload) {
				break;
			}
			await Promise.resolve();
		}

		expect(preload).not.toBeNull();
		expect(route_render_commit_count(commit)).toBe(boot_commit_count);

		preload!.dispatchEvent(new Event("load"));
		await nav;

		expect(route_render_commit_count(commit)).toBe(boot_commit_count + 1);
	});

	it("unblocks navigation when CSS preload errors", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		const boot_commit_count = route_render_commit_count(commit);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ css_bundles: ["/error.css"] }),
		);

		const nav = core.navigate("/css-error");

		let preload: Element | null = null;
		for (let i = 0; i < 200; i++) {
			preload = document.head.querySelector(
				'link[data-vorma-css-preload="/error.css"]',
			);
			if (preload) {
				break;
			}
			await Promise.resolve();
		}

		expect(preload).not.toBeNull();
		preload!.dispatchEvent(new Event("error"));
		await nav;

		expect(route_render_commit_count(commit)).toBe(boot_commit_count + 1);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loader promise reuse on hash change
/////////////////////////////////////////////////////////////////////
