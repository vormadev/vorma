import { Effect, TestContext } from "effect";
import { describe, expect, it } from "vitest";
import type {
	ClientCommit,
	ScrollState,
} from "./effect_runtime/client_contract.ts";
import type {
	PreparedRoute,
	RouteModule,
	RouteRecord,
} from "./effect_runtime/route_preparer.ts";
import {
	make_route_publisher,
	type HistoryPosition,
	type RouteSnapshot,
} from "./effect_runtime/route_publisher.ts";
import type { BeforeRouteTransitionArgs } from "./types.ts";

const ROOT_PATTERN = "/";
const CHILD_PATTERN = "/child";
const ROOT_MODULE_URL = "/root.js";
const CHILD_MODULE_URL = "/child.js";
const FIRST_HREF = "http://localhost/first";
const SECOND_HREF = "http://localhost/second#details";
const BUILD_ID = "build-1";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(
		program.pipe(Effect.provide(TestContext.TestContext)),
	);
}

function route_record(input: {
	client_build_id?: string;
	modules?: RouteModule[];
	loader_data?: unknown[];
	client_loader_data?: unknown[];
}): RouteRecord {
	const modules = input.modules ?? [{}, {}];
	const loader_data = input.loader_data ?? ["root-loader", "child-loader"];
	const client_loader_data = input.client_loader_data ?? [
		"root-client",
		"child-client",
	];
	return {
		params: { org: "acme" },
		splat_values: ["tail"],
		error: null,
		client_build_id: input.client_build_id ?? BUILD_ID,
		matches: [
			{
				pattern: ROOT_PATTERN,
				input: { q: "root" },
				module_url: ROOT_MODULE_URL,
				module: modules[0]!,
				loader_data: loader_data[0],
				client_loader_data: client_loader_data[0],
			},
			{
				pattern: CHILD_PATTERN,
				input: { q: "child" },
				module_url: CHILD_MODULE_URL,
				module: modules[1]!,
				loader_data: loader_data[1],
				client_loader_data: client_loader_data[1],
			},
		],
	};
}

function position(
	href: string,
	input?: {
		key?: string;
		state?: unknown;
	},
): HistoryPosition {
	return {
		href,
		key: input?.key ?? "history-key",
		state: input?.state,
	};
}

function prepared_route(
	route: RouteRecord,
	side_effects: string[],
	label: string,
): PreparedRoute {
	return {
		route,
		apply_dom_side_effects: Effect.sync(() => {
			side_effects.push(label);
		}),
	};
}

function scroll_hash(hash: string): ScrollState {
	return { hash };
}

