import { describe, expect, it } from "vitest";
import {
	core6_abort_error_name,
	core6_route_error_source,
	core6_route_payload_field,
	core6_route_preparation_trigger,
	decode_core6_route_payload,
	prepare_core6_route,
	type Core6ClientLoaderFn,
	type Core6ClientLoaderServerState,
	type Core6DecodedRoutePayload,
	type Core6RawRoutePayload,
	type Core6RouteModule,
	type Core6RoutePreparationHost,
	type Core6RoutePreparationTrigger,
} from "./route_preparation.ts";

type ParseCall = {
	pattern: string;
	query: string | null;
	schema: unknown;
};

function make_raw_payload(
	overrides: Partial<Core6RawRoutePayload> = {},
): Core6RawRoutePayload {
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: ["/root.js"],
		[core6_route_payload_field.loaders_data]: [{ server: "root" }],
		[core6_route_payload_field.matched_patterns]: ["/root"],
		[core6_route_payload_field.params]: {},
		[core6_route_payload_field.search_schemas]: [null],
		[core6_route_payload_field.splat_values]: [],
		...overrides,
	};
}

function decode_test_payload(
	raw_payload: Core6RawRoutePayload,
	parse_calls: ParseCall[] = [],
): Core6DecodedRoutePayload {
	return decode_core6_route_payload({
		host: {
			parse_input: ({ pattern, schema, search_params }) => {
				const query = search_params.get("q");
				parse_calls.push({ pattern, query, schema });
				return { pattern, query, schema };
			},
		},
		raw_payload,
		search_params: new URLSearchParams("q=Ada"),
	});
}

function make_prepare_host(input: {
	import_module: (
		url: string,
		signal: AbortSignal,
	) => Promise<Core6RouteModule>;
}): Core6RoutePreparationHost {
	return {
		import_module: input.import_module,
		parse_input: ({ pattern }) => {
			return { pattern };
		},
	};
}

async function prepare_test_route(input: {
	host: Core6RoutePreparationHost;
	payload: Core6DecodedRoutePayload;
	signal?: AbortSignal;
	trigger?: Core6RoutePreparationTrigger;
}): Promise<Awaited<ReturnType<typeof prepare_core6_route>>> {
	return await prepare_core6_route({
		history_state: { from: "test" },
		host: input.host,
		href: "https://example.test/root?q=Ada",
		payload: input.payload,
		signal: input.signal ?? new AbortController().signal,
		trigger: input.trigger ?? core6_route_preparation_trigger.navigation,
	});
}

