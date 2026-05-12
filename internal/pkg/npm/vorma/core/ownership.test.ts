// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	deferred,
	json_response,
	last_route_render_commit,
	redirect_response,
	register_ccc_lifecycle,
	route_render_commit_count,
	route_response,
	setup,
	tick,
} from "./___ccc_test_helpers.ts";
import {
	BUILD_ID_HEADER,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";
import { REVALIDATION_DEBOUNCE_MS } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

type UncooperativeFetchCall = {
	resolve: (response: Response) => void;
	signal: AbortSignal;
	url: string;
};

function install_uncooperative_fetch(): {
	call: (index: number) => UncooperativeFetchCall;
	calls: UncooperativeFetchCall[];
	wait_for: (count: number) => Promise<void>;
} {
	const calls: UncooperativeFetchCall[] = [];

	function call(index: number): UncooperativeFetchCall {
		const entry = calls[index];
		if (!entry) {
			throw new Error(
				`Expected fetch call at index ${index}, but only ${calls.length} calls have been made`,
			);
		}
		return entry;
	}

	async function wait_for(count: number): Promise<void> {
		for (let i = 0; i < 50; i++) {
			if (calls.length >= count) {
				return;
			}
			await Promise.resolve();
		}
		throw new Error(`Expected ${count} fetch calls, got ${calls.length}`);
	}

	vi.spyOn(globalThis, "fetch").mockImplementation(
		(input: string | URL | Request, init?: RequestInit) => {
			const signal = init?.signal;
			if (!signal) {
				throw new Error("Expected fetch to receive a signal");
			}

			const pending_response = deferred<Response>();
			const url =
				input instanceof URL
					? input.href
					: typeof input === "string"
						? input
						: input.url;
			calls.push({
				resolve: pending_response.resolve,
				signal,
				url,
			});
			return pending_response.promise;
		},
	);

	return { call, calls, wait_for };
}

describe("misc ownership tests", () => {
	it("settles a replaced submit without waiting for an uncooperative aborted fetch", async () => {
		const { core } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(1);

		const second = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(2);

		let first_resolved = false;
		let first_result: unknown;
		void first.then((result) => {
			first_resolved = true;
			first_result = result;
		});
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(first_resolved).toBe(true);
		expect(first_result).toMatchObject({
			error: "Aborted",
			success: false,
		});

		fetch.call(1).resolve(json_response({ winner: true }));
		await expect(second).resolves.toMatchObject({
			data: { winner: true },
			success: true,
		});
	});

	it("does not let a late response make a replaced submit succeed", async () => {
		const { core } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(1);

		const second = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(2);

		fetch.call(1).resolve(json_response({ winner: true }));
		await expect(second).resolves.toMatchObject({
			data: { winner: true },
			success: true,
		});

		fetch.call(0).resolve(json_response({ stale: true }));
		await expect(first).resolves.toMatchObject({
			error: "Aborted",
			success: false,
		});
	});

	it("preserves revalidation requested by a replaced mutation", async () => {
		const { core } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
			},
		);
		await fetch.wait_for(1);

		const second = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(2);

		let first_resolved = false;
		let first_result:
			| Awaited<ReturnType<typeof core.submit_inner>>
			| undefined;
		void first.then((result) => {
			first_resolved = true;
			first_result = result;
		});
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(first_resolved).toBe(true);
		expect(first_result).toMatchObject({
			error: "Aborted",
			success: false,
		});

		fetch.call(1).resolve(json_response({ winner: true }));
		await expect(second).resolves.toMatchObject({
			data: { winner: true },
			success: true,
		});

		await fetch.wait_for(3);
		fetch.call(2).resolve(route_response());
		await expect(first_result!.revalidationPromise).resolves.toEqual({
			ok: true,
		});
	});

	it("resolves a submit revalidation that finishes before the caller observes it", async () => {
		const { core } = await setup();
		const fetch = install_uncooperative_fetch();

		const submit = core.submit_inner("/api/action", { method: "POST" });
		await fetch.wait_for(1);

		fetch.call(0).resolve(json_response({ saved: true }));
		await fetch.wait_for(2);
		fetch.call(1).resolve(route_response());
		await tick();

		const result = await submit;

		expect(result).toMatchObject({
			data: { saved: true },
			success: true,
		});
		await expect(result.revalidationPromise).resolves.toEqual({
			ok: true,
		});
	});

	it("does not report build skew from a replaced submit response", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			clientOptions: {
				onBuildSkewDetected: on_build_skew,
			},
		});
		const fetch = install_uncooperative_fetch();

		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(1);

		const second = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(2);

		fetch.call(1).resolve(json_response({ winner: true }));
		await expect(second).resolves.toMatchObject({
			data: { winner: true },
			success: true,
		});

		fetch.call(0).resolve(
			new Response(JSON.stringify({ stale: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: "1",
				},
			}),
		);
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(on_build_skew).not.toHaveBeenCalled();
	});

	it("ignores a late redirect from a replaced submit fetch", async () => {
		const { core, hard_redirect } = await setup();
		const fetch = install_uncooperative_fetch();

		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(1);

		const second = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "save",
				revalidate: false,
			},
		);
		await fetch.wait_for(2);

		fetch.call(1).resolve(json_response({ winner: true }));
		await expect(second).resolves.toMatchObject({
			data: { winner: true },
			success: true,
		});

		fetch.call(0).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "https://example.com/stale-submit",
			}),
		);
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(hard_redirect).not.toHaveBeenCalled();
	});

	it("does not reuse a stopped prefetch after its fetch resolves late", async () => {
		const { core, commit } = await setup();
		const fetch = install_uncooperative_fetch();

		core.start_prefetch("/products");
		await fetch.wait_for(1);
		core.stop_prefetch("/products");

		fetch.call(0).resolve(
			route_response({
				MatchedPatterns: ["/products"],
				LoadersData: [{ stale_prefetch: true }],
			}),
		);
		await tick();

		const nav = core.navigate("/products");
		await fetch.wait_for(2);
		fetch.call(1).resolve(
			route_response({
				MatchedPatterns: ["/products"],
				LoadersData: [{ fresh_navigation: true }],
			}),
		);
		await expect(nav).resolves.toEqual({ didNavigate: true });

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(route_render_commit_count(commit)).toBe(1);
		expect(last_route_render_commit(commit).entries[0].loader_data).toEqual(
			{ fresh_navigation: true },
		);
	});

	it("does not follow a stale revalidation redirect after navigation takes ownership", async () => {
		vi.useFakeTimers();

		try {
			const { core, commit } = await setup();
			const fetch = install_uncooperative_fetch();

			void core.revalidate();
			await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
			await fetch.wait_for(1);

			const nav = core.navigate("/winner");
			await fetch.wait_for(2);
			fetch.call(1).resolve(
				route_response({
					MatchedPatterns: ["/winner"],
					LoadersData: [{ winner: true }],
				}),
			);
			await expect(nav).resolves.toEqual({ didNavigate: true });
			commit.mockClear();

			fetch.call(0).resolve(
				redirect_response({
					[X_CLIENT_REDIRECT]: "/stale-redirect",
				}),
			);
			await vi.advanceTimersByTimeAsync(0);
			await tick();

			expect(fetch.call(0).signal.aborted).toBe(true);
			expect(fetch.calls).toHaveLength(2);
			expect(route_render_commit_count(commit)).toBe(0);
			expect(window.location.pathname).toBe("/winner");
			expect(core.getWorkState().navigation).toBeNull();
			expect(core.getWorkState().revalidation).toBeNull();
		} finally {
			vi.useRealTimers();
		}
	});

	it("does not report build skew from a stopped prefetch response", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			clientOptions: {
				onBuildSkewDetected: on_build_skew,
			},
		});
		const fetch = install_uncooperative_fetch();

		core.start_prefetch("/products");
		await fetch.wait_for(1);
		core.stop_prefetch("/products");

		fetch.call(0).resolve(route_response({}, "build-2"));
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(on_build_skew).not.toHaveBeenCalled();
		expect(core.getWorkState().prefetch).toBeNull();
	});

	it("does not report build skew from a superseded navigation response", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			clientOptions: {
				onBuildSkewDetected: on_build_skew,
			},
		});
		const fetch = install_uncooperative_fetch();

		const first = core.navigate("/stale");
		await fetch.wait_for(1);

		const second = core.navigate("/winner");
		await fetch.wait_for(2);
		fetch.call(1).resolve(
			route_response({
				MatchedPatterns: ["/winner"],
				LoadersData: [{ winner: true }],
			}),
		);
		await expect(second).resolves.toEqual({ didNavigate: true });

		fetch.call(0).resolve(route_response({}, "build-2"));
		await expect(first).resolves.toEqual({ didNavigate: false });
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(on_build_skew).not.toHaveBeenCalled();
	});

	it("does not hard redirect from a superseded navigation response", async () => {
		const { core, hard_redirect } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.navigate("/stale");
		await fetch.wait_for(1);

		const second = core.navigate("/winner");
		await fetch.wait_for(2);
		fetch.call(1).resolve(route_response());
		await expect(second).resolves.toEqual({ didNavigate: true });

		fetch.call(0).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "https://example.com/stale-hard",
			}),
		);
		await expect(first).resolves.toEqual({ didNavigate: false });
		await tick();

		expect(fetch.call(0).signal.aborted).toBe(true);
		expect(hard_redirect).not.toHaveBeenCalled();
	});

	it("does not report build skew from a stale revalidation response", async () => {
		vi.useFakeTimers();

		try {
			const on_build_skew = vi.fn();
			const { core } = await setup({
				clientOptions: {
					onBuildSkewDetected: on_build_skew,
				},
			});
			const fetch = install_uncooperative_fetch();

			void core.revalidate();
			await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
			await fetch.wait_for(1);

			const nav = core.navigate("/winner");
			await fetch.wait_for(2);
			fetch.call(1).resolve(
				route_response({
					MatchedPatterns: ["/winner"],
					LoadersData: [{ winner: true }],
				}),
			);
			await expect(nav).resolves.toEqual({ didNavigate: true });

			fetch.call(0).resolve(route_response({}, "build-2"));
			await vi.advanceTimersByTimeAsync(0);
			await tick();

			expect(fetch.call(0).signal.aborted).toBe(true);
			expect(on_build_skew).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
	});

	it("does not hard redirect from a stale revalidation response", async () => {
		vi.useFakeTimers();

		try {
			const { core, hard_redirect } = await setup();
			const fetch = install_uncooperative_fetch();

			void core.revalidate();
			await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
			await fetch.wait_for(1);

			const nav = core.navigate("/winner");
			await fetch.wait_for(2);
			fetch.call(1).resolve(route_response());
			await expect(nav).resolves.toEqual({ didNavigate: true });

			fetch.call(0).resolve(
				redirect_response({
					[X_CLIENT_REDIRECT]: "https://example.com/stale-hard",
				}),
			);
			await vi.advanceTimersByTimeAsync(0);
			await tick();

			expect(fetch.call(0).signal.aborted).toBe(true);
			expect(hard_redirect).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
	});

	it("settles a revalidation when a navigation satisfies freshness first", async () => {
		vi.useFakeTimers();

		try {
			const { core } = await setup();
			const fetch = install_uncooperative_fetch();

			const revalidation = core.revalidate();
			await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
			await fetch.wait_for(1);

			const nav = core.navigate("/fresh");
			await fetch.wait_for(2);
			fetch.call(1).resolve(
				route_response({
					MatchedPatterns: ["/fresh"],
					LoadersData: [{ fresh: true }],
				}),
			);

			await expect(nav).resolves.toEqual({ didNavigate: true });
			await expect(revalidation).resolves.toEqual({ ok: true });
			expect(fetch.call(0).signal.aborted).toBe(true);
			expect(core.getWorkState().revalidation).toBeNull();
		} finally {
			vi.useRealTimers();
		}
	});

	it("does not publish a stale redirect-chain target after supersession", async () => {
		const { core, commit } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.navigate("/start");
		await fetch.wait_for(1);
		fetch
			.call(0)
			.resolve(
				redirect_response({ [X_CLIENT_REDIRECT]: "/redirect-target" }),
			);
		await fetch.wait_for(2);

		const second = core.navigate("/winner");
		await fetch.wait_for(3);
		fetch.call(2).resolve(
			route_response({
				MatchedPatterns: ["/winner"],
				LoadersData: [{ winner: true }],
			}),
		);
		await expect(second).resolves.toEqual({ didNavigate: true });

		fetch.call(1).resolve(
			route_response({
				MatchedPatterns: ["/redirect-target"],
				LoadersData: [{ stale_redirect: true }],
			}),
		);
		await expect(first).resolves.toEqual({ didNavigate: false });
		await tick();

		expect(fetch.call(1).signal.aborted).toBe(true);
		expect(route_render_commit_count(commit)).toBe(1);
		expect(last_route_render_commit(commit).entries[0].loader_data).toEqual(
			{ winner: true },
		);
		expect(window.location.pathname).toBe("/winner");
	});

	it("does not report build skew from a stale redirect-chain target", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			clientOptions: {
				onBuildSkewDetected: on_build_skew,
			},
		});
		const fetch = install_uncooperative_fetch();

		const first = core.navigate("/start");
		await fetch.wait_for(1);
		fetch
			.call(0)
			.resolve(
				redirect_response({ [X_CLIENT_REDIRECT]: "/redirect-target" }),
			);
		await fetch.wait_for(2);

		const second = core.navigate("/winner");
		await fetch.wait_for(3);
		fetch.call(2).resolve(route_response());
		await expect(second).resolves.toEqual({ didNavigate: true });

		fetch.call(1).resolve(route_response({}, "build-2"));
		await expect(first).resolves.toEqual({ didNavigate: false });
		await tick();

		expect(fetch.call(1).signal.aborted).toBe(true);
		expect(on_build_skew).not.toHaveBeenCalled();
	});

	it("does not hard redirect from a stale redirect-chain target", async () => {
		const { core, hard_redirect } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.navigate("/start");
		await fetch.wait_for(1);
		fetch
			.call(0)
			.resolve(
				redirect_response({ [X_CLIENT_REDIRECT]: "/redirect-target" }),
			);
		await fetch.wait_for(2);

		const second = core.navigate("/winner");
		await fetch.wait_for(3);
		fetch.call(2).resolve(route_response());
		await expect(second).resolves.toEqual({ didNavigate: true });

		fetch.call(1).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "https://example.com/stale-hard",
			}),
		);
		await expect(first).resolves.toEqual({ didNavigate: false });
		await tick();

		expect(fetch.call(1).signal.aborted).toBe(true);
		expect(hard_redirect).not.toHaveBeenCalled();
	});

	it("does not publish a superseded navigation after client loader resolves late", async () => {
		const loader_gate = deferred<unknown>();

		vi.doMock("/late-loader.js", () => {
			return {
				default: {
					pattern: "/late-loader",
					component: () => {
						return null;
					},
					client_loader: async () => {
						await loader_gate.promise;
						return { stale_client_loader: true };
					},
				},
			};
		});

		const { core, commit } = await setup();
		const fetch = install_uncooperative_fetch();

		const first = core.navigate("/late-loader");
		await fetch.wait_for(1);
		fetch.call(0).resolve(
			route_response({
				MatchedPatterns: ["/late-loader"],
				LoadersData: [{ stale_server: true }],
				ImportURLs: ["/late-loader.js"],
			}),
		);
		await tick();

		const second = core.navigate("/winner");
		await fetch.wait_for(2);
		fetch.call(1).resolve(
			route_response({
				MatchedPatterns: ["/winner"],
				LoadersData: [{ winner: true }],
			}),
		);
		await expect(second).resolves.toEqual({ didNavigate: true });

		loader_gate.resolve(null);
		await expect(first).resolves.toEqual({ didNavigate: false });
		await tick();

		expect(route_render_commit_count(commit)).toBe(1);
		expect(last_route_render_commit(commit).entries[0].loader_data).toEqual(
			{ winner: true },
		);
	});
});
