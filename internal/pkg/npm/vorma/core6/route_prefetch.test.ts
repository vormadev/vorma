import { describe, expect, it } from "vitest";
import {
	BUILD_ID_HEADER,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import { create_core6_deferred } from "./deferred.ts";
import { core6_route_fetch_result_kind } from "./route_fetch.ts";
import {
	core6_route_navigation_build_skew_default_behavior,
	type Core6RouteNavigationBuildSkewEvent,
} from "./route_navigation.ts";
import {
	core6_route_prefetch_result_kind,
	core6_route_prefetch_status,
	create_core6_route_prefetch_owner,
	type Core6RoutePrefetchConfig,
} from "./route_prefetch.ts";
import {
	core6_route_payload_field,
	type Core6ClientLoaderFn,
	type Core6RawRoutePayload,
} from "./route_preparation.ts";
import type { Core6RouteState } from "./route_publication.ts";
import { core6_scope_cancelled_reason } from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

function make_route(overrides: Partial<Core6RouteState> = {}): Core6RouteState {
	return {
		clientBuildID: "build-1",
		error: null,
		historyState: { from: "prefetch" },
		href: "https://example.test/current?q=Ada",
		matches: [],
		params: {},
		splatValues: [],
		...overrides,
	};
}

function make_raw_payload(
	overrides: Partial<Core6RawRoutePayload> = {},
): Core6RawRoutePayload {
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: ["/page.js"],
		[core6_route_payload_field.loaders_data]: [{ server: "page" }],
		[core6_route_payload_field.matched_patterns]: ["/page"],
		[core6_route_payload_field.params]: { slug: "page" },
		[core6_route_payload_field.search_schemas]: [{ route: "page" }],
		[core6_route_payload_field.splat_values]: [],
		...overrides,
	};
}

function make_host(
	input: {
		build_skews?: Core6RouteNavigationBuildSkewEvent[];
		current_route?: () => Core6RouteState | null;
		fetch_route_response?: Core6RoutePrefetchConfig["host"]["fetch_route_response"];
		import_module?: Core6RoutePrefetchConfig["host"]["import_module"];
		parse_input?: Core6RoutePrefetchConfig["host"]["parse_input"];
	} = {},
): Core6RoutePrefetchConfig {
	return {
		active_client_build_id: () => {
			return "build-1";
		},
		current_route:
			input.current_route ??
			(() => {
				return make_route();
			}),
		host: {
			fetch_route_response:
				input.fetch_route_response ??
				(async () => {
					return new Response(JSON.stringify(make_raw_payload()));
				}),
			import_module:
				input.import_module ??
				(async () => {
					return {};
				}),
			notify_build_skew: (event) => {
				input.build_skews?.push(event);
			},
			parse_input:
				input.parse_input ??
				(({ pattern, search_params }) => {
					return {
						pattern,
						query: search_params.get("q"),
					};
				}),
		},
	};
}

