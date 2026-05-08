import { Effect, Fiber } from "effect";
import { TestClock } from "effect/testing";
import { describe, expect, it } from "vitest";
import type { ClientLoaderFn } from "./effect_runtime/client_contract.ts";
import {
	ROUTE_PAYLOAD_FIELDS,
	make_route_preparer,
	type RouteModule,
} from "./effect_runtime/route_preparer.ts";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program.pipe(Effect.provide(TestClock.layer())));
}

function drain(): Effect.Effect<void> {
	return Effect.gen(function* () {
		for (let i = 0; i < 10; i++) {
			yield* Effect.yieldNow;
		}
	});
}

function route_payload(input: {
	patterns: string[];
	import_urls: string[];
	loaders_data?: unknown[];
	server_error_idx?: number | null;
	server_error?: unknown;
	params?: Record<string, string>;
	splat_values?: string[];
	css_bundles?: string[];
	deps?: string[];
}): Record<string, unknown> {
	const fields = ROUTE_PAYLOAD_FIELDS;
	return {
		[fields.matched_patterns]: input.patterns,
		[fields.search_schemas]: input.patterns.map(() => {
			return undefined;
		}),
		[fields.loaders_data]: input.loaders_data ?? [],
		[fields.import_urls]: input.import_urls,
		[fields.outermost_server_error_index]: input.server_error_idx ?? null,
		[fields.outermost_server_error]: input.server_error,
		[fields.params]: input.params ?? {},
		[fields.splat_values]: input.splat_values ?? [],
		[fields.css_bundles]: input.css_bundles ?? [],
		[fields.deps]: input.deps ?? [],
	};
}

function view_module(loader: ClientLoaderFn): RouteModule {
	return {
		default: {
			pattern: "/",
			component: (props: unknown) => {
				return props;
			},
			client_loader: loader,
		},
	};
}

