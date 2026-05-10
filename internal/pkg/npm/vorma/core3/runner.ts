import type { RouteHooksFacts, RoutePreparationFacts } from "./classify.ts";
import type {
	APIResponseFacts,
	CoreEffect,
	OperationID,
	PublicCallID,
	ResourceKey,
	RouteFacts,
	RouteRenderFacts,
	RouteResponseFacts,
	RouteTrigger,
	SubmissionKey,
	TimerID,
	VisibleRouteTrigger,
} from "./model.ts";

export type Awaitable<T> = Promise<T> | T;

export type EffectOf<Type extends CoreEffect["type"]> = Extract<
	CoreEffect,
	{ type: Type }
>;

export type ImmediateEffectHost = {
	apply_publication_dom: (
		effect: EffectOf<"apply_publication_dom">,
	) => Awaitable<void>;
	apply_scroll: (effect: EffectOf<"apply_scroll">) => Awaitable<void>;
	commit: (effect: EffectOf<"commit">) => Awaitable<void>;
	hard_redirect: (effect: EffectOf<"hard_redirect">) => Awaitable<void>;
	install_browser_listeners: (
		effect: EffectOf<"install_browser_listeners">,
	) => Awaitable<void>;
	reload: (effect: EffectOf<"reload">) => Awaitable<void>;
	render: (effect: EffectOf<"render">) => Awaitable<void>;
	report_build_skew: (
		effect: EffectOf<"report_build_skew">,
	) => Awaitable<void>;
	run_view_transition: (publish: () => Promise<void>) => Awaitable<void>;
	save_current_scroll: (
		effect: EffectOf<"save_current_scroll">,
	) => Awaitable<void>;
	save_scroll_position: (
		effect: EffectOf<"save_scroll_position">,
	) => Awaitable<void>;
	write_history: (effect: EffectOf<"write_history">) => Awaitable<void>;
};

export type ImmediateEffectRunner = {
	abortable_operations: AbortableOperationRegistry;
	host: ImmediateEffectHost;
	public_waiters: PublicWaiterRegistry;
	timers: TimerRegistry<unknown>;
};

export type VisibleRoutePreparationEffect = EffectOf<"prepare_route"> & {
	trigger: VisibleRouteTrigger;
};

export type PrefetchRoutePreparationEffect = EffectOf<"prepare_route"> & {
	trigger: "prefetch";
};

export type RoutePrefetchPreparationFacts =
	| {
			kind: "aborted";
	  }
	| {
			cause: unknown;
			kind: "failed";
	  }
	| {
			kind: "prepared";
			prepared_resource_key: ResourceKey;
	  };

export type BootPayloadFacts = {
	client_build_id: string;
	deployment_id: string;
	payload: unknown;
	render: RouteRenderFacts;
	route: RouteFacts;
};

export type AsyncEffectHost<TimerHandle> = {
	fetch_api: (
		effect: EffectOf<"fetch_api">,
		signal: AbortSignal,
	) => Awaitable<APIResponseFacts>;
	fetch_route: (
		effect: EffectOf<"fetch_route">,
		signal: AbortSignal,
	) => Awaitable<RouteResponseFacts>;
	prepare_prefetch_route: (
		effect: PrefetchRoutePreparationEffect,
		signal: AbortSignal,
	) => Awaitable<RoutePrefetchPreparationFacts>;
	prepare_route: (
		effect: VisibleRoutePreparationEffect,
		signal: AbortSignal,
	) => Awaitable<RoutePreparationFacts>;
	promote_prefetch_route: (
		effect: EffectOf<"promote_prefetch_route">,
		signal: AbortSignal,
	) => Awaitable<RoutePreparationFacts>;
	read_boot_payload: (
		effect: EffectOf<"read_boot_payload">,
	) => Awaitable<BootPayloadFacts>;
	run_route_hooks: (
		effect: EffectOf<"run_route_hooks">,
		signal: AbortSignal,
	) => Awaitable<RouteHooksFacts>;
	start_timer: (
		effect: EffectOf<"start_timer">,
		fire: () => void,
	) => TimerHandle;
};

