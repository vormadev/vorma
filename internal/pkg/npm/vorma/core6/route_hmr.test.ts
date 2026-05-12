import { describe, expect, it } from "vitest";
import { create_core6_deferred } from "./deferred.ts";
import {
	core6_route_hmr_result_kind,
	create_core6_route_hmr_owner,
	type Core6RouteHMRHost,
	type Core6RouteHMROwner,
} from "./route_hmr.ts";
import {
	core6_route_payload_field,
	core6_route_preparation_trigger,
	type Core6ClientLoaderFn,
	type Core6RawRoutePayload,
	type Core6RouteModule,
	type Core6RoutePreparationParseInputArgs,
} from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RouteCommit,
	type Core6RouteState,
} from "./route_publication.ts";
import {
	create_core6_route_runtime,
	type Core6RouteRuntime,
	type Core6RouteRuntimeHost,
} from "./route_runtime.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

const current_href = "https://example.test/root?q=Ada";
const next_href = "https://example.test/next?q=Grace";
const root_pattern = "/root";
const next_pattern = "/next";
const root_module_url = "/root.js";
const next_module_url = "/next.js";

function make_raw_payload(href: string = current_href): Core6RawRoutePayload {
	const target = new URL(href);
	const pattern = target.pathname;
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: [`${pattern}.js`],
		[core6_route_payload_field.loaders_data]: [
			{
				pathname: target.pathname,
				search: target.search,
			},
		],
		[core6_route_payload_field.matched_patterns]: [pattern],
		[core6_route_payload_field.params]: {
			name: target.searchParams.get("q") ?? "",
		},
		[core6_route_payload_field.search_schemas]: [{ pattern }],
		[core6_route_payload_field.splat_values]: [],
	};
}

function same_route_state(
	previous_route: Core6RouteState,
	next_route: Core6RouteState,
): boolean {
	return JSON.stringify(previous_route) === JSON.stringify(next_route);
}

function normalize_module_url(url: string): string {
	return new URL(url, "https://example.test").pathname;
}

function make_intent(href: string = current_href) {
	const target = new URL(href);
	return {
		history_state: { href },
		href: target.href,
		preparation_trigger: core6_route_preparation_trigger.navigation,
		publish_reason: core6_route_publish_reason.navigation,
		search_params: new URLSearchParams(target.search),
	};
}

function make_fixture(
	input: {
		fetch_route_payload?: Core6RouteRuntimeHost["fetch_route_payload"];
		modules?: Record<string, Core6RouteModule>;
	} = {},
): {
	cached_modules: Map<string, Core6RouteModule>;
	commits: Core6RouteCommit[];
	errors: unknown[];
	modules: Record<string, Core6RouteModule>;
	owner: Core6RouteHMROwner;
	runtime: Core6RouteRuntime;
} {
	const cached_modules = new Map<string, Core6RouteModule>();
	const commits: Core6RouteCommit[] = [];
	const errors: unknown[] = [];
	const modules = input.modules ?? {};
	const runtime = create_core6_route_runtime({
		commit_publication: (publication) => {
			commits.push(publication.commit);
		},
		fetch_route_payload:
			input.fetch_route_payload ??
			(async ({ intent }) => {
				return make_raw_payload(intent.href);
			}),
		fetch_route_response: async () => {
			return new Response(JSON.stringify(make_raw_payload()));
		},
		import_module: async (url) => {
			return modules[url] ?? {};
		},
		parse_input: (args: Core6RoutePreparationParseInputArgs) => {
			return {
				pattern: args.pattern,
				query: args.search_params.get("q"),
				schema: args.schema,
			};
		},
		route_state_equal: same_route_state,
	});
	const host: Core6RouteHMRHost = {
		cache_module: (module_url, module) => {
			cached_modules.set(module_url, module);
			modules[module_url] = module;
		},
		normalize_module_url,
		report_client_loader_error: (error) => {
			errors.push(error);
		},
		route_state_equal: same_route_state,
	};
	return {
		cached_modules,
		commits,
		errors,
		modules,
		owner: create_core6_route_hmr_owner({
			host,
			runtime,
		}),
		runtime,
	};
}

