import { describe, expect, it } from "vitest";
import { create_core6_deferred } from "./deferred.ts";
import {
	core6_route_fetch_failure_reason,
	core6_route_fetch_result_kind,
	type Core6RouteFetchResponseMeta,
} from "./route_fetch.ts";
import {
	core6_route_fetch_driver_result_kind,
	type Core6RouteFetchDriverResult,
} from "./route_fetch_driver.ts";
import type { Core6RouteState } from "./route_publication.ts";
import { core6_route_publish_reason } from "./route_publication.ts";
import {
	CORE6_MAX_REVALIDATION_RETRIES,
	CORE6_REVALIDATION_BACKOFF_BASE_MS,
	CORE6_REVALIDATION_BACKOFF_CAP_MS,
	CORE6_REVALIDATION_DEBOUNCE_MS,
	core6_route_revalidation_failure_reason,
	core6_route_revalidation_status,
	create_core6_route_revalidation_owner,
	type Core6RouteRevalidationRuntime,
	type Core6RouteRevalidationTimer,
} from "./route_revalidation.ts";
import type { Core6RouteRuntimeRunFetchInput } from "./route_runtime.ts";
import type { Core6ScopeCommitResult } from "./scope.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import {
	core6_route_transaction_kind,
	type Core6RouteTransactionKind,
} from "./transaction.ts";

type QueuedRouteFetchResult = Promise<
	Core6ScopeCommitResult<Core6RouteFetchDriverResult>
>;

type FakeTimer = {
	active: boolean;
	fn: () => void;
	id: number;
	ms: number;
};

async function tick(): Promise<void> {
	await Promise.resolve();
	await Promise.resolve();
}

function make_route(overrides: Partial<Core6RouteState> = {}): Core6RouteState {
	return {
		clientBuildID: "build-1",
		error: null,
		historyState: { from: "revalidation" },
		href: "https://example.test/current?q=Ada",
		matches: [
			{
				clientLoaderData: undefined,
				input: { route: "current" },
				loaderData: { ok: true },
				pattern: "/current",
			},
		],
		params: {},
		splatValues: [],
		...overrides,
	};
}

function make_response_meta(
	overrides: Partial<Core6RouteFetchResponseMeta> = {},
): Core6RouteFetchResponseMeta {
	return {
		final_href: "https://example.test/current?q=Ada",
		ok: true,
		requested_href: "https://example.test/current?q=Ada",
		server_build_id: "build-1",
		status: 200,
		status_text: "",
		...overrides,
	};
}

function published_result(
	route: Core6RouteState = make_route(),
): Core6ScopeCommitResult<Core6RouteFetchDriverResult> {
	return {
		ok: true,
		value: {
			kind: core6_route_fetch_driver_result_kind.published,
			publication: {
				client_loaders: [],
				commit: {
					route_render: {
						state: {
							client_build_id: route.clientBuildID,
							entries: [],
							error: route.error,
							history_state: route.historyState,
							params: route.params,
							splat_values: route.splatValues,
						},
					},
					route_update: {
						previous_route: null,
						reason: core6_route_publish_reason.revalidation,
						route,
					},
				},
				route,
				side_effects: null,
			},
			response: make_response_meta({
				final_href: route.href,
				requested_href: route.href,
			}),
		},
	};
}

function failed_result(): Core6ScopeCommitResult<Core6RouteFetchDriverResult> {
	return {
		ok: true,
		value: {
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.http_error,
			response: make_response_meta({
				ok: false,
				status: 503,
				status_text: "Service Unavailable",
			}),
		},
	};
}

function build_skew_result(): Core6ScopeCommitResult<Core6RouteFetchDriverResult> {
	return {
		ok: true,
		value: {
			kind: core6_route_fetch_result_kind.build_skew,
			redirect: null,
			response: make_response_meta({
				server_build_id: "build-2",
			}),
		},
	};
}

function create_timer_host(): {
	active_timers: () => FakeTimer[];
	clear_timer: (timer: Core6RouteRevalidationTimer) => void;
	fire_next: () => void;
	set_timer: (fn: () => void, ms: number) => Core6RouteRevalidationTimer;
	timers: FakeTimer[];
} {
	const timers: FakeTimer[] = [];
	let next_id = 0;
	return {
		active_timers: () => {
			return timers.filter((timer) => {
				return timer.active;
			});
		},
		clear_timer: (timer) => {
			(timer as FakeTimer).active = false;
		},
		fire_next: () => {
			const timer = timers.find((candidate) => {
				return candidate.active;
			});
			if (!timer) {
				throw new Error("expected active timer");
			}
			timer.active = false;
			timer.fn();
		},
		set_timer: (fn, ms) => {
			const timer: FakeTimer = {
				active: true,
				fn,
				id: next_id++,
				ms,
			};
			timers.push(timer);
			return timer;
		},
		timers,
	};
}

