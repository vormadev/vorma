import { create_core6_deferred, type Core6Deferred } from "./deferred.ts";
import {
	core6_route_navigation_result_kind,
	run_core6_route_navigation,
	type Core6RouteNavigationHost,
	type Core6RouteNavigationResult,
} from "./route_navigation.ts";
import { core6_route_preparation_trigger } from "./route_preparation.ts";
import { core6_route_publish_reason } from "./route_publication.ts";
import type { Core6RouteRuntime } from "./route_runtime.ts";
import type { Core6RouteTransactionFlowIntent } from "./route_transaction_flow.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

export const CORE6_REVALIDATION_DEBOUNCE_MS = 8;
export const CORE6_MAX_REVALIDATION_RETRIES = 8;
export const CORE6_REVALIDATION_BACKOFF_BASE_MS = 500;
export const CORE6_REVALIDATION_BACKOFF_CAP_MS = 30000;

export const core6_route_revalidation_reason = {
	manual: "manual",
	mutation: "mutation",
	retry: "retry",
	window_focus: "window_focus",
} as const;

export const core6_route_revalidation_status = {
	debouncing: "debouncing",
	pending: "pending",
	retrying: "retrying",
	running: "running",
} as const;

export const core6_route_revalidation_failure_reason = {
	build_skew: "build_skew",
	cancelled: "cancelled",
	max_retries_exhausted: "max_retries_exhausted",
} as const;

export type Core6RouteRevalidationReason =
	(typeof core6_route_revalidation_reason)[keyof typeof core6_route_revalidation_reason];

export type Core6RouteRevalidationStatusValue =
	(typeof core6_route_revalidation_status)[keyof typeof core6_route_revalidation_status];

export type Core6RouteRevalidationFailureReason =
	(typeof core6_route_revalidation_failure_reason)[keyof typeof core6_route_revalidation_failure_reason];

export type Core6RouteRevalidationResult =
	| {
			ok: true;
	  }
	| {
			ok: false;
			reason: Core6RouteRevalidationFailureReason;
	  };

export type Core6RouteRevalidationTimer = unknown;

export type Core6RouteRevalidationTimerHost = {
	clear_timer: (timer: Core6RouteRevalidationTimer) => void;
	set_timer: (fn: () => void, ms: number) => Core6RouteRevalidationTimer;
};

export type Core6RouteRevalidationRuntime = Pick<
	Core6RouteRuntime,
	| "current_route"
	| "current_transaction_kind"
	| "publish_same_document"
	| "run_route_fetch"
	| "run_route_prepared"
>;

export type Core6RouteRevalidationConfig = {
	active_client_build_id: () => string;
	backoff_base_ms?: number;
	backoff_cap_ms?: number;
	debounce_ms?: number;
	deployment_id?: () => string | null | undefined;
	host: Core6RouteNavigationHost & Core6RouteRevalidationTimerHost;
	max_retries?: number;
	runtime: Core6RouteRevalidationRuntime;
};

export type Core6RouteRevalidationRequestInput = {
	debounce?: boolean;
	reason?: Core6RouteRevalidationReason;
	skip_work_indicator?: boolean;
};

export type Core6RouteRevalidationStatus = {
	attempt: number;
	reason: Core6RouteRevalidationReason;
	skip_work_indicator: boolean;
	status: Core6RouteRevalidationStatusValue;
	waiter_count: number;
};

export type Core6RouteRevalidationOwner = {
	cancel: () => boolean;
	current_status: () => Core6RouteRevalidationStatus | null;
	mark_fresh: () => boolean;
	request: (
		input?: Core6RouteRevalidationRequestInput,
	) => Promise<Core6RouteRevalidationResult>;
	start_pending: () => boolean;
};

type Core6RouteRevalidationDemand = {
	reason: Core6RouteRevalidationReason;
	skip_work_indicator: boolean;
	waiters: Core6Deferred<Core6RouteRevalidationResult>[];
};

type Core6RouteRevalidationState =
	| {
			kind: "idle";
	  }
	| {
			demand: Core6RouteRevalidationDemand;
			kind: typeof core6_route_revalidation_status.debouncing;
			timer: Core6RouteRevalidationTimer;
	  }
	| {
			attempt: number;
			demand: Core6RouteRevalidationDemand;
			kind: typeof core6_route_revalidation_status.pending;
	  }
	| {
			attempt: number;
			demand: Core6RouteRevalidationDemand;
			kind: typeof core6_route_revalidation_status.retrying;
			timer: Core6RouteRevalidationTimer;
	  }
	| {
			attempt: number;
			demand: Core6RouteRevalidationDemand;
			kind: typeof core6_route_revalidation_status.running;
			run_id: number;
	  };

