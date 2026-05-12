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
import { core6_route_navigation_result_kind } from "./route_navigation.ts";
import {
	core6_route_popstate_result_kind,
	create_core6_route_popstate_owner,
	type Core6RoutePopstateHost,
	type Core6RoutePopstatePosition,
	type Core6RoutePopstateRuntime,
	type Core6RoutePopstateScroll,
} from "./route_popstate.ts";
import { core6_route_preparation_trigger } from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RouteCommit,
	type Core6RoutePublication,
	type Core6RouteState,
} from "./route_publication.ts";
import type { Core6RouteRuntimeRunFetchInput } from "./route_runtime.ts";
import type { Core6ScopeCommitResult } from "./scope.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import {
	core6_route_transaction_kind,
	type Core6RouteTransactionKind,
} from "./transaction.ts";

const current_href = "https://example.test/current";
const current_key = "current-key";
const target_href = "https://example.test/target?q=1";
const target_key = "target-key";

type QueuedRouteFetchResult = Promise<
	Core6ScopeCommitResult<Core6RouteFetchDriverResult>
>;

function make_position(
	overrides: Partial<Core6RoutePopstatePosition> = {},
): Core6RoutePopstatePosition {
	return {
		href: current_href,
		key: current_key,
		state: { from: "history" },
		...overrides,
	};
}

function make_route(overrides: Partial<Core6RouteState> = {}): Core6RouteState {
	return {
		clientBuildID: "build-1",
		error: null,
		historyState: { from: "route" },
		href: current_href,
		matches: [],
		params: {},
		splatValues: [],
		...overrides,
	};
}

function make_response_meta(
	overrides: Partial<Core6RouteFetchResponseMeta> = {},
): Core6RouteFetchResponseMeta {
	return {
		final_href: target_href,
		ok: true,
		requested_href: target_href,
		server_build_id: "build-1",
		status: 200,
		status_text: "",
		...overrides,
	};
}

function make_publication(input: {
	previous_route?: Core6RouteState | null;
	route: Core6RouteState;
	scroll?: Core6RoutePopstateScroll;
}): Core6RoutePublication {
	const previous_route = input.previous_route ?? null;
	const commit: Core6RouteCommit = {
		route_render: {
			scroll: input.scroll,
			state: {
				client_build_id: input.route.clientBuildID,
				entries: [],
				error: input.route.error,
				history_state: input.route.historyState,
				params: input.route.params,
				splat_values: input.route.splatValues,
			},
		},
		route_update: {
			previous_route,
			reason: core6_route_publish_reason.popstate,
			route: input.route,
		},
	};
	return {
		client_loaders: [],
		commit,
		route: input.route,
		side_effects: null,
	};
}

function published_result(
	route: Core6RouteState,
): Core6ScopeCommitResult<Core6RouteFetchDriverResult> {
	return {
		ok: true,
		value: {
			kind: core6_route_fetch_driver_result_kind.published,
			publication: make_publication({ route }),
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
				status: 500,
				status_text: "Err",
			}),
		},
	};
}

