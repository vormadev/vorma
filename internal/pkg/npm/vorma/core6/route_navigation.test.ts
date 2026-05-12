import { describe, expect, it } from "vitest";
import {
	BUILD_ID_HEADER,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import {
	CORE6_MAX_REDIRECTS,
	core6_route_navigation_build_skew_default_behavior,
	core6_route_navigation_result_kind,
	run_core6_route_navigation,
	type Core6RouteNavigationBuildSkewEvent,
	type Core6RouteNavigationHost,
	type Core6RouteNavigationRedirectLoopEvent,
} from "./route_navigation.ts";
import {
	core6_route_prefetch_result_kind,
	create_core6_route_prefetch_owner,
} from "./route_prefetch.ts";
import {
	core6_route_payload_field,
	core6_route_preparation_trigger,
	type Core6RawRoutePayload,
} from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RouteCommit,
	type Core6RouteState,
} from "./route_publication.ts";
import {
	create_core6_route_runtime,
	type Core6RouteRuntimeHost,
} from "./route_runtime.ts";
import type { Core6RouteTransactionFlowIntent } from "./route_transaction_flow.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

function make_intent(
	overrides: Partial<Core6RouteTransactionFlowIntent> = {},
): Core6RouteTransactionFlowIntent {
	return {
		history_state: { from: "navigation" },
		href: "https://example.test/start",
		preparation_trigger: core6_route_preparation_trigger.navigation,
		publish_reason: core6_route_publish_reason.navigation,
		search_params: new URLSearchParams(),
		...overrides,
	};
}

function make_raw_payload(
	overrides: Partial<Core6RawRoutePayload> = {},
): Core6RawRoutePayload {
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: [],
		[core6_route_payload_field.loaders_data]: [{ ok: true }],
		[core6_route_payload_field.matched_patterns]: ["/final"],
		[core6_route_payload_field.params]: {},
		[core6_route_payload_field.search_schemas]: [{}],
		[core6_route_payload_field.splat_values]: [],
		...overrides,
	};
}

function same_route_state(
	previous_route: Core6RouteState,
	next_route: Core6RouteState,
): boolean {
	return JSON.stringify(previous_route) === JSON.stringify(next_route);
}

function make_runtime_host(input: {
	commits?: Core6RouteCommit[];
	fetch_route_response: Core6RouteRuntimeHost["fetch_route_response"];
}): Core6RouteRuntimeHost {
	return {
		commit_publication: (publication) => {
			input.commits?.push(publication.commit);
		},
		fetch_route_payload: async () => {
			return make_raw_payload();
		},
		fetch_route_response: input.fetch_route_response,
		import_module: async () => {
			return {};
		},
		parse_input: ({ pattern, search_params }) => {
			return {
				pattern,
				query: search_params.get("q"),
			};
		},
		route_state_equal: same_route_state,
	};
}

function make_navigation_host(input: {
	build_skews?: Core6RouteNavigationBuildSkewEvent[];
	hard_redirects?: string[];
	redirect_loops?: Core6RouteNavigationRedirectLoopEvent[];
}): Core6RouteNavigationHost {
	return {
		hard_redirect: (href) => {
			input.hard_redirects?.push(href);
		},
		notify_build_skew: (event) => {
			input.build_skews?.push(event);
		},
		warn_redirect_loop: (event) => {
			input.redirect_loops?.push(event);
		},
	};
}