export function create_core6_route_revalidation_owner(
	config: Core6RouteRevalidationConfig,
): Core6RouteRevalidationOwner {
	const debounce_ms = config.debounce_ms ?? CORE6_REVALIDATION_DEBOUNCE_MS;
	const max_retries = config.max_retries ?? CORE6_MAX_REVALIDATION_RETRIES;
	const backoff_base_ms =
		config.backoff_base_ms ?? CORE6_REVALIDATION_BACKOFF_BASE_MS;
	const backoff_cap_ms =
		config.backoff_cap_ms ?? CORE6_REVALIDATION_BACKOFF_CAP_MS;
	let state: Core6RouteRevalidationState = { kind: "idle" };
	let next_run_id = 0;

	function current_demand(): Core6RouteRevalidationDemand | null {
		if (state.kind === "idle") {
			return null;
		}
		return state.demand;
	}

	function clear_state_timer(): void {
		if (
			state.kind === core6_route_revalidation_status.debouncing ||
			state.kind === core6_route_revalidation_status.retrying
		) {
			config.host.clear_timer(state.timer);
		}
	}

	function settle_demand(
		demand: Core6RouteRevalidationDemand,
		result: Core6RouteRevalidationResult,
	): void {
		for (const waiter of demand.waiters) {
			waiter.resolve(result);
		}
	}

	function schedule_debounce(demand: Core6RouteRevalidationDemand): void {
		let debouncing_state!: Extract<
			Core6RouteRevalidationState,
			{ kind: "debouncing" }
		>;
		const timer = config.host.set_timer(() => {
			if (state !== debouncing_state) {
				return;
			}
			state = {
				attempt: 0,
				demand,
				kind: core6_route_revalidation_status.pending,
			};
			start_pending();
		}, debounce_ms);
		debouncing_state = {
			demand,
			kind: core6_route_revalidation_status.debouncing,
			timer,
		};
		state = debouncing_state;
	}

	function schedule_retry(
		demand: Core6RouteRevalidationDemand,
		next_attempt: number,
	): void {
		if (next_attempt >= max_retries) {
			state = { kind: "idle" };
			settle_demand(demand, {
				ok: false,
				reason: core6_route_revalidation_failure_reason.max_retries_exhausted,
			});
			return;
		}
		const ms = Math.min(
			backoff_base_ms * Math.pow(2, next_attempt - 1),
			backoff_cap_ms,
		);
		const retry_demand: Core6RouteRevalidationDemand = {
			...demand,
			reason: core6_route_revalidation_reason.retry,
		};
		let retrying_state!: Extract<
			Core6RouteRevalidationState,
			{ kind: "retrying" }
		>;
		const timer = config.host.set_timer(() => {
			if (state !== retrying_state) {
				return;
			}
			state = {
				attempt: next_attempt,
				demand: retry_demand,
				kind: core6_route_revalidation_status.pending,
			};
			start_pending();
		}, ms);
		retrying_state = {
			attempt: next_attempt,
			demand: retry_demand,
			kind: core6_route_revalidation_status.retrying,
			timer,
		};
		state = retrying_state;
	}

	function settle_run(
		run_id: number,
		demand: Core6RouteRevalidationDemand,
		attempt: number,
		result: Core6RouteNavigationResult,
	): void {
		if (
			state.kind !== core6_route_revalidation_status.running ||
			state.run_id !== run_id ||
			state.demand !== demand
		) {
			return;
		}
		if (
			result.kind === core6_route_navigation_result_kind.published ||
			result.kind === core6_route_navigation_result_kind.same_document ||
			result.kind === core6_route_navigation_result_kind.hard_redirect
		) {
			state = { kind: "idle" };
			settle_demand(demand, { ok: true });
			return;
		}
		if (result.kind === core6_route_navigation_result_kind.build_skew) {
			config.host.notify_build_skew?.({
				active_client_build_id: config.active_client_build_id(),
				default_behavior: result.behavior,
				response: result.result.response,
				revalidation_reason: demand.reason,
				trigger: core6_route_transaction_kind.revalidation,
			});
			state = { kind: "idle" };
			settle_demand(demand, {
				ok: false,
				reason: core6_route_revalidation_failure_reason.build_skew,
			});
			return;
		}
		if (result.kind === core6_route_navigation_result_kind.interrupted) {
			state = {
				attempt,
				demand,
				kind: core6_route_revalidation_status.pending,
			};
			start_pending();
			return;
		}
		schedule_retry(demand, attempt + 1);
	}

	function reject_run(
		run_id: number,
		demand: Core6RouteRevalidationDemand,
		reason: unknown,
	): void {
		if (
			state.kind !== core6_route_revalidation_status.running ||
			state.run_id !== run_id ||
			state.demand !== demand
		) {
			return;
		}
		state = { kind: "idle" };
		for (const waiter of demand.waiters) {
			waiter.reject(reason);
		}
	}

	function start_pending(): boolean {
		if (state.kind !== core6_route_revalidation_status.pending) {
			return false;
		}
		const route = config.runtime.current_route();
		if (!route || config.runtime.current_transaction_kind() !== null) {
			return false;
		}
		const href = route.href;
		const intent: Core6RouteTransactionFlowIntent = {
			history_state: route.historyState,
			href,
			preparation_trigger: core6_route_preparation_trigger.revalidation,
			publish_reason: core6_route_publish_reason.revalidation,
			search_params: new URL(href).searchParams,
		};
		const demand = state.demand;
		const attempt = state.attempt;
		const run_id = next_run_id++;
		state = {
			attempt,
			demand,
			kind: core6_route_revalidation_status.running,
			run_id,
		};
		void run_core6_route_navigation({
			active_client_build_id: config.active_client_build_id(),
			deployment_id: config.deployment_id?.(),
			host: config.host,
			intent,
			kind: core6_route_transaction_kind.revalidation,
			runtime: config.runtime,
		}).then(
			(result) => {
				settle_run(run_id, demand, attempt, result);
			},
			(reason) => {
				reject_run(run_id, demand, reason);
			},
		);
		return true;
	}

	return {
		cancel: () => {
			const demand = current_demand();
			if (!demand) {
				return false;
			}
			clear_state_timer();
			state = { kind: "idle" };
			settle_demand(demand, {
				ok: false,
				reason: core6_route_revalidation_failure_reason.cancelled,
			});
			return true;
		},
		current_status: () => {
			if (state.kind === "idle") {
				return null;
			}
			return {
				attempt:
					state.kind === core6_route_revalidation_status.debouncing
						? 0
						: state.attempt,
				reason: state.demand.reason,
				skip_work_indicator: state.demand.skip_work_indicator,
				status: state.kind,
				waiter_count: state.demand.waiters.length,
			};
		},
		mark_fresh: () => {
			const demand = current_demand();
			if (!demand) {
				return false;
			}
			clear_state_timer();
			state = { kind: "idle" };
			settle_demand(demand, { ok: true });
			return true;
		},
		request: (input = {}) => {
			if (
				!config.runtime.current_route() &&
				config.runtime.current_transaction_kind() ===
					core6_route_transaction_kind.boot
			) {
				state = {
					attempt: 0,
					demand: {
						reason:
							input.reason ??
							core6_route_revalidation_reason.manual,
						skip_work_indicator: input.skip_work_indicator === true,
						waiters: [],
					},
					kind: core6_route_revalidation_status.pending,
				};
				return Promise.resolve({ ok: true });
			}
			const waiter =
				create_core6_deferred<Core6RouteRevalidationResult>();
			const previous_demand = current_demand();
			const demand: Core6RouteRevalidationDemand = {
				reason: input.reason ?? core6_route_revalidation_reason.manual,
				skip_work_indicator:
					(previous_demand?.skip_work_indicator ?? true) &&
					input.skip_work_indicator === true,
				waiters: [...(previous_demand?.waiters ?? []), waiter],
			};
			clear_state_timer();
			if (input.debounce === false) {
				state = {
					attempt: 0,
					demand,
					kind: core6_route_revalidation_status.pending,
				};
				start_pending();
			} else {
				schedule_debounce(demand);
			}
			return waiter.promise;
		},
		start_pending,
	};
}