function create_runtime_fixture(
	input: {
		current_route?: Core6RouteState | null;
		current_transaction_kind?: Core6RouteTransactionKind | null;
	} = {},
): {
	enqueue_result: (
		result: Core6ScopeCommitResult<Core6RouteFetchDriverResult>,
	) => void;
	enqueue_wait: (result: QueuedRouteFetchResult) => void;
	fetches: Core6RouteRuntimeRunFetchInput[];
	runtime: Core6RouteRevalidationRuntime;
	set_current_route: (route: Core6RouteState | null) => void;
	set_current_transaction_kind: (
		kind: Core6RouteTransactionKind | null,
	) => void;
} {
	let current_route: Core6RouteState | null =
		input.current_route ?? make_route();
	let current_transaction_kind = input.current_transaction_kind ?? null;
	const fetches: Core6RouteRuntimeRunFetchInput[] = [];
	const queue: QueuedRouteFetchResult[] = [];
	return {
		enqueue_result: (result) => {
			queue.push(Promise.resolve(result));
		},
		enqueue_wait: (result) => {
			queue.push(result);
		},
		fetches,
		runtime: {
			current_route: () => {
				return current_route;
			},
			current_transaction_kind: () => {
				return current_transaction_kind;
			},
			publish_same_document: () => {
				throw new Error(
					"revalidation should not publish same-document routes",
				);
			},
			run_route_fetch: async (run_input) => {
				fetches.push(run_input);
				const result = queue.shift();
				if (!result) {
					throw new Error("missing queued route fetch result");
				}
				return await result;
			},
			run_route_prepared: () => {
				throw new Error("revalidation should not promote prefetch");
			},
		},
		set_current_route: (route) => {
			current_route = route;
		},
		set_current_transaction_kind: (kind) => {
			current_transaction_kind = kind;
		},
	};
}

