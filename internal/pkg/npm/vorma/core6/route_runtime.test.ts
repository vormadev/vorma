import { describe, expect, it } from "vitest";
import { VORMA_JSON_KEY, X_CLIENT_REDIRECT } from "../core/constants.ts";
import { core6_route_fetch_driver_result_kind } from "./route_fetch_driver.ts";
import {
	core6_route_payload_field,
	core6_route_preparation_trigger,
	type Core6ClientLoaderFn,
	type Core6PreparedRoute,
	type Core6RawRoutePayload,
	type Core6RouteModule,
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
import {
	core6_scope_cancelled_reason,
	core6_scope_stale_reason,
} from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

function make_intent(
	overrides: Partial<Core6RouteTransactionFlowIntent> = {},
): Core6RouteTransactionFlowIntent {
	return {
		history_state: { from: "runtime" },
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

function make_runtime_host(input: {
	commits?: Core6RouteCommit[];
	fetch_route_payload?: Core6RouteRuntimeHost["fetch_route_payload"];
	fetch_route_response?: Core6RouteRuntimeHost["fetch_route_response"];
	import_module?: Core6RouteRuntimeHost["import_module"];
	parse_input?: Core6RouteRuntimeHost["parse_input"];
	route_state_equal?: Core6RouteRuntimeHost["route_state_equal"];
}): Core6RouteRuntimeHost {
	return {
		commit_publication: (publication) => {
			input.commits?.push(publication.commit);
		},
		fetch_route_payload:
			input.fetch_route_payload ??
			(async () => {
				return make_raw_payload();
			}),
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
		parse_input:
			input.parse_input ??
			(({ pattern, schema, search_params }) => {
				return {
					pattern,
					query: search_params.get("q"),
					schema,
				};
			}),
		route_state_equal: input.route_state_equal ?? same_route_state,
	};
}

describe("core6 route runtime", () => {
	it("composes transaction flow with publication store state", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		const client_loader: Core6ClientLoaderFn = async ({
			pattern,
			serverPromise,
		}) => {
			calls.push(`client:${pattern}`);
			const server_state = await serverPromise;
			return { server: server_state.loaderData };
		};
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
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
				parse_input: ({ pattern, search_params }) => {
					calls.push(`parse:${pattern}`);
					return {
						pattern,
						query: search_params.get("q"),
					};
				},
			}),
		);

		const result = await runtime.run_route({
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/root?q=Ada:false",
			"parse:/root",
			"import:/root.js:false",
			"client:/root",
		]);
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()).toMatchObject({
			clientBuildID: "build-1",
			href: "https://example.test/root?q=Ada",
			matches: [
				{
					clientLoaderData: { server: { server: "root" } },
					loaderData: { server: "root" },
					pattern: "/root",
				},
			],
		});
		expect(runtime.client_loader("/root")).toBe(client_loader);
		expect(runtime.client_loader_patterns()).toEqual(["/root"]);
		if (result.ok) {
			expect(result.value.commit).toBe(commits[0]);
		}
	});

	it("runs fetch-aware route work through the runtime store", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		const client_loader: Core6ClientLoaderFn = async ({
			pattern,
			serverPromise,
		}) => {
			calls.push(`client:${pattern}`);
			const server_state = await serverPromise;
			return { server: server_state.loaderData };
		};
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href, init }) => {
					const request = new URL(href);
					calls.push(
						`fetch:${request.searchParams.get(VORMA_JSON_KEY)}:${init.signal.aborted}`,
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

		const result = await runtime.run_route_fetch({
			active_client_build_id: "runtime-build",
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:runtime-build:false",
			"parse:/root",
			"import:/root.js:false",
			"client:/root",
		]);
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()).toMatchObject({
			clientBuildID: "build-1",
			href: "https://example.test/root?q=Ada",
			matches: [
				{
					clientLoaderData: { server: { server: "root" } },
					loaderData: { server: "root" },
					pattern: "/root",
				},
			],
		});
		expect(runtime.client_loader("/root")).toBe(client_loader);
		if (!result.ok) {
			throw new Error("expected published result");
		}
		expect(result.value).toMatchObject({
			kind: core6_route_fetch_driver_result_kind.published,
			publication: {
				commit: commits[0],
			},
			response: {
				requested_href: "https://example.test/root?q=Ada",
				status: 200,
			},
		});
	});

	it("returns fetch redirects without changing publication state", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					calls.push(
						`fetch:${request.searchParams.get(VORMA_JSON_KEY)}`,
					);
					return new Response("redirect", {
						headers: {
							[X_CLIENT_REDIRECT]: "/login",
						},
					});
				},
			}),
		);

		const result = await runtime.run_route_fetch({
			active_client_build_id: "runtime-build",
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});

		expect(result.ok).toBe(true);
		expect(calls).toEqual(["fetch:runtime-build"]);
		expect(commits).toEqual([]);
		expect(runtime.current_route()).toBeNull();
		if (!result.ok) {
			throw new Error("expected redirect result");
		}
		expect(result.value).toMatchObject({
			kind: core6_route_fetch_driver_result_kind.redirect,
			redirect: {
				hard: false,
				href: "https://example.test/login",
			},
		});
	});

	it("publishes prepared routes through the runtime publication boundary", () => {
		const commits: Core6RouteCommit[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({ commits }),
		);
		const prepared: Core6PreparedRoute = {
			client_build_id: "build-1",
			client_loaders: [],
			css_bundles: [],
			deps: [],
			error: null,
			history_state: { from: "prepared" },
			href: "https://example.test/prepared",
			matches: [
				{
					client_loader_data: undefined,
					input: { route: "prepared" },
					loader_data: { prefetched: true },
					module: {},
					module_url: "/prepared.js",
					pattern: "/prepared",
				},
			],
			meta_head_els: [],
			params: { route: "prepared" },
			rest_head_els: [],
			splat_values: [],
			title_html: null,
		};

		const result = runtime.run_route_prepared({
			intent: make_intent({
				href: "https://example.test/prepared",
				search_params: new URLSearchParams(),
			}),
			kind: core6_route_transaction_kind.navigation,
			prepared,
		});

		expect(result.ok).toBe(true);
		expect(commits).toHaveLength(1);
		expect(runtime.current_transaction_kind()).toBeNull();
		expect(runtime.current_route()).toMatchObject({
			href: "https://example.test/prepared",
			matches: [
				{
					loaderData: { prefetched: true },
					pattern: "/prepared",
				},
			],
		});
		if (result.ok) {
			expect(result.value.commit).toBe(commits[0]);
		}
	});

	it("publishes same-document route commits through the runtime publication boundary", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_payload: async () => {
					calls.push("fetch");
					return make_raw_payload();
				},
				import_module: async (url) => {
					calls.push(`import:${url}`);
					return {};
				},
			}),
		);

		await runtime.run_route({
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});
		calls.length = 0;
		const publication = runtime.publish_same_document({
			history_state: { popped: true },
			href: "https://example.test/root?q=Ada#section",
			reason: core6_route_publish_reason.popstate,
			scroll: { hash: "#section" },
		});

		expect(publication).toMatchObject({
			client_loaders: [],
			commit: {
				route_render: {
					scroll: { hash: "#section" },
					state: {
						history_state: { popped: true },
					},
				},
				route_update: {
					reason: core6_route_publish_reason.popstate,
				},
			},
			route: {
				historyState: { popped: true },
				href: "https://example.test/root?q=Ada#section",
			},
		});
		expect(calls).toEqual([]);
		expect(commits).toHaveLength(2);
		expect(commits[1]).toBe(publication?.commit);
		expect(runtime.current_route()).toMatchObject({
			historyState: { popped: true },
			href: "https://example.test/root?q=Ada#section",
		});
		expect(runtime.client_loader_patterns()).toEqual([]);
	});

	it("returns null for same-document publication before the first route publication", () => {
		const runtime = create_core6_route_runtime(make_runtime_host({}));

		const publication = runtime.publish_same_document({
			history_state: { popped: true },
			href: "https://example.test/root#section",
			reason: core6_route_publish_reason.popstate,
			scroll: { hash: "#section" },
		});

		expect(publication).toBeNull();
		expect(runtime.current_route()).toBeNull();
	});

	it("uses one transaction manager across route runs", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		let resolve_first_fetch!: (payload: Core6RawRoutePayload) => void;
		let mark_first_fetch_started!: () => void;
		const first_fetch_started = new Promise<void>((resolve) => {
			mark_first_fetch_started = resolve;
		});
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_payload: async ({ intent }) => {
					calls.push(`fetch:${intent.href}`);
					if (intent.href.endsWith("/first")) {
						mark_first_fetch_started();
						return await new Promise<Core6RawRoutePayload>(
							(resolve) => {
								resolve_first_fetch = resolve;
							},
						);
					}
					return make_raw_payload({
						[core6_route_payload_field.params]: { user: "second" },
					});
				},
			}),
		);

		const first = runtime.run_route({
			intent: make_intent({
				href: "https://example.test/first",
				search_params: new URLSearchParams(),
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		await first_fetch_started;
		const second = await runtime.run_route({
			intent: make_intent({
				href: "https://example.test/second",
				search_params: new URLSearchParams(),
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_first_fetch(make_raw_payload());

		await expect(first).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(second.ok).toBe(true);
		expect(calls).toEqual([
			"fetch:https://example.test/first",
			"fetch:https://example.test/second",
		]);
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()?.href).toBe(
			"https://example.test/second",
		);
		expect(runtime.current_route()?.params).toEqual({ user: "second" });
	});

	it("reports the current transaction kind while route work is active", async () => {
		let resolve_fetch!: (payload: Core6RawRoutePayload) => void;
		let mark_fetch_started!: () => void;
		const fetch_started = new Promise<void>((resolve) => {
			mark_fetch_started = resolve;
		});
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				fetch_route_payload: async () => {
					mark_fetch_started();
					return await new Promise<Core6RawRoutePayload>(
						(resolve) => {
							resolve_fetch = resolve;
						},
					);
				},
			}),
		);

		const result = runtime.run_route({
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});
		await fetch_started;

		expect(runtime.current_transaction_kind()).toBe(
			core6_route_transaction_kind.navigation,
		);
		resolve_fetch(make_raw_payload());
		await result;
		expect(runtime.current_transaction_kind()).toBeNull();
	});

	it("uses one transaction manager across fetch-aware and payload route runs", async () => {
		const commits: Core6RouteCommit[] = [];
		const calls: string[] = [];
		let resolve_first_response!: (response: Response) => void;
		let mark_first_fetch_started!: () => void;
		const first_fetch_started = new Promise<void>((resolve) => {
			mark_first_fetch_started = resolve;
		});
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_payload: async ({ intent }) => {
					calls.push(`payload:${intent.href}`);
					return make_raw_payload({
						[core6_route_payload_field.params]: {
							user: "payload",
						},
					});
				},
				fetch_route_response: async ({ href }) => {
					const request = new URL(href);
					calls.push(
						`response:${request.pathname}:${request.searchParams.get(VORMA_JSON_KEY)}`,
					);
					mark_first_fetch_started();
					return await new Promise<Response>((resolve) => {
						resolve_first_response = resolve;
					});
				},
			}),
		);

		const first = runtime.run_route_fetch({
			active_client_build_id: "runtime-build",
			intent: make_intent({
				href: "https://example.test/first",
				search_params: new URLSearchParams(),
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		await first_fetch_started;
		const second = await runtime.run_route({
			intent: make_intent({
				href: "https://example.test/payload",
				search_params: new URLSearchParams(),
			}),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_first_response(
			new Response(JSON.stringify(make_raw_payload())),
		);

		await expect(first).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(second.ok).toBe(true);
		expect(calls).toEqual([
			"response:/first:runtime-build",
			"payload:https://example.test/payload",
		]);
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()?.href).toBe(
			"https://example.test/payload",
		);
		expect(runtime.current_route()?.params).toEqual({ user: "payload" });
	});

	it("cancels the current route run through the shared transaction manager", async () => {
		const commits: Core6RouteCommit[] = [];
		let resolve_fetch!: (payload: Core6RawRoutePayload) => void;
		let captured_signal!: AbortSignal;
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				fetch_route_payload: async ({ signal }) => {
					captured_signal = signal;
					return await new Promise<Core6RawRoutePayload>(
						(resolve) => {
							resolve_fetch = resolve;
						},
					);
				},
			}),
		);

		const result = runtime.run_route({
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});

		expect(runtime.cancel_current()).toBe(true);
		expect(captured_signal.aborted).toBe(true);
		resolve_fetch(make_raw_payload());
		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_cancelled_reason,
		});
		expect(commits).toEqual([]);
		expect(runtime.current_route()).toBeNull();
		expect(runtime.cancel_current()).toBe(false);
	});

	it("keeps route snapshots detached at the runtime boundary", async () => {
		const runtime = create_core6_route_runtime(make_runtime_host({}));

		await runtime.run_route({
			intent: make_intent(),
			kind: core6_route_transaction_kind.navigation,
		});
		const current_route = runtime.current_route();
		if (!current_route) {
			throw new Error("expected current route");
		}
		current_route.params.user = "mutated";
		current_route.splatValues.push("mutated");
		current_route.matches[0] = {
			clientLoaderData: "mutated",
			input: "mutated",
			loaderData: "mutated",
			pattern: "/mutated",
		};

		expect(runtime.current_route()).toMatchObject({
			matches: [{ pattern: "/root" }],
			params: { user: "ada" },
			splatValues: ["tail"],
		});
	});

	it("bubbles commit emission failures after publication state advances", async () => {
		const error = new Error("emit failed");
		const client_loader: Core6ClientLoaderFn = async () => {
			return { ok: true };
		};
		const runtime = create_core6_route_runtime({
			commit_publication: () => {
				throw error;
			},
			fetch_route_payload: async () => {
				return make_raw_payload();
			},
			fetch_route_response: async () => {
				return new Response(JSON.stringify(make_raw_payload()));
			},
			import_module: async (): Promise<Core6RouteModule> => {
				return {
					default: {
						client_loader,
					},
				};
			},
			parse_input: ({ pattern }) => {
				return { pattern };
			},
			route_state_equal: same_route_state,
		});

		await expect(
			runtime.run_route({
				intent: make_intent(),
				kind: core6_route_transaction_kind.navigation,
			}),
		).rejects.toBe(error);
		expect(runtime.current_route()).toMatchObject({
			href: "https://example.test/root?q=Ada",
		});
		expect(runtime.client_loader("/root")).toBe(client_loader);
	});
});
