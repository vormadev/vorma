import type {
	CoreEffect,
	CoreModel,
	OperationID,
	PublicCallID,
	RefreshDemand,
	RefreshState,
	RouteRevalidationOperation,
	TimerID,
} from "./model.ts";

export type RefreshRequestInput = {
	debounce: boolean;
	debounce_ms: number;
	demand: RefreshDemand;
	refresh: RefreshState;
	timer_id?: TimerID;
};

export type RefreshRequestPlan = {
	effects: readonly CoreEffect[];
	refresh: RefreshState;
};

export type RouteRevalidationRequestInput = {
	debounce: boolean;
	debounce_ms: number;
	demand: RefreshDemand;
	model: CoreModel;
	timer_id?: TimerID;
};

export type RouteRevalidationRequestPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export type ManualRevalidationRequestInput = {
	debounce: boolean;
	debounce_ms: number;
	model: CoreModel;
	operation_id: OperationID;
	public_call_id?: PublicCallID;
	skip_work_indicator: boolean;
	timer_id?: TimerID;
};

export type FocusRevalidationRequestInput = {
	debounce_ms: number;
	model: CoreModel;
	now_ms: number;
	operation_id: OperationID;
	timer_id?: TimerID;
};

export type RefreshTimerInput = {
	model: CoreModel;
	operation_id: OperationID;
	timer_id: TimerID;
};

export type PendingRefreshStartInput = {
	model: CoreModel;
};

export type PendingRefreshStartPlan = {
	effects: readonly CoreEffect[];
	model: CoreModel;
};

export type RefreshRetryInput = {
	backoff_base_ms: number;
	backoff_cap_ms: number;
	demand: RefreshDemand;
	max_retries: number;
	next_operation_id: OperationID;
	previous_attempt: number;
	timer_id: TimerID;
};

export type RefreshRetryPlan =
	| {
			kind: "exhausted";
			public_call_ids: readonly PublicCallID[];
			refresh: RefreshState;
	  }
	| {
			effects: readonly CoreEffect[];
			kind: "retrying";
			refresh: RefreshState;
	  };

export function request_route_revalidation(
	input: RouteRevalidationRequestInput,
): RouteRevalidationRequestPlan {
	const requested = request_refresh({
		debounce: input.debounce,
		debounce_ms: input.debounce_ms,
		demand: input.demand,
		refresh: input.model.refresh,
		timer_id: input.timer_id,
	});
	const requested_model = {
		...input.model,
		refresh: requested.refresh,
	};
	const started = begin_pending_refresh({ model: requested_model });
	if (started) {
		return {
			effects: requested.effects.concat(started.effects),
			model: started.model,
		};
	}
	return {
		effects: requested.effects,
		model: requested_model,
	};
}

export function request_manual_revalidation(
	input: ManualRevalidationRequestInput,
): RouteRevalidationRequestPlan | undefined {
	if (input.model.phase !== "ready" || !input.model.current) {
		return undefined;
	}
	return request_route_revalidation({
		debounce: input.debounce,
		debounce_ms: input.debounce_ms,
		demand: {
			operation_id: input.operation_id,
			public_call_ids:
				input.public_call_id !== undefined
					? [input.public_call_id]
					: [],
			reason: "manual",
			skip_work_indicator: input.skip_work_indicator,
		},
		model: input.model,
		timer_id: input.timer_id,
	});
}

export function request_focus_revalidation(
	input: FocusRevalidationRequestInput,
): RouteRevalidationRequestPlan | undefined {
	if (
		input.model.phase !== "ready" ||
		input.model.active_route_operation_id !== null ||
		!input.model.current ||
		!input.model.focus_revalidation ||
		input.model.refresh.kind !== "idle" ||
		Object.keys(input.model.submissions).length > 0
	) {
		return undefined;
	}

	const last_activity_ms =
		input.model.focus_revalidation.last_activity_ms ?? input.now_ms;
	if (
		input.now_ms - last_activity_ms <
		input.model.focus_revalidation.stale_ms
	) {
		return undefined;
	}
	return request_route_revalidation({
		debounce: true,
		debounce_ms: input.debounce_ms,
		demand: {
			operation_id: input.operation_id,
			public_call_ids: [],
			reason: "windowFocus",
			skip_work_indicator: true,
		},
		model: input.model,
		timer_id: input.timer_id,
	});
}

