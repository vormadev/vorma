import { jsonDeepEquals } from "vorma/kit/json";
import type {
	APIFetchOutcome,
	APIToken,
	BrowserKey,
	BuildSkewTriggeringResponse,
	Effect,
	HistoryAction,
	HMRRouteUpdateInput,
	PublicCallID,
	RouteFetchOutcome,
	RouteToken,
} from "./events.ts";
import type {
	ActiveRoute,
	ActiveRouteIntent,
	APIRouteKind,
	BootingModel,
	BrowserPosition,
	ClientCommitPlan,
	CurrentRoute,
	NavigationIntent,
	PendingBootRevalidations,
	Prefetch,
	PreparedRoute,
	ReadyModel,
	RefreshDemand,
	RevalidationIntent,
	ScrollState,
	Submission,
	WorkIndicatorPolicy,
} from "./model.ts";
import {
	empty_counters,
	next_api_token,
	next_browser_key,
	next_public_call,
	next_refresh_timer,
	next_route_token,
	next_sequence,
} from "./model.ts";
import type { AbortHandle } from "./platform.ts";
import {
	derive_work_state,
	is_http_url,
	is_same_document,
	is_same_origin,
	MAX_REDIRECTS,
	MAX_REVALIDATION_RETRIES,
	merge_refresh_demand,
	refresh_backoff_ms,
	refresh_demand_of,
	render_plan_from_prepared,
	REVALIDATION_BUILD_SKEW,
	REVALIDATION_DEBOUNCE_MS,
	REVALIDATION_EXHAUSTED,
	REVALIDATION_OK,
	route_state_from_prepared,
	scroll_for_navigation,
	scroll_intent_for,
	settle_navigation_calls_unsuccessful,
	settle_refresh_waiters,
	url_hash_normalized,
} from "./reducer-core.ts";

export type Transition<M = ReadyModel> = {
	model: M;
	effects: readonly Effect[];
};

/////////////////////////////////////////////////////////////////////
/////// Effects
/////////////////////////////////////////////////////////////////////

function settle_revalidation_ok_effects(
	call_id: PublicCallID | undefined,
): Effect[] {
	if (!call_id) {
		return [];
	}
	return [
		{ type: "settle_revalidate_call", call_id, result: REVALIDATION_OK },
	];
}

function settle_submit_effect(
	call_id: PublicCallID,
	result: import("./events.ts").SubmitResult,
): Effect {
	return { type: "settle_submit_call", call_id, result };
}

function popstate_reload_effects(route: ActiveRoute): Effect[] {
	if (route.intent.kind !== "navigation" || !route.intent.nav.is_popstate) {
		return [];
	}
	return [{ type: "reload" }];
}

type BuildSkewResponseFacts = {
	ok: boolean;
	server_build_id: string;
	status: number;
};

function notify_build_skew_effects(
	model: ReadyModel | BootingModel,
	response: BuildSkewResponseFacts,
	triggering_response: BuildSkewTriggeringResponse,
): Effect[] {
	if (
		model.phase !== "ready" ||
		!response.server_build_id ||
		response.server_build_id === model.config.client_build_id
	) {
		return [];
	}
	return [
		{
			type: "notify_build_skew",
			event: {
				activeClientBuildID: model.config.client_build_id,
				serverBuildID: response.server_build_id,
				triggeringResponse: triggering_response,
				currentRouteState: model.current.route_state,
				currentWorkState: derive_work_state(model),
			},
		},
	];
}

function route_build_skew_response(
	outcome: RouteFetchOutcome,
): BuildSkewResponseFacts | null {
	if (outcome.kind === "aborted" || outcome.kind === "network_error") {
		return null;
	}
	if (outcome.kind === "http_error") {
		return {
			ok: false,
			server_build_id: outcome.server_build_id,
			status: outcome.status,
		};
	}
	return {
		ok: outcome.ok,
		server_build_id: outcome.server_build_id,
		status: outcome.status,
	};
}

function route_skew_effects_for_active(
	model: ReadyModel,
	active: ActiveRoute & { phase: "fetching" },
	outcome: RouteFetchOutcome,
): Effect[] {
	const response = route_build_skew_response(outcome);
	if (!response) {
		return [];
	}
	const triggering_response: BuildSkewTriggeringResponse =
		active.intent.kind === "revalidation"
			? {
					kind: "route",
					trigger: "revalidation",
					revalidationReason: active.intent.reval.reason,
					requestedHref: active.url,
					status: response.status,
					ok: response.ok,
				}
			: {
					kind: "route",
					trigger: active.intent.nav.is_popstate
						? "popstate"
						: "navigation",
					requestedHref: active.url,
					status: response.status,
					ok: response.ok,
				};
	return notify_build_skew_effects(model, response, triggering_response);
}

function route_skew_effects_for_prefetch(
	model: ReadyModel,
	prefetch: Prefetch & { phase: "fetching" },
	outcome: RouteFetchOutcome,
): Effect[] {
	const response = route_build_skew_response(outcome);
	if (!response) {
		return [];
	}
	return notify_build_skew_effects(model, response, {
		kind: "route",
		trigger: "prefetch",
		requestedHref: prefetch.url,
		status: response.status,
		ok: response.ok,
	});
}