describe("core6 route revalidation owner", () => {
	it("debounces coalesced freshness demand and resolves all waiters", async () => {
		const timers = create_timer_host();
		const runtime = create_runtime_fixture();
		runtime.enqueue_result(published_result());
		const owner = create_core6_route_revalidation_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			host: {
				...timers,
				hard_redirect: () => {},
			},
			runtime: runtime.runtime,
		});

		const first = owner.request();
		const second = owner.request();
		const third = owner.request();

		expect(owner.current_status()).toMatchObject({
			status: core6_route_revalidation_status.debouncing,
			waiter_count: 3,
		});
		expect(timers.active_timers()).toHaveLength(1);
		expect(timers.active_timers()[0]?.ms).toBe(
			CORE6_REVALIDATION_DEBOUNCE_MS,
		);

		timers.fire_next();
		await tick();

		await expect(Promise.all([first, second, third])).resolves.toEqual([
			{ ok: true },
			{ ok: true },
			{ ok: true },
		]);
		expect(runtime.fetches).toHaveLength(1);
		expect(runtime.fetches[0]).toMatchObject({
			active_client_build_id: "build-1",
			kind: core6_route_transaction_kind.revalidation,
			intent: {
				href: "https://example.test/current?q=Ada",
			},
		});
		expect(runtime.fetches[0]?.intent.search_params.get("q")).toBe("Ada");
		expect(owner.current_status()).toBeNull();
	});

	it("keeps pending demand while route runtime is busy", async () => {
		const timers = create_timer_host();
		const runtime = create_runtime_fixture({
			current_transaction_kind: core6_route_transaction_kind.navigation,
		});
		const owner = create_core6_route_revalidation_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			host: {
				...timers,
				hard_redirect: () => {},
			},
			runtime: runtime.runtime,
		});

		const result = owner.request({ debounce: false });

		expect(owner.current_status()).toMatchObject({
			status: core6_route_revalidation_status.pending,
		});
		expect(runtime.fetches).toEqual([]);
		expect(owner.start_pending()).toBe(false);

		runtime.enqueue_result(published_result());
		runtime.set_current_transaction_kind(null);
		expect(owner.start_pending()).toBe(true);

		await expect(result).resolves.toEqual({ ok: true });
		expect(runtime.fetches).toHaveLength(1);
	});

	it("does not satisfy newer demand with older in-flight data", async () => {
		const timers = create_timer_host();
		const runtime = create_runtime_fixture();
		const first_result =
			create_core6_deferred<
				Core6ScopeCommitResult<Core6RouteFetchDriverResult>
			>();
		runtime.enqueue_wait(first_result.promise);
		const owner = create_core6_route_revalidation_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			host: {
				...timers,
				hard_redirect: () => {},
			},
			runtime: runtime.runtime,
		});

		const first = owner.request({ debounce: false });
		const second = owner.request();
		let settled = false;
		void Promise.all([first, second]).then(() => {
			settled = true;
		});

		first_result.resolve(published_result());
		await tick();

		expect(settled).toBe(false);
		expect(owner.current_status()).toMatchObject({
			status: core6_route_revalidation_status.debouncing,
			waiter_count: 2,
		});

		runtime.enqueue_result(published_result());
		timers.fire_next();

		await expect(Promise.all([first, second])).resolves.toEqual([
			{ ok: true },
			{ ok: true },
		]);
		expect(runtime.fetches).toHaveLength(2);
		expect(owner.current_status()).toBeNull();
	});

	it("retries failed runs with capped backoff and then exhausts waiters", async () => {
		const timers = create_timer_host();
		const runtime = create_runtime_fixture();
		const owner = create_core6_route_revalidation_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			backoff_base_ms: CORE6_REVALIDATION_BACKOFF_BASE_MS,
			backoff_cap_ms: CORE6_REVALIDATION_BACKOFF_CAP_MS,
			host: {
				...timers,
				hard_redirect: () => {},
			},
			max_retries: 3,
			runtime: runtime.runtime,
		});

		runtime.enqueue_result(failed_result());
		const result = owner.request({ debounce: false });
		await tick();

		expect(owner.current_status()).toMatchObject({
			attempt: 1,
			status: core6_route_revalidation_status.retrying,
		});
		expect(timers.active_timers()[0]?.ms).toBe(
			CORE6_REVALIDATION_BACKOFF_BASE_MS,
		);

		runtime.enqueue_result(failed_result());
		timers.fire_next();
		await tick();

		expect(owner.current_status()).toMatchObject({
			attempt: 2,
			status: core6_route_revalidation_status.retrying,
		});
		expect(timers.active_timers()[0]?.ms).toBe(
			CORE6_REVALIDATION_BACKOFF_BASE_MS * 2,
		);

		runtime.enqueue_result(failed_result());
		timers.fire_next();
		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_route_revalidation_failure_reason.max_retries_exhausted,
		});
		expect(runtime.fetches).toHaveLength(3);
		expect(owner.current_status()).toBeNull();
		expect(CORE6_MAX_REVALIDATION_RETRIES).toBe(8);
	});

	it("settles build skew without scheduling a retry", async () => {
		const timers = create_timer_host();
		const runtime = create_runtime_fixture();
		runtime.enqueue_result(build_skew_result());
		const owner = create_core6_route_revalidation_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			host: {
				...timers,
				hard_redirect: () => {},
			},
			runtime: runtime.runtime,
		});

		const result = owner.request({ debounce: false });

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_route_revalidation_failure_reason.build_skew,
		});
		expect(timers.active_timers()).toEqual([]);
		expect(owner.current_status()).toBeNull();
	});

	it("lets an external fresh route settle demand and ignores late run results", async () => {
		const timers = create_timer_host();
		const runtime = create_runtime_fixture();
		const late_result =
			create_core6_deferred<
				Core6ScopeCommitResult<Core6RouteFetchDriverResult>
			>();
		runtime.enqueue_wait(late_result.promise);
		const owner = create_core6_route_revalidation_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			host: {
				...timers,
				hard_redirect: () => {},
			},
			runtime: runtime.runtime,
		});

		const result = owner.request({ debounce: false });
		expect(owner.current_status()).toMatchObject({
			status: core6_route_revalidation_status.running,
		});

		expect(owner.mark_fresh()).toBe(true);
		await expect(result).resolves.toEqual({ ok: true });
		expect(owner.current_status()).toBeNull();

		late_result.resolve({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		await tick();

		expect(owner.current_status()).toBeNull();
	});
});
