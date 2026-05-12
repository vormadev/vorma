import { describe, expect, it } from "vitest";
import { create_core6_deferred } from "./deferred.ts";
import {
	core6_route_boot_result_kind,
	create_core6_route_boot_owner,
} from "./route_boot.ts";
import {
	core6_route_payload_field,
	core6_route_preparation_trigger,
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
import { core6_scope_stale_reason } from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

const boot_href = "https://example.test/current?q=1";

function make_payload(
	overrides: Partial<Core6RawRoutePayload> = {},
): Core6RawRoutePayload {
	return {
		[core6_route_payload_field.client_build_id]: "build-1",
		[core6_route_payload_field.import_urls]: ["/current.js"],
		[core6_route_payload_field.loaders_data]: [{ server: "boot" }],
		[core6_route_payload_field.matched_patterns]: ["/current"],
		[core6_route_payload_field.params]: { user: "ada" },
		[core6_route_payload_field.search_schemas]: [{ route: "current" }],
		[core6_route_payload_field.splat_values]: ["tail"],
		...overrides,
	};
}

function make_runtime_host(
	input: {
		commits?: Core6RouteCommit[];
		import_module?: Core6RouteRuntimeHost["import_module"];
	} = {},
): Core6RouteRuntimeHost {
	return {
		commit_publication: (publication) => {
			input.commits?.push(publication.commit);
		},
		fetch_route_payload: async ({ intent }) => {
			return make_payload({
				[core6_route_payload_field.matched_patterns]: [
					new URL(intent.href).pathname,
				],
			});
		},
		fetch_route_response: async () => {
			return new Response(JSON.stringify(make_payload()));
		},
		import_module:
			input.import_module ??
			(async (): Promise<Core6RouteModule> => {
				return {};
			}),
		parse_input: ({ pattern, schema, search_params }) => {
			return {
				pattern,
				query: search_params.get("q"),
				schema,
			};
		},
		route_state_equal: (
			previous_route: Core6RouteState,
			next_route: Core6RouteState,
		) => {
			return (
				JSON.stringify(previous_route) === JSON.stringify(next_route)
			);
		},
	};
}

describe("core6 route boot owner", () => {
	it("publishes the initial payload through the runtime publication boundary", async () => {
		const commits: Core6RouteCommit[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({ commits }),
		);
		const boot = create_core6_route_boot_owner({ runtime });

		const result = await boot.boot({
			history_state: { from: "history" },
			href: boot_href,
			payload: make_payload(),
			scroll: { hash: "#section" },
		});

		expect(result).toMatchObject({
			kind: core6_route_boot_result_kind.booted,
			publication: {
				commit: {
					route_render: {
						scroll: { hash: "#section" },
					},
					route_update: {
						previous_route: null,
						reason: core6_route_publish_reason.boot,
					},
				},
				route: {
					clientBuildID: "build-1",
					historyState: { from: "history" },
					href: boot_href,
					matches: [
						{
							loaderData: { server: "boot" },
							pattern: "/current",
						},
					],
				},
			},
		});
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()).toMatchObject({
			historyState: { from: "history" },
			href: boot_href,
		});
		expect(boot.current_status()).toBeNull();
	});

	it("reports already booted once a current route exists", async () => {
		const runtime = create_core6_route_runtime(make_runtime_host());
		const boot = create_core6_route_boot_owner({ runtime });

		await boot.boot({
			history_state: undefined,
			href: boot_href,
			payload: make_payload(),
		});
		const second = await boot.boot({
			history_state: undefined,
			href: "https://example.test/other",
			payload: make_payload(),
		});

		expect(second).toMatchObject({
			kind: core6_route_boot_result_kind.already_booted,
			route: {
				href: boot_href,
			},
		});
	});

	it("surfaces boot status while payload preparation is active", async () => {
		let resolve_import!: (module: Core6RouteModule) => void;
		const import_wait = create_core6_deferred<Core6RouteModule>();
		resolve_import = import_wait.resolve;
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				import_module: async () => {
					return await import_wait.promise;
				},
			}),
		);
		const boot = create_core6_route_boot_owner({ runtime });

		const result = boot.boot({
			history_state: undefined,
			href: boot_href,
			payload: make_payload(),
		});

		expect(boot.current_status()).toEqual({ href: boot_href });
		resolve_import({});
		await expect(result).resolves.toMatchObject({
			kind: core6_route_boot_result_kind.booted,
		});
		expect(boot.current_status()).toBeNull();
	});

	it("prevents stale boot preparation from publishing after navigation takes ownership", async () => {
		const commits: Core6RouteCommit[] = [];
		let resolve_import!: (module: Core6RouteModule) => void;
		const import_wait = create_core6_deferred<Core6RouteModule>();
		resolve_import = import_wait.resolve;
		const runtime = create_core6_route_runtime(
			make_runtime_host({
				commits,
				import_module: async () => {
					return await import_wait.promise;
				},
			}),
		);
		const boot = create_core6_route_boot_owner({ runtime });

		const first = boot.boot({
			history_state: undefined,
			href: boot_href,
			payload: make_payload(),
		});
		const target = new URL("https://example.test/next");
		const second = await runtime.run_route_payload({
			intent: {
				history_state: { from: "navigation" },
				href: target.href,
				preparation_trigger: core6_route_preparation_trigger.navigation,
				publish_reason: core6_route_publish_reason.navigation,
				search_params: target.searchParams,
			},
			kind: core6_route_transaction_kind.navigation,
			payload: make_payload({
				[core6_route_payload_field.import_urls]: [],
				[core6_route_payload_field.matched_patterns]: ["/next"],
			}),
		});
		resolve_import({});

		expect(second.ok).toBe(true);
		await expect(first).resolves.toEqual({
			kind: core6_route_boot_result_kind.interrupted,
			reason: core6_scope_stale_reason,
		});
		expect(commits).toHaveLength(1);
		expect(runtime.current_route()).toMatchObject({
			href: "https://example.test/next",
		});
	});

	it("rejects invalid boot hrefs without starting route work", async () => {
		const commits: Core6RouteCommit[] = [];
		const runtime = create_core6_route_runtime(
			make_runtime_host({ commits }),
		);
		const boot = create_core6_route_boot_owner({ runtime });

		const result = await boot.boot({
			history_state: undefined,
			href: "http://[",
			payload: make_payload(),
		});

		expect(result).toMatchObject({
			kind: core6_route_boot_result_kind.invalid,
		});
		expect(commits).toEqual([]);
		expect(runtime.current_route()).toBeNull();
	});
});
