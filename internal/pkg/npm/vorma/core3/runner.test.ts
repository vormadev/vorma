import { describe, expect, it } from "vitest";
import type { CoreEffect } from "./model.ts";
import {
	create_abortable_operation_registry,
	create_public_waiter_registry,
	create_timer_registry,
	run_async_effect,
	run_immediate_effects,
	type AsyncEffectRunner,
	type ImmediateEffectRunner,
} from "./runner.ts";

describe("create_abortable_operation_registry", () => {
	it("aborts each operation at most once", () => {
		const registry = create_abortable_operation_registry();
		const aborted: string[] = [];

		expect(
			registry.start_operation("op-1", {
				abort: () => {
					aborted.push("op-1");
				},
			}),
		).toBe(true);
		expect(
			registry.start_operation("op-1", {
				abort: () => {
					aborted.push("duplicate");
				},
			}),
		).toBe(false);
		expect(registry.abort_operation("op-1")).toBe(true);
		expect(registry.abort_operation("op-1")).toBe(false);
		expect(aborted).toEqual(["op-1"]);
	});

	it("clears completed operations without aborting them", () => {
		const registry = create_abortable_operation_registry();
		const aborted: string[] = [];

		registry.start_operation("op-1", {
			abort: () => {
				aborted.push("op-1");
			},
		});

		expect(registry.clear_operation("op-1")).toBe(true);
		expect(registry.has_operation("op-1")).toBe(false);
		expect(aborted).toEqual([]);
	});
});

describe("create_timer_registry", () => {
	it("clears each timer handle at most once", () => {
		const cleared: string[] = [];
		const registry = create_timer_registry((handle: string) => {
			cleared.push(handle);
		});

		expect(registry.start_timer("timer-1", "handle-1")).toBe(true);
		expect(registry.start_timer("timer-1", "duplicate")).toBe(false);
		expect(registry.clear_timer("timer-1")).toBe(true);
		expect(registry.clear_timer("timer-1")).toBe(false);
		expect(cleared).toEqual(["handle-1"]);
	});
});

describe("create_public_waiter_registry", () => {
	it("settles each public waiter at most once", () => {
		const registry = create_public_waiter_registry();
		const settled: unknown[] = [];

		expect(
			registry.add_waiter("call-1", {
				reject: (cause) => {
					settled.push(cause);
				},
				resolve: (result) => {
					settled.push(result);
				},
			}),
		).toBe(true);
		expect(
			registry.add_waiter("call-1", {
				reject: (cause) => {
					settled.push(["duplicate", cause]);
				},
				resolve: (result) => {
					settled.push(["duplicate", result]);
				},
			}),
		).toBe(false);

		expect(registry.resolve_waiter("call-1", { ok: true })).toBe(true);
		expect(registry.resolve_waiter("call-1", { ok: false })).toBe(false);
		expect(registry.reject_waiter("call-1", "late")).toBe(false);
		expect(settled).toEqual([{ ok: true }]);
	});

	it("deletes a waiter before invoking it", () => {
		const registry = create_public_waiter_registry();
		const settled: unknown[] = [];

		registry.add_waiter("call-1", {
			reject: (cause) => {
				settled.push(cause);
			},
			resolve: (result) => {
				settled.push(result);
				settled.push(registry.resolve_waiter("call-1", "again"));
			},
		});

		expect(registry.resolve_waiter("call-1", "first")).toBe(true);
		expect(settled).toEqual(["first", false]);
	});
});