export type AsyncEffectRunner<TimerHandle> = {
	abortable_operations: AbortableOperationRegistry;
	host: AsyncEffectHost<TimerHandle>;
	timers: TimerRegistry<TimerHandle>;
};

export type CoreAsyncOutcomeInput =
	| {
			client_build_id: string;
			deployment_id: string;
			operation_id: OperationID;
			payload: unknown;
			render: RouteRenderFacts;
			route: RouteFacts;
			type: "boot_payload_read";
	  }
	| {
			cause: unknown;
			operation_id: OperationID;
			type: "boot_payload_failed";
	  }
	| {
			operation_id: OperationID;
			response: RouteResponseFacts;
			trigger: RouteTrigger;
			type: "route_response_received";
	  }
	| {
			cause: unknown;
			operation_id: OperationID;
			trigger: RouteTrigger;
			type: "route_response_failed";
	  }
	| {
			operation_id: OperationID;
			result: RoutePreparationFacts;
			trigger: Exclude<RouteTrigger, "prefetch">;
			type: "route_preparation_finished";
	  }
	| {
			operation_id: OperationID;
			result: RoutePrefetchPreparationFacts;
			type: "route_prefetch_preparation_finished";
	  }
	| {
			operation_id: OperationID;
			result: RoutePreparationFacts;
			type: "prefetch_promotion_finished";
	  }
	| {
			operation_id: OperationID;
			result: RouteHooksFacts;
			type: "route_hooks_finished";
	  }
	| {
			operation_id: OperationID;
			response: APIResponseFacts;
			submission_key: SubmissionKey;
			type: "api_response_received";
	  }
	| {
			cause: unknown;
			operation_id: OperationID;
			submission_key: SubmissionKey;
			type: "api_response_failed";
	  }
	| {
			operation_id?: OperationID;
			timer_id: TimerID;
			type: "timer_fired";
	  };

export type AbortableOperationHandle = {
	abort: () => void;
};

export type AbortableOperationRegistry = {
	abort_all_operations: () => void;
	abort_operation: (operation_id: OperationID) => boolean;
	clear_operation: (operation_id: OperationID) => boolean;
	has_operation: (operation_id: OperationID) => boolean;
	start_operation: (
		operation_id: OperationID,
		handle: AbortableOperationHandle,
	) => boolean;
};

export type TimerRegistry<TimerHandle> = {
	clear_all_timers: () => void;
	clear_timer: (timer_id: TimerID) => boolean;
	has_timer: (timer_id: TimerID) => boolean;
	start_timer: (
		timer_id: TimerID,
		handle: TimerHandle,
		cancel?: () => void,
	) => boolean;
};

export type PublicWaiter = {
	reject: (cause: unknown) => void;
	resolve: (result: unknown) => void;
};

export type PublicWaiterRegistry = {
	add_waiter: (public_call_id: PublicCallID, waiter: PublicWaiter) => boolean;
	has_waiter: (public_call_id: PublicCallID) => boolean;
	reject_all_waiters: (cause: unknown) => void;
	reject_waiter: (public_call_id: PublicCallID, cause: unknown) => boolean;
	resolve_waiter: (public_call_id: PublicCallID, result: unknown) => boolean;
};

export async function run_immediate_effects(input: {
	effects: readonly CoreEffect[];
	runner: ImmediateEffectRunner;
}): Promise<readonly CoreEffect[]> {
	const deferred_effects: CoreEffect[] = [];
	for (const effect of input.effects) {
		const handled = await run_immediate_effect(input.runner, effect);
		if (!handled) {
			deferred_effects.push(effect);
		}
	}
	return deferred_effects;
}