export function consume_refresh_timer(
	input: RefreshTimerInput,
): RouteRevalidationRequestPlan | undefined {
	const refresh = input.model.refresh;
	if (
		(refresh.kind !== "debouncing" && refresh.kind !== "retrying") ||
		refresh.timer_id !== input.timer_id ||
		refresh.demand.operation_id !== input.operation_id
	) {
		return undefined;
	}

	const requested_model = {
		...input.model,
		refresh: {
			attempt: refresh.kind === "retrying" ? refresh.attempt : 0,
			demand: refresh.demand,
			kind: "pending",
		},
	} satisfies CoreModel;
	const started = begin_pending_refresh({ model: requested_model });
	if (started) {
		return started;
	}
	return {
		effects: [],
		model: requested_model,
	};
}

export function request_refresh(
	input: RefreshRequestInput,
): RefreshRequestPlan {
	const demand =
		input.refresh.kind === "idle"
			? input.demand
			: {
					...input.demand,
					public_call_ids:
						input.refresh.demand.public_call_ids.concat(
							input.demand.public_call_ids,
						),
					skip_work_indicator:
						input.refresh.demand.skip_work_indicator &&
						input.demand.skip_work_indicator,
				};
	const effects: CoreEffect[] = [];
	if (
		input.refresh.kind === "debouncing" ||
		input.refresh.kind === "retrying"
	) {
		effects.push({
			timer_id: input.refresh.timer_id,
			type: "clear_timer",
		});
	}
	if (input.debounce) {
		const timer_id = input.timer_id ?? `${demand.operation_id}:debounce`;
		effects.push({
			delay_ms: input.debounce_ms,
			operation_id: demand.operation_id,
			timer_id,
			type: "start_timer",
		});
		return {
			effects,
			refresh: {
				demand,
				kind: "debouncing",
				timer_id,
			},
		};
	}
	return {
		effects,
		refresh: {
			attempt: 0,
			demand,
			kind: "pending",
		},
	};
}

export function begin_pending_refresh(
	input: PendingRefreshStartInput,
): PendingRefreshStartPlan | undefined {
	const refresh = input.model.refresh;
	if (
		input.model.phase !== "ready" ||
		input.model.active_route_operation_id !== null ||
		!input.model.browser ||
		!input.model.current ||
		refresh.kind !== "pending"
	) {
		return undefined;
	}
	if (
		refresh.demand.after_operation_id &&
		input.model.operations[refresh.demand.after_operation_id]?.status !==
			"settled"
	) {
		return undefined;
	}
	if (input.model.operations[refresh.demand.operation_id]) {
		return undefined;
	}

	const operation = {
		attempt: refresh.attempt,
		href: input.model.browser.href,
		id: refresh.demand.operation_id,
		kind: "route_revalidation",
		public_call_ids: refresh.demand.public_call_ids,
		reason: refresh.demand.reason,
		rights: ["publish_route", "satisfy_refresh_demand"],
		status: "started",
	} satisfies RouteRevalidationOperation;

	return {
		effects: [
			{
				client_build_id: input.model.client_build_id,
				...(input.model.deployment_id
					? { deployment_id: input.model.deployment_id }
					: {}),
				href: operation.href,
				operation_id: operation.id,
				trigger: "revalidation",
				type: "fetch_route",
			},
		],
		model: {
			...input.model,
			operations: {
				...input.model.operations,
				[operation.id]: operation,
			},
			refresh: {
				attempt: refresh.attempt,
				demand: refresh.demand,
				kind: "running",
				operation_id: operation.id,
			},
		},
	};
}

export function plan_refresh_retry(input: RefreshRetryInput): RefreshRetryPlan {
	const next_attempt = input.previous_attempt + 1;
	if (next_attempt > input.max_retries) {
		return {
			kind: "exhausted",
			public_call_ids: input.demand.public_call_ids,
			refresh: { kind: "idle" },
		};
	}
	const delay_ms = Math.min(
		input.backoff_cap_ms,
		input.backoff_base_ms * 2 ** (next_attempt - 1),
	);
	const demand = {
		...input.demand,
		operation_id: input.next_operation_id,
		reason: "retry",
	} satisfies RefreshDemand;
	return {
		effects: [
			{
				delay_ms,
				operation_id: input.next_operation_id,
				timer_id: input.timer_id,
				type: "start_timer",
			},
		],
		kind: "retrying",
		refresh: {
			attempt: next_attempt,
			demand,
			kind: "retrying",
			timer_id: input.timer_id,
		},
	};
}