describe("core6 route prefetch owner", () => {
	it("prepares route data without publishing durable route state", async () => {
		const calls: string[] = [];
		const client_loader: Core6ClientLoaderFn = async ({
			pattern,
			serverPromise,
			trigger,
		}) => {
			calls.push(`client:${pattern}:${trigger}`);
			const server_state = await serverPromise;
			return { loader: server_state.loaderData };
		};
		const owner = create_core6_route_prefetch_owner(
			make_host({
				fetch_route_response: async ({ href, init }) => {
					const request = new URL(href);
					calls.push(
						`fetch:${request.pathname}:${request.searchParams.get(VORMA_JSON_KEY)}:${init.signal.aborted}`,
					);
					return new Response(JSON.stringify(make_raw_payload()));
				},
				import_module: async (url, signal) => {
					calls.push(`import:${url}:${signal.aborted}`);
					return {
						default: {
							client_loader,
						},
					};
				},
				parse_input: ({ pattern, search_params }) => {
					calls.push(`parse:${pattern}`);
					return {
						pattern,
						query: search_params.get("q"),
					};
				},
			}),
		);

		const result = owner.start({ href: "/page?q=Ada" });

		expect(result).not.toBeNull();
		expect(owner.current_status()).toMatchObject({
			href: "https://example.test/page?q=Ada",
			status: core6_route_prefetch_status.fetching,
		});
		await expect(result).resolves.toMatchObject({
			kind: core6_route_prefetch_result_kind.prepared,
			prepared: {
				href: "https://example.test/page?q=Ada",
				matches: [
					{
						client_loader_data: {
							loader: { server: "page" },
						},
						loader_data: { server: "page" },
						pattern: "/page",
					},
				],
			},
		});
		expect(calls).toEqual([
			"fetch:/page:build-1:false",
			"parse:/page",
			"import:/page.js:false",
			"client:/page:prefetch",
		]);
		expect(owner.current_status()).toMatchObject({
			href: "https://example.test/page?q=Ada",
			status: core6_route_prefetch_status.prepared,
		});
	});

	it("skips current, cross-origin, and duplicate same-document targets", async () => {
		const calls: string[] = [];
		const owner = create_core6_route_prefetch_owner(
			make_host({
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					calls.push(request.pathname);
					return new Response(JSON.stringify(make_raw_payload()));
				},
			}),
		);

		expect(owner.start({ href: "/current?q=Ada#panel" })).toBeNull();
		expect(
			owner.start({ href: "https://external.example/page" }),
		).toBeNull();

		const result = owner.start({ href: "/page#first" });
		expect(result).not.toBeNull();
		expect(owner.start({ href: "/page#second" })).toBeNull();
		await result;

		expect(calls).toEqual(["/page"]);
	});

	it("aborts old in-flight prefetch when a different target starts", async () => {
		const first_response = create_core6_deferred<Response>();
		const captured_signals: AbortSignal[] = [];
		const owner = create_core6_route_prefetch_owner(
			make_host({
				fetch_route_response: async ({ href, init }) => {
					captured_signals.push(init.signal);
					const request = new URL(href);
					if (request.pathname === "/first") {
						return await first_response.promise;
					}
					return new Response(
						JSON.stringify(
							make_raw_payload({
								[core6_route_payload_field.matched_patterns]: [
									"/second",
								],
							}),
						),
					);
				},
			}),
		);

		const first = owner.start({ href: "/first" });
		const second = owner.start({ href: "/second" });

		expect(first).not.toBeNull();
		expect(second).not.toBeNull();
		expect(captured_signals[0]?.aborted).toBe(true);
		await expect(second).resolves.toMatchObject({
			kind: core6_route_prefetch_result_kind.prepared,
			prepared: {
				href: "https://example.test/second",
			},
		});

		first_response.resolve(
			new Response(JSON.stringify(make_raw_payload())),
		);
		await expect(first).resolves.toEqual({
			kind: core6_route_prefetch_result_kind.interrupted,
			reason: core6_scope_cancelled_reason,
		});
		expect(owner.current_status()).toMatchObject({
			href: "https://example.test/second",
			status: core6_route_prefetch_status.prepared,
		});
	});

	it("cancels matching in-flight prefetch and ignores non-matching stops", async () => {
		const response = create_core6_deferred<Response>();
		let captured_signal!: AbortSignal;
		const owner = create_core6_route_prefetch_owner(
			make_host({
				fetch_route_response: async ({ init }) => {
					captured_signal = init.signal;
					return await response.promise;
				},
			}),
		);

		const result = owner.start({ href: "/page" });

		expect(result).not.toBeNull();
		expect(owner.cancel("/other")).toBe(false);
		expect(captured_signal.aborted).toBe(false);
		expect(owner.cancel("/page#later")).toBe(true);
		expect(captured_signal.aborted).toBe(true);
		response.resolve(new Response(JSON.stringify(make_raw_payload())));
		await expect(result).resolves.toEqual({
			kind: core6_route_prefetch_result_kind.interrupted,
			reason: core6_scope_cancelled_reason,
		});
		expect(owner.current_status()).toBeNull();
	});

	it("reports build skew as drop-response and clears the slot", async () => {
		const build_skews: Core6RouteNavigationBuildSkewEvent[] = [];
		const owner = create_core6_route_prefetch_owner(
			make_host({
				build_skews,
				fetch_route_response: async () => {
					return new Response("skew", {
						headers: {
							[BUILD_ID_HEADER]: "build-2",
							[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
						},
					});
				},
			}),
		);

		const result = owner.start({ href: "/page" });

		await expect(result).resolves.toMatchObject({
			kind: core6_route_fetch_result_kind.build_skew,
		});
		expect(build_skews).toMatchObject([
			{
				active_client_build_id: "build-1",
				default_behavior:
					core6_route_navigation_build_skew_default_behavior.drop_response,
				response: {
					requested_href: "https://example.test/page",
					server_build_id: "build-2",
				},
				trigger: core6_route_transaction_kind.prefetch,
			},
		]);
		expect(owner.current_status()).toBeNull();
	});

	it("drops redirect responses instead of publishing or following", async () => {
		const owner = create_core6_route_prefetch_owner(
			make_host({
				fetch_route_response: async () => {
					return new Response("redirect", {
						headers: {
							[X_CLIENT_REDIRECT]: "/elsewhere",
						},
					});
				},
			}),
		);

		const result = owner.start({ href: "/page" });

		await expect(result).resolves.toMatchObject({
			kind: core6_route_fetch_result_kind.redirect,
			redirect: {
				href: "https://example.test/elsewhere",
			},
		});
		expect(owner.current_status()).toBeNull();
	});

	it("takes prepared prefetches only for matching same-document hrefs", async () => {
		const owner = create_core6_route_prefetch_owner(make_host());

		const result = owner.start({ href: "/page#first" });
		await result;

		expect(owner.take_prepared("/other")).toBeNull();
		const promotion = owner.take_prepared("/page#second");
		expect(promotion).toMatchObject({
			href: "https://example.test/page#first",
			prepared: {
				href: "https://example.test/page#first",
			},
		});
		expect(owner.current_status()).toBeNull();
		expect(owner.take_prepared("/page")).toBeNull();
	});
});