describe("ccc Effect route preparer experiment", () => {
	it("loads unique modules, registers client loaders, and prepares route records", async () => {
		const loaded_modules: string[] = [];
		const loader_calls: Array<{
			pattern: string;
			known_patterns: string[];
			loader_data: unknown;
			client_build_id: string;
		}> = [];
		const side_effects: string[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const loader: ClientLoaderFn = async (args) => {
					const server_state = await args.serverPromise;
					loader_calls.push({
						pattern: args.pattern,
						known_patterns: args.knownMatches.map((match) => {
							return match.pattern;
						}),
						loader_data: server_state.loaderData,
						client_build_id: server_state.clientBuildID,
					});
					return `${args.pattern}:${String(server_state.loaderData)}`;
				};
				const modules: Record<string, RouteModule> = {
					"/shared.js": view_module(loader),
					"/child.js": view_module(loader),
				};
				const preparer = yield* make_route_preparer({
					client_build_id: "build-1",
					load_module: (module_url) => {
						return Effect.sync(() => {
							loaded_modules.push(module_url);
							return modules[module_url]!;
						});
					},
					parse_search_params: (_schema, search_params) => {
						return {
							q: search_params.get("q"),
						};
					},
					preload_css: (css_bundles) => {
						return Effect.sync(() => {
							side_effects.push(
								`preload:${css_bundles.join(",")}`,
							);
						});
					},
					wait_for_css: (css_bundles) => {
						return Effect.sync(() => {
							side_effects.push(`wait:${css_bundles.join(",")}`);
						});
					},
					apply_payload_side_effects: (payload) => {
						return Effect.sync(() => {
							side_effects.push(
								`apply:${payload.deps.join(",")}`,
							);
						});
					},
				});
				const prepared = yield* preparer.prepare_route({
					raw_payload: route_payload({
						patterns: ["/", "/child", "/again"],
						import_urls: ["/shared.js", "/child.js", "/shared.js"],
						loaders_data: ["root", "child", "again"],
						params: { org: "acme" },
						splat_values: ["tail"],
						css_bundles: ["/app.css"],
						deps: ["/dep.js"],
					}),
					url: new URL("http://localhost/dashboard?q=hello"),
					trigger: "navigation",
					href: "http://localhost/dashboard?q=hello",
					history_state: { from: "test" },
				});
				const catalog = yield* preparer.catalog.snapshot;
				yield* prepared.apply_dom_side_effects;
				return { prepared, catalog };
			}),
		);

		expect(loaded_modules).toEqual(["/shared.js", "/child.js"]);
		expect([...result.catalog.keys()]).toEqual(["/", "/child", "/again"]);
		expect(loader_calls).toEqual([
			{
				pattern: "/",
				known_patterns: ["/", "/child", "/again"],
				loader_data: "root",
				client_build_id: "build-1",
			},
			{
				pattern: "/child",
				known_patterns: ["/", "/child", "/again"],
				loader_data: "child",
				client_build_id: "build-1",
			},
			{
				pattern: "/again",
				known_patterns: ["/", "/child", "/again"],
				loader_data: "again",
				client_build_id: "build-1",
			},
		]);
		expect(result.prepared.route).toMatchObject({
			params: { org: "acme" },
			splat_values: ["tail"],
			error: null,
			client_build_id: "build-1",
		});
		expect(
			result.prepared.route.matches.map((match) => {
				return match.client_loader_data;
			}),
		).toEqual(["/:root", "/child:child", "/again:again"]);
		expect(side_effects).toEqual([
			"preload:/app.css",
			"wait:/app.css",
			"apply:/dep.js",
		]);
	});

	it("skips client loaders at and below the server error boundary", async () => {
		const called_patterns: string[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const loader: ClientLoaderFn = async (args) => {
					called_patterns.push(args.pattern);
					return args.pattern;
				};
				const preparer = yield* make_route_preparer({
					client_build_id: "build-2",
					load_module: () => {
						return Effect.succeed(view_module(loader));
					},
				});
				return yield* preparer.prepare_route({
					raw_payload: route_payload({
						patterns: ["/", "/bad", "/child"],
						import_urls: ["/a.js", "/b.js", "/c.js"],
						loaders_data: ["a", "b", "c"],
						server_error_idx: 1,
						server_error: "server failed",
					}),
					url: new URL("http://localhost/bad"),
					trigger: "navigation",
					href: "http://localhost/bad",
					history_state: undefined,
				});
			}),
		);

		expect(called_patterns).toEqual(["/"]);
		expect(result.route.error).toEqual({
			idx: 1,
			error: "server failed",
			source: "server",
		});
		expect(
			result.route.matches.map((match) => {
				return match.client_loader_data;
			}),
		).toEqual(["/", undefined, undefined]);
	});

	it("records the first client loader failure and interrupts later route loaders", async () => {
		let release_first!: () => void;
		const first_ready = new Promise<void>((resolve) => {
			release_first = resolve;
		});
		let interrupted = 0;

		const result = await run_effect(
			Effect.gen(function* () {
				const first_loader: ClientLoaderFn = async () => {
					await first_ready;
					throw new Error("loader exploded");
				};
				const second_loader: ClientLoaderFn = async (args) => {
					return new Promise((_resolve, reject) => {
						args.signal.addEventListener(
							"abort",
							() => {
								interrupted++;
								reject(
									new DOMException("Aborted", "AbortError"),
								);
							},
							{ once: true },
						);
					});
				};
				const modules: Record<string, RouteModule> = {
					"/first.js": view_module(first_loader),
					"/second.js": view_module(second_loader),
				};
				const preparer = yield* make_route_preparer({
					client_build_id: "build-3",
					load_module: (module_url) => {
						return Effect.succeed(modules[module_url]!);
					},
				});
				const fiber = yield* Effect.forkChild(
					preparer.prepare_route({
						raw_payload: route_payload({
							patterns: ["/first", "/second"],
							import_urls: ["/first.js", "/second.js"],
							loaders_data: ["first", "second"],
						}),
						url: new URL("http://localhost/first"),
						trigger: "navigation",
						href: "http://localhost/first",
						history_state: undefined,
					}),
				);
				yield* drain();
				yield* Effect.sync(() => {
					release_first();
				});
				return yield* Fiber.join(fiber);
			}),
		);

		expect(interrupted).toBe(1);
		expect(result.route.error).toMatchObject({
			idx: 0,
			source: "clientLoader",
		});
		if (!result.route.error) {
			throw new Error("expected a client loader route error");
		}
		expect(result.route.error.error).toBe("loader exploded");
	});
});