describe("run_immediate_effects", () => {
	it("runs view transition publication phases in transaction order", async () => {
		const log: string[] = [];
		const runner = test_immediate_runner(log);
		runner.public_waiters.add_waiter("call-1", {
			reject: (cause) => {
				log.push(`reject:${String(cause)}`);
			},
			resolve: (result) => {
				log.push(`resolve:${String(result)}`);
			},
		});

		const deferred_effects = await run_immediate_effects({
			effects: [
				{
					transaction: {
						after_transition: [
							{
								public_call_id: "call-1",
								result: "done",
								type: "resolve_public_call",
							},
						],
						before_transition: [{ type: "save_current_scroll" }],
						inside_transition: [
							{
								position: {
									href: "https://example.com/next",
									key: "browser-2",
									state: undefined,
								},
								replace: false,
								type: "write_history",
							},
							{ type: "render" },
						],
						operation_id: "nav-1",
						reason: "navigation",
					},
					type: "run_view_transition",
				},
			],
			runner,
		});

		expect(deferred_effects).toEqual([]);
		expect(log).toEqual([
			"save_current_scroll",
			"view:start",
			"write_history:https://example.com/next",
			"render",
			"view:end",
			"resolve:done",
		]);
	});

	it("returns async effects for the async coordinator", async () => {
		const log: string[] = [];
		const runner = test_immediate_runner(log);
		const async_effect = {
			client_build_id: "build-1",
			href: "https://example.com/next",
			operation_id: "nav-1",
			trigger: "navigation",
			type: "fetch_route",
		} satisfies CoreEffect;

		const deferred_effects = await run_immediate_effects({
			effects: [async_effect, { type: "render" }],
			runner,
		});

		expect(deferred_effects).toEqual([async_effect]);
		expect(log).toEqual(["render"]);
	});

	it("rejects async work inside publication transactions", async () => {
		const runner = test_immediate_runner([]);

		await expect(
			run_immediate_effects({
				effects: [
					{
						transaction: {
							after_transition: [],
							before_transition: [],
							inside_transition: [
								{
									client_build_id: "build-1",
									href: "https://example.com/next",
									operation_id: "nav-1",
									trigger: "navigation",
									type: "fetch_route",
								},
							],
							operation_id: "nav-1",
							reason: "navigation",
						},
						type: "run_view_transition",
					},
				],
				runner,
			}),
		).rejects.toThrow(
			"Core3 publication transactions must contain only immediate effects.",
		);
	});

	it("clears abortable route work at the commit boundary", async () => {
		const log: string[] = [];
		const runner = test_immediate_runner(log);
		runner.abortable_operations.start_operation("nav-1", {
			abort: () => {
				log.push("abort");
			},
		});

		await run_immediate_effects({
			effects: [
				{
					commit: {
						route_render: {
							state: route_render,
						},
					},
					next: route_snapshot,
					operation_id: "nav-1",
					type: "commit",
				},
			],
			runner,
		});

		expect(runner.abortable_operations.has_operation("nav-1")).toBe(false);
		expect(log).toEqual(["commit:nav-1"]);
	});
});

describe("run_async_effect", () => {
	it("returns operation-correlated route response inputs", async () => {
		const log: string[] = [];
		const runner = test_async_runner(log);

		const outcome = await run_async_effect({
			effect: {
				client_build_id: "build-1",
				href: "https://example.com/next",
				operation_id: "nav-1",
				trigger: "navigation",
				type: "fetch_route",
			},
			runner,
		});

		expect(outcome).toEqual({
			operation_id: "nav-1",
			response: {
				kind: "data",
				ok: true,
				payload: { route: true },
				server_build_id: "build-1",
				status: 200,
			},
			trigger: "navigation",
			type: "route_response_received",
		});
		expect(runner.abortable_operations.has_operation("nav-1")).toBe(false);
		expect(log).toEqual(["fetch_route:nav-1:false"]);
	});

	it("keeps prefetch preparation outputs resource-key based", async () => {
		const log: string[] = [];
		const runner = test_async_runner(log);

		const outcome = await run_async_effect({
			effect: {
				client_build_id: "build-1",
				history_state: undefined,
				href: "https://example.com/prefetch",
				operation_id: "prefetch-1",
				payload: { route: true },
				trigger: "prefetch",
				type: "prepare_route",
			},
			runner,
		});

		expect(outcome).toEqual({
			operation_id: "prefetch-1",
			result: {
				kind: "prepared",
				prepared_resource_key: "resource-1",
			},
			type: "route_prefetch_preparation_finished",
		});
		expect(log).toEqual(["prepare_prefetch_route:prefetch-1:false"]);
	});

	it("returns correlated failures instead of anonymous rejections", async () => {
		const runner = test_async_runner([]);
		const cause = new Error("nope");
		runner.host.fetch_api = () => {
			throw cause;
		};

		const outcome = await run_async_effect({
			effect: {
				href: "https://example.com/action",
				method: "POST",
				operation_id: "api-1",
				submission_key: "submission-1",
				type: "fetch_api",
			},
			runner,
		});

		expect(outcome).toEqual({
			cause,
			operation_id: "api-1",
			submission_key: "submission-1",
			type: "api_response_failed",
		});
	});

	it("returns timer-fired inputs after storing the timer handle", async () => {
		const log: string[] = [];
		const runner = test_async_runner(log);
		const running = run_async_effect({
			effect: {
				delay_ms: 25,
				operation_id: "reval-1",
				timer_id: "timer-1",
				type: "start_timer",
			},
			runner,
		});

		expect(runner.timers.has_timer("timer-1")).toBe(true);
		expect(timer_fire).toBeDefined();
		timer_fire?.();

		await expect(running).resolves.toEqual({
			operation_id: "reval-1",
			timer_id: "timer-1",
			type: "timer_fired",
		});
		expect(runner.timers.has_timer("timer-1")).toBe(false);
		expect(log).toEqual(["start_timer:timer-1:25", "clear_timer:timer-1"]);
	});

	it("settles timer work when the timer is cleared before firing", async () => {
		const log: string[] = [];
		const runner = test_async_runner(log);
		const running = run_async_effect({
			effect: {
				delay_ms: 25,
				operation_id: "reval-1",
				timer_id: "timer-1",
				type: "start_timer",
			},
			runner,
		});

		expect(runner.timers.clear_timer("timer-1")).toBe(true);
		await expect(running).resolves.toBeUndefined();
		timer_fire?.();

		expect(runner.timers.has_timer("timer-1")).toBe(false);
		expect(log).toEqual(["start_timer:timer-1:25", "clear_timer:timer-1"]);
	});
});

