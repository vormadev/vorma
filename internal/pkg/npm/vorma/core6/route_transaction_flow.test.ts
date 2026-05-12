import { describe, expect, it } from "vitest";
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
import {
	run_core6_route_transaction_flow,
	type Core6RouteTransactionFlowHost,
	type Core6RouteTransactionFlowIntent,
} from "./route_transaction_flow.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import {
	core6_route_transaction_kind,
	create_core6_route_transaction_manager,
} from "./transaction.ts";

function make_flow_intent(
	overrides: Partial<Core6RouteTransactionFlowIntent> = {},
): Core6RouteTransactionFlowIntent {
	return {
		history_state: { from: "intent" },
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

describe("core6 route transaction flow", () => {
	it("runs fetch, decode, prepare, and publish through the route transaction pipeline", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const calls: string[] = [];
		const publications: Core6RoutePublication[] = [];
		const client_loader: Core6ClientLoaderFn = async ({
			input,
			pattern,
			serverPromise,
		}) => {
			calls.push(`client:${pattern}`);
			const server_state = await serverPromise;
			return {
				input,
				server: server_state.loaderData,
			};
		};
		const host: Core6RouteTransactionFlowHost = {
			current_route: () => {
				calls.push("current");
				return null;
			},
			fetch_route_payload: async ({ intent, signal }) => {
				calls.push(`fetch:${intent.href}:${signal.aborted}`);
				return make_raw_payload();
			},
			import_module: async (url, signal) => {
				calls.push(`import:${url}:${signal.aborted}`);
				return {
					default: {
						client_loader,
					},
				};
			},
			parse_input: ({ pattern, schema, search_params }) => {
				calls.push(`parse:${pattern}:${search_params.get("q")}`);
				return {
					pattern,
					query: search_params.get("q"),
					schema,
				};
			},
			publish_route: (publication) => {
				calls.push(`publish:${publication.route.href}`);
				publications.push(publication);
			},
			route_state_equal: same_route_state,
		};

		const result = await run_core6_route_transaction_flow({
			host,
			intent: make_flow_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada:false",
			"parse:/root:Ada",
			"import:/root.js:false",
			"client:/root",
			"current",
			"publish:https://example.test/root?q=Ada",
		]);
		expect(publications).toHaveLength(1);
		expect(publications[0]!.route).toMatchObject({
			clientBuildID: "build-1",
			historyState: { from: "intent" },
			href: "https://example.test/root?q=Ada",
			matches: [
				{
					clientLoaderData: {
						input: {
							pattern: "/root",
							query: "Ada",
							schema: { route: "root" },
						},
						server: { server: "root" },
					},
					loaderData: { server: "root" },
					pattern: "/root",
				},
			],
			params: { user: "ada" },
			splatValues: ["tail"],
		});
		expect(publications[0]!.commit.route_update?.reason).toBe(
			core6_route_publish_reason.navigation,
		);
		expect(transaction_manager.current()).toBeNull();
	});

	it("does not decode, prepare, or publish stale fetch results", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const calls: string[] = [];
		let resolve_fetch!: (raw_payload: Core6RawRoutePayload) => void;
		const host: Core6RouteTransactionFlowHost = {
			current_route: () => {
				calls.push("current");
				return null;
			},
			fetch_route_payload: async () => {
				calls.push("fetch");
				return await new Promise<Core6RawRoutePayload>((resolve) => {
					resolve_fetch = resolve;
				});
			},
			import_module: async () => {
				calls.push("import");
				return {};
			},
			parse_input: () => {
				calls.push("parse");
				return {};
			},
			publish_route: () => {
				calls.push("publish");
			},
			route_state_equal: same_route_state,
		};

		const result = run_core6_route_transaction_flow({
			host,
			intent: make_flow_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		transaction_manager.start({
			intent: make_flow_intent({
				href: "https://example.test/second",
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_fetch(make_raw_payload());

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual(["fetch"]);
	});

	it("does not publish stale preparation results", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const calls: string[] = [];
		let resolve_import!: (module: Core6RouteModule) => void;
		let mark_import_started!: () => void;
		const import_started = new Promise<void>((resolve) => {
			mark_import_started = resolve;
		});
		const host: Core6RouteTransactionFlowHost = {
			current_route: () => {
				calls.push("current");
				return null;
			},
			fetch_route_payload: async () => {
				calls.push("fetch");
				return make_raw_payload();
			},
			import_module: async () => {
				calls.push("import");
				mark_import_started();
				return await new Promise<Core6RouteModule>((resolve) => {
					resolve_import = resolve;
				});
			},
			parse_input: () => {
				calls.push("parse");
				return {};
			},
			publish_route: () => {
				calls.push("publish");
			},
			route_state_equal: same_route_state,
		};

		const result = run_core6_route_transaction_flow({
			host,
			intent: make_flow_intent(),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		await import_started;
		transaction_manager.start({
			intent: make_flow_intent({
				href: "https://example.test/second",
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_import({
			default: {
				client_loader: async () => {
					calls.push("client");
					return {};
				},
			},
		});

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual(["fetch", "parse", "import"]);
	});

	it("reads previous route and compares state only at the publish boundary", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.revalidation,
			Core6RouteTransactionFlowIntent
		>();
		const calls: string[] = [];
		const previous_route: Core6RouteState = {
			clientBuildID: "build-1",
			error: null,
			historyState: { from: "previous" },
			href: "https://example.test/root?q=Ada",
			matches: [],
			params: {},
			splatValues: [],
		};
		const host: Core6RouteTransactionFlowHost = {
			current_route: () => {
				calls.push("current");
				return previous_route;
			},
			fetch_route_payload: async () => {
				calls.push("fetch");
				return make_raw_payload();
			},
			import_module: async () => {
				calls.push("import");
				return {};
			},
			parse_input: () => {
				calls.push("parse");
				return {};
			},
			publish_route: (publication) => {
				calls.push(
					publication.commit.route_update
						? "publish:update"
						: "publish:render",
				);
			},
			route_state_equal: (previous, next) => {
				calls.push(`equal:${previous.href}:${next.href}`);
				return true;
			},
		};

		const result = await run_core6_route_transaction_flow({
			host,
			intent: make_flow_intent({
				preparation_trigger:
					core6_route_preparation_trigger.revalidation,
				publish_reason: core6_route_publish_reason.revalidation,
			}),
			kind: core6_route_transaction_kind.revalidation,
			transaction_manager,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch",
			"parse",
			"import",
			"current",
			"equal:https://example.test/root?q=Ada:https://example.test/root?q=Ada",
			"publish:render",
		]);
		if (result.ok) {
			expect(result.value.commit.route_update).toBeUndefined();
		}
	});

	it("lets publish failures bubble while releasing transaction ownership", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			Core6RouteTransactionFlowIntent
		>();
		const error = new Error("publish failed");
		const host: Core6RouteTransactionFlowHost = {
			current_route: () => {
				return null;
			},
			fetch_route_payload: async () => {
				return make_raw_payload();
			},
			import_module: async () => {
				return {};
			},
			parse_input: () => {
				return {};
			},
			publish_route: () => {
				throw error;
			},
			route_state_equal: same_route_state,
		};

		await expect(
			run_core6_route_transaction_flow({
				host,
				intent: make_flow_intent(),
				kind: core6_route_transaction_kind.navigation,
				transaction_manager,
			}),
		).rejects.toBe(error);
		expect(transaction_manager.current()).toBeNull();
	});
});