describe("ccc Effect route publisher experiment", () => {
	it("publishes initial route render and route update commits", async () => {
		const commits: ClientCommit[] = [];
		const events: string[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const publisher = yield* make_route_publisher({
					commit: (commit) => {
						events.push("commit");
						commits.push(commit);
					},
				});
				const publish_result = yield* publisher.publish({
					reason: "initial",
					prepared: prepared_route(
						route_record({}),
						events,
						"side-effect",
					),
					position: position(FIRST_HREF, {
						key: "boot-key",
						state: { boot: true },
					}),
					scroll: scroll_hash("#intro"),
				});
				const route_state = yield* publisher.route_state;
				return { publish_result, route_state };
			}),
		);

		expect(events).toEqual(["side-effect", "commit"]);
		expect(result.publish_result).toMatchObject({
			did_publish: true,
			did_update_route: true,
		});
		expect(commits).toHaveLength(1);
		expect(commits[0]?.route_update).toMatchObject({
			previous_route: null,
			reason: "boot",
			route: {
				href: FIRST_HREF,
				clientBuildID: BUILD_ID,
				params: { org: "acme" },
			},
		});
		expect(commits[0]?.route_render?.state).toMatchObject({
			client_build_id: BUILD_ID,
			history_state: { boot: true },
			splat_values: ["tail"],
		});
		expect(commits[0]?.route_render?.scroll_intent).toEqual({
			scroll: { hash: "#intro" },
			target_route_id: "1:/child",
		});
		expect(result.route_state).toMatchObject({
			href: FIRST_HREF,
			historyState: { boot: true },
			matches: [
				{
					pattern: ROOT_PATTERN,
					loaderData: "root-loader",
					clientLoaderData: "root-client",
				},
				{
					pattern: CHILD_PATTERN,
					loaderData: "child-loader",
					clientLoaderData: "child-client",
				},
			],
		});
	});

	it("omits route_update when the public route state is unchanged", async () => {
		const commits: ClientCommit[] = [];
		const events: string[] = [];
		const route = route_record({});
		const initial_snapshot: RouteSnapshot = {
			position: position(FIRST_HREF, { state: { stable: true } }),
			route,
		};

		const result = await run_effect(
			Effect.gen(function* () {
				const publisher = yield* make_route_publisher({
					initial_snapshot,
					commit: (commit) => {
						commits.push(commit);
					},
				});
				return yield* publisher.publish({
					reason: "hmr",
					prepared: prepared_route(route, events, "side-effect"),
					position: position(FIRST_HREF, { state: { stable: true } }),
				});
			}),
		);

		expect(result).toMatchObject({
			did_publish: true,
			did_update_route: false,
		});
		expect(events).toEqual(["side-effect"]);
		expect(commits).toHaveLength(1);
		expect(commits[0]?.route_render).toBeDefined();
		expect(commits[0]?.route_update).toBeUndefined();
	});

	it("moves the current route to a new history position without DOM side effects", async () => {
		const commits: ClientCommit[] = [];
		const events: string[] = [];
		const route = route_record({});
		const initial_snapshot: RouteSnapshot = {
			position: position(FIRST_HREF, { state: { tab: "old" } }),
			route,
		};

		const result = await run_effect(
			Effect.gen(function* () {
				const publisher = yield* make_route_publisher({
					initial_snapshot,
					commit: (commit) => {
						events.push("commit");
						commits.push(commit);
					},
				});
				const publish_result = yield* publisher.move_position({
					reason: "navigation",
					position: position(`${FIRST_HREF}#section`, {
						state: { tab: "new" },
					}),
					scroll: { hash: "#section" },
				});
				const route_state = yield* publisher.route_state;
				return { publish_result, route_state };
			}),
		);

		expect(events).toEqual(["commit"]);
		expect(result.publish_result).toMatchObject({
			did_publish: true,
			did_update_route: true,
		});
		expect(result.route_state).toMatchObject({
			href: `${FIRST_HREF}#section`,
			historyState: { tab: "new" },
		});
		expect(commits[0]?.route_render?.scroll_intent).toEqual({
			scroll: { hash: "#section" },
			target_route_id: "1:/child",
		});
	});

	it("runs yield and commit hooks before guarded publication", async () => {
		const commits: ClientCommit[] = [];
		const events: string[] = [];
		let can_publish = false;
		const previous_module: RouteModule = {
			default: {
				before_route_yield: (args: BeforeRouteTransitionArgs) => {
					events.push(`yield:${args.trigger}:${args.next.href}`);
				},
			},
		};
		const next_module: RouteModule = {
			default: {
				before_route_commit: (args: BeforeRouteTransitionArgs) => {
					events.push(`commit-hook:${args.current.href}`);
				},
			},
		};
		const previous_route = route_record({
			modules: [previous_module, {}],
		});
		const next_route = route_record({
			modules: [{}, next_module],
			loader_data: ["next-root", "next-child"],
		});
		const initial_snapshot: RouteSnapshot = {
			position: position(FIRST_HREF),
			route: previous_route,
		};

		const result = await run_effect(
			Effect.gen(function* () {
				const publisher = yield* make_route_publisher({
					initial_snapshot,
					commit: (commit) => {
						commits.push(commit);
					},
				});
				const first = yield* publisher.publish({
					reason: "navigation",
					prepared: prepared_route(next_route, events, "side-effect"),
					position: position(SECOND_HREF),
					guard: Effect.sync(() => {
						return can_publish;
					}),
				});
				can_publish = true;
				const second = yield* publisher.publish({
					reason: "navigation",
					prepared: prepared_route(next_route, events, "side-effect"),
					position: position(SECOND_HREF),
					guard: Effect.sync(() => {
						return can_publish;
					}),
				});
				return { first, second };
			}),
		);

		expect(result.first).toMatchObject({
			did_publish: false,
			did_update_route: false,
		});
		expect(result.second).toMatchObject({
			did_publish: true,
			did_update_route: true,
		});
		expect(events).toEqual([
			`yield:navigation:${SECOND_HREF}`,
			`commit-hook:${FIRST_HREF}`,
			`yield:navigation:${SECOND_HREF}`,
			`commit-hook:${FIRST_HREF}`,
			"side-effect",
		]);
		expect(commits).toHaveLength(1);
		expect(commits[0]?.route_update?.reason).toBe("navigation");
		expect(commits[0]?.route_update?.previous_route?.href).toBe(FIRST_HREF);
		expect(commits[0]?.route_update?.route.href).toBe(SECOND_HREF);
	});
});