export async function run_async_effect<TimerHandle>(input: {
	effect: CoreEffect;
	runner: AsyncEffectRunner<TimerHandle>;
}): Promise<CoreAsyncOutcomeInput | undefined> {
	const effect = input.effect;
	if (effect.type === "read_boot_payload") {
		try {
			const facts = await input.runner.host.read_boot_payload(effect);
			return {
				client_build_id: facts.client_build_id,
				deployment_id: facts.deployment_id,
				operation_id: effect.operation_id,
				payload: facts.payload,
				render: facts.render,
				route: facts.route,
				type: "boot_payload_read",
			};
		} catch (cause) {
			return {
				cause,
				operation_id: effect.operation_id,
				type: "boot_payload_failed",
			};
		}
	}
	if (effect.type === "fetch_route") {
		return run_abortable_effect({
			failed: (cause) => {
				return {
					cause,
					operation_id: effect.operation_id,
					trigger: effect.trigger,
					type: "route_response_failed",
				};
			},
			operation_id: effect.operation_id,
			run: async (signal) => {
				const response = await input.runner.host.fetch_route(
					effect,
					signal,
				);
				return {
					operation_id: effect.operation_id,
					response,
					trigger: effect.trigger,
					type: "route_response_received",
				};
			},
			runner: input.runner,
		});
	}
	if (effect.type === "prepare_route" && effect.trigger === "prefetch") {
		return run_abortable_effect({
			failed: (cause) => {
				return {
					operation_id: effect.operation_id,
					result: {
						cause,
						kind: "failed",
					},
					type: "route_prefetch_preparation_finished",
				};
			},
			operation_id: effect.operation_id,
			run: async (signal) => {
				const result = await input.runner.host.prepare_prefetch_route(
					effect,
					signal,
				);
				return {
					operation_id: effect.operation_id,
					result,
					type: "route_prefetch_preparation_finished",
				};
			},
			runner: input.runner,
		});
	}
	if (effect.type === "prepare_route") {
		return run_abortable_effect({
			failed: (cause) => {
				return {
					operation_id: effect.operation_id,
					result: {
						cause,
						kind: "failed",
					},
					trigger: effect.trigger,
					type: "route_preparation_finished",
				};
			},
			operation_id: effect.operation_id,
			run: async (signal) => {
				const result = await input.runner.host.prepare_route(
					effect,
					signal,
				);
				return {
					operation_id: effect.operation_id,
					result,
					trigger: effect.trigger,
					type: "route_preparation_finished",
				};
			},
			runner: input.runner,
		});
	}
	if (effect.type === "promote_prefetch_route") {
		return run_abortable_effect({
			failed: (cause) => {
				return {
					operation_id: effect.operation_id,
					result: {
						cause,
						kind: "failed",
					},
					type: "prefetch_promotion_finished",
				};
			},
			operation_id: effect.operation_id,
			run: async (signal) => {
				const result = await input.runner.host.promote_prefetch_route(
					effect,
					signal,
				);
				return {
					operation_id: effect.operation_id,
					result,
					type: "prefetch_promotion_finished",
				};
			},
			runner: input.runner,
		});
	}
	if (effect.type === "run_route_hooks") {
		return run_abortable_effect({
			failed: (cause) => {
				return {
					operation_id: effect.operation_id,
					result: {
						cause,
						kind: "failed",
					},
					type: "route_hooks_finished",
				};
			},
			operation_id: effect.operation_id,
			run: async (signal) => {
				const result = await input.runner.host.run_route_hooks(
					effect,
					signal,
				);
				return {
					operation_id: effect.operation_id,
					result,
					type: "route_hooks_finished",
				};
			},
			runner: input.runner,
		});
	}
	if (effect.type === "fetch_api") {
		return run_abortable_effect({
			failed: (cause) => {
				return {
					cause,
					operation_id: effect.operation_id,
					submission_key: effect.submission_key,
					type: "api_response_failed",
				};
			},
			operation_id: effect.operation_id,
			run: async (signal) => {
				const response = await input.runner.host.fetch_api(
					effect,
					signal,
				);
				return {
					operation_id: effect.operation_id,
					response,
					submission_key: effect.submission_key,
					type: "api_response_received",
				};
			},
			runner: input.runner,
		});
	}
	if (effect.type === "start_timer") {
		if (input.runner.timers.has_timer(effect.timer_id)) {
			return undefined;
		}
		let finish_timer!: (fired: boolean) => void;
		const finished = new Promise<boolean>((resolve) => {
			finish_timer = resolve;
		});
		const fire_timer = (): void => {
			finish_timer(true);
			input.runner.timers.clear_timer(effect.timer_id);
		};
		const handle = input.runner.host.start_timer(effect, fire_timer);
		input.runner.timers.start_timer(effect.timer_id, handle, () => {
			finish_timer(false);
		});
		if (!(await finished)) {
			return undefined;
		}
		return {
			...(effect.operation_id !== undefined
				? { operation_id: effect.operation_id }
				: {}),
			timer_id: effect.timer_id,
			type: "timer_fired",
		};
	}
	return undefined;
}