function api_skew_effects(
	model: ReadyModel | BootingModel,
	submission: Submission,
	outcome: APIFetchOutcome,
): Effect[] {
	if (outcome.kind === "aborted" || outcome.kind === "network_error") {
		return [];
	}
	const response: BuildSkewResponseFacts =
		outcome.kind === "http_error"
			? {
					ok: false,
					server_build_id: outcome.server_build_id,
					status: outcome.status,
				}
			: {
					ok: outcome.kind === "redirect" ? outcome.ok : true,
					server_build_id: outcome.server_build_id,
					status: outcome.status,
				};
	return notify_build_skew_effects(model, response, {
		kind: "apiRoute",
		apiRouteKind: submission.route_kind,
		requestedHref: submission.href,
		method: submission.method,
		status: response.status,
		ok: response.ok,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Active Route
/////////////////////////////////////////////////////////////////////

export function supersede_active_route(model: ReadyModel): Transition {
	if (!model.active_route) {
		return { model, effects: [] };
	}
	const active = model.active_route;
	const settle = settle_navigation_calls_unsuccessful(model.active_route);
	let next_model: ReadyModel = { ...model, active_route: null };
	if (
		active.intent.kind === "revalidation" &&
		model.refresh.kind === "running" &&
		model.refresh.route_token === active.token
	) {
		next_model = {
			...next_model,
			refresh: {
				kind: "pending",
				demand: model.refresh.demand,
				attempt: model.refresh.attempt,
			},
		};
	}
	const effects: Effect[] =
		active.abort_handle === null
			? [...settle]
			: [{ type: "abort", handle: active.abort_handle }, ...settle];
	return { model: next_model, effects };
}

export function cancel_prefetch(model: ReadyModel): Transition {
	const pf = model.prefetch;
	if (!pf) {
		return { model, effects: [] };
	}
	const effects: Effect[] =
		pf.phase === "prepared"
			? []
			: [{ type: "abort", handle: pf.abort_handle }];
	return {
		model: { ...model, prefetch: null },
		effects,
	};
}

export type BeginNavigationFetchInput = {
	target_href: string;
	replace: boolean;
	scroll_to_top: boolean | undefined;
	state: unknown;
	skip_work_indicator: boolean;
	source: "navigate" | "popstate" | "redirect";
	is_popstate: boolean;
	popstate_restored_scroll: ScrollState | undefined;
	public_calls: readonly PublicCallID[];
	redirect_count: number;
};

export function begin_navigation_fetch(
	model: ReadyModel,
	input: BeginNavigationFetchInput,
	abort_handle: AbortHandle,
): Transition {
	const seq = next_sequence(model.counters);
	const tok = next_route_token(seq.counters);
	const key = next_browser_key(tok.counters);

	const nav: NavigationIntent = {
		href: input.target_href,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		state: input.state,
		skip_work_indicator: input.skip_work_indicator,
		source: input.source,
		is_popstate: input.is_popstate,
		is_initial: false,
		popstate_restored_scroll: input.popstate_restored_scroll,
		browser_key: key.key,
	};

	const intent: ActiveRouteIntent = {
		kind: "navigation",
		nav,
		public_calls: input.public_calls,
	};

	return {
		model: {
			...model,
			counters: key.counters,
			active_route: {
				phase: "fetching",
				token: tok.token,
				abort_handle,
				url: input.target_href,
				intent,
				redirect_count: input.redirect_count,
				sequence: seq.sequence,
			},
		},
		effects: [
			{
				type: "fetch_route",
				token: tok.token,
				url: input.target_href,
				abort_handle,
				is_revalidation: false,
				client_build_id: model.config.client_build_id,
				deployment_id: model.config.deployment_id,
				trigger: input.is_popstate ? "popstate" : "navigation",
			},
		],
	};
}

export type MergeNavigateInput = {
	target_href: string;
	replace: boolean;
	scroll_to_top: boolean | undefined;
	state: unknown;
	skip_work_indicator: boolean;
	call_id: PublicCallID;
};

export function try_merge_navigate_into_active(
	model: ReadyModel,
	input: MergeNavigateInput,
): Transition | null {
	const active = model.active_route;
	if (!active || active.intent.kind !== "navigation") {
		return null;
	}
	if (!is_same_document(active.url, input.target_href)) {
		return null;
	}
	const current_nav = active.intent.nav;
	const identical =
		current_nav.href === input.target_href &&
		current_nav.replace === input.replace &&
		current_nav.scroll_to_top === input.scroll_to_top &&
		current_nav.state === input.state &&
		current_nav.skip_work_indicator === input.skip_work_indicator &&
		current_nav.source === "navigate";

	if (identical) {
		return {
			model: {
				...model,
				active_route: {
					...active,
					url: input.target_href,
					intent: {
						kind: "navigation",
						nav: current_nav,
						public_calls: [
							...active.intent.public_calls,
							input.call_id,
						],
					},
				},
			},
			effects: [],
		};
	}

	const key = next_browser_key(model.counters);
	const next_nav: NavigationIntent = {
		href: input.target_href,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		state: input.state,
		skip_work_indicator: input.skip_work_indicator,
		source: "navigate",
		is_popstate: false,
		is_initial: false,
		popstate_restored_scroll: undefined,
		browser_key: key.key,
	};
	const settle_old: Effect[] = active.intent.public_calls.map((cid) => {
		return {
			type: "settle_navigation_call",
			call_id: cid,
			result: { didNavigate: false },
		};
	});
	return {
		model: {
			...model,
			counters: key.counters,
			active_route: {
				...active,
				url: input.target_href,
				intent: {
					kind: "navigation",
					nav: next_nav,
					public_calls: [input.call_id],
				},
			},
		},
		effects: settle_old,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Prefetch Promotion
/////////////////////////////////////////////////////////////////////

type PrefetchPromotionInput = {
	pf: Prefetch;
	model: ReadyModel;
	target_href: string;
	intent: ActiveRouteIntent;
	state: unknown;
	is_popstate: boolean;
};

function promote_prefetch_to_active(input: PrefetchPromotionInput): Transition {
	const seq = next_sequence(input.model.counters);
	const pf = input.pf;

	if (pf.phase === "fetching") {
		return {
			model: {
				...input.model,
				counters: seq.counters,
				prefetch: null,
				active_route: {
					phase: "fetching",
					token: pf.token,
					abort_handle: pf.abort_handle,
					url: input.target_href,
					intent: input.intent,
					redirect_count: 0,
					sequence: seq.sequence,
				},
			},
			effects: [],
		};
	}

	if (pf.phase === "preparing") {
		return {
			model: {
				...input.model,
				counters: seq.counters,
				prefetch: null,
				active_route: {
					phase: "preparing",
					token: pf.token,
					abort_handle: pf.abort_handle,
					url: input.target_href,
					intent: input.intent,
					redirect_count: 0,
					sequence: seq.sequence,
					payload: pf.payload,
				},
			},
			effects: [],
		};
	}

	const tok = next_route_token(seq.counters);
	const publish_effect = build_publish_effect(
		input.model,
		tok.token,
		null,
		pf.prepared,
		input.target_href,
		input.state,
		input.intent,
	);
	return {
		model: {
			...input.model,
			counters: tok.counters,
			prefetch: null,
			active_route: {
				phase: "publishing",
				token: tok.token,
				abort_handle: null,
				url: input.target_href,
				intent: input.intent,
				redirect_count: 0,
				sequence: seq.sequence,
				prepared: pf.prepared,
			},
		},
		effects: [publish_effect],
	};
}

export type PromotePrefetchInput = {
	target_href: string;
	replace: boolean;
	scroll_to_top: boolean | undefined;
	state: unknown;
	skip_work_indicator: boolean;
	call_id: PublicCallID;
	source: "navigate" | "redirect";
};

export function try_promote_prefetch(
	model: ReadyModel,
	input: PromotePrefetchInput,
): Transition | null {
	if (
		!model.prefetch ||
		!is_same_document(model.prefetch.url, input.target_href)
	) {
		return null;
	}
	const counters_after_key = next_browser_key(model.counters);
	const nav: NavigationIntent = {
		href: input.target_href,
		replace: input.replace,
		scroll_to_top: input.scroll_to_top,
		state: input.state,
		skip_work_indicator: input.skip_work_indicator,
		source: input.source,
		is_popstate: false,
		is_initial: false,
		popstate_restored_scroll: undefined,
		browser_key: counters_after_key.key,
	};
	return promote_prefetch_to_active({
		pf: model.prefetch,
		model: { ...model, counters: counters_after_key.counters },
		target_href: input.target_href,
		intent: {
			kind: "navigation",
			nav,
			public_calls: [input.call_id],
		},
		state: input.state,
		is_popstate: false,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Publication Planning
/////////////////////////////////////////////////////////////////////

export function build_publish_effect(
	model: ReadyModel | BootingModel,
	token: RouteToken,
	abort_handle: AbortHandle | null,
	prepared: PreparedRoute,
	target_href: string,
	state: unknown,
	intent: ActiveRouteIntent,
): Effect {
	const previous_route_state = model.current?.route_state ?? null;
	const next_route_state = route_state_from_prepared(
		prepared,
		target_href,
		state,
	);
	const route_render_state = render_plan_from_prepared(prepared, state);

	const scroll: ScrollState | null =
		intent.kind === "navigation"
			? scroll_for_navigation(intent.nav, target_href)
			: null;

	const reason: "boot" | "navigation" | "popstate" | "revalidation" =
		intent.kind === "revalidation"
			? "revalidation"
			: intent.nav.is_initial
				? "boot"
				: intent.nav.is_popstate
					? "popstate"
					: "navigation";

	const commit: ClientCommitPlan = {
		route_render: scroll
			? {
					state: route_render_state,
					scroll_intent: scroll_intent_for(prepared, scroll),
				}
			: { state: route_render_state },
	};
	if (
		!previous_route_state ||
		!jsonDeepEquals(previous_route_state, next_route_state)
	) {
		commit.route_update = {
			previous_route: previous_route_state,
			reason,
			route: next_route_state,
		};
	}

	const history_action: HistoryAction =
		intent.kind === "navigation" &&
		!intent.nav.is_popstate &&
		!intent.nav.is_initial
			? {
					kind: intent.nav.replace ? "replace" : "push",
					href: target_href,
					state,
					browser_key: intent.nav.browser_key,
				}
			: { kind: "none" };

	const yield_hooks = (model.current?.prepared.matches ?? []).map(
		(m) => m.client_loader_id,
	);
	const commit_hooks = prepared.matches.map((m) => m.client_loader_id);

	const hook_trigger: "navigation" | "popstate" | "revalidation" =
		intent.kind === "revalidation"
			? "revalidation"
			: intent.nav.is_popstate
				? "popstate"
				: "navigation";

	return {
		type: "publish_route",
		token,
		abort_handle,
		commit,
		history_action,
		save_current_scroll:
			intent.kind === "navigation" &&
			!intent.nav.is_popstate &&
			!intent.nav.is_initial,
		scroll,
		apply_dom_side_effects: {
			title: prepared.title,
			meta_head_els: prepared.meta_head_els,
			rest_head_els: prepared.rest_head_els,
			css_bundles: prepared.css_bundles,
			deps: prepared.deps,
		},
		use_view_transition:
			(model.phase === "ready" || model.phase === "booting") &&
			model.config.use_view_transitions &&
			intent.kind === "navigation" &&
			!intent.nav.is_initial,
		run_yield_hooks_for: yield_hooks,
		run_commit_hooks_for: commit_hooks,
		hook_trigger,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Same-Document Navigation
/////////////////////////////////////////////////////////////////////

export type SameDocumentInput = {
	target_href: string;
	replace: boolean;
	state: unknown;
	source: "navigate" | "popstate" | "redirect";
	popstate_browser_key: BrowserPosition["key"] | null;
	popstate_restored_scroll: ScrollState | undefined;
};

export type SameDocumentResult = {
	model: ReadyModel;
	effects: readonly Effect[];
	did_navigate: boolean;
};

function same_document_commit(
	model: ReadyModel,
	next_current: CurrentRoute,
	reason: "navigation" | "popstate",
): ClientCommitPlan {
	const commit: ClientCommitPlan = {
		route_render: {
			state: render_plan_from_prepared(
				next_current.prepared,
				next_current.position.state,
			),
		},
	};
	if (!jsonDeepEquals(model.current.route_state, next_current.route_state)) {
		commit.route_update = {
			previous_route: model.current.route_state,
			reason,
			route: next_current.route_state,
		};
	}
	return commit;
}

export function apply_same_document_navigation(
	model: ReadyModel,
	input: SameDocumentInput,
): SameDocumentResult {
	const target_hash = url_hash_normalized(input.target_href);
	const current_hash = url_hash_normalized(model.current.position.href);

	if (input.source === "popstate") {
		const new_position: BrowserPosition = {
			href: input.target_href,
			key: input.popstate_browser_key as BrowserPosition["key"],
			state: input.state,
		};
		const next_current: CurrentRoute = {
			position: new_position,
			prepared: model.current.prepared,
			route_state: {
				...model.current.route_state,
				href: input.target_href,
				historyState: input.state,
			},
			sequence: model.current.sequence,
		};
		const scroll: ScrollState =
			target_hash.length > 0
				? { hash: new URL(input.target_href).hash }
				: (input.popstate_restored_scroll ?? { x: 0, y: 0 });
		return {
			model: { ...model, browser: new_position, current: next_current },
			effects: [
				{
					type: "emit_client_commit",
					commit: same_document_commit(
						model,
						next_current,
						"popstate",
					),
				},
				{ type: "apply_scroll", scroll },
			],
			did_navigate: true,
		};
	}

	if (target_hash !== current_hash) {
		const key = next_browser_key(model.counters);
		const new_position: BrowserPosition = {
			href: input.target_href,
			key: key.key,
			state: input.state,
		};
		const next_current: CurrentRoute = {
			position: new_position,
			prepared: model.current.prepared,
			route_state: {
				...model.current.route_state,
				href: input.target_href,
				historyState: input.state,
			},
			sequence: model.current.sequence,
		};
		return {
			model: {
				...model,
				counters: key.counters,
				browser: new_position,
				current: next_current,
			},
			effects: [
				{
					type: "save_current_scroll",
					browser_key: model.browser.key,
				},
				{
					type: "apply_history",
					action: {
						kind: input.replace ? "replace" : "push",
						href: input.target_href,
						state: input.state,
						browser_key: key.key,
					},
				},
				{
					type: "emit_client_commit",
					commit: same_document_commit(
						model,
						next_current,
						"navigation",
					),
				},
				{
					type: "apply_scroll",
					scroll: { hash: new URL(input.target_href).hash },
				},
			],
			did_navigate: true,
		};
	}

	if (input.replace) {
		const key = next_browser_key(model.counters);
		const new_position: BrowserPosition = {
			href: input.target_href,
			key: key.key,
			state: input.state,
		};
		const next_current: CurrentRoute = {
			position: new_position,
			prepared: model.current.prepared,
			route_state: {
				...model.current.route_state,
				href: input.target_href,
				historyState: input.state,
			},
			sequence: model.current.sequence,
		};
		return {
			model: {
				...model,
				counters: key.counters,
				browser: new_position,
				current: next_current,
			},
			effects: [
				{
					type: "apply_history",
					action: {
						kind: "replace",
						href: input.target_href,
						state: input.state,
						browser_key: key.key,
					},
				},
				{
					type: "emit_client_commit",
					commit: same_document_commit(
						model,
						next_current,
						"navigation",
					),
				},
				{ type: "apply_scroll", scroll: { x: 0, y: 0 } },
			],
			did_navigate: false,
		};
	}

	return {
		model,
		effects: [{ type: "apply_scroll", scroll: { x: 0, y: 0 } }],
		did_navigate: false,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Refresh
/////////////////////////////////////////////////////////////////////

export type ScheduleRefreshInput = {
	reason: RefreshDemand["reason"];
	skip_work_indicator: boolean;
	waiter_call_id: PublicCallID | undefined;
	debounce: boolean;
};

export function schedule_refresh(
	model: ReadyModel,
	input: ScheduleRefreshInput,
): Transition {
	const seq = next_sequence(model.counters);
	const previous_demand = refresh_demand_of(model.refresh);
	const new_demand: RefreshDemand = {
		after_sequence: seq.sequence,
		reason: input.reason,
		skip_work_indicator: input.skip_work_indicator,
		waiters: input.waiter_call_id
			? [{ call_id: input.waiter_call_id }]
			: [],
	};
	const merged = merge_refresh_demand(previous_demand, new_demand);

	const clear_timer: Effect[] =
		model.refresh.kind === "debouncing" || model.refresh.kind === "retrying"
			? [{ type: "clear_timer", id: model.refresh.timer_id }]
			: [];

	if (input.debounce) {
		const timer = next_refresh_timer(seq.counters);
		return {
			model: {
				...model,
				counters: timer.counters,
				refresh: {
					kind: "debouncing",
					demand: merged,
					timer_id: timer.id,
				},
			},
			effects: [
				...clear_timer,
				{
					type: "start_timer",
					id: timer.id,
					ms: REVALIDATION_DEBOUNCE_MS,
				},
			],
		};
	}

	return {
		model: {
			...model,
			counters: seq.counters,
			refresh: { kind: "pending", demand: merged, attempt: 0 },
		},
		effects: clear_timer,
	};
}

export function start_pending_revalidation(
	model: ReadyModel,
	abort_handle: AbortHandle,
): Transition | null {
	if (model.refresh.kind !== "pending" || model.active_route) {
		return null;
	}
	const seq = next_sequence(model.counters);
	const tok = next_route_token(seq.counters);
	const reval: RevalidationIntent = {
		attempt: model.refresh.attempt,
		reason: model.refresh.demand.reason,
		skip_work_indicator: model.refresh.demand.skip_work_indicator,
	};
	return {
		model: {
			...model,
			counters: tok.counters,
			active_route: {
				phase: "fetching",
				token: tok.token,
				abort_handle,
				url: model.browser.href,
				intent: { kind: "revalidation", reval },
				redirect_count: 0,
				sequence: seq.sequence,
			},
			refresh: {
				kind: "running",
				demand: model.refresh.demand,
				attempt: model.refresh.attempt,
				route_token: tok.token,
			},
		},
		effects: [
			{
				type: "fetch_route",
				token: tok.token,
				url: model.browser.href,
				abort_handle,
				is_revalidation: true,
				client_build_id: model.config.client_build_id,
				deployment_id: model.config.deployment_id,
				trigger: "revalidation",
			},
		],
	};
}

export function settle_refresh_success(model: ReadyModel): Transition {
	if (model.refresh.kind === "idle") {
		return { model, effects: [] };
	}
	const settle = settle_refresh_waiters(
		model.refresh.demand,
		REVALIDATION_OK,
	);
	const clear_timer: Effect[] =
		model.refresh.kind === "debouncing" || model.refresh.kind === "retrying"
			? [{ type: "clear_timer", id: model.refresh.timer_id }]
			: [];
	return {
		model: { ...model, refresh: { kind: "idle" } },
		effects: [...clear_timer, ...settle],
	};
}

export function settle_refresh_retry(model: ReadyModel): Transition {
	if (model.refresh.kind !== "running") {
		return { model, effects: [] };
	}
	const next_attempt = model.refresh.attempt + 1;
	if (next_attempt >= MAX_REVALIDATION_RETRIES) {
		return {
			model: { ...model, refresh: { kind: "idle" } },
			effects: settle_refresh_waiters(
				model.refresh.demand,
				REVALIDATION_EXHAUSTED,
			),
		};
	}
	const timer = next_refresh_timer(model.counters);
	return {
		model: {
			...model,
			counters: timer.counters,
			refresh: {
				kind: "retrying",
				demand: model.refresh.demand,
				attempt: next_attempt,
				timer_id: timer.id,
			},
		},
		effects: [
			{
				type: "start_timer",
				id: timer.id,
				ms: refresh_backoff_ms(next_attempt),
			},
		],
	};
}

export function settle_refresh_build_skew(model: ReadyModel): Transition {
	if (model.refresh.kind === "idle") {
		return { model, effects: [] };
	}
	const settle = settle_refresh_waiters(
		model.refresh.demand,
		REVALIDATION_BUILD_SKEW,
	);
	const clear_timer: Effect[] =
		model.refresh.kind === "debouncing" || model.refresh.kind === "retrying"
			? [{ type: "clear_timer", id: model.refresh.timer_id }]
			: [];
	return {
		model: { ...model, refresh: { kind: "idle" } },
		effects: [...clear_timer, ...settle],
	};
}

export function fire_refresh_timer(
	model: ReadyModel,
	timer_id: import("./events.ts").RefreshTimerID,
): Transition {
	if (
		model.refresh.kind !== "debouncing" &&
		model.refresh.kind !== "retrying"
	) {
		return { model, effects: [] };
	}
	if (model.refresh.timer_id !== timer_id) {
		return { model, effects: [] };
	}
	return {
		model: {
			...model,
			refresh: {
				kind: "pending",
				demand: model.refresh.demand,
				attempt:
					model.refresh.kind === "retrying"
						? model.refresh.attempt
						: 0,
			},
		},
		effects: [],
	};
}

/////////////////////////////////////////////////////////////////////
/////// Route Response
/////////////////////////////////////////////////////////////////////

export type SettleRouteResponseInput = {
	token: RouteToken;
	outcome: import("./events.ts").RouteFetchOutcome;
};

export function settle_route_response(
	model: ReadyModel,
	input: SettleRouteResponseInput,
	abort_handle_factory: () => AbortHandle,
): Transition {
	const active = model.active_route;
	if (active && active.token === input.token && active.phase === "fetching") {
		return settle_active_route_response(
			model,
			active,
			input.outcome,
			abort_handle_factory,
		);
	}
	const prefetch = model.prefetch;
	if (
		prefetch &&
		prefetch.token === input.token &&
		prefetch.phase === "fetching"
	) {
		return settle_prefetch_response(model, prefetch, input.outcome);
	}
	return { model, effects: [] };
}

function settle_active_route_response(
	model: ReadyModel,
	active: ActiveRoute & { phase: "fetching" },
	outcome: import("./events.ts").RouteFetchOutcome,
	abort_handle_factory: () => AbortHandle,
): Transition {
	if (active.intent.kind === "revalidation") {
		return settle_revalidation_response(
			model,
			active,
			outcome,
			abort_handle_factory,
		);
	}
	return settle_navigation_response(
		model,
		active,
		outcome,
		abort_handle_factory,
	);
}

function settle_revalidation_response(
	model: ReadyModel,
	active: ActiveRoute & { phase: "fetching" },
	outcome: import("./events.ts").RouteFetchOutcome,
	abort_handle_factory: () => AbortHandle,
): Transition {
	if (outcome.kind === "aborted") {
		return { model: { ...model, active_route: null }, effects: [] };
	}
	const skew_effects = route_skew_effects_for_active(model, active, outcome);
	if (outcome.kind === "build_skew") {
		const cleared = settle_refresh_build_skew(model);
		return {
			model: { ...cleared.model, active_route: null },
			effects: [...skew_effects, ...cleared.effects],
		};
	}
	if (outcome.kind === "network_error" || outcome.kind === "http_error") {
		const retried = settle_refresh_retry({ ...model, active_route: null });
		return {
			model: retried.model,
			effects: [...skew_effects, ...retried.effects],
		};
	}
	if (outcome.kind === "redirect_only") {
		const redirected = follow_revalidation_redirect(
			model,
			active,
			outcome.redirect,
			abort_handle_factory,
		);
		return {
			model: redirected.model,
			effects: [...skew_effects, ...redirected.effects],
		};
	}
	if (outcome.redirect) {
		const redirected = follow_revalidation_redirect(
			model,
			active,
			outcome.redirect,
			abort_handle_factory,
		);
		return {
			model: redirected.model,
			effects: [...skew_effects, ...redirected.effects],
		};
	}
	return {
		model: {
			...model,
			active_route: {
				phase: "preparing",
				token: active.token,
				abort_handle: active.abort_handle,
				url: active.url,
				intent: active.intent,
				redirect_count: active.redirect_count,
				sequence: active.sequence,
				payload: outcome.payload,
			},
		},
		effects: [
			...skew_effects,
			{
				type: "prepare_route",
				token: active.token,
				payload: outcome.payload,
				client_build_id:
					outcome.server_build_id || model.config.client_build_id,
				href: active.url,
				history_state: model.browser.state,
				abort_handle: active.abort_handle,
				trigger: "revalidation",
			},
		],
	};
}

function follow_revalidation_redirect(
	model: ReadyModel,
	active: ActiveRoute & { phase: "fetching" },
	redirect: { href: string; hard: boolean },
	abort_handle_factory: () => AbortHandle,
): Transition {
	if (
		redirect.hard ||
		!is_http_url(redirect.href) ||
		!is_same_origin(redirect.href, active.url)
	) {
		const cleared_refresh = settle_refresh_success({
			...model,
			active_route: null,
		});
		if (!is_http_url(redirect.href)) {
			return cleared_refresh;
		}
		return {
			model: cleared_refresh.model,
			effects: [
				...cleared_refresh.effects,
				{ type: "hard_redirect", url: redirect.href },
			],
		};
	}

	if (is_same_document(redirect.href, model.current.position.href)) {
		const after_clear: ReadyModel = { ...model, active_route: null };
		const sd = apply_same_document_navigation(after_clear, {
			target_href: redirect.href,
			replace: true,
			state: model.browser.state,
			source: "redirect",
			popstate_browser_key: null,
			popstate_restored_scroll: undefined,
		});
		const refresh_settled = settle_refresh_success(sd.model);
		return {
			model: refresh_settled.model,
			effects: [...sd.effects, ...refresh_settled.effects],
		};
	}

	const cleared_refresh = settle_refresh_success({
		...model,
		active_route: null,
	});
	const new_handle = abort_handle_factory();
	const seq = next_sequence(cleared_refresh.model.counters);
	const tok = next_route_token(seq.counters);
	const key = next_browser_key(tok.counters);
	const nav: NavigationIntent = {
		href: redirect.href,
		replace: true,
		scroll_to_top: undefined,
		state: model.browser.state,
		skip_work_indicator: false,
		source: "redirect",
		is_popstate: false,
		is_initial: false,
		popstate_restored_scroll: undefined,
		browser_key: key.key,
	};
	return {
		model: {
			...cleared_refresh.model,
			counters: key.counters,
			active_route: {
				phase: "fetching",
				token: tok.token,
				abort_handle: new_handle,
				url: redirect.href,
				intent: { kind: "navigation", nav, public_calls: [] },
				redirect_count: 0,
				sequence: seq.sequence,
			},
		},
		effects: [
			...cleared_refresh.effects,
			{
				type: "fetch_route",
				token: tok.token,
				url: redirect.href,
				abort_handle: new_handle,
				is_revalidation: false,
				client_build_id: model.config.client_build_id,
				deployment_id: model.config.deployment_id,
				trigger: "navigation",
			},
		],
	};
}

function settle_navigation_response(
	model: ReadyModel,
	active: ActiveRoute & { phase: "fetching" },
	outcome: import("./events.ts").RouteFetchOutcome,
	abort_handle_factory: () => AbortHandle,
): Transition {
	if (outcome.kind === "aborted") {
		return { model: { ...model, active_route: null }, effects: [] };
	}
	const skew_effects = route_skew_effects_for_active(model, active, outcome);
	if (outcome.kind === "build_skew") {
		return {
			model: { ...model, active_route: null },
			effects: [
				...skew_effects,
				{ type: "hard_redirect", url: active.url },
				...settle_navigation_calls_unsuccessful(active),
			],
		};
	}
	if (outcome.kind === "network_error" || outcome.kind === "http_error") {
		return {
			model: { ...model, active_route: null },
			effects: [
				...skew_effects,
				...popstate_reload_effects(active),
				...settle_navigation_calls_unsuccessful(active),
			],
		};
	}
	if (outcome.kind === "redirect_only") {
		const redirected = follow_route_redirect(
			model,
			active,
			outcome.redirect,
			abort_handle_factory,
		);
		return {
			model: redirected.model,
			effects: [...skew_effects, ...redirected.effects],
		};
	}
	if (outcome.redirect) {
		const redirected = follow_route_redirect(
			model,
			active,
			outcome.redirect,
			abort_handle_factory,
		);
		return {
			model: redirected.model,
			effects: [...skew_effects, ...redirected.effects],
		};
	}
	if (active.intent.kind !== "navigation") {
		return { model: { ...model, active_route: null }, effects: [] };
	}
	return {
		model: {
			...model,
			active_route: {
				phase: "preparing",
				token: active.token,
				abort_handle: active.abort_handle,
				url: active.url,
				intent: active.intent,
				redirect_count: active.redirect_count,
				sequence: active.sequence,
				payload: outcome.payload,
			},
		},
		effects: [
			...skew_effects,
			{
				type: "prepare_route",
				token: active.token,
				payload: outcome.payload,
				client_build_id:
					outcome.server_build_id || model.config.client_build_id,
				href: active.url,
				history_state: active.intent.nav.state,
				abort_handle: active.abort_handle,
				trigger: "navigation",
			},
		],
	};
}

function settle_prefetch_response(
	model: ReadyModel,
	prefetch: Prefetch & { phase: "fetching" },
	outcome: import("./events.ts").RouteFetchOutcome,
): Transition {
	const skew_effects = route_skew_effects_for_prefetch(
		model,
		prefetch,
		outcome,
	);
	if (outcome.kind === "data" && !outcome.redirect) {
		return {
			model: {
				...model,
				prefetch: {
					phase: "preparing",
					token: prefetch.token,
					abort_handle: prefetch.abort_handle,
					url: prefetch.url,
					payload: outcome.payload,
				},
			},
			effects: [
				...skew_effects,
				{
					type: "prepare_route",
					token: prefetch.token,
					payload: outcome.payload,
					client_build_id:
						outcome.server_build_id || model.config.client_build_id,
					href: prefetch.url,
					history_state: undefined,
					abort_handle: prefetch.abort_handle,
					trigger: "prefetch",
				},
			],
		};
	}
	return { model: { ...model, prefetch: null }, effects: skew_effects };
}

function follow_route_redirect(
	model: ReadyModel,
	active: ActiveRoute & { phase: "fetching" },
	redirect: { href: string; hard: boolean },
	abort_handle_factory: () => AbortHandle,
): Transition {
	if (
		redirect.hard ||
		!is_http_url(redirect.href) ||
		!is_same_origin(redirect.href, active.url)
	) {
		const settle = settle_navigation_calls_unsuccessful(active);
		if (!is_http_url(redirect.href)) {
			return { model: { ...model, active_route: null }, effects: settle };
		}
		return {
			model: { ...model, active_route: null },
			effects: [{ type: "hard_redirect", url: redirect.href }, ...settle],
		};
	}

	if (active.redirect_count >= MAX_REDIRECTS) {
		return {
			model: { ...model, active_route: null },
			effects: [
				{
					type: "warn_redirect_loop",
					url: redirect.href,
					redirect_count: active.redirect_count + 1,
				},
				...settle_navigation_calls_unsuccessful(active),
			],
		};
	}

	if (is_same_document(redirect.href, model.current.position.href)) {
		const cleared = { ...model, active_route: null } as ReadyModel;
		const call_ids =
			active.intent.kind === "navigation"
				? active.intent.public_calls
				: [];
		const sd = apply_same_document_navigation(cleared, {
			target_href: redirect.href,
			replace:
				active.intent.kind === "navigation"
					? active.intent.nav.replace
					: true,
			state:
				active.intent.kind === "navigation"
					? active.intent.nav.state
					: model.browser.state,
			source: "redirect",
			popstate_browser_key: null,
			popstate_restored_scroll: undefined,
		});
		const settle: Effect[] = call_ids.map((cid) => {
			return {
				type: "settle_navigation_call",
				call_id: cid,
				result: { didNavigate: sd.did_navigate },
			};
		});
		return {
			model: sd.model,
			effects: [...sd.effects, ...settle],
		};
	}

	const cleared: ReadyModel = { ...model, active_route: null };
	const call_ids =
		active.intent.kind === "navigation" ? active.intent.public_calls : [];
	const new_handle = abort_handle_factory();
	const seq = next_sequence(cleared.counters);
	const tok = next_route_token(seq.counters);
	const key = next_browser_key(tok.counters);
	const nav: NavigationIntent = {
		href: redirect.href,
		replace:
			active.intent.kind === "navigation"
				? active.intent.nav.replace
				: true,
		scroll_to_top:
			active.intent.kind === "navigation"
				? active.intent.nav.scroll_to_top
				: undefined,
		state:
			active.intent.kind === "navigation"
				? active.intent.nav.state
				: model.browser.state,
		skip_work_indicator:
			active.intent.kind === "navigation"
				? active.intent.nav.skip_work_indicator
				: false,
		source: "redirect",
		is_popstate: false,
		is_initial: false,
		popstate_restored_scroll: undefined,
		browser_key: key.key,
	};
	return {
		model: {
			...cleared,
			counters: key.counters,
			active_route: {
				phase: "fetching",
				token: tok.token,
				abort_handle: new_handle,
				url: redirect.href,
				intent: { kind: "navigation", nav, public_calls: call_ids },
				redirect_count: active.redirect_count + 1,
				sequence: seq.sequence,
			},
		},
		effects: [
			{
				type: "fetch_route",
				token: tok.token,
				url: redirect.href,
				abort_handle: new_handle,
				is_revalidation: false,
				client_build_id: model.config.client_build_id,
				deployment_id: model.config.deployment_id,
				trigger: "navigation",
			},
		],
	};
}

/////////////////////////////////////////////////////////////////////
/////// Route Preparation
/////////////////////////////////////////////////////////////////////

export type SettleRoutePreparationInput = {
	token: RouteToken;
	outcome: import("./events.ts").RoutePreparationOutcome;
};

export function settle_route_preparation(
	model: ReadyModel | BootingModel,
	input: SettleRoutePreparationInput,
): Transition<ReadyModel | BootingModel> {
	const active = model.active_route;
	if (
		active &&
		active.token === input.token &&
		active.phase === "preparing"
	) {
		return settle_active_route_preparation(model, active, input.outcome);
	}
	if (model.phase === "ready") {
		const prefetch = model.prefetch;
		if (
			prefetch &&
			prefetch.token === input.token &&
			prefetch.phase === "preparing"
		) {
			return settle_prefetch_preparation(model, prefetch, input.outcome);
		}
	}
	return { model, effects: [] };
}

function settle_active_route_preparation(
	model: ReadyModel | BootingModel,
	active: ActiveRoute & { phase: "preparing" },
	outcome: import("./events.ts").RoutePreparationOutcome,
): Transition<ReadyModel | BootingModel> {
	if (outcome.kind === "aborted") {
		if (model.phase === "booting") {
			return {
				model: { ...model, active_route: null },
				effects: [
					{
						type: "settle_boot",
						result: {
							ok: false,
							err: "Boot route preparation was aborted.",
						},
					},
				],
			};
		}
		return {
			model: { ...model, active_route: null } as
				| ReadyModel
				| BootingModel,
			effects: [],
		};
	}
	if (outcome.kind === "failed") {
		if (active.intent.kind === "revalidation") {
			if (model.phase !== "ready") {
				return { model, effects: [] };
			}
			return settle_refresh_retry({ ...model, active_route: null });
		}
		if (model.phase === "booting") {
			return {
				model: { ...model, active_route: null },
				effects: [
					{
						type: "settle_boot",
						result: {
							ok: false,
							err: outcome.error,
						},
					},
				],
			};
		}
		return {
			model: { ...model, active_route: null } as
				| ReadyModel
				| BootingModel,
			effects: [
				...popstate_reload_effects(active),
				...settle_navigation_calls_unsuccessful(active),
			],
		};
	}
	const next_active: ActiveRoute = {
		phase: "publishing",
		token: active.token,
		abort_handle: active.abort_handle,
		url: active.url,
		intent: active.intent,
		redirect_count: active.redirect_count,
		sequence: active.sequence,
		prepared: outcome.prepared,
	};
	const publish_effect = build_publish_effect(
		model,
		active.token,
		active.abort_handle,
		outcome.prepared,
		active.url,
		active.intent.kind === "navigation"
			? active.intent.nav.state
			: model.browser.state,
		active.intent,
	);
	return {
		model: { ...model, active_route: next_active } as
			| ReadyModel
			| BootingModel,
		effects: [publish_effect],
	};
}

function settle_prefetch_preparation(
	model: ReadyModel,
	prefetch: Prefetch & { phase: "preparing" },
	outcome: import("./events.ts").RoutePreparationOutcome,
): Transition {
	if (outcome.kind === "aborted" || outcome.kind === "failed") {
		return { model: { ...model, prefetch: null }, effects: [] };
	}
	return {
		model: {
			...model,
			prefetch: {
				phase: "prepared",
				token: prefetch.token,
				url: prefetch.url,
				prepared: outcome.prepared,
			},
		},
		effects: [],
	};
}

/////////////////////////////////////////////////////////////////////
/////// Publication Commit
/////////////////////////////////////////////////////////////////////

export type CommitPublicationInput = {
	token: RouteToken;
	now_ms: number;
};

export function commit_publication(
	model: ReadyModel | BootingModel,
	input: CommitPublicationInput,
): Transition<ReadyModel | BootingModel> {
	const active = model.active_route;
	if (
		!active ||
		active.token !== input.token ||
		active.phase !== "publishing"
	) {
		return { model, effects: [] };
	}

	const intent = active.intent;
	const target_href = active.url;
	const new_state =
		intent.kind === "navigation" ? intent.nav.state : model.browser.state;
	const new_key =
		intent.kind === "navigation"
			? intent.nav.browser_key
			: model.browser.key;

	const new_position: BrowserPosition = {
		href: target_href,
		key: new_key,
		state: new_state,
	};
	const next_current: CurrentRoute = {
		position: new_position,
		prepared: active.prepared,
		route_state: route_state_from_prepared(
			active.prepared,
			target_href,
			new_state,
		),
		sequence: active.sequence,
	};

	const settle_calls: Effect[] =
		intent.kind === "navigation"
			? intent.public_calls.map((cid) => {
					return {
						type: "settle_navigation_call",
						call_id: cid,
						result: { didNavigate: true },
					};
				})
			: [];

	const promoted: ReadyModel =
		model.phase === "booting"
			? {
					phase: "ready",
					config: model.config,
					browser: new_position,
					current: next_current,
					active_route: null,
					prefetch: null,
					refresh: { kind: "idle" },
					submissions: model.submissions,
					submissions_by_dedupe: model.submissions_by_dedupe,
					deferred_api_redirect: model.deferred_api_redirect,
					counters: model.counters,
					activity: { last_activity_ms: input.now_ms },
					work_indicator: model.work_indicator,
				}
			: {
					...model,
					browser: new_position,
					current: next_current,
					active_route: null,
					activity: { last_activity_ms: input.now_ms },
				};

	const demand = refresh_demand_of(promoted.refresh);
	if (
		demand &&
		active.sequence >= demand.after_sequence &&
		is_same_document(next_current.route_state.href, new_position.href)
	) {
		const settled = settle_refresh_success(promoted);
		return {
			model: settled.model,
			effects: [...settle_calls, ...settled.effects],
		};
	}

	return {
		model: promoted,
		effects: settle_calls,
	};
}

export function fail_publication(
	model: ReadyModel | BootingModel,
	token: RouteToken,
	error = "Route publication failed.",
): Transition<ReadyModel | BootingModel> {
	const active = model.active_route;
	if (!active || active.token !== token || active.phase !== "publishing") {
		return { model, effects: [] };
	}
	if (model.phase === "booting") {
		return {
			model: { ...model, active_route: null },
			effects: [
				{
					type: "settle_boot",
					result: {
						ok: false,
						err: error,
					},
				},
			],
		};
	}
	return {
		model: { ...model, active_route: null } as ReadyModel | BootingModel,
		effects: [
			...popstate_reload_effects(active),
			...settle_navigation_calls_unsuccessful(active),
		],
	};
}

/////////////////////////////////////////////////////////////////////
/////// Boot Revalidation
/////////////////////////////////////////////////////////////////////

export function pickup_pending_boot_revalidations(
	model: ReadyModel,
	pending: PendingBootRevalidations | null,
): Transition {
	if (!pending || pending.count === 0) {
		return { model, effects: [] };
	}
	const seq = next_sequence(model.counters);
	const demand: RefreshDemand = {
		after_sequence: seq.sequence,
		reason: "apiRequest",
		skip_work_indicator: pending.skip_work_indicator,
		waiters: pending.waiter_call_ids.map((call_id) => {
			return { call_id };
		}),
	};
	return {
		model: {
			...model,
			counters: seq.counters,
			refresh: { kind: "pending", demand, attempt: 0 },
		},
		effects: [],
	};
}

/////////////////////////////////////////////////////////////////////
/////// Prefetch
/////////////////////////////////////////////////////////////////////

export type BeginPrefetchInput = { href: string };

export function begin_prefetch(
	model: ReadyModel,
	input: BeginPrefetchInput,
	abort_handle: AbortHandle,
): Transition {
	const target = (() => {
		try {
			return new URL(input.href, model.browser.href).href;
		} catch {
			return null;
		}
	})();
	if (!target) {
		return { model, effects: [] };
	}
	if (!is_http_url(target) || !is_same_origin(target, model.browser.href)) {
		return { model, effects: [] };
	}
	if (is_same_document(target, model.current.position.href)) {
		return { model, effects: [] };
	}
	if (
		model.active_route &&
		is_same_document(target, model.active_route.url)
	) {
		return { model, effects: [] };
	}
	if (model.prefetch && is_same_document(target, model.prefetch.url)) {
		return { model, effects: [] };
	}

	const cancel: Effect[] =
		model.prefetch && model.prefetch.phase !== "prepared"
			? [{ type: "abort", handle: model.prefetch.abort_handle }]
			: [];
	const tok = next_route_token(model.counters);
	return {
		model: {
			...model,
			counters: tok.counters,
			prefetch: {
				phase: "fetching",
				token: tok.token,
				abort_handle,
				url: target,
			},
		},
		effects: [
			...cancel,
			{
				type: "fetch_route",
				token: tok.token,
				url: target,
				abort_handle,
				is_revalidation: false,
				client_build_id: model.config.client_build_id,
				deployment_id: model.config.deployment_id,
				trigger: "prefetch",
			},
		],
	};
}

export function cancel_prefetch_by_href(
	model: ReadyModel,
	href: string,
): Transition {
	if (!model.prefetch) {
		return { model, effects: [] };
	}
	const target = (() => {
		try {
			return new URL(href, model.browser.href).href;
		} catch {
			return null;
		}
	})();
	if (!target || !is_same_document(target, model.prefetch.url)) {
		return { model, effects: [] };
	}
	const effects: Effect[] =
		model.prefetch.phase !== "prepared"
			? [{ type: "abort", handle: model.prefetch.abort_handle }]
			: [];
	return { model: { ...model, prefetch: null }, effects };
}

/////////////////////////////////////////////////////////////////////
/////// Popstate
/////////////////////////////////////////////////////////////////////

export type BeginCrossDocumentPopstateInput = {
	target_href: string;
	state: unknown;
	browser_key: BrowserKey;
	restored_scroll: ScrollState | undefined;
};

export function begin_cross_document_popstate(
	model: ReadyModel,
	input: BeginCrossDocumentPopstateInput,
	abort_handle: AbortHandle,
): Transition {
	const cleared = supersede_active_route(model);

	if (
		cleared.model.prefetch &&
		is_same_document(cleared.model.prefetch.url, input.target_href)
	) {
		const promoted = promote_prefetch_to_active({
			pf: cleared.model.prefetch,
			model: cleared.model,
			target_href: input.target_href,
			intent: {
				kind: "navigation",
				nav: {
					href: input.target_href,
					replace: true,
					scroll_to_top: true,
					state: input.state,
					skip_work_indicator: true,
					source: "navigate",
					is_popstate: true,
					is_initial: false,
					popstate_restored_scroll: input.restored_scroll,
					browser_key: input.browser_key,
				},
				public_calls: [],
			},
			state: input.state,
			is_popstate: true,
		});
		return {
			model: promoted.model,
			effects: [...cleared.effects, ...promoted.effects],
		};
	}

	const cancel_prefetch_effects: Effect[] =
		cleared.model.prefetch && cleared.model.prefetch.phase !== "prepared"
			? [{ type: "abort", handle: cleared.model.prefetch.abort_handle }]
			: [];

	const seq = next_sequence(cleared.model.counters);
	const tok = next_route_token(seq.counters);
	const nav: NavigationIntent = {
		href: input.target_href,
		replace: true,
		scroll_to_top: true,
		state: input.state,
		skip_work_indicator: true,
		source: "navigate",
		is_popstate: true,
		is_initial: false,
		popstate_restored_scroll: input.restored_scroll,
		browser_key: input.browser_key,
	};
	return {
		model: {
			...cleared.model,
			counters: tok.counters,
			prefetch: null,
			active_route: {
				phase: "fetching",
				token: tok.token,
				abort_handle,
				url: input.target_href,
				intent: { kind: "navigation", nav, public_calls: [] },
				redirect_count: 0,
				sequence: seq.sequence,
			},
		},
		effects: [
			...cleared.effects,
			...cancel_prefetch_effects,
			{
				type: "fetch_route",
				token: tok.token,
				url: input.target_href,
				abort_handle,
				is_revalidation: false,
				client_build_id: model.config.client_build_id,
				deployment_id: model.config.deployment_id,
				trigger: "popstate",
			},
		],
	};
}

/////////////////////////////////////////////////////////////////////
/////// API Submissions
/////////////////////////////////////////////////////////////////////

export type BeginAPISubmissionInput = {
	href: string;
	method: string;
	route_kind: APIRouteKind;
	dedupe_key: string | undefined;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
	call_id: PublicCallID;
	request_init: RequestInit | undefined;
};

export function begin_api_submission(
	model: ReadyModel | BootingModel,
	input: BeginAPISubmissionInput,
	abort_handle: AbortHandle,
): Transition<ReadyModel | BootingModel> {
	const base = model.browser.href;
	const target = (() => {
		try {
			return new URL(input.href, base).href;
		} catch {
			return null;
		}
	})();
	if (!target || !is_same_origin(target, base)) {
		return {
			model,
			effects: [
				settle_submit_effect(input.call_id, {
					success: false,
					error: `submit only supports same-origin targets. Received: "${target ?? input.href}".`,
					revalidation_call_id: null,
				}),
			],
		};
	}

	let working_model: ReadyModel | BootingModel = model;
	let setup_effects: Effect[] = [];
	if (input.dedupe_key) {
		const prior_token = model.submissions_by_dedupe[input.dedupe_key];
		if (prior_token) {
			const prior = model.submissions[prior_token];
			if (prior) {
				const next_submissions = { ...model.submissions };
				delete next_submissions[prior_token];
				const next_dedupe = { ...model.submissions_by_dedupe };
				delete next_dedupe[input.dedupe_key];
				working_model = {
					...model,
					submissions: next_submissions,
					submissions_by_dedupe: next_dedupe,
				};
				const reval = schedule_api_revalidation(working_model, prior);
				working_model = reval.model;
				setup_effects = [
					{ type: "abort", handle: prior.abort_handle },
					...reval.effects,
					settle_submit_effect(prior.public_call_id, {
						success: false,
						error: "Aborted",
						revalidation_call_id: prior.revalidation_call_id,
					}),
				];
			}
		}
	}

	let revalidation_call_id: PublicCallID | null = null;
	let counters_for_token = working_model.counters;
	if (input.should_revalidate && working_model.phase === "ready") {
		const reval_call = next_public_call(counters_for_token);
		revalidation_call_id = reval_call.call_id;
		counters_for_token = reval_call.counters;
	}

	const tok = next_api_token(counters_for_token);
	const submission: Submission = {
		token: tok.token,
		abort_handle,
		dedupe_key: input.dedupe_key,
		href: target,
		method: input.method,
		route_kind: input.route_kind,
		should_revalidate: input.should_revalidate,
		skip_work_indicator: input.skip_work_indicator,
		public_call_id: input.call_id,
		revalidation_call_id,
	};

	const next_submissions = {
		...working_model.submissions,
		[tok.token]: submission,
	};
	const next_dedupe = input.dedupe_key
		? {
				...working_model.submissions_by_dedupe,
				[input.dedupe_key]: tok.token,
			}
		: working_model.submissions_by_dedupe;

	return {
		model: {
			...working_model,
			counters: tok.counters,
			submissions: next_submissions,
			submissions_by_dedupe: next_dedupe,
		} as ReadyModel | BootingModel,
		effects: [
			...setup_effects,
			{
				type: "fetch_api",
				token: tok.token,
				url: target,
				method: input.method,
				request_init: input.request_init,
				abort_handle,
				deployment_id: model.config.deployment_id,
				route_kind: input.route_kind,
			},
		],
	};
}

export type SettleAPISubmissionInput = {
	token: APIToken;
	outcome: import("./events.ts").APIFetchOutcome;
};

export function settle_api_submission(
	model: ReadyModel | BootingModel,
	input: SettleAPISubmissionInput,
): Transition<ReadyModel | BootingModel> {
	const submission = model.submissions[input.token];
	if (!submission) {
		return { model, effects: [] };
	}
	const next_submissions = { ...model.submissions };
	delete next_submissions[input.token];
	const next_dedupe = submission.dedupe_key
		? (() => {
				const copy = { ...model.submissions_by_dedupe };
				if (copy[submission.dedupe_key!] === input.token) {
					delete copy[submission.dedupe_key!];
				}
				return copy;
			})()
		: model.submissions_by_dedupe;

	const cleared: ReadyModel | BootingModel = {
		...model,
		submissions: next_submissions,
		submissions_by_dedupe: next_dedupe,
	} as ReadyModel | BootingModel;
	const skew_effects = api_skew_effects(model, submission, input.outcome);

	return apply_api_outcome(cleared, submission, input.outcome, skew_effects);
}

function apply_api_outcome(
	model: ReadyModel | BootingModel,
	submission: Submission,
	outcome: import("./events.ts").APIFetchOutcome,
	skew_effects: readonly Effect[],
): Transition<ReadyModel | BootingModel> {
	if (outcome.kind === "aborted") {
		const reval = outcome.dispatched
			? schedule_api_revalidation(model, submission)
			: {
					model,
					effects: settle_revalidation_ok_effects(
						submission.revalidation_call_id ?? undefined,
					),
				};
		return {
			model: reval.model,
			effects: [
				...reval.effects,
				settle_submit_effect(submission.public_call_id, {
					success: false,
					error: "Aborted",
					revalidation_call_id: submission.revalidation_call_id,
				}),
			],
		};
	}
	if (outcome.kind === "network_error") {
		const reval = outcome.dispatched
			? schedule_api_revalidation(model, submission)
			: {
					model,
					effects: settle_revalidation_ok_effects(
						submission.revalidation_call_id ?? undefined,
					),
				};
		return {
			model: reval.model,
			effects: [
				...reval.effects,
				settle_submit_effect(submission.public_call_id, {
					success: false,
					error: outcome.error,
					revalidation_call_id: submission.revalidation_call_id,
				}),
			],
		};
	}
	if (outcome.kind === "http_error") {
		const reval = schedule_api_revalidation(model, submission);
		return {
			model: reval.model,
			effects: [
				...skew_effects,
				...reval.effects,
				settle_submit_effect(submission.public_call_id, {
					success: false,
					error: outcome.status_text,
					revalidation_call_id: submission.revalidation_call_id,
				}),
			],
		};
	}
	if (outcome.kind === "redirect") {
		const redirected = apply_api_redirect(
			model,
			submission,
			outcome.redirect,
		);
		return {
			model: redirected.model,
			effects: [...skew_effects, ...redirected.effects],
		};
	}
	if (outcome.redirect) {
		const redirected = apply_api_redirect(
			model,
			submission,
			outcome.redirect,
		);
		return {
			model: redirected.model,
			effects: [...skew_effects, ...redirected.effects],
		};
	}
	const reval = schedule_api_revalidation(model, submission);
	return {
		model: reval.model,
		effects: [
			...skew_effects,
			...reval.effects,
			settle_submit_effect(submission.public_call_id, {
				success: true,
				data: outcome.data,
				revalidation_call_id: submission.revalidation_call_id,
			}),
		],
	};
}

function apply_api_redirect(
	model: ReadyModel | BootingModel,
	submission: Submission,
	redirect: { href: string; hard: boolean },
): Transition<ReadyModel | BootingModel> {
	if (!is_http_url(redirect.href)) {
		return {
			model,
			effects: [
				settle_submit_effect(submission.public_call_id, {
					success: false,
					error: `Redirect target must use an HTTP(S) scheme. Received: "${redirect.href}".`,
					revalidation_call_id: submission.revalidation_call_id,
				}),
				...settle_revalidation_ok_effects(
					submission.revalidation_call_id ?? undefined,
				),
			],
		};
	}
	if (redirect.hard || !is_same_origin(redirect.href, model.browser.href)) {
		return {
			model,
			effects: [
				{ type: "hard_redirect", url: redirect.href },
				settle_submit_effect(submission.public_call_id, {
					success: true,
					data: undefined,
					revalidation_call_id: submission.revalidation_call_id,
				}),
				...settle_revalidation_ok_effects(
					submission.revalidation_call_id ?? undefined,
				),
			],
		};
	}
	if (model.phase !== "ready") {
		return {
			model: {
				...model,
				deferred_api_redirect: { href: redirect.href },
			} as ReadyModel | BootingModel,
			effects: [
				settle_submit_effect(submission.public_call_id, {
					success: true,
					data: undefined,
					revalidation_call_id: submission.revalidation_call_id,
				}),
				...settle_revalidation_ok_effects(
					submission.revalidation_call_id ?? undefined,
				),
			],
		};
	}
	return {
		model,
		effects: [
			settle_submit_effect(submission.public_call_id, {
				success: true,
				data: undefined,
				revalidation_call_id: submission.revalidation_call_id,
			}),
			...settle_revalidation_ok_effects(
				submission.revalidation_call_id ?? undefined,
			),
			{
				type: "dispatch_navigation",
				href: redirect.href,
				replace: true,
				state: undefined,
			},
		],
	};
}

function schedule_api_revalidation(
	model: ReadyModel | BootingModel,
	submission: Submission,
): Transition<ReadyModel | BootingModel> {
	if (!submission.should_revalidate) {
		return { model, effects: [] };
	}
	if (model.phase !== "ready") {
		const previous = (model as BootingModel).pending_boot_revalidations ?? {
			count: 0,
			waiter_call_ids: [],
			skip_work_indicator: true,
		};
		const next: PendingBootRevalidations = {
			count: previous.count + 1,
			waiter_call_ids: submission.revalidation_call_id
				? [...previous.waiter_call_ids, submission.revalidation_call_id]
				: previous.waiter_call_ids,
			skip_work_indicator:
				previous.skip_work_indicator && submission.skip_work_indicator,
		};
		return {
			model: {
				...(model as BootingModel),
				pending_boot_revalidations: next,
			} as ReadyModel | BootingModel,
			effects: [],
		};
	}
	const scheduled = schedule_refresh(model, {
		reason: "apiRequest",
		skip_work_indicator: submission.skip_work_indicator,
		waiter_call_id: submission.revalidation_call_id ?? undefined,
		debounce: false,
	});
	return scheduled as Transition<ReadyModel | BootingModel>;
}

/////////////////////////////////////////////////////////////////////
/////// Deferred Redirects
/////////////////////////////////////////////////////////////////////

export function take_deferred_api_redirect(model: ReadyModel): {
	model: ReadyModel;
	redirect: { href: string } | null;
} {
	const redirect = model.deferred_api_redirect;
	if (!redirect) {
		return { model, redirect: null };
	}
	return {
		model: { ...model, deferred_api_redirect: null },
		redirect,
	};
}

/////////////////////////////////////////////////////////////////////
/////// HMR
/////////////////////////////////////////////////////////////////////

export function apply_hmr_route_update(
	model: ReadyModel,
	event: HMRRouteUpdateInput,
): Transition {
	if (model.current.sequence !== event.route_sequence) {
		return { model, effects: [] };
	}
	const match = model.current.prepared.matches[event.match_idx];
	if (!match || match.module_url !== event.module_url) {
		return { model, effects: [] };
	}
	const next_matches = model.current.prepared.matches.map((current, idx) => {
		if (idx !== event.match_idx) {
			return current;
		}
		return {
			...current,
			client_loader_data: event.client_loader_data,
		};
	});
	const next_prepared: PreparedRoute = {
		...model.current.prepared,
		matches: next_matches,
	};
	const route = route_state_from_prepared(
		next_prepared,
		model.current.position.href,
		model.current.position.state,
	);
	const prev = model.current.route_state;
	const next_current: CurrentRoute = {
		position: model.current.position,
		prepared: next_prepared,
		route_state: route,
		sequence: model.current.sequence,
	};
	const commit: ClientCommitPlan = {
		route_render: {
			state: render_plan_from_prepared(
				next_prepared,
				model.current.position.state,
			),
		},
	};
	if (!jsonDeepEquals(prev, route)) {
		commit.route_update = {
			previous_route: prev,
			reason: "revalidation",
			route,
		};
	}
	return {
		model: { ...model, current: next_current },
		effects: [{ type: "emit_client_commit", commit }],
	};
}

export type BeginBootInput = {
	href: string;
	browser_key: BrowserKey;
	browser_state: unknown;
	payload: import("./model.ts").RoutePayload;
	restored_scroll: ScrollState | undefined;
	options: {
		use_view_transitions: boolean;
		revalidate_on_focus: {
			stale_time_ms: number;
			skip_work_indicator: boolean;
		} | null;
		work_indicator: WorkIndicatorPolicy | null;
	};
	now_ms: number;
};

export function begin_boot(
	input: BeginBootInput,
	abort_handle: AbortHandle,
): Transition<BootingModel> {
	const counters_0 = empty_counters();
	const seq = next_sequence(counters_0);
	const tok = next_route_token(seq.counters);
	const client_build_id = input.payload.server_build_id;
	const deployment_id = input.payload.deployment_id;

	const browser_position: BrowserPosition = {
		href: input.href,
		key: input.browser_key,
		state: input.browser_state,
	};
	const nav: NavigationIntent = {
		href: input.href,
		replace: true,
		scroll_to_top: true,
		state: input.browser_state,
		skip_work_indicator: true,
		source: "navigate",
		is_popstate: false,
		is_initial: true,
		popstate_restored_scroll: input.restored_scroll,
		browser_key: input.browser_key,
	};

	return {
		model: {
			phase: "booting",
			config: {
				client_build_id,
				deployment_id,
				use_view_transitions: input.options.use_view_transitions,
				revalidate_on_focus: input.options.revalidate_on_focus,
				work_indicator: input.options.work_indicator,
			},
			browser: browser_position,
			active_route: {
				phase: "preparing",
				token: tok.token,
				abort_handle,
				url: input.href,
				intent: { kind: "navigation", nav, public_calls: [] },
				redirect_count: 0,
				sequence: seq.sequence,
				payload: input.payload,
			},
			current: null,
			submissions: {},
			submissions_by_dedupe: {},
			pending_boot_revalidations: null,
			deferred_api_redirect: null,
			counters: tok.counters,
			activity: { last_activity_ms: input.now_ms },
			work_indicator: { should_be_active: false },
		},
		effects: [
			{
				type: "prepare_route",
				token: tok.token,
				payload: input.payload,
				client_build_id:
					input.payload.server_build_id || client_build_id,
				href: input.href,
				history_state: input.browser_state,
				abort_handle,
				trigger: "boot",
			},
		],
	};
}
