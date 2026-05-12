import { describe, expect, it } from "vitest";
import { create_core6_deferred } from "./deferred.ts";
import type { Core6RouteFetchResponseMeta } from "./route_fetch.ts";
import {
	core6_route_fetch_driver_result_kind,
	type Core6RouteFetchDriverResult,
} from "./route_fetch_driver.ts";
import {
	core6_route_navigation_owner_result_kind,
	create_core6_route_navigation_owner,
	type Core6RouteNavigationOwnerHost,
	type Core6RouteNavigationOwnerRuntime,
} from "./route_navigation_owner.ts";
import { core6_route_preparation_trigger } from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RouteCommit,
	type Core6RoutePublication,
	type Core6RouteScroll,
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
const target_href = "https://example.test/target?q=1";

type QueuedRouteFetchResult = Promise<
	Core6ScopeCommitResult<Core6RouteFetchDriverResult>
>;

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
	scroll?: Core6RouteScroll;
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
			reason: core6_route_publish_reason.navigation,
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
	hard_redirects: string[];
	host: Core6RouteNavigationOwnerHost;
	owner: ReturnType<typeof create_core6_route_navigation_owner>;
	runtime: Core6RouteNavigationOwnerRuntime;
	same_document_publications: Core6RoutePublication[];
	set_current_href: (href: string) => void;
	set_current_transaction_kind: (
		kind: Core6RouteTransactionKind | null,
	) => void;
} {
	let current_href_value = current_href;
	let current_route: Core6RouteState | null =
		input.current_route ?? make_route();
	let current_transaction_kind = input.current_transaction_kind ?? null;
	let cancel_count = 0;
	const fetches: Core6RouteRuntimeRunFetchInput[] = [];
	const hard_redirects: string[] = [];
	const queue: QueuedRouteFetchResult[] = [];
	const same_document_publications: Core6RoutePublication[] = [];
	const host: Core6RouteNavigationOwnerHost = {
		current_href: () => {
			return current_href_value;
		},
		hard_redirect: (href) => {
			hard_redirects.push(href);
		},
	};
	const runtime: Core6RouteNavigationOwnerRuntime = {
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
			if (same_document_input.history) {
				publication.history = {
					href: route.href,
					replace: same_document_input.history.replace,
					state: route.historyState,
				};
			}
			current_route = route;
			same_document_publications.push(publication);
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
				if (
					!settled.value.publication.history &&
					run_input.intent.history
				) {
					settled.value.publication.history = {
						href: settled.value.publication.route.href,
						replace: run_input.intent.history.replace,
						state: settled.value.publication.route.historyState,
					};
				}
				current_route = settled.value.publication.route;
			}
			current_transaction_kind = null;
			return settled;
		},
		run_route_prepared: () => {
			return { ok: false, reason: core6_scope_stale_reason };
		},
	};
	const owner = create_core6_route_navigation_owner({
		active_client_build_id: () => {
			return "build-1";
		},
		deployment_id: () => {
			return "deployment-1";
		},
		host,
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
		hard_redirects,
		host,
		owner,
		runtime,
		same_document_publications,
		set_current_href: (href) => {
			current_href_value = href;
		},
		set_current_transaction_kind: (kind) => {
			current_transaction_kind = kind;
		},
	};
}

describe("core6 route navigation owner", () => {
	it("hard redirects non-SPA targets without starting route work", async () => {
		const fixture = create_fixture();

		const result = await fixture.owner.navigate({
			href: "https://external.test/target",
		});

		expect(result).toEqual({
			did_navigate: false,
			history: null,
			href: "https://external.test/target",
			kind: core6_route_navigation_owner_result_kind.hard_redirect,
		});
		expect(fixture.hard_redirects).toEqual([
			"https://external.test/target",
		]);
		expect(fixture.fetches).toEqual([]);
		expect(fixture.owner.current_status()).toBeNull();
	});

	it("publishes same-document navigation synchronously and cancels stale navigation work", async () => {
		const fixture = create_fixture({
			current_transaction_kind: core6_route_transaction_kind.navigation,
		});

		const result = await fixture.owner.navigate({
			href: "#section",
			state: { from: "navigation" },
		});

		expect(result).toMatchObject({
			did_navigate: true,
			history: {
				href: `${current_href}#section`,
				replace: false,
				state: { from: "navigation" },
			},
			kind: core6_route_navigation_owner_result_kind.same_document,
			publication: {
				commit: {
					route_render: {
						scroll: { hash: "#section" },
					},
				},
				route: {
					historyState: { from: "navigation" },
					href: `${current_href}#section`,
				},
			},
			scroll: { hash: "#section" },
		});
		expect(fixture.cancel_count()).toBe(1);
		expect(fixture.fetches).toEqual([]);
		expect(fixture.same_document_publications).toHaveLength(1);
	});

	it("does not cancel revalidation work for same-document navigation", async () => {
		const fixture = create_fixture({
			current_transaction_kind: core6_route_transaction_kind.revalidation,
		});

		await fixture.owner.navigate({
			href: "#section",
		});

		expect(fixture.cancel_count()).toBe(0);
		expect(fixture.runtime.current_transaction_kind()).toBe(
			core6_route_transaction_kind.revalidation,
		);
	});

	it("keeps same-hash non-replace navigation as a scroll-only result", async () => {
		const fixture = create_fixture({
			current_route: make_route({
				href: `${current_href}#section`,
			}),
		});
		fixture.set_current_href(`${current_href}#section`);

		const result = await fixture.owner.navigate({
			href: "#section",
			state: { ignored: true },
		});

		expect(result).toEqual({
			did_navigate: false,
			history: null,
			kind: core6_route_navigation_owner_result_kind.same_document,
			publication: null,
			scroll: { hash: "#section" },
		});
		expect(fixture.same_document_publications).toEqual([]);
		expect(fixture.fetches).toEqual([]);
	});

	it("routes cross-document navigation through route fetch ownership", async () => {
		const fixture = create_fixture();
		const target_route = make_route({
			historyState: { page: "target" },
			href: target_href,
		});
		const route_fetch =
			create_core6_deferred<
				Core6ScopeCommitResult<Core6RouteFetchDriverResult>
			>();
		fixture.enqueue_wait(route_fetch.promise);

		const navigation = fixture.owner.navigate({
			href: "/target?q=1",
			replace: true,
			state: { page: "target" },
		});

		expect(fixture.owner.current_status()).toEqual({
			href: target_href,
			replace: true,
			state: { page: "target" },
		});
		expect(fixture.fetches).toMatchObject([
			{
				active_client_build_id: "build-1",
				deployment_id: "deployment-1",
				intent: {
					history_state: { page: "target" },
					href: target_href,
					preparation_trigger:
						core6_route_preparation_trigger.navigation,
					publish_reason: core6_route_publish_reason.navigation,
				},
				kind: core6_route_transaction_kind.navigation,
			},
		]);
		expect(fixture.fetches[0]?.intent.search_params.get("q")).toBe("1");

		route_fetch.resolve(published_result(target_route));
		await expect(navigation).resolves.toMatchObject({
			did_navigate: true,
			history: {
				href: target_href,
				replace: true,
				state: { page: "target" },
			},
			kind: core6_route_navigation_owner_result_kind.routed,
		});
		expect(fixture.owner.current_status()).toBeNull();
		expect(fixture.runtime.current_route()).toMatchObject({
			href: target_href,
		});
	});
});