export function create_abortable_operation_registry(): AbortableOperationRegistry {
	const operations = new Map<OperationID, AbortableOperationHandle>();
	return {
		abort_all_operations: () => {
			for (const operation_id of operations.keys()) {
				abort_operation(operation_id);
			}
		},
		abort_operation,
		clear_operation: (operation_id) => {
			return operations.delete(operation_id);
		},
		has_operation: (operation_id) => {
			return operations.has(operation_id);
		},
		start_operation: (operation_id, handle) => {
			if (operations.has(operation_id)) {
				return false;
			}
			operations.set(operation_id, handle);
			return true;
		},
	};

	function abort_operation(operation_id: OperationID): boolean {
		const operation = operations.get(operation_id);
		if (!operation) {
			return false;
		}
		operations.delete(operation_id);
		operation.abort();
		return true;
	}
}

async function run_abortable_effect<TimerHandle>(input: {
	failed: (cause: unknown) => CoreAsyncOutcomeInput;
	operation_id: OperationID;
	run: (signal: AbortSignal) => Promise<CoreAsyncOutcomeInput>;
	runner: AsyncEffectRunner<TimerHandle>;
}): Promise<CoreAsyncOutcomeInput> {
	const controller = new AbortController();
	if (
		!input.runner.abortable_operations.start_operation(input.operation_id, {
			abort: () => {
				controller.abort();
			},
		})
	) {
		return input.failed(
			new Error("Core3 async operation is already running."),
		);
	}
	try {
		return await input.run(controller.signal);
	} catch (cause) {
		return input.failed(cause);
	} finally {
		input.runner.abortable_operations.clear_operation(input.operation_id);
	}
}

async function run_immediate_effect(
	runner: ImmediateEffectRunner,
	effect: CoreEffect,
): Promise<boolean> {
	if (effect.type === "read_boot_payload") {
		return false;
	}
	if (effect.type === "fetch_route") {
		return false;
	}
	if (effect.type === "prepare_route") {
		return false;
	}
	if (effect.type === "promote_prefetch_route") {
		return false;
	}
	if (effect.type === "run_route_hooks") {
		return false;
	}
	if (effect.type === "fetch_api") {
		return false;
	}
	if (effect.type === "start_timer") {
		return false;
	}
	if (effect.type === "abort_operation") {
		runner.abortable_operations.abort_operation(effect.operation_id);
		return true;
	}
	if (effect.type === "clear_timer") {
		runner.timers.clear_timer(effect.timer_id);
		return true;
	}
	if (effect.type === "save_current_scroll") {
		await runner.host.save_current_scroll(effect);
		return true;
	}
	if (effect.type === "save_scroll_position") {
		await runner.host.save_scroll_position(effect);
		return true;
	}
	if (effect.type === "write_history") {
		await runner.host.write_history(effect);
		return true;
	}
	if (effect.type === "apply_publication_dom") {
		await runner.host.apply_publication_dom(effect);
		return true;
	}
	if (effect.type === "commit") {
		runner.abortable_operations.clear_operation(effect.operation_id);
		await runner.host.commit(effect);
		return true;
	}
	if (effect.type === "render") {
		await runner.host.render(effect);
		return true;
	}
	if (effect.type === "apply_scroll") {
		await runner.host.apply_scroll(effect);
		return true;
	}
	if (effect.type === "hard_redirect") {
		await runner.host.hard_redirect(effect);
		return true;
	}
	if (effect.type === "reload") {
		await runner.host.reload(effect);
		return true;
	}
	if (effect.type === "install_browser_listeners") {
		await runner.host.install_browser_listeners(effect);
		return true;
	}
	if (effect.type === "report_build_skew") {
		await runner.host.report_build_skew(effect);
		return true;
	}
	if (effect.type === "resolve_public_call") {
		runner.public_waiters.resolve_waiter(
			effect.public_call_id,
			effect.result,
		);
		return true;
	}
	if (effect.type === "resolve_api_call") {
		runner.public_waiters.resolve_waiter(
			effect.public_call_id,
			effect.result,
		);
		return true;
	}
	if (effect.type === "reject_public_call") {
		runner.public_waiters.reject_waiter(
			effect.public_call_id,
			effect.cause,
		);
		return true;
	}
	await run_publication_phase(runner, effect.transaction.before_transition);
	await runner.host.run_view_transition(async () => {
		await run_publication_phase(
			runner,
			effect.transaction.inside_transition,
		);
	});
	await run_publication_phase(runner, effect.transaction.after_transition);
	return true;
}