describe("core6 route preparation", () => {
	it("decodes route payload data without DOM or adapter state", () => {
		const parse_calls: ParseCall[] = [];
		const server_error = new Error("server failed");
		const decoded = decode_test_payload(
			make_raw_payload({
				[core6_route_payload_field.client_build_id]: "build-42",
				[core6_route_payload_field.css_bundles]: ["/style.css"],
				[core6_route_payload_field.deps]: ["/dep.js"],
				[core6_route_payload_field.import_urls]: [
					"/root.js",
					"/child.js",
				],
				[core6_route_payload_field.loaders_data]: [
					{ root: true },
					{ child: true },
				],
				[core6_route_payload_field.matched_patterns]: [
					"/root",
					"/root/child",
				],
				[core6_route_payload_field.meta_head_els]: [{ tag: "meta" }],
				[core6_route_payload_field.outermost_server_error]:
					server_error,
				[core6_route_payload_field.outermost_server_error_idx]: 1,
				[core6_route_payload_field.params]: { user: "ada" },
				[core6_route_payload_field.rest_head_els]: [{ tag: "link" }],
				[core6_route_payload_field.search_schemas]: [
					{ route: "root" },
					{ route: "child" },
				],
				[core6_route_payload_field.splat_values]: ["tail"],
				[core6_route_payload_field.title]: {
					dangerousInnerHTML: "Profile",
				},
			}),
			parse_calls,
		);

		expect(parse_calls).toEqual([
			{ pattern: "/root", query: "Ada", schema: { route: "root" } },
			{
				pattern: "/root/child",
				query: "Ada",
				schema: { route: "child" },
			},
		]);
		expect(decoded).toEqual({
			client_build_id: "build-42",
			css_bundles: ["/style.css"],
			deps: ["/dep.js"],
			meta_head_els: [{ tag: "meta" }],
			params: { user: "ada" },
			rest_head_els: [{ tag: "link" }],
			routes: [
				{
					input: {
						pattern: "/root",
						query: "Ada",
						schema: { route: "root" },
					},
					loader_data: { root: true },
					module_url: "/root.js",
					pattern: "/root",
				},
				{
					input: {
						pattern: "/root/child",
						query: "Ada",
						schema: { route: "child" },
					},
					loader_data: { child: true },
					module_url: "/child.js",
					pattern: "/root/child",
				},
			],
			server_error: {
				error: server_error,
				idx: 1,
			},
			splat_values: ["tail"],
			title_html: "Profile",
		});
	});

	it("imports each route module once and returns discovered client loaders as prepared data", async () => {
		const import_calls: string[] = [];
		const shared_loader: Core6ClientLoaderFn = async ({ pattern }) => {
			return { pattern };
		};
		const shared_module = {
			default: {
				client_loader: shared_loader,
			},
			name: "shared",
		} satisfies Core6RouteModule;
		const payload = decode_test_payload(
			make_raw_payload({
				[core6_route_payload_field.import_urls]: [
					"/shared.js",
					"/shared.js",
				],
				[core6_route_payload_field.loaders_data]: [
					{ root: true },
					{ child: true },
				],
				[core6_route_payload_field.matched_patterns]: [
					"/root",
					"/root/child",
				],
			}),
		);

		const prepared = await prepare_test_route({
			host: make_prepare_host({
				import_module: async (url) => {
					import_calls.push(url);
					return shared_module;
				},
			}),
			payload,
		});

		expect(import_calls).toEqual(["/shared.js"]);
		expect(prepared.client_loaders).toEqual([
			{ loader: shared_loader, pattern: "/root" },
			{ loader: shared_loader, pattern: "/root/child" },
		]);
		expect(prepared.matches).toMatchObject([
			{
				client_loader_data: { pattern: "/root" },
				loader_data: { root: true },
				module: shared_module,
				module_url: "/shared.js",
				pattern: "/root",
			},
			{
				client_loader_data: { pattern: "/root/child" },
				loader_data: { child: true },
				module: shared_module,
				module_url: "/shared.js",
				pattern: "/root/child",
			},
		]);
	});

	it("passes stable route context and server state into client loaders", async () => {
		const loader_observations: Array<{
			history_state: unknown;
			href: string;
			input: unknown;
			known_matches: unknown;
			params: Record<string, string>;
			pattern: string;
			server_state: Core6ClientLoaderServerState;
			signal_aborted: boolean;
			splat_values: string[];
			trigger: Core6RoutePreparationTrigger;
		}> = [];
		const make_loader = (): Core6ClientLoaderFn => {
			return async ({
				historyState,
				href,
				input,
				knownMatches,
				params,
				pattern,
				serverPromise,
				signal,
				splatValues,
				trigger,
			}) => {
				const server_state = await serverPromise;
				loader_observations.push({
					history_state: historyState,
					href,
					input,
					known_matches: knownMatches,
					params,
					pattern,
					server_state,
					signal_aborted: signal.aborted,
					splat_values: splatValues,
					trigger,
				});
				return {
					loader_data: server_state.loaderData,
					pattern,
				};
			};
		};
		const modules = new Map<string, Core6RouteModule>([
			["/root.js", { default: { client_loader: make_loader() } }],
			["/child.js", { default: { client_loader: make_loader() } }],
		]);
		const payload = decode_test_payload(
			make_raw_payload({
				[core6_route_payload_field.client_build_id]: "build-2",
				[core6_route_payload_field.import_urls]: [
					"/root.js",
					"/child.js",
				],
				[core6_route_payload_field.loaders_data]: [
					{ root_server: true },
					{ child_server: true },
				],
				[core6_route_payload_field.matched_patterns]: [
					"/root",
					"/root/child",
				],
				[core6_route_payload_field.params]: { account: "a1" },
				[core6_route_payload_field.splat_values]: ["rest"],
			}),
		);

		const prepared = await prepare_test_route({
			host: make_prepare_host({
				import_module: async (url) => {
					return modules.get(url)!;
				},
			}),
			payload,
			trigger: core6_route_preparation_trigger.revalidation,
		});

		expect(loader_observations).toEqual([
			{
				history_state: { from: "test" },
				href: "https://example.test/root?q=Ada",
				input: {
					pattern: "/root",
					query: "Ada",
					schema: null,
				},
				known_matches: [
					{
						input: {
							pattern: "/root",
							query: "Ada",
							schema: null,
						},
						pattern: "/root",
					},
					{
						input: {
							pattern: "/root/child",
							query: "Ada",
							schema: undefined,
						},
						pattern: "/root/child",
					},
				],
				params: { account: "a1" },
				pattern: "/root",
				server_state: {
					clientBuildID: "build-2",
					loaderData: { root_server: true },
					matches: [
						{
							input: {
								pattern: "/root",
								query: "Ada",
								schema: null,
							},
							loaderData: { root_server: true },
							pattern: "/root",
						},
						{
							input: {
								pattern: "/root/child",
								query: "Ada",
								schema: undefined,
							},
							loaderData: { child_server: true },
							pattern: "/root/child",
						},
					],
					outermostServerError: null,
				},
				signal_aborted: false,
				splat_values: ["rest"],
				trigger: core6_route_preparation_trigger.revalidation,
			},
			{
				history_state: { from: "test" },
				href: "https://example.test/root?q=Ada",
				input: {
					pattern: "/root/child",
					query: "Ada",
					schema: undefined,
				},
				known_matches: [
					{
						input: {
							pattern: "/root",
							query: "Ada",
							schema: null,
						},
						pattern: "/root",
					},
					{
						input: {
							pattern: "/root/child",
							query: "Ada",
							schema: undefined,
						},
						pattern: "/root/child",
					},
				],
				params: { account: "a1" },
				pattern: "/root/child",
				server_state: {
					clientBuildID: "build-2",
					loaderData: { child_server: true },
					matches: [
						{
							input: {
								pattern: "/root",
								query: "Ada",
								schema: null,
							},
							loaderData: { root_server: true },
							pattern: "/root",
						},
						{
							input: {
								pattern: "/root/child",
								query: "Ada",
								schema: undefined,
							},
							loaderData: { child_server: true },
							pattern: "/root/child",
						},
					],
					outermostServerError: null,
				},
				signal_aborted: false,
				splat_values: ["rest"],
				trigger: core6_route_preparation_trigger.revalidation,
			},
		]);
		expect(prepared.matches).toMatchObject([
			{
				client_loader_data: {
					loader_data: { root_server: true },
					pattern: "/root",
				},
			},
			{
				client_loader_data: {
					loader_data: { child_server: true },
					pattern: "/root/child",
				},
			},
		]);
	});

	it("skips client loaders at and below the outermost server error", async () => {
		const calls: string[] = [];
		const make_loader = (name: string): Core6ClientLoaderFn => {
			return async () => {
				calls.push(name);
				return { name };
			};
		};
		const modules = new Map<string, Core6RouteModule>([
			["/root.js", { default: { client_loader: make_loader("root") } }],
			["/child.js", { default: { client_loader: make_loader("child") } }],
			["/leaf.js", { default: { client_loader: make_loader("leaf") } }],
		]);
		const payload = decode_test_payload(
			make_raw_payload({
				[core6_route_payload_field.import_urls]: [
					"/root.js",
					"/child.js",
					"/leaf.js",
				],
				[core6_route_payload_field.matched_patterns]: [
					"/root",
					"/root/child",
					"/root/child/leaf",
				],
				[core6_route_payload_field.outermost_server_error]:
					"child boom",
				[core6_route_payload_field.outermost_server_error_idx]: 1,
			}),
		);

		const prepared = await prepare_test_route({
			host: make_prepare_host({
				import_module: async (url) => {
					return modules.get(url)!;
				},
			}),
			payload,
		});

		expect(calls).toEqual(["root"]);
		expect(prepared.error).toEqual({
			error: "child boom",
			idx: 1,
			source: core6_route_error_source.server,
		});
		expect(
			prepared.matches.map((match) => match.client_loader_data),
		).toEqual([{ name: "root" }, undefined, undefined]);
	});

	it("turns client loader failure into a route error and hides later loader data", async () => {
		const root_error = new Error("root loader failed");
		const child_signals: AbortSignal[] = [];
		const root_loader: Core6ClientLoaderFn = async () => {
			throw root_error;
		};
		const child_loader: Core6ClientLoaderFn = async ({ signal }) => {
			child_signals.push(signal);
			return await new Promise((resolve) => {
				if (signal.aborted) {
					resolve({ child: "ignored" });
					return;
				}
				signal.addEventListener(
					"abort",
					() => {
						resolve({ child: "ignored" });
					},
					{ once: true },
				);
			});
		};
		const modules = new Map<string, Core6RouteModule>([
			["/root.js", { default: { client_loader: root_loader } }],
			["/child.js", { default: { client_loader: child_loader } }],
		]);
		const payload = decode_test_payload(
			make_raw_payload({
				[core6_route_payload_field.import_urls]: [
					"/root.js",
					"/child.js",
				],
				[core6_route_payload_field.matched_patterns]: [
					"/root",
					"/root/child",
				],
			}),
		);

		const prepared = await prepare_test_route({
			host: make_prepare_host({
				import_module: async (url) => {
					return modules.get(url)!;
				},
			}),
			payload,
		});

		expect(child_signals[0]?.aborted).toBe(true);
		expect(prepared.error).toEqual({
			error: root_error,
			idx: 0,
			source: core6_route_error_source.client_loader,
		});
		expect(
			prepared.matches.map((match) => match.client_loader_data),
		).toEqual([undefined, undefined]);
	});

	it("does not run client loaders after preparation is aborted", async () => {
		const controller = new AbortController();
		let loader_ran = false;
		const payload = decode_test_payload(make_raw_payload());

		await expect(
			prepare_test_route({
				host: make_prepare_host({
					import_module: async () => {
						controller.abort();
						return {
							default: {
								client_loader: async () => {
									loader_ran = true;
									return { ok: true };
								},
							},
						};
					},
				}),
				payload,
				signal: controller.signal,
			}),
		).rejects.toMatchObject({ name: core6_abort_error_name });

		expect(loader_ran).toBe(false);
	});
});
