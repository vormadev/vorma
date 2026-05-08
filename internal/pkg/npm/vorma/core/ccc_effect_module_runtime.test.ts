// @vitest-environment jsdom

import { Effect } from "effect";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
	ClientCommit,
	ClientLoaderFn,
} from "./effect_runtime/client_contract.ts";
import { make_module_runtime } from "./effect_runtime/module_runtime.ts";
import {
	type PreparedRoute,
	type RouteModule,
	type RouteRecord,
	make_route_preparer,
} from "./effect_runtime/route_preparer.ts";
import { make_route_publisher } from "./effect_runtime/route_publisher.ts";
import { make_runtime_lifecycle } from "./effect_runtime/runtime_lifecycle.ts";
import type { WorkStateActor } from "./effect_runtime/work_state_actor.ts";

const client_build_id = "build-1";
const hmr_href = "http://localhost/hmr";
const hmr_module_url = "/hmr-module.js";
const hmr_pattern = "/hmr";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function view_module(input: {
	component_value: string;
	client_loader?: ClientLoaderFn;
}): RouteModule {
	return {
		default: {
			pattern: hmr_pattern,
			component: () => {
				return input.component_value;
			},
			client_loader: input.client_loader,
		},
	};
}

function route_record(route_module: RouteModule): RouteRecord {
	return {
		params: {},
		splat_values: [],
		matches: [
			{
				pattern: hmr_pattern,
				input: {},
				module_url: hmr_module_url,
				module: route_module,
				loader_data: { from_server: true },
				client_loader_data: { from_client: "initial" },
			},
		],
		error: null,
		client_build_id,
	};
}

function prepared_route(route_module: RouteModule): PreparedRoute {
	return {
		route: route_record(route_module),
		apply_dom_side_effects: Effect.void,
	};
}

beforeEach(() => {
	vi.restoreAllMocks();
	window.__vorma_hmr_route_update = undefined;
});

describe("ccc Effect module runtime experiment", () => {
	it("caches dynamically loaded modules in dev mode", async () => {
		const loaded_urls: string[] = [];
		const loaded_module = view_module({ component_value: "loaded" });
		const runtime = Effect.runSync(
			make_module_runtime({
				dev: true,
				import_module: async (module_url) => {
					loaded_urls.push(module_url);
					return loaded_module;
				},
			}),
		);

		const first = await run_effect(runtime.load_module(hmr_module_url));
		const second = await run_effect(runtime.load_module(hmr_module_url));

		expect(first).toBe(loaded_module);
		expect(second).toBe(loaded_module);
		expect(loaded_urls).toEqual([hmr_module_url]);
	});

	it("replaces the active route module and reruns opted-in client loaders on HMR", async () => {
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const initial_module = view_module({ component_value: "initial" });
		const updated_module = view_module({
			component_value: "updated",
			client_loader: async ({ serverPromise }) => {
				const server_state = await serverPromise;
				return {
					loaderData: server_state.loaderData,
					ran: true,
				};
			},
		});
		const runtime = Effect.runSync(make_module_runtime({ dev: true }));
		const lifecycle = Effect.runSync(make_runtime_lifecycle());
		const route_preparer = Effect.runSync(
			make_route_preparer({
				client_build_id,
				load_module: () => {
					return Effect.succeed(initial_module);
				},
			}),
		);
		const route_publisher = Effect.runSync(
			make_route_publisher({ commit }),
		);
		const work_actor: WorkStateActor = {
			set_navigation: () => {
				return Effect.void;
			},
			set_revalidation: () => {
				return Effect.void;
			},
			set_prefetch: () => {
				return Effect.void;
			},
			set_api_requests: () => {
				return Effect.void;
			},
			begin_api_request: () => {
				return Effect.void;
			},
			end_api_request: () => {
				return Effect.void;
			},
			emit_current: Effect.void,
			snapshot: Effect.succeed({
				navigation: null,
				revalidation: null,
				prefetch: null,
				apiRequests: [],
			}),
			indicator_activity: Effect.succeed({
				navigation: false,
				revalidation: false,
				apiRequests: false,
			}),
			shutdown: Effect.void,
		};
		const publish_result = await run_effect(
			Effect.either(
				route_publisher.publish({
					reason: "initial",
					prepared: prepared_route(initial_module),
					position: {
						href: hmr_href,
						key: "hmr-key",
						state: { page: "hmr" },
					},
				}),
			),
		);
		expect(publish_result._tag).toBe("Right");
		commit.mockClear();
		await run_effect(runtime.set_hmr_rerun(hmr_pattern, true));
		await run_effect(
			runtime.install_hmr_handler({
				lifecycle,
				route_preparer,
				route_publisher,
				work_actor,
			}),
		);

		await window.__vorma_hmr_route_update?.(hmr_module_url, updated_module);

		const route_render = commit.mock.calls[0]?.[0].route_render;
		expect(route_render?.state.entries[0]?.module).toBe(updated_module);
		expect(route_render?.state.entries[0]?.client_loader_data).toEqual({
			loaderData: { from_server: true },
			ran: true,
		});
		await run_effect(lifecycle.shutdown);
		expect(window.__vorma_hmr_route_update).toBeUndefined();
	});
});