function create_fixture(
	input: {
		current_route?: Core6RouteState | null;
		current_transaction_kind?: Core6RouteTransactionKind | null;
	} = {},
): {
	cancel_count: () => number;
	enqueue_result: (
		result: Core6ScopeCommitResult<Core6RouteFetchDriverResult>,
	) => void;
	enqueue_wait: (result: QueuedRouteFetchResult) => void;
	fetches: Core6RouteRuntimeRunFetchInput[];
	host: Core6RoutePopstateHost;
	owner: ReturnType<typeof create_core6_route_popstate_owner>;
	reloads: string[];
	runtime: Core6RoutePopstateRuntime;
	same_document_publications: Core6RoutePublication[];
	saved_scroll_keys: string[];
	set_current_route: (route: Core6RouteState | null) => void;
	set_current_transaction_kind: (
		kind: Core6RouteTransactionKind | null,
	) => void;
} {
	let current_route: Core6RouteState | null =
		input.current_route ?? make_route();
	let current_transaction_kind = input.current_transaction_kind ?? null;
	let cancel_count = 0;
	const fetches: Core6RouteRuntimeRunFetchInput[] = [];
	const queue: QueuedRouteFetchResult[] = [];
	const reloads: string[] = [];
	const same_document_publications: Core6RoutePublication[] = [];
	const saved_scroll_keys: string[] = [];
	const host: Core6RoutePopstateHost = {
		hard_redirect: () => {},
		reload: (href) => {
			reloads.push(href);
		},
		save_scroll_for_key: (key) => {
			saved_scroll_keys.push(key);
		},
	};
	const runtime: Core6RoutePopstateRuntime = {
		cancel_current: () => {
			if (current_transaction_kind === null) {
				return false;
			}
			cancel_count++;
			current_transaction_kind = null;
			return true;
		},
		current_route: () => {
			return current_route;
		},
		current_transaction_kind: () => {
			return current_transaction_kind;
		},
		publish_same_document: (same_document_input) => {
			if (!current_route) {
				return null;
			}
			const previous_route = current_route;
			const route = {
				...previous_route,
				historyState: same_document_input.history_state,
				href: same_document_input.href,
				matches: previous_route.matches.map((match) => {
					return {
						clientLoaderData: match.clientLoaderData,
						input: match.input,
						loaderData: match.loaderData,
						pattern: match.pattern,
					};
				}),
				params: { ...previous_route.params },
				splatValues: [...previous_route.splatValues],
			};
			const publication = make_publication({
				previous_route,
				route,
				scroll: same_document_input.scroll,
			});
			same_document_publications.push(publication);
			current_route = route;
			return publication;
		},
		run_route_fetch: async (run_input) => {
			fetches.push(run_input);
			current_transaction_kind = run_input.kind;
			const result = queue.shift();
			if (!result) {
				throw new Error("missing queued route fetch result");
			}
			const settled = await result;
			if (
				settled.ok &&
				settled.value.kind ===
					core6_route_fetch_driver_result_kind.published
			) {
				current_route = settled.value.publication.route;
			}
			current_transaction_kind = null;
			return settled;
		},
		run_route_prepared: () => {
			return { ok: false, reason: core6_scope_stale_reason };
		},
	};
	const owner = create_core6_route_popstate_owner({
		active_client_build_id: () => {
			return "build-1";
		},
		host,
		initial_position: make_position(),
		runtime,
	});
	return {
		cancel_count: () => {
			return cancel_count;
		},
		enqueue_result: (result) => {
			queue.push(Promise.resolve(result));
		},
		enqueue_wait: (result) => {
			queue.push(result);
		},
		fetches,
		host,
		owner,
		reloads,
		runtime,
		same_document_publications,
		saved_scroll_keys,
		set_current_route: (route) => {
			current_route = route;
		},
		set_current_transaction_kind: (kind) => {
			current_transaction_kind = kind;
		},
	};
}