async function run_publication_phase(
	runner: ImmediateEffectRunner,
	effects: readonly CoreEffect[],
): Promise<void> {
	const deferred_effects = await run_immediate_effects({ effects, runner });
	if (deferred_effects.length > 0) {
		throw new Error(
			"Core3 publication transactions must contain only immediate effects.",
		);
	}
}

export function create_timer_registry<TimerHandle>(
	clear_handle: (handle: TimerHandle) => void,
): TimerRegistry<TimerHandle> {
	const timers = new Map<
		TimerID,
		{ cancel?: () => void; handle: TimerHandle }
	>();
	return {
		clear_all_timers: () => {
			for (const timer_id of timers.keys()) {
				clear_timer(timer_id);
			}
		},
		clear_timer,
		has_timer: (timer_id) => {
			return timers.has(timer_id);
		},
		start_timer: (timer_id, handle, cancel) => {
			if (timers.has(timer_id)) {
				return false;
			}
			timers.set(
				timer_id,
				cancel === undefined ? { handle } : { cancel, handle },
			);
			return true;
		},
	};

	function clear_timer(timer_id: TimerID): boolean {
		if (!timers.has(timer_id)) {
			return false;
		}
		const timer = timers.get(timer_id) as {
			cancel?: () => void;
			handle: TimerHandle;
		};
		timers.delete(timer_id);
		timer.cancel?.();
		clear_handle(timer.handle);
		return true;
	}
}

export function create_public_waiter_registry(): PublicWaiterRegistry {
	const waiters = new Map<PublicCallID, PublicWaiter>();
	return {
		add_waiter: (public_call_id, waiter) => {
			if (waiters.has(public_call_id)) {
				return false;
			}
			waiters.set(public_call_id, waiter);
			return true;
		},
		has_waiter: (public_call_id) => {
			return waiters.has(public_call_id);
		},
		reject_all_waiters: (cause) => {
			for (const public_call_id of waiters.keys()) {
				reject_waiter(public_call_id, cause);
			}
		},
		reject_waiter,
		resolve_waiter,
	};

	function reject_waiter(
		public_call_id: PublicCallID,
		cause: unknown,
	): boolean {
		const waiter = waiters.get(public_call_id);
		if (!waiter) {
			return false;
		}
		waiters.delete(public_call_id);
		waiter.reject(cause);
		return true;
	}

	function resolve_waiter(
		public_call_id: PublicCallID,
		result: unknown,
	): boolean {
		const waiter = waiters.get(public_call_id);
		if (!waiter) {
			return false;
		}
		waiters.delete(public_call_id);
		waiter.resolve(result);
		return true;
	}
}