async function seed_current_route(
	runtime: Core6RouteRuntime,
	href: string = current_href,
): Promise<void> {
	const result = await runtime.run_route_payload({
		intent: make_intent(href),
		kind: core6_route_transaction_kind.boot,
		payload: make_raw_payload(href),
	});
	if (!result.ok) {
		throw new Error(`expected seed route to publish: ${result.reason}`);
	}
}

describe("core6 route HMR owner", () => {
	it("publishes a matching module update through the route publication boundary", async () => {
		const initial_loader: Core6ClientLoaderFn = async () => {
			return { client: "initial" };
		};
		const next_loader: Core6ClientLoaderFn = async () => {
			return { client: "next" };
		};
		const next_module: Core6RouteModule = {
			default: {
				client_loader: next_loader,
			},
		};
		const fixture = make_fixture({
			modules: {
				[root_module_url]: {
					default: {
						client_loader: initial_loader,
					},
				},
			},
		});
		await seed_current_route(fixture.runtime);

		const result = await fixture.owner.update({
			module: next_module,
			raw_module_url: `${root_module_url}?t=1`,
		});

		expect(result).toMatchObject({
			kind: core6_route_hmr_result_kind.published,
		});
		expect(fixture.cached_modules.get(root_module_url)).toBe(next_module);
		expect(fixture.commits).toHaveLength(2);
		expect(fixture.commits[1]?.route_update).toBeUndefined();
		expect(fixture.runtime.client_loader(root_pattern)).toBe(next_loader);
		expect(fixture.runtime.current_route()).toMatchObject({
			matches: [
				{
					clientLoaderData: { client: "initial" },
					pattern: root_pattern,
				},
			],
		});
		expect(fixture.runtime.current_render_state()?.entries[0]?.module).toBe(
			next_module,
		);
		expect(fixture.owner.current_status()).toBeNull();
	});

	it("reruns configured client loaders with revalidation inputs", async () => {
		const calls: Array<{
			href: string;
			known_patterns: string[];
			loader_data: unknown;
			signal_aborted: boolean;
			trigger: string;
		}> = [];
		const next_loader: Core6ClientLoaderFn = async (args) => {
			const server_state = await args.serverPromise;
			calls.push({
				href: args.href,
				known_patterns: args.knownMatches.map((match) => {
					return match.pattern;
				}),
				loader_data: server_state.loaderData,
				signal_aborted: args.signal.aborted,
				trigger: args.trigger,
			});
			return { client: "rerun" };
		};
		const fixture = make_fixture({
			modules: {
				[root_module_url]: {},
			},
		});
		await seed_current_route(fixture.runtime);
		fixture.owner.configure_view({
			pattern: root_pattern,
			run_client_loader: true,
		});

		const result = await fixture.owner.update({
			module: {
				default: {
					client_loader: next_loader,
				},
			},
			raw_module_url: root_module_url,
		});

		expect(result).toMatchObject({
			kind: core6_route_hmr_result_kind.published,
		});
		expect(calls).toEqual([
			{
				href: current_href,
				known_patterns: [root_pattern],
				loader_data: {
					pathname: root_pattern,
					search: "?q=Ada",
				},
				signal_aborted: false,
				trigger: core6_route_preparation_trigger.revalidation,
			},
		]);
		expect(fixture.commits[1]?.route_update).toMatchObject({
			reason: core6_route_publish_reason.revalidation,
			route: {
				matches: [
					{
						clientLoaderData: { client: "rerun" },
						pattern: root_pattern,
					},
				],
			},
		});
		expect(fixture.runtime.current_route()).toMatchObject({
			matches: [
				{
					clientLoaderData: { client: "rerun" },
					pattern: root_pattern,
				},
			],
		});
	});

	it("keeps late loader reruns from publishing after the route changes", async () => {
		const loader_started = create_core6_deferred<void>();
		const loader_release = create_core6_deferred<void>();
		const next_loader: Core6ClientLoaderFn = async () => {
			loader_started.resolve(undefined);
			await loader_release.promise;
			return { client: "late" };
		};
		const fixture = make_fixture({
			modules: {
				[next_module_url]: {},
				[root_module_url]: {},
			},
		});
		await seed_current_route(fixture.runtime);
		fixture.owner.configure_view({
			pattern: root_pattern,
			run_client_loader: true,
		});

		const update = fixture.owner.update({
			module: {
				default: {
					client_loader: next_loader,
				},
			},
			raw_module_url: root_module_url,
		});
		await loader_started.promise;
		expect(fixture.owner.current_status()).toEqual({
			module_url: root_module_url,
			pattern: root_pattern,
			run_client_loader: true,
		});
		await seed_current_route(fixture.runtime, next_href);
		loader_release.resolve(undefined);

		await expect(update).resolves.toEqual({
			kind: core6_route_hmr_result_kind.interrupted,
			reason: core6_scope_stale_reason,
		});
		expect(fixture.commits).toHaveLength(2);
		expect(fixture.runtime.current_route()).toMatchObject({
			href: next_href,
			matches: [{ pattern: next_pattern }],
		});
		expect(fixture.owner.current_status()).toBeNull();
	});

	it("stales an earlier HMR update when a newer update starts", async () => {
		const first_loader_started = create_core6_deferred<void>();
		const first_loader_release = create_core6_deferred<void>();
		const first_loader: Core6ClientLoaderFn = async () => {
			first_loader_started.resolve(undefined);
			await first_loader_release.promise;
			return { client: "first" };
		};
		const second_loader: Core6ClientLoaderFn = async () => {
			return { client: "second" };
		};
		const fixture = make_fixture({
			modules: {
				[root_module_url]: {},
			},
		});
		await seed_current_route(fixture.runtime);
		fixture.owner.configure_view({
			pattern: root_pattern,
			run_client_loader: true,
		});

		const first = fixture.owner.update({
			module: {
				default: {
					client_loader: first_loader,
				},
			},
			raw_module_url: root_module_url,
		});
		await first_loader_started.promise;
		const second = await fixture.owner.update({
			module: {
				default: {
					client_loader: second_loader,
				},
			},
			raw_module_url: root_module_url,
		});
		first_loader_release.resolve(undefined);

		expect(second).toMatchObject({
			kind: core6_route_hmr_result_kind.published,
		});
		await expect(first).resolves.toEqual({
			kind: core6_route_hmr_result_kind.interrupted,
			reason: core6_scope_stale_reason,
		});
		expect(fixture.commits).toHaveLength(2);
		expect(fixture.runtime.current_route()).toMatchObject({
			matches: [
				{
					clientLoaderData: { client: "second" },
					pattern: root_pattern,
				},
			],
		});
	});

	it("does not start HMR publication while route transactions are active", async () => {
		const fetch_started = create_core6_deferred<void>();
		const fetch_release = create_core6_deferred<Core6RawRoutePayload>();
		const fixture = make_fixture({
			fetch_route_payload: async ({ intent }) => {
				if (intent.href === next_href) {
					fetch_started.resolve(undefined);
					return await fetch_release.promise;
				}
				return make_raw_payload(intent.href);
			},
			modules: {
				[next_module_url]: {},
				[root_module_url]: {},
			},
		});
		await seed_current_route(fixture.runtime);
		const route_run = fixture.runtime.run_route({
			intent: make_intent(next_href),
			kind: core6_route_transaction_kind.navigation,
		});
		await fetch_started.promise;

		const result = await fixture.owner.update({
			module: {},
			raw_module_url: root_module_url,
		});
		fetch_release.resolve(make_raw_payload(next_href));

		expect(result).toEqual({ kind: core6_route_hmr_result_kind.busy });
		await expect(route_run).resolves.toMatchObject({ ok: true });
		expect(fixture.commits).toHaveLength(2);
		expect(fixture.runtime.current_route()).toMatchObject({
			href: next_href,
		});
	});

	it("ignores module updates that do not match the current render tree", async () => {
		const fixture = make_fixture({
			modules: {
				[root_module_url]: {},
			},
		});
		await seed_current_route(fixture.runtime);

		const result = await fixture.owner.update({
			module: {},
			raw_module_url: "/other.js",
		});

		expect(result).toEqual({ kind: core6_route_hmr_result_kind.ignored });
		expect(fixture.cached_modules.has("/other.js")).toBe(true);
		expect(fixture.commits).toHaveLength(1);
	});
});
