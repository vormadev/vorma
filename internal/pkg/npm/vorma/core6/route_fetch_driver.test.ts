import { describe, expect, it } from "vitest";
import {
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import {
	core6_route_fetch_failure_reason,
	core6_route_fetch_result_kind,
} from "./route_fetch.ts";
import {
	core6_route_fetch_driver_result_kind,
	run_core6_route_fetch_driver,
	type Core6RouteFetchDriverHost,
} from "./route_fetch_driver.ts";
import {
	core6_route_payload_field,
	core6_route_preparation_trigger,
	type Core6ClientLoaderFn,
	type Core6RawRoutePayload,
	type Core6RouteModule,
} from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RoutePublication,
	type Core6RouteState,
} from "./route_publication.ts";
import type { Core6RouteTransactionFlowIntent } from "./route_transaction_flow.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import {
	core6_route_transaction_kind,
	create_core6_route_transaction_manager,
} from "./transaction.ts";

function make_intent(
	overrides: Partial<Core6RouteTransactionFlowIntent> = {},
): Core6RouteTransactionFlowIntent {
	return {
		history_state: { from: "driver" },
		href: "https://example.test/root?q=Ada",
		preparation_trigger: core6_route_preparation_trigger.navigation,
		publish_reason: core6_route_publish_reason.navigation,
		search_params: new URLSearchParams("q=Ada"),
		...overrides,
	};
}

function make_raw_payload(
	overrides: Partial<Core6RawRoutePayload> = {},
): Core6RawRoutePayload {
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: ["/root.js"],
		[core6_route_payload_field.loaders_data]: [{ server: "root" }],
		[core6_route_payload_field.matched_patterns]: ["/root"],
		[core6_route_payload_field.params]: { user: "ada" },
		[core6_route_payload_field.search_schemas]: [{ route: "root" }],
		[core6_route_payload_field.splat_values]: ["tail"],
		...overrides,
	};
}

function same_route_state(
	previous_route: Core6RouteState,
	next_route: Core6RouteState,
): boolean {
	return JSON.stringify(previous_route) === JSON.stringify(next_route);
}

function make_driver_host(input: {
	calls?: string[];
	current_route?: () => Core6RouteState | null;
	fetch_route_response?: Core6RouteFetchDriverHost["fetch_route_response"];
	import_module?: Core6RouteFetchDriverHost["import_module"];
	parse_input?: Core6RouteFetchDriverHost["parse_input"];
	publications?: Core6RoutePublication[];
	route_state_equal?: Core6RouteFetchDriverHost["route_state_equal"];
}): Core6RouteFetchDriverHost {
	return {
		current_route:
			input.current_route ??
			(() => {
				input.calls?.push("current");
				return null;
			}),
		fetch_route_response:
			input.fetch_route_response ??
			(async ({ href }) => {
				input.calls?.push(`fetch:${href}`);
				return new Response(JSON.stringify(make_raw_payload()));
			}),
		import_module:
			input.import_module ??
			(async (url) => {
				input.calls?.push(`import:${url}`);
				return {};
			}),
		parse_input:
			input.parse_input ??
			(({ pattern, search_params }) => {
				input.calls?.push(`parse:${pattern}`);
				return {
					pattern,
					query: search_params.get("q"),
				};
			}),
		publish_route: (publication) => {
			input.calls?.push(`publish:${publication.route.href}`);
			input.publications?.push(publication);
		},
		route_state_equal: input.route_state_equal ?? same_route_state,
	};
}