const route_render = {
	client_build_id: "build-1",
	entries: [],
	error: null,
	history_state: undefined,
	params: {},
	splat_values: [],
};

const route_snapshot = {
	position: {
		href: "https://example.com/next",
		key: "browser-2",
		state: undefined,
	},
	provisional: false,
	render: route_render,
	route: {
		client_build_id: "build-1",
		error: null,
		history_state: undefined,
		href: "https://example.com/next",
		matches: [],
		params: {},
		splat_values: [],
	},
};

let timer_fire: (() => void) | undefined;

function test_immediate_runner(log: string[]): ImmediateEffectRunner {
	return {
		abortable_operations: create_abortable_operation_registry(),
		host: {
			apply_publication_dom: () => {
				log.push("apply_publication_dom");
			},
			apply_scroll: () => {
				log.push("apply_scroll");
			},
			commit: (effect) => {
				log.push(`commit:${effect.operation_id}`);
			},
			hard_redirect: (effect) => {
				log.push(`hard_redirect:${effect.href}`);
			},
			install_browser_listeners: () => {
				log.push("install_browser_listeners");
			},
			reload: () => {
				log.push("reload");
			},
			render: () => {
				log.push("render");
			},
			report_build_skew: () => {
				log.push("report_build_skew");
			},
			run_view_transition: async (publish) => {
				log.push("view:start");
				await publish();
				log.push("view:end");
			},
			save_current_scroll: () => {
				log.push("save_current_scroll");
			},
			save_scroll_position: (effect) => {
				log.push(`save_scroll_position:${effect.key}`);
			},
			write_history: (effect) => {
				log.push(`write_history:${effect.position.href}`);
			},
		},
		public_waiters: create_public_waiter_registry(),
		timers: create_timer_registry((handle) => {
			log.push(`clear_timer:${String(handle)}`);
		}),
	};
}

function test_async_runner(log: string[]): AsyncEffectRunner<string> {
	timer_fire = undefined;
	return {
		abortable_operations: create_abortable_operation_registry(),
		host: {
			fetch_api: (effect, signal) => {
				log.push(
					`fetch_api:${effect.operation_id}:${String(signal.aborted)}`,
				);
				return {
					data: { ok: true },
					kind: "success",
					ok: true,
					response: {},
					server_build_id: "build-1",
					status: 200,
				};
			},
			fetch_route: (effect, signal) => {
				log.push(
					`fetch_route:${effect.operation_id}:${String(signal.aborted)}`,
				);
				return {
					kind: "data",
					ok: true,
					payload: { route: true },
					server_build_id: "build-1",
					status: 200,
				};
			},
			prepare_prefetch_route: (effect, signal) => {
				log.push(
					`prepare_prefetch_route:${effect.operation_id}:${String(signal.aborted)}`,
				);
				return {
					kind: "prepared",
					prepared_resource_key: "resource-1",
				};
			},
			prepare_route: (effect, signal) => {
				log.push(
					`prepare_route:${effect.operation_id}:${String(signal.aborted)}`,
				);
				return {
					kind: "prepared",
					prepared: {
						dom: {},
						render: route_render,
						route: route_snapshot.route,
					},
				};
			},
			promote_prefetch_route: (effect, signal) => {
				log.push(
					`promote_prefetch_route:${effect.operation_id}:${String(signal.aborted)}`,
				);
				return {
					kind: "prepared",
					prepared: {
						dom: {},
						render: route_render,
						route: route_snapshot.route,
					},
				};
			},
			read_boot_payload: (effect) => {
				log.push(`read_boot_payload:${effect.operation_id}`);
				return {
					client_build_id: "build-1",
					deployment_id: "",
					payload: { route: true },
					render: route_render,
					route: route_snapshot.route,
				};
			},
			run_route_hooks: (effect, signal) => {
				log.push(
					`run_route_hooks:${effect.operation_id}:${String(signal.aborted)}`,
				);
				return {
					kind: "completed",
					prepared: {
						dom: {},
						render: route_render,
						route: route_snapshot.route,
					},
				};
			},
			start_timer: (effect, fire) => {
				log.push(`start_timer:${effect.timer_id}:${effect.delay_ms}`);
				timer_fire = fire;
				return effect.timer_id;
			},
		},
		timers: create_timer_registry((handle) => {
			log.push(`clear_timer:${handle}`);
		}),
	};
}