describe("core6 route navigation", () => {
	it("publishes a prepared prefetch without starting a route fetch", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					calls.push(`runtime:${href}`);
					throw new Error("navigation should use prepared prefetch");
				},
			}),
		);
		await runtime.run_route({
			intent: make_intent({
				href: "https://example.test/current",
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		commits.length = 0;
		const prefetch = create_core6_route_prefetch_owner({
			active_client_build_id: () => {
				return "build-1";
			},
			current_route: runtime.current_route,
			host: {
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					calls.push(`prefetch:${request.pathname}`);
					return new Response(
						JSON.stringify(
							make_raw_payload({
								[core6_route_payload_field.loaders_data]: [
									{ prefetched: true },
								],
								[core6_route_payload_field.matched_patterns]: [
									"/prefetched",
								],
							}),
						),
					);
				},
				import_module: async () => {
					return {};
				},
				parse_input: ({ pattern }) => {
					return { pattern };
				},
			},
		});

		const prefetch_result = prefetch.start({
			href: "/prefetched",
		});
		await expect(prefetch_result).resolves.toMatchObject({
			kind: core6_route_prefetch_result_kind.prepared,
		});
		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({}),
			intent: make_intent({
				href: "https://example.test/prefetched",
			}),
			kind: core6_route_transaction_kind.navigation,
			prefetch,
			runtime,
		});

		expect(result).toMatchObject({
			kind: core6_route_navigation_result_kind.published,
			redirect_count: 0,
			result: {
				publication: {
					route: {
						historyState: { from: "navigation" },
						href: "https://example.test/prefetched",
						matches: [
							{
								loaderData: { prefetched: true },
								pattern: "/prefetched",
							},
						],
					},
				},
			},
		});
		expect(calls).toEqual(["prefetch:/prefetched"]);
		expect(commits).toHaveLength(1);
		expect(prefetch.current_status()).toBeNull();
	});

	it("follows same-origin redirects through the runtime until publication", async () => {
		const commits: Core6RouteCommit[] = [];
		const requests: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					requests.push(
						`${request.pathname}:${request.searchParams.get(VORMA_JSON_KEY)}`,
					);
					if (request.pathname === "/start") {
						return new Response("redirect", {
							headers: {
								[X_CLIENT_REDIRECT]: "/final",
							},
						});
					}
					return new Response(JSON.stringify(make_raw_payload()));
				},
			}),
		);

		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			runtime,
		});

		expect(result).toMatchObject({
			kind: core6_route_navigation_result_kind.published,
			redirect_count: 1,
		});
		expect(requests).toEqual(["/start:build-1", "/final:build-1"]);
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()).toMatchObject({
			href: "https://example.test/final",
			matches: [{ pattern: "/final" }],
		});
	});

	it("publishes same-document redirects through the runtime publication boundary", async () => {
		const commits: Core6RouteCommit[] = [];
		const requests: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					requests.push(
						`${request.pathname}:${request.searchParams.get(VORMA_JSON_KEY)}`,
					);
					return new Response("redirect", {
						headers: {
							[X_CLIENT_REDIRECT]: "/current#section",
						},
					});
				},
			}),
		);
		await runtime.run_route({
			intent: make_intent({
				href: "https://example.test/current",
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		commits.length = 0;
		requests.length = 0;

		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			runtime,
		});

		expect(result).toMatchObject({
			href: "https://example.test/current#section",
			kind: core6_route_navigation_result_kind.same_document,
			publication: {
				commit: {
					route_render: {
						scroll: { hash: "#section" },
					},
					route_update: {
						reason: core6_route_publish_reason.navigation,
					},
				},
				route: {
					historyState: { from: "navigation" },
					href: "https://example.test/current#section",
				},
			},
			redirect_count: 0,
		});
		expect(requests).toEqual(["/start:build-1"]);
		expect(commits).toHaveLength(1);
		if (result.kind !== core6_route_navigation_result_kind.same_document) {
			throw new Error("expected same-document result");
		}
		expect(commits[0]).toBe(result.publication.commit);
		expect(runtime.current_route()).toMatchObject({
			historyState: { from: "navigation" },
			href: "https://example.test/current#section",
		});
	});

	it("hard redirects cross-origin soft redirects without publishing", async () => {
		const commits: Core6RouteCommit[] = [];
		const hard_redirects: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async () => {
					return new Response("redirect", {
						headers: {
							[X_CLIENT_REDIRECT]:
								"https://external.example/target",
						},
					});
				},
			}),
		);

		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({ hard_redirects }),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			runtime,
		});

		expect(result).toMatchObject({
			href: "https://external.example/target",
			kind: core6_route_navigation_result_kind.hard_redirect,
			redirect_count: 0,
		});
		expect(hard_redirects).toEqual(["https://external.example/target"]);
		expect(commits).toEqual([]);
		expect(runtime.current_route()).toBeNull();
	});

	it("hard reloads navigation build skew to the requested href", async () => {
		const build_skews: Core6RouteNavigationBuildSkewEvent[] = [];
		const hard_redirects: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				fetch_route_response: async () => {
					return new Response("skew", {
						headers: {
							[BUILD_ID_HEADER]: "build-2",
							[X_CLIENT_REDIRECT]: "/soft",
							[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
						},
					});
				},
			}),
		);

		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({ build_skews, hard_redirects }),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			runtime,
		});

		expect(result).toMatchObject({
			behavior:
				core6_route_navigation_build_skew_default_behavior.hard_reload,
			kind: core6_route_navigation_result_kind.build_skew,
		});
		expect(hard_redirects).toEqual(["https://example.test/start"]);
		expect(build_skews).toMatchObject([
			{
				active_client_build_id: "build-1",
				default_behavior:
					core6_route_navigation_build_skew_default_behavior.hard_reload,
				response: {
					requested_href: "https://example.test/start",
					server_build_id: "build-2",
				},
				trigger: core6_route_transaction_kind.navigation,
			},
		]);
	});

	it("follows revalidation redirects as navigation fetches", async () => {
		const commits: Core6RouteCommit[] = [];
		const requests: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					requests.push(
						[
							request.pathname,
							request.searchParams.get(VORMA_JSON_KEY),
							request.searchParams.get(
								VERCEL_DPL_QUERY_PARAM_KEY,
							),
						].join(":"),
					);
					if (request.pathname === "/start") {
						return new Response("redirect", {
							headers: {
								[X_CLIENT_REDIRECT]: "/new-page",
							},
						});
					}
					return new Response(
						JSON.stringify(
							make_raw_payload({
								[core6_route_payload_field.matched_patterns]: [
									"/new-page",
								],
							}),
						),
					);
				},
			}),
		);

		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			deployment_id: "deployment-1",
			host: make_navigation_host({}),
			intent: make_intent({
				preparation_trigger:
					core6_route_preparation_trigger.revalidation,
				publish_reason: core6_route_publish_reason.revalidation,
			}),
			kind: core6_route_transaction_kind.revalidation,
			runtime,
		});

		expect(result).toMatchObject({
			kind: core6_route_navigation_result_kind.published,
			redirect_count: 1,
		});
		expect(requests).toEqual([
			"/start:build-1:deployment-1",
			"/new-page:build-1:",
		]);
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()?.href).toBe(
			"https://example.test/new-page",
		);
	});

	it("caps redirect chains before starting another fetch", async () => {
		const commits: Core6RouteCommit[] = [];
		const redirect_loops: Core6RouteNavigationRedirectLoopEvent[] = [];
		const requests: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					requests.push(request.pathname);
					return new Response("redirect", {
						headers: {
							[X_CLIENT_REDIRECT]: `/loop-${requests.length}`,
						},
					});
				},
			}),
		);

		const result = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({ redirect_loops }),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			max_redirects: 2,
			runtime,
		});

		expect(result).toMatchObject({
			href: "https://example.test/loop-3",
			kind: core6_route_navigation_result_kind.redirect_loop,
			redirect_count: 3,
		});
		expect(requests).toEqual(["/start", "/loop-1", "/loop-2"]);
		expect(redirect_loops).toEqual([
			{
				href: "https://example.test/loop-3",
				redirect_count: 3,
			},
		]);
		expect(commits).toEqual([]);
		expect(CORE6_MAX_REDIRECTS).toBe(10);
	});

	it("stops a redirect chain when its target fetch is superseded", async () => {
		const commits: Core6RouteCommit[] = [];
		let resolve_target!: (response: Response) => void;
		let mark_target_started!: () => void;
		const target_started = new Promise<void>((resolve) => {
			mark_target_started = resolve;
		});
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					if (request.pathname === "/start") {
						return new Response("redirect", {
							headers: {
								[X_CLIENT_REDIRECT]: "/target",
							},
						});
					}
					if (request.pathname === "/target") {
						mark_target_started();
						return await new Promise<Response>((resolve) => {
							resolve_target = resolve;
						});
					}
					return new Response(
						JSON.stringify(
							make_raw_payload({
								[core6_route_payload_field.loaders_data]: [
									{ winner: true },
								],
								[core6_route_payload_field.matched_patterns]: [
									"/winner",
								],
							}),
						),
					);
				},
			}),
		);

		const first = run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			runtime,
		});
		await target_started;
		const second = await run_core6_route_navigation({
			active_client_build_id: "build-1",
			host: make_navigation_host({}),
			intent: make_intent({
				href: "https://example.test/winner",
			}),
			kind: core6_route_transaction_kind.navigation,
			runtime,
		});
		resolve_target(new Response(JSON.stringify(make_raw_payload())));

		await expect(first).resolves.toMatchObject({
			kind: core6_route_navigation_result_kind.interrupted,
			reason: core6_scope_stale_reason,
			redirect_count: 1,
		});
		expect(second).toMatchObject({
			kind: core6_route_navigation_result_kind.published,
		});
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()).toMatchObject({
			href: "https://example.test/winner",
			matches: [
				{
					loaderData: { winner: true },
					pattern: "/winner",
				},
			],
		});
	});
});