describe("core6 route popstate owner", () => {
	it("ignores identical browser positions without touching route work", async () => {
		const fixture = create_fixture();

		const result = await fixture.owner.handle({
			position: make_position(),
		});

		expect(result).toEqual({
			kind: core6_route_popstate_result_kind.ignored,
			position: make_position(),
		});
		expect(fixture.fetches).toEqual([]);
		expect(fixture.saved_scroll_keys).toEqual([]);
		expect(fixture.same_document_publications).toEqual([]);
		expect(fixture.owner.current_status()).toBeNull();
	});

	it("commits same-document popstate synchronously and cancels stale navigation work", async () => {
		const fixture = create_fixture({
			current_transaction_kind: core6_route_transaction_kind.navigation,
		});
		const position = make_position({
			href: `${current_href}#section`,
		});

		const result = await fixture.owner.handle({ position });

		expect(result).toMatchObject({
			kind: core6_route_popstate_result_kind.same_document,
			position,
			scroll: { hash: "#section" },
		});
		expect(fixture.cancel_count()).toBe(1);
		expect(fixture.fetches).toEqual([]);
		expect(fixture.same_document_publications).toMatchObject([
			{
				commit: {
					route_render: {
						scroll: { hash: "#section" },
					},
					route_update: {
						reason: core6_route_publish_reason.popstate,
					},
				},
				route: {
					historyState: position.state,
					href: `${current_href}#section`,
				},
			},
		]);
		expect(fixture.runtime.current_route()).toMatchObject({
			historyState: position.state,
			href: `${current_href}#section`,
		});
		expect(fixture.owner.current_position()).toEqual(position);
	});

	it("does not cancel revalidation work for same-document popstate", async () => {
		const fixture = create_fixture({
			current_transaction_kind: core6_route_transaction_kind.revalidation,
		});

		const result = await fixture.owner.handle({
			position: make_position({
				href: `${current_href}#section`,
			}),
		});

		expect(result).toMatchObject({
			kind: core6_route_popstate_result_kind.same_document,
		});
		expect(fixture.cancel_count()).toBe(0);
		expect(fixture.runtime.current_transaction_kind()).toBe(
			core6_route_transaction_kind.revalidation,
		);
	});

	it("uses restored scroll for same-document popstate without a hash", async () => {
		const fixture = create_fixture({
			current_route: make_route({
				href: `${current_href}#old`,
			}),
		});
		const position = make_position({
			href: current_href,
			key: "shared-key",
		});

		await fixture.owner.handle({
			position,
			restored_scroll: { x: 10, y: 20 },
		});

		expect(fixture.saved_scroll_keys).toEqual([current_key]);
		expect(
			fixture.same_document_publications[0]?.commit.route_render.scroll,
		).toEqual({ x: 10, y: 20 });
	});

	it("routes cross-document popstate through a popstate transaction", async () => {
		const fixture = create_fixture();
		const target_route = make_route({
			historyState: { popped: true },
			href: target_href,
		});
		fixture.enqueue_result(published_result(target_route));
		const position = make_position({
			href: target_href,
			key: target_key,
			state: { popped: true },
		});

		const routed = fixture.owner.handle({
			position,
			restored_scroll: { x: 7, y: 8 },
		});

		expect(fixture.owner.current_status()).toEqual({
			href: target_href,
			key: target_key,
			restored_scroll: { x: 7, y: 8 },
		});

		await expect(routed).resolves.toMatchObject({
			kind: core6_route_popstate_result_kind.routed,
			navigation: {
				kind: core6_route_navigation_result_kind.published,
			},
			position,
		});
		expect(fixture.saved_scroll_keys).toEqual([current_key]);
		expect(fixture.fetches).toMatchObject([
			{
				active_client_build_id: "build-1",
				intent: {
					history_state: { popped: true },
					href: target_href,
					preparation_trigger:
						core6_route_preparation_trigger.navigation,
					publish_reason: core6_route_publish_reason.popstate,
				},
				kind: core6_route_transaction_kind.popstate,
			},
		]);
		expect(fixture.fetches[0]?.intent.search_params.get("q")).toBe("1");
		expect(fixture.runtime.current_route()).toMatchObject({
			href: target_href,
		});
		expect(fixture.owner.current_status()).toBeNull();
	});

	it("reloads when cross-document popstate route work fails to restore the browser route", async () => {
		const fixture = create_fixture();
		const position = make_position({
			href: target_href,
			key: target_key,
		});
		fixture.enqueue_result(failed_result());

		const result = await fixture.owner.handle({ position });

		expect(result).toMatchObject({
			kind: core6_route_popstate_result_kind.reloaded,
			navigation: {
				kind: core6_route_navigation_result_kind.failed,
			},
			position,
		});
		expect(fixture.reloads).toEqual([target_href]);
	});

	it("does not reload from stale earlier popstate failures", async () => {
		const fixture = create_fixture();
		const late_failure =
			create_core6_deferred<
				Core6ScopeCommitResult<Core6RouteFetchDriverResult>
			>();
		fixture.enqueue_wait(late_failure.promise);
		const first = fixture.owner.handle({
			position: make_position({
				href: target_href,
				key: target_key,
			}),
		});

		const second_position = make_position({
			href: `${current_href}#safe`,
			key: "same-document-key",
		});
		const second = await fixture.owner.handle({
			position: second_position,
		});
		expect(second).toMatchObject({
			kind: core6_route_popstate_result_kind.same_document,
		});

		late_failure.resolve(failed_result());
		await expect(first).resolves.toMatchObject({
			kind: core6_route_popstate_result_kind.routed,
			navigation: {
				kind: core6_route_navigation_result_kind.failed,
			},
		});
		expect(fixture.reloads).toEqual([]);
	});
});