describe("core6 route fetch driver", () => {
	it("publishes payload fetch results through the route pipeline", async () => {
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		const client_loader: Core6ClientLoaderFn = async ({
			pattern,
			serverPromise,
		}) => {
			calls.push(`client:${pattern}`);
			const server_state = await serverPromise;
			return { server: server_state.loaderData };
		};
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const result = await run_core6_route_fetch_driver({
			active_client_build_id: "build-1",
			host: make_driver_host({
				calls,
				import_module: async (url) => {
					calls.push(`import:${url}`);
					return {
						default: {
							client_loader,
						},
					};
				},
				publications,
			}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada&vorma-json=build-1",
			"parse:/root",
			"import:/root.js",
			"client:/root",
			"current",
			"publish:https://example.test/root?q=Ada",
		]);
		expect(publications).toHaveLength(1);
		if (!result.ok) {
			throw new Error("expected published result");
		}
		expect(result.value).toMatchObject({
			kind: core6_route_fetch_driver_result_kind.published,
			publication: publications[0],
			response: {
				ok: true,
				requested_href: "https://example.test/root?q=Ada",
				status: 200,
			},
		});
		if (
			result.value.kind !== core6_route_fetch_driver_result_kind.published
		) {
			throw new Error("expected published result");
		}
		expect(result.value.publication.route).toMatchObject({
			historyState: { from: "driver" },
			href: "https://example.test/root?q=Ada",
			matches: [
				{
					clientLoaderData: { server: { server: "root" } },
					loaderData: { server: "root" },
					pattern: "/root",
				},
			],
			params: { user: "ada" },
			splatValues: ["tail"],
		});
		expect(transaction_manager.current()).toBeNull();
	});

	it("returns redirect fetch results without preparing or publishing", async () => {
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();

		const result = await run_core6_route_fetch_driver({
			active_client_build_id: "build-1",
			host: make_driver_host({
				calls,
				fetch_route_response: async ({ href }) => {
					calls.push(`fetch:${href}`);
					return new Response("not json", {
						headers: {
							[X_CLIENT_REDIRECT]: "/login",
						},
					});
				},
				publications,
			}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada&vorma-json=build-1",
		]);
		expect(publications).toEqual([]);
		if (!result.ok) {
			throw new Error("expected redirect result");
		}
		expect(result.value).toMatchObject({
			kind: core6_route_fetch_result_kind.redirect,
			redirect: {
				hard: false,
				href: "https://example.test/login",
			},
		});
		expect(transaction_manager.current()).toBeNull();
	});

	it("returns build skew fetch results without preparing or publishing", async () => {
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.revalidation,
			Core6RouteTransactionFlowIntent
		>();

		const result = await run_core6_route_fetch_driver({
			active_client_build_id: "build-1",
			deployment_id: "deployment-1",
			host: make_driver_host({
				calls,
				fetch_route_response: async ({ href }) => {
					calls.push(`fetch:${href}`);
					return new Response("not json", {
						headers: {
							[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
						},
					});
				},
				publications,
			}),
			intent: make_intent({
				preparation_trigger:
					core6_route_preparation_trigger.revalidation,
				publish_reason: core6_route_publish_reason.revalidation,
			}),
			kind: core6_route_transaction_kind.revalidation,
			transaction_manager,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada&vorma-json=build-1&dpl=deployment-1",
		]);
		expect(publications).toEqual([]);
		if (!result.ok) {
			throw new Error("expected build skew result");
		}
		expect(result.value).toMatchObject({
			kind: core6_route_fetch_result_kind.build_skew,
			redirect: null,
		});
		expect(transaction_manager.current()).toBeNull();
	});

	it("returns failure fetch results without preparing or publishing", async () => {
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();

		const result = await run_core6_route_fetch_driver({
			active_client_build_id: "build-1",
			host: make_driver_host({
				calls,
				fetch_route_response: async ({ href }) => {
					calls.push(`fetch:${href}`);
					return new Response("failed", { status: 503 });
				},
				publications,
			}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada&vorma-json=build-1",
		]);
		expect(publications).toEqual([]);
		if (!result.ok) {
			throw new Error("expected failure result");
		}
		expect(result.value).toMatchObject({
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.http_error,
		});
		expect(transaction_manager.current()).toBeNull();
	});

	it("does not prepare or publish stale fetch results", async () => {
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		let resolve_fetch!: (response: Response) => void;
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const result = run_core6_route_fetch_driver({
			active_client_build_id: "build-1",
			host: make_driver_host({
				calls,
				fetch_route_response: async ({ href }) => {
					calls.push(`fetch:${href}`);
					return await new Promise<Response>((resolve) => {
						resolve_fetch = resolve;
					});
				},
				publications,
			}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		transaction_manager.start({
			intent: make_intent({
				href: "https://example.test/second",
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_fetch(new Response(JSON.stringify(make_raw_payload())));

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada&vorma-json=build-1",
		]);
		expect(publications).toEqual([]);
	});

	it("does not publish stale preparation results", async () => {
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		let resolve_import!: (module: Core6RouteModule) => void;
		let mark_import_started!: () => void;
		const import_started = new Promise<void>((resolve) => {
			mark_import_started = resolve;
		});
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const result = run_core6_route_fetch_driver({
			active_client_build_id: "build-1",
			host: make_driver_host({
				calls,
				import_module: async (url) => {
					calls.push(`import:${url}`);
					mark_import_started();
					return await new Promise<Core6RouteModule>((resolve) => {
						resolve_import = resolve;
					});
				},
				publications,
			}),
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		await import_started;
		transaction_manager.start({
			intent: make_intent({
				href: "https://example.test/second",
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_import({});

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada&vorma-json=build-1",
			"parse:/root",
			"import:/root.js",
		]);
		expect(publications).toEqual([]);
	});

	it("bubbles publish errors while releasing ownership", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const error = new Error("publish failed");

		await expect(
			run_core6_route_fetch_driver({
				active_client_build_id: "build-1",
				host: {
					...make_driver_host({}),
					publish_route: () => {
						throw error;
					},
				},
				intent: make_intent(),
				kind: core6_route_transaction_kind.navigation,
				transaction_manager,
			}),
		).rejects.toBe(error);
		expect(transaction_manager.current()).toBeNull();
	});
});
