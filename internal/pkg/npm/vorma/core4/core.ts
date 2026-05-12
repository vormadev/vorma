/// <reference types="vite/client" />

import { jsonDeepEquals, parseSearchParams } from "vorma/kit/json";
import {
	createPatternRegistry,
	findNestedMatches,
	registerPattern,
	type PatternRegistry,
} from "vorma/kit/matcher";
import { R, type Result } from "vorma/kit/result";
import {
	API_SUBMIT_CROSS_ORIGIN_ERROR,
	BUILD_ID_HEADER,
	CONTENT_TYPE_HEADER,
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	JSON_CONTENT_TYPE,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	VORMA_ROOT_EL_ID,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import type {
	ClientCommit,
	ClientCore,
	ClientOptions,
	CommitFn,
	RouteRenderState,
	ScrollIntent,
	ScrollState,
	ViewDefinition,
	WorkIndicatorOptions,
} from "../core/create_client_core.ts";
import { apply_css_bundles, preload_css, wait_for_css } from "../core/css.ts";
import { apply_head_and_title, type HeadEl } from "../core/head.ts";
import { preload_modules } from "../core/modules.ts";
import type {
	AppConfig,
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RouteErrorState,
} from "../core/types.ts";
import type {
	ActiveRouteSlot,
	APIResponseClassificationInput,
	APIRouteKind,
	APISubmissionBuildSkewReport,
	APISubmissionOutcome,
	APISubmissionRequest,
	APISubmissionResult,
	APISubmissionTransition,
	BootRequest,
	BrowserKey,
	BrowserPosition,
	BuildSkewDefaultBehavior,
	BuildSkewNotification,
	BuildSkewResponseFacts,
	BuildSkewTriggeringResponse,
	ClientLoaderKnownMatch,
	Core4APIResult,
	Core4BootingInit,
	Core4BootingModel,
	Core4ClientLoader,
	Core4ClientLoaderPrefetch,
	Core4ClientLoaderServerState,
	Core4Effect,
	Core4FetchResult,
	Core4Init,
	Core4Model,
	Core4PreparedRenderPayload,
	Core4ReadyInit,
	Core4ReadyModel,
	Core4ResponseFacts,
	Core4ScrollState,
	Core4TestOptions,
	Core4Token,
	Core4Transition,
	Core4WorkIndicatorController,
	Core4WorkProjection,
	Deferred,
	DeferredAPIRedirectRequest,
	NavigationActiveRouteSlot,
	NavigationRequest,
	NavigationStartTransition,
	PopstateRequest,
	PrefetchRequest,
	PrefetchSlot,
	PreparedRoute,
	PublicationCommit,
	PublicationHistoryAction,
	PublicationHookPlan,
	PublicationOptions,
	PublicationPlan,
	PublicationReason,
	PublicationScrollPlan,
	PublicationSlot,
	PublicCallID,
	RefreshDemand,
	RefreshSlot,
	RevalidationRequest,
	RevalidationResult,
	RevalidationStartRequest,
	RouteBuildSkewReport,
	RoutePayload,
	RoutePreparationOutcome,
	RoutePreparationTransition,
	RouteResponseClassificationInput,
	RouteResponseOutcome,
	RouteResponseOwner,
	RouteResponseTransition,
	RouteState,
	RouteUpdateReason,
	SubmissionSlot,
	TimerID,
	WorkState,
} from "./types.ts";

export const CORE4_MAX_REDIRECTS = 10;
export const CORE4_REVALIDATION_BACKOFF_BASE_MS = 500;
export const CORE4_REVALIDATION_BACKOFF_CAP_MS = 30000;
export const CORE4_REVALIDATION_MAX_RETRIES = 8;

const api_invalid_redirect_error_prefix =
	"Redirect target must use an HTTP(S) scheme. Received:";
const api_aborted_error = "Aborted";
const abort_error_name = "AbortError";
const route_id_separator = ":";

export function create_core4_model(input: Core4ReadyInit): Core4ReadyModel;
export function create_core4_model(input: Core4BootingInit): Core4BootingModel;
export function create_core4_model(input: Core4Init): Core4Model {
	const base = {
		active_route: null,
		client_build_id: input.client_build_id,
		deferred_api_redirect: input.deferred_api_redirect ?? null,
		prefetch: null,
		publication: null,
		refresh: { kind: "idle" as const },
		sequence: input.current?.sequence ?? 0,
		submissions: {},
		use_view_transitions: input.use_view_transitions ?? false,
	};
	if (input.phase === "ready") {
		return {
			...base,
			browser: input.browser,
			current: input.current,
			phase: "ready",
		};
	}
	return {
		...base,
		browser: input.browser ?? null,
		current: input.current ?? null,
		phase: "booting",
	};
}

export function initialize_core4_boot(
	model: Core4Model,
	input: {
		browser: BrowserPosition;
		client_build_id: string;
		use_view_transitions: boolean;
	},
): Core4Transition<"boot_initialized"> | undefined {
	if (model.phase !== "booting" || model.current || model.active_route) {
		return undefined;
	}
	return {
		effects: [],
		kind: "boot_initialized",
		model: create_core4_model({
			browser: input.browser,
			client_build_id: input.client_build_id,
			phase: "booting",
			use_view_transitions: input.use_view_transitions,
		}),
	};
}

export function accept_boot_provisional_route(
	model: Core4Model,
	input: {
		position: BrowserPosition;
		route: RouteState;
		token: Core4Token;
	},
): Core4Transition<"boot_provisional"> | undefined {
	const active_route = model.active_route;
	if (
		!active_route ||
		active_route.kind !== "boot" ||
		active_route.token !== input.token
	) {
		return undefined;
	}
	return {
		effects: [],
		kind: "boot_provisional",
		model: {
			...model,
			browser: input.position,
			current: {
				position: input.position,
				route: input.route,
				sequence: active_route.sequence,
			},
		},
	};
}

export function classify_route_response(
	input: RouteResponseClassificationInput,
): RouteResponseOutcome {
	if (input.owner.kind === "stale") {
		return {
			kind: "ignored_stale",
			owner_kind: "stale",
			token: input.token,
		};
	}
	const build_skew_report = route_response_build_skew_report(input);
	if (
		input.response.headers.get(X_VORMA_BUILD_SKEW) ===
		VORMA_PROTOCOL_ENABLED
	) {
		const behavior =
			input.owner.kind === "prefetch" ||
			input.owner.active_route.kind === "revalidation"
				? "drop"
				: "reload";
		return {
			behavior,
			default_behavior:
				behavior === "drop" ? "dropResponse" : "hardReload",
			href: input.requested_href,
			kind: "build_skew",
			owner_kind: input.owner.kind,
			response: build_skew_response_facts(input.response),
			token: input.token,
		};
	}
	const redirect_href = response_redirect_href(
		input.response,
		input.requested_href,
	);
	if (redirect_href) {
		return attach_route_build_skew_report(
			{
				href: redirect_href,
				kind: "soft_redirect",
				owner_kind: input.owner.kind,
				token: input.token,
			},
			build_skew_report,
		);
	}
	if (!input.response.ok || !input.payload) {
		return attach_route_build_skew_report(
			{
				kind: "failed",
				owner_kind: input.owner.kind,
				retryable:
					input.owner.kind === "active_route" &&
					input.owner.active_route.kind === "revalidation",
				token: input.token,
			},
			build_skew_report,
		);
	}
	return attach_route_build_skew_report(
		{
			kind: "data",
			owner_kind: input.owner.kind,
			payload: input.payload,
			token: input.token,
		},
		build_skew_report,
	);
}

export function classify_api_response(
	input: APIResponseClassificationInput,
): APISubmissionOutcome {
	if (!input.submission) {
		return {
			kind: "ignored_stale",
			token: input.token,
		};
	}
	const base = {
		token: input.token,
	};
	const build_skew_report = api_response_build_skew_report(input);
	const redirect_href = response_redirect_href(
		input.response,
		input.requested_href,
	);
	if (redirect_href) {
		if (!is_http_href(redirect_href)) {
			return attach_api_build_skew_report(
				{
					error: `${api_invalid_redirect_error_prefix} "${redirect_href}".`,
					kind: "invalid_redirect",
					response: input.response.raw_response,
					...base,
				},
				build_skew_report,
			);
		}
		if (!same_origin(redirect_href, input.requested_href)) {
			return attach_api_build_skew_report(
				{
					href: redirect_href,
					kind: "hard_redirect",
					response: input.response.raw_response,
					...base,
				},
				build_skew_report,
			);
		}
		return attach_api_build_skew_report(
			{
				browser_key: input.browser_key,
				href: redirect_href,
				kind: "soft_redirect",
				navigation_token: input.navigation_token,
				response: input.response.raw_response,
				state: input.state,
				...base,
			},
			build_skew_report,
		);
	}
	if (!input.response.ok) {
		return attach_api_build_skew_report(
			{
				error: input.response.status_text,
				kind: "http_error",
				response: input.response.raw_response,
				...base,
			},
			build_skew_report,
		);
	}
	return attach_api_build_skew_report(
		{
			data: input.data,
			kind: "success",
			response: input.response.raw_response,
			...base,
		},
		build_skew_report,
	);
}

function classify_route_failure(input: {
	owner: RouteResponseOwner;
	retryable: boolean;
	token: Core4Token;
}): RouteResponseOutcome {
	if (input.owner.kind === "stale") {
		return {
			kind: "ignored_stale",
			owner_kind: "stale",
			token: input.token,
		};
	}
	return {
		kind: "failed",
		owner_kind: input.owner.kind,
		retryable: input.retryable,
		token: input.token,
	};
}

function classify_api_runtime_failure(input: {
	dispatched: boolean;
	error: string;
	kind: "aborted" | "network_error";
	submission: SubmissionSlot | null;
	token: Core4Token;
}): APISubmissionOutcome {
	if (!input.submission) {
		return {
			kind: "ignored_stale",
			token: input.token,
		};
	}
	if (input.kind === "aborted") {
		return {
			dispatched: input.dispatched,
			kind: "aborted",
			token: input.token,
		};
	}
	return {
		dispatched: input.dispatched,
		error: input.error,
		kind: "network_error",
		token: input.token,
	};
}

function attach_route_build_skew_report(
	outcome: RouteResponseOutcome,
	build_skew_report: RouteBuildSkewReport | undefined,
): RouteResponseOutcome {
	if (build_skew_report) {
		outcome.build_skew_report = build_skew_report;
	}
	return outcome;
}

function attach_api_build_skew_report(
	outcome: APISubmissionOutcome,
	build_skew_report: APISubmissionBuildSkewReport | undefined,
): APISubmissionOutcome {
	if (build_skew_report) {
		outcome.build_skew_report = build_skew_report;
	}
	return outcome;
}

export function derive_core4_work_state(model: Core4Model): WorkState {
	const active_route = model.active_route;
	let revalidation: WorkState["revalidation"] = null;
	if (active_route?.kind === "revalidation") {
		revalidation = {
			attempt: running_revalidation_attempt(model, active_route.token),
			status: "running",
		};
	} else if (model.refresh.kind === "debouncing") {
		revalidation = {
			attempt: 0,
			status: "debouncing",
		};
	} else if (model.refresh.kind === "pending") {
		revalidation = {
			attempt: model.refresh.attempt,
			status: "running",
		};
	} else if (model.refresh.kind === "retrying") {
		revalidation = {
			attempt: model.refresh.attempt,
			status: "retrying",
		};
	}

	return {
		apiRequests: running_submissions(model).map((submission) => {
			return {
				href: submission.href,
				key: submission.key,
				method: submission.method,
			};
		}),
		navigation:
			active_route &&
			active_route.source !== null &&
			active_route.phase !== "publishing"
				? {
						href: active_route.href,
						replace: active_route.replace,
						source: active_route.source,
					}
				: null,
		prefetch:
			model.prefetch && model.prefetch.phase !== "prepared"
				? { href: model.prefetch.href }
				: null,
		revalidation,
	};
}

export function derive_core4_work_projection(
	model: Core4Model,
): readonly Core4WorkProjection[] {
	const projection: Core4WorkProjection[] = [];
	const active_route = model.active_route;
	if (
		active_route &&
		active_route.source !== null &&
		active_route.phase !== "publishing"
	) {
		projection.push({
			kind: "navigation",
			skip_work_indicator: active_route.skip_work_indicator,
		});
	}
	if (active_route?.kind === "revalidation") {
		projection.push({
			kind: "revalidation",
			skip_work_indicator: active_route.skip_work_indicator,
		});
	} else if (
		model.refresh.kind === "debouncing" ||
		model.refresh.kind === "retrying" ||
		(model.refresh.kind === "pending" && !model.active_route)
	) {
		const demand = refresh_demand(model.refresh);
		projection.push({
			kind: "revalidation",
			skip_work_indicator: demand?.skip_work_indicator ?? false,
		});
	}
	for (const submission of running_submissions(model)) {
		projection.push({
			kind: "apiRequest",
			skip_work_indicator: submission.skip_work_indicator,
		});
	}
	if (model.prefetch && model.prefetch.phase !== "prepared") {
		projection.push({ kind: "prefetch" });
	}
	return projection;
}

export function begin_api_submission(
	model: Core4Model,
	request: APISubmissionRequest,
): APISubmissionTransition | undefined {
	if (!model.current) {
		return undefined;
	}
	if (model.submissions[request.token]) {
		return undefined;
	}
	const base_href = model.browser?.href ?? model.current.route.href;
	const href = resolve_href(request.href, base_href);
	if (!href || !is_http_href(href) || !same_origin(href, base_href)) {
		const effects: Core4Effect[] = [
			{
				result: {
					error: `${API_SUBMIT_CROSS_ORIGIN_ERROR} "${href ?? request.href}".`,
					success: false,
				},
				token: request.token,
				type: "settle_api_submission",
			},
		];
		if (request.should_revalidate) {
			append_refresh_waiter_settlement(
				effects,
				request.refresh_waiter_id,
				REVALIDATION_OK,
			);
		}
		return {
			effects,
			kind: "rejected_cross_origin",
			model,
		};
	}

	const effects: Core4Effect[] = [];
	let next_model = model;
	let submissions = model.submissions;
	const previous = request.dedupe_key
		? find_running_submission_by_dedupe_key(model, request.dedupe_key)
		: undefined;
	if (previous) {
		effects.push({
			token: previous.token,
			type: "abort_api_submission",
		});
		submissions = {
			...submissions,
			[previous.token]: undefined,
		};
		next_model = schedule_api_revalidation(
			{
				...next_model,
				submissions,
			},
			previous,
			effects,
		);
		effects.push(
			api_settlement_effect(previous.token, {
				error: api_aborted_error,
				success: false,
			}),
		);
	}

	const submission: SubmissionSlot = {
		dedupe_key: request.dedupe_key,
		href,
		key: request.key,
		method: request.method,
		refresh_waiter_id: request.refresh_waiter_id,
		route_kind: request.route_kind,
		should_revalidate: request.should_revalidate,
		skip_work_indicator: request.skip_work_indicator,
		token: request.token,
	};
	effects.push({
		href,
		method: request.method,
		request_init: request.request_init,
		route_kind: request.route_kind,
		token: request.token,
		type: "fetch_api",
	});
	return {
		effects,
		kind: previous ? "replaced" : "started",
		model: {
			...next_model,
			submissions: {
				...next_model.submissions,
				[request.token]: submission,
			},
		},
	};
}

export function accept_api_submission_outcome(
	model: Core4Model,
	outcome: APISubmissionOutcome,
): APISubmissionTransition | undefined {
	if (outcome.kind === "ignored_stale") {
		return {
			effects: [
				{
					token: outcome.token,
					type: "release_api_submission",
				},
			],
			kind: "ignored_stale",
			model,
		};
	}
	const submission = model.submissions[outcome.token];
	if (!submission) {
		return {
			effects: [
				{
					token: outcome.token,
					type: "release_api_submission",
				},
			],
			kind: "ignored_stale",
			model,
		};
	}

	const effects: Core4Effect[] = [
		{
			token: outcome.token,
			type: "release_api_submission",
		},
	];
	const build_skew_report = outcome.build_skew_report;
	if (build_skew_report) {
		append_build_skew_notification(
			effects,
			model,
			build_skew_report.response,
			{
				apiRouteKind: submission.route_kind,
				kind: "apiRoute",
				method: submission.method,
				ok: build_skew_report.response.ok,
				requestedHref: submission.href,
				status: build_skew_report.response.status,
			},
		);
	}
	const settled_model = remove_submission(model, submission);
	if (outcome.kind === "success") {
		const next_model = schedule_api_revalidation(
			settled_model,
			submission,
			effects,
		);
		effects.push(
			api_settlement_effect(submission.token, {
				data: outcome.data,
				response: outcome.response,
				success: true,
			}),
		);
		return {
			effects,
			kind: "settled",
			model: next_model,
		};
	}
	if (outcome.kind === "http_error") {
		const next_model = schedule_api_revalidation(
			settled_model,
			submission,
			effects,
		);
		effects.push(
			api_settlement_effect(submission.token, {
				error: outcome.error,
				response: outcome.response,
				success: false,
			}),
		);
		return {
			effects,
			kind: "settled",
			model: next_model,
		};
	}
	if (outcome.kind === "aborted") {
		const next_model = outcome.dispatched
			? schedule_api_revalidation(settled_model, submission, effects)
			: settled_model;
		effects.push(
			api_settlement_effect(submission.token, {
				error: api_aborted_error,
				success: false,
			}),
		);
		return {
			effects,
			kind: "settled",
			model: next_model,
		};
	}
	if (outcome.kind === "network_error") {
		const next_model = outcome.dispatched
			? schedule_api_revalidation(settled_model, submission, effects)
			: settled_model;
		effects.push(
			api_settlement_effect(submission.token, {
				error: outcome.error,
				success: false,
			}),
		);
		return {
			effects,
			kind: "settled",
			model: next_model,
		};
	}
	if (outcome.kind === "invalid_redirect") {
		settle_unscheduled_api_revalidation(effects, submission);
		effects.push(
			api_settlement_effect(submission.token, {
				error: outcome.error,
				response: outcome.response,
				success: false,
			}),
		);
		return {
			effects,
			kind: "settled",
			model: settled_model,
		};
	}
	if (outcome.kind === "hard_redirect") {
		settle_unscheduled_api_revalidation(effects, submission);
		effects.push(
			api_settlement_effect(submission.token, {
				data: undefined,
				response: outcome.response,
				success: true,
			}),
			{
				href: outcome.href,
				type: "hard_redirect",
			},
		);
		return {
			effects,
			kind: "hard_redirect",
			model: settled_model,
		};
	}

	settle_unscheduled_api_revalidation(effects, submission);
	effects.push(
		api_settlement_effect(submission.token, {
			data: undefined,
			response: outcome.response,
			success: true,
		}),
	);
	if (settled_model.phase !== "ready") {
		return {
			effects,
			kind: "deferred_redirect",
			model: {
				...settled_model,
				deferred_api_redirect: {
					href: outcome.href,
				},
			},
		};
	}
	const redirect = begin_navigation(settled_model, {
		browser_key: outcome.browser_key,
		href: outcome.href,
		public_call_ids: [],
		replace: true,
		skip_work_indicator: false,
		source: "redirect",
		state: outcome.state,
		token: outcome.navigation_token,
	});
	if (!redirect) {
		return {
			effects,
			kind: "soft_redirect",
			model: settled_model,
		};
	}
	return {
		effects: effects.concat(redirect.effects),
		kind: "soft_redirect",
		model: redirect.model,
	};
}

export function begin_deferred_api_redirect(
	model: Core4Model,
	request: DeferredAPIRedirectRequest,
): NavigationStartTransition | undefined {
	const deferred = model.deferred_api_redirect;
	if (!deferred) {
		return undefined;
	}
	return begin_navigation(
		{
			...model,
			deferred_api_redirect: null,
		},
		{
			browser_key: request.browser_key,
			href: deferred.href,
			public_call_ids: [],
			replace: true,
			skip_work_indicator: false,
			source: "redirect",
			state: request.state,
			token: request.token,
		},
	);
}

export function begin_boot(
	model: Core4Model,
	request: BootRequest,
): Core4Transition<"preparing_boot"> | undefined {
	if (model.phase !== "booting" || model.active_route) {
		return undefined;
	}
	const sequenced = take_sequence(model);
	return {
		effects: [
			{
				history_state: request.browser.state,
				href: request.browser.href,
				payload: request.payload,
				target: "active_route",
				token: request.token,
				trigger: "boot",
				type: "prepare_route",
			},
		],
		kind: "preparing_boot",
		model: {
			...sequenced.model,
			active_route: {
				browser_key: request.browser.key,
				href: request.browser.href,
				kind: "boot",
				phase: "preparing",
				public_call_ids: [],
				redirect_count: 0,
				restored_scroll: request.restored_scroll,
				replace: true,
				scroll_to_top: true,
				sequence: sequenced.sequence,
				skip_work_indicator: true,
				source: null,
				state: request.browser.state,
				token: request.token,
			},
			browser: request.browser,
		} as Core4Model,
	};
}

export function begin_navigation(
	model: Core4Model,
	request: NavigationRequest,
): NavigationStartTransition | undefined {
	if (model.phase !== "ready" || !model.browser || !model.current) {
		return undefined;
	}
	const target_href = resolve_href(request.href, model.browser.href);
	if (!target_href) {
		return settle_navigation_without_route(model, request.public_call_ids);
	}
	if (
		!is_http_href(target_href) ||
		!same_origin(target_href, model.browser.href)
	) {
		const effects: Core4Effect[] = [
			{ href: target_href, type: "hard_redirect" },
		];
		append_navigation_settlement(effects, request.public_call_ids, false);
		return {
			effects,
			kind: "hard_redirect",
			model,
		};
	}
	if (same_document_href(target_href, model.current.route.href)) {
		return begin_same_document_navigation(model, request, target_href);
	}
	const retargeted = retarget_active_navigation(model, request, target_href);
	if (retargeted) {
		return retargeted;
	}
	const cleared = supersede_active_route(model);
	const promoted = promote_prefetch(cleared.model, request, target_href);
	if (promoted) {
		return {
			effects: cleared.effects.concat(promoted.effects),
			kind: "promoted_prefetch",
			model: promoted.model,
		};
	}
	const sequenced = take_sequence(cleared.model);
	const active_route: ActiveRouteSlot = {
		browser_key: request.browser_key,
		href: target_href,
		kind: "navigation",
		phase: "fetching",
		public_call_ids: request.public_call_ids,
		redirect_count: 0,
		restored_scroll: request.restored_scroll,
		replace: request.replace,
		scroll_to_top: request.scroll_to_top,
		sequence: sequenced.sequence,
		skip_work_indicator: request.skip_work_indicator,
		source: navigation_active_route_source(request.source),
		state: request.state,
		token: request.token,
	};
	return {
		effects: cleared.effects.concat({
			client_build_id: sequenced.model.client_build_id,
			href: target_href,
			token: request.token,
			trigger: "navigation",
			type: "fetch_route",
		}),
		kind: "started",
		model: {
			...sequenced.model,
			active_route,
		} as Core4Model,
	};
}

export function begin_popstate(
	model: Core4Model,
	request: PopstateRequest,
): Core4Transition<"ignored" | "same_document" | "started"> | undefined {
	if (model.phase !== "ready" || !model.browser || !model.current) {
		return undefined;
	}
	if (
		request.browser.key === model.browser.key &&
		request.browser.href === model.browser.href
	) {
		const effects: Core4Effect[] = [];
		append_navigation_settlement(effects, request.public_call_ids, false);
		return {
			effects,
			kind: "ignored",
			model,
		};
	}
	if (same_document_href(request.browser.href, model.current.route.href)) {
		return begin_same_document_navigation(
			{ ...model, browser: request.browser },
			{
				browser_key: request.browser.key,
				href: request.browser.href,
				public_call_ids: request.public_call_ids,
				restored_scroll: request.restored_scroll,
				replace: true,
				skip_work_indicator: true,
				source: "popstate",
				state: request.browser.state,
				token: request.token,
			},
			request.browser.href,
		);
	}
	const cleared = supersede_active_route({
		...model,
		browser: request.browser,
	});
	const sequenced = take_sequence(cleared.model);
	return {
		effects: cleared.effects.concat({
			client_build_id: sequenced.model.client_build_id,
			href: request.browser.href,
			token: request.token,
			trigger: "popstate",
			type: "fetch_route",
		}),
		kind: "started",
		model: {
			...sequenced.model,
			active_route: {
				browser_key: request.browser.key,
				href: request.browser.href,
				kind: "popstate",
				phase: "fetching",
				public_call_ids: request.public_call_ids,
				redirect_count: 0,
				restored_scroll: request.restored_scroll,
				replace: true,
				scroll_to_top: true,
				sequence: sequenced.sequence,
				skip_work_indicator: true,
				source: "popstate",
				state: request.browser.state,
				token: request.token,
			},
		} as Core4Model,
	};
}

export function begin_prefetch(
	model: Core4Model,
	request: PrefetchRequest,
): Core4Transition<"started"> | undefined {
	if (model.phase !== "ready" || !model.browser || !model.current) {
		return undefined;
	}
	const target_href = resolve_href(request.href, model.browser.href);
	if (
		!target_href ||
		!is_http_href(target_href) ||
		!same_origin(target_href, model.browser.href) ||
		same_document_href(target_href, model.current.route.href)
	) {
		return undefined;
	}
	if (
		model.active_route &&
		same_document_href(target_href, model.active_route.href)
	) {
		return undefined;
	}
	if (
		model.prefetch &&
		same_document_href(model.prefetch.href, target_href)
	) {
		return undefined;
	}
	const effects: Core4Effect[] = [];
	if (model.prefetch) {
		effects.push({
			token: model.prefetch.token,
			type: "abort_route_work",
		});
	}
	effects.push({
		client_build_id: model.client_build_id,
		href: target_href,
		token: request.token,
		trigger: "prefetch",
		type: "fetch_route",
	});
	return {
		effects,
		kind: "started",
		model: {
			...model,
			prefetch: {
				href: target_href,
				phase: "fetching",
				token: request.token,
			},
		},
	};
}

export function cancel_prefetch(
	model: Core4Model,
	href: string,
): Core4Transition<"canceled"> | undefined {
	if (!model.browser || !model.prefetch) {
		return undefined;
	}
	const target_href = resolve_href(href, model.browser.href);
	if (!target_href || !same_document_href(target_href, model.prefetch.href)) {
		return undefined;
	}
	return {
		effects: [
			{
				token: model.prefetch.token,
				type: "abort_route_work",
			},
		],
		kind: "canceled",
		model: {
			...model,
			prefetch: null,
		},
	};
}

export function accept_route_response(
	model: Core4Model,
	outcome: RouteResponseOutcome,
): RouteResponseTransition | undefined {
	if (outcome.kind === "ignored_stale") {
		return {
			effects: [],
			kind: "ignored_stale",
			model,
		};
	}
	const owner = route_response_owner(model, outcome.token);
	if (
		owner.kind === "active_route" &&
		outcome.owner_kind === "active_route"
	) {
		return accept_active_route_response(model, owner.active_route, outcome);
	}
	if (owner.kind === "prefetch" && outcome.owner_kind === "prefetch") {
		return accept_prefetch_response(model, owner.prefetch, outcome);
	}
	return {
		effects: [],
		kind: "ignored_stale",
		model,
	};
}

export function accept_route_preparation(
	model: Core4Model,
	outcome: RoutePreparationOutcome,
): RoutePreparationTransition | undefined {
	const owner = route_response_owner(model, outcome.token);
	if (outcome.kind === "failed") {
		return accept_route_response(
			model,
			classify_route_failure({
				owner,
				retryable: outcome.retryable,
				token: outcome.token,
			}),
		);
	}
	if (owner.kind === "stale") {
		return {
			effects: [],
			kind: "ignored_stale",
			model,
		};
	}
	if (outcome.kind === "aborted") {
		return undefined;
	}
	if (owner.kind === "prefetch") {
		return {
			effects: [
				{
					token: outcome.token,
					type: "release_route_work",
				},
			],
			kind: "prefetch_prepared",
			model: {
				...model,
				prefetch: {
					href: owner.prefetch.href,
					phase: "prepared",
					prepared: outcome.prepared,
					token: outcome.token,
				},
			},
		};
	}
	const active_route = owner.active_route;
	if (active_route.phase !== "preparing") {
		return undefined;
	}
	return begin_route_publication(model, active_route, outcome.prepared);
}

function begin_route_publication(
	model: Core4Model,
	active_route: ActiveRouteSlot,
	prepared: PreparedRoute,
	overrides: PublicationOptions = {},
): Core4Transition<"publishing"> {
	const position = publication_position(model, active_route);
	const history = publication_history(active_route, position);
	const hooks = overrides.hooks ?? publication_hooks(model, active_route);
	const next: PreparedRoute = {
		...prepared,
		route: {
			...prepared.route,
			historyState: position.state,
			href: position.href,
		},
	};
	const scroll = overrides.scroll ?? publication_scroll(active_route, next);
	const save_current_scroll =
		overrides.save_current_scroll ??
		publication_saves_current_scroll(active_route);
	const plan: PublicationPlan = {
		history,
		hooks,
		next,
		position,
		previous: active_route.kind === "boot" ? null : model.current,
		reason: publication_reason(active_route),
		route_sequence: active_route.sequence,
		save_current_scroll,
		scroll,
		token: active_route.token,
		use_view_transition:
			overrides.use_view_transition ??
			publication_uses_view_transition(model, active_route),
	};
	const publication = {
		did_navigate:
			overrides.did_navigate ?? active_route.kind !== "revalidation",
		phase: "publishing",
		plan,
		public_call_ids: active_route.public_call_ids,
		token: plan.token,
	} satisfies PublicationSlot;
	const effects: Core4Effect[] = [];
	effects.push({
		plan,
		type: "publish_route",
	});
	return {
		effects,
		kind: "publishing",
		model: {
			...model,
			active_route: {
				...active_route,
				phase: "publishing",
			},
			publication,
		} as Core4Model,
	};
}

export function commit_publication(
	model: Core4Model,
	commit: PublicationCommit,
): Core4Transition<"committed"> | undefined {
	const publication = owned_by(model, commit.token, "publication");
	if (!publication || publication.phase !== "publishing") {
		return undefined;
	}
	const active_route = owned_by(model, commit.token, "active_route");
	if (!active_route || active_route.phase !== "publishing") {
		return undefined;
	}
	const plan = publication.plan;
	const refresh =
		active_route.kind === "revalidation" && model.refresh.kind === "running"
			? {
					demand: model.refresh.demand,
					kind: "settling" as const,
				}
			: model.refresh;
	return {
		effects: [],
		kind: "committed",
		model: {
			...model,
			active_route:
				model.active_route?.token === commit.token
					? null
					: model.active_route,
			browser: plan.position,
			current: {
				position: plan.position,
				route: plan.next.route,
				sequence: plan.route_sequence,
			},
			phase: model.phase === "booting" ? "ready" : model.phase,
			publication: {
				...publication,
				phase: "committed",
			},
			refresh,
		} as Core4Model,
	};
}

export function settle_publication(
	model: Core4Model,
	token: Core4Token,
): Core4Transition<"settled"> | undefined {
	const publication = owned_by(model, token, "publication");
	if (!publication || publication.phase !== "committed") {
		return undefined;
	}
	const effects: Core4Effect[] = [];
	append_navigation_settlement(
		effects,
		publication.public_call_ids,
		publication.did_navigate,
	);
	effects.push({
		token,
		type: "release_route_work",
	});
	const next_model = settle_refresh_if_publication_is_fresh(
		{
			...model,
			publication: null,
		} as Core4Model,
		publication,
		effects,
	);
	return {
		effects,
		kind: "settled",
		model: next_model,
	};
}

export function fail_publication(
	model: Core4Model,
	token: Core4Token,
): Core4Transition<"publication_failed"> | undefined {
	if (!owned_by(model, token, "publication")) {
		return undefined;
	}
	const active_route = owned_by(model, token, "active_route");
	if (!active_route) {
		return undefined;
	}
	const transition = fail_active_route(model, active_route, false);
	return {
		effects: transition.effects,
		kind: "publication_failed",
		model: transition.model,
	};
}

export function accept_hmr_route_update(
	model: Core4Model,
	route: RouteState,
): Core4Transition<"hmr_updated"> | undefined {
	if (!model.current) {
		return undefined;
	}
	return {
		effects: [],
		kind: "hmr_updated",
		model: {
			...model,
			current: {
				...model.current,
				route,
			},
		},
	};
}

export function request_revalidation(
	model: Core4Model,
	request: RevalidationRequest,
): Core4Transition<"debouncing" | "pending"> | undefined {
	if (model.phase !== "ready" || !model.current) {
		return undefined;
	}
	return plan_refresh_request(model, request, false);
}

function plan_refresh_request(
	model: Core4Model,
	request: RevalidationRequest,
	allow_boot: boolean,
): Core4Transition<"debouncing" | "pending"> | undefined {
	if (
		!model.current ||
		(model.phase !== "ready" && !(allow_boot && model.phase === "booting"))
	) {
		return undefined;
	}
	const previous = refresh_demand(model.refresh);
	const waiters = previous ? previous.waiters.slice() : [];
	if (request.waiter_id) {
		waiters.push({ id: request.waiter_id });
	}
	const effects: Core4Effect[] = [];
	if (
		model.refresh.kind === "debouncing" ||
		model.refresh.kind === "retrying"
	) {
		effects.push({
			id: model.refresh.timer_id,
			type: "clear_refresh_timer",
		});
	}
	const demand: RefreshDemand = {
		after_sequence: model.sequence + 1,
		reason: request.reason,
		skip_work_indicator: previous
			? previous.skip_work_indicator && request.skip_work_indicator
			: request.skip_work_indicator,
		waiters,
	};
	if (request.timer_id) {
		effects.push({
			id: request.timer_id,
			ms: CORE4_REFRESH_DEBOUNCE_MS,
			type: "start_refresh_timer",
		});
		return {
			effects,
			kind: "debouncing",
			model: {
				...model,
				refresh: {
					demand,
					kind: "debouncing",
					timer_id: request.timer_id,
				},
			} as Core4Model,
		};
	}
	return {
		effects,
		kind: "pending",
		model: {
			...model,
			refresh: {
				attempt: 0,
				demand,
				kind: "pending",
			},
		} as Core4Model,
	};
}

export function fire_refresh_timer(
	model: Core4Model,
	timer_id: TimerID,
): Core4Transition<"ignored" | "pending"> {
	if (
		model.refresh.kind !== "debouncing" &&
		model.refresh.kind !== "retrying"
	) {
		return {
			effects: [],
			kind: "ignored",
			model,
		};
	}
	if (model.refresh.timer_id !== timer_id) {
		return {
			effects: [],
			kind: "ignored",
			model,
		};
	}
	return {
		effects: [],
		kind: "pending",
		model: {
			...model,
			refresh: {
				attempt:
					model.refresh.kind === "retrying"
						? model.refresh.attempt
						: 0,
				demand: model.refresh.demand,
				kind: "pending",
			},
		} as Core4Model,
	};
}

export function begin_pending_revalidation(
	model: Core4Model,
	request: RevalidationStartRequest,
): Core4Transition<"started"> | undefined {
	if (
		model.phase !== "ready" ||
		!model.browser ||
		!model.current ||
		model.active_route ||
		model.publication ||
		model.refresh.kind !== "pending"
	) {
		return undefined;
	}
	if (!same_document_href(model.browser.href, model.current.route.href)) {
		return undefined;
	}
	const sequenced = take_sequence(model);
	const refresh = model.refresh;
	return {
		effects: [
			{
				client_build_id: model.client_build_id,
				href: model.browser.href,
				token: request.token,
				trigger: "revalidation",
				type: "fetch_route",
			},
		],
		kind: "started",
		model: {
			...sequenced.model,
			active_route: {
				browser_key: model.browser.key,
				href: model.browser.href,
				kind: "revalidation",
				phase: "fetching",
				public_call_ids: [],
				redirect_count: 0,
				replace: true,
				scroll_to_top: false,
				sequence: sequenced.sequence,
				skip_work_indicator: refresh.demand.skip_work_indicator,
				source: null,
				state: model.browser.state,
				token: request.token,
			},
			refresh: {
				attempt: refresh.attempt,
				demand: refresh.demand,
				kind: "running",
			},
		} as Core4Model,
	};
}

function find_running_submission_by_dedupe_key(
	model: Core4Model,
	dedupe_key: string,
): SubmissionSlot | undefined {
	for (const submission of running_submissions(model)) {
		if (submission.dedupe_key === dedupe_key) {
			return submission;
		}
	}
	return undefined;
}

function running_submissions(model: Core4Model): SubmissionSlot[] {
	return Object.values(model.submissions).filter(
		(submission): submission is SubmissionSlot => {
			return !!submission;
		},
	);
}

function running_revalidation_attempt(
	model: Core4Model,
	token: Core4Token,
): number {
	if (
		model.refresh.kind !== "running" ||
		model.active_route?.kind !== "revalidation" ||
		model.active_route.token !== token
	) {
		return 0;
	}
	return model.refresh.attempt;
}

function owned_by<T extends "active_route" | "prefetch" | "publication">(
	model: Core4Model,
	token: Core4Token,
	property: T,
): Extract<Core4Model[T], { token: Core4Token }> | null {
	const slot = model[property];
	if (!slot || slot.token !== token) {
		return null;
	}
	return slot as Extract<Core4Model[T], { token: Core4Token }>;
}

function route_response_owner(
	model: Core4Model,
	token: Core4Token,
): RouteResponseOwner {
	const active_route = owned_by(model, token, "active_route");
	if (active_route) {
		return {
			active_route,
			kind: "active_route",
		};
	}
	const prefetch = owned_by(model, token, "prefetch");
	if (prefetch) {
		return { kind: "prefetch", prefetch };
	}
	return { kind: "stale" };
}

function navigation_active_route_source(
	source: NavigationRequest["source"],
): NavigationActiveRouteSlot["source"] {
	if (source === "redirect") {
		return "redirect";
	}
	return "navigate";
}

function remove_submission(
	model: Core4Model,
	submission: SubmissionSlot,
): Core4Model {
	return {
		...model,
		submissions: {
			...model.submissions,
			[submission.token]: undefined,
		},
	} as Core4Model;
}

function api_settlement_effect(
	token: Core4Token,
	result: APISubmissionResult,
): Core4Effect {
	return {
		result,
		token,
		type: "settle_api_submission",
	};
}

function append_refresh_waiter_settlement(
	effects: Core4Effect[],
	waiter_id: PublicCallID | undefined,
	result: RevalidationResult,
): void {
	if (!waiter_id) {
		return;
	}
	effects.push({
		ids: [waiter_id],
		result,
		type: "settle_refresh_calls",
	});
}

function settle_unscheduled_api_revalidation(
	effects: Core4Effect[],
	submission: SubmissionSlot,
): void {
	if (!submission.should_revalidate) {
		return;
	}
	append_refresh_waiter_settlement(
		effects,
		submission.refresh_waiter_id,
		REVALIDATION_OK,
	);
}

function schedule_api_revalidation(
	model: Core4Model,
	submission: SubmissionSlot,
	effects: Core4Effect[],
): Core4Model {
	if (!submission.should_revalidate) {
		return model;
	}
	const plan = plan_refresh_request(
		model,
		{
			reason: "apiRequest",
			skip_work_indicator: submission.skip_work_indicator,
			waiter_id:
				model.phase === "ready"
					? submission.refresh_waiter_id
					: undefined,
		},
		true,
	);
	if (!plan) {
		settle_unscheduled_api_revalidation(effects, submission);
		return model;
	}
	effects.push(...plan.effects);
	return plan.model;
}

function accept_active_route_response(
	model: Core4Model,
	active_route: ActiveRouteSlot,
	outcome: RouteResponseOutcome,
): RouteResponseTransition | undefined {
	const effects: Core4Effect[] = [];
	const build_skew_report = route_outcome_build_skew_report(outcome);
	if (build_skew_report) {
		append_build_skew_notification(
			effects,
			model,
			build_skew_report.response,
			route_build_skew_triggering_response(
				model,
				active_route,
				build_skew_report.requested_href,
				build_skew_report.response,
			),
		);
	}
	if (outcome.kind === "build_skew") {
		if (outcome.behavior === "reload") {
			effects.push({
				href: outcome.href,
				type: "hard_redirect",
			});
			effects.push({
				token: active_route.token,
				type: "release_route_work",
			});
			settle_active_route_as_not_navigated(effects, active_route);
			return {
				effects,
				kind: "build_skew",
				model: finish_active_route(model, active_route),
			};
		}
		const next_model = finish_refresh_with_result(
			finish_active_route(model, active_route),
			{ ok: false, reason: "build_skew" },
			effects,
		);
		effects.push({
			token: active_route.token,
			type: "release_route_work",
		});
		return {
			effects,
			kind: "build_skew",
			model: next_model,
		};
	}
	if (outcome.kind === "failed") {
		return fail_active_route(
			model,
			active_route,
			outcome.retryable,
			effects,
		);
	}
	if (outcome.kind === "soft_redirect") {
		if (active_route.redirect_count >= CORE4_MAX_REDIRECTS) {
			return fail_active_route(model, active_route, false, effects);
		}
		return redirect_active_route(
			model,
			active_route,
			outcome.href,
			effects,
		);
	}
	if (outcome.kind === "ignored_stale") {
		return {
			effects,
			kind: "ignored_stale",
			model,
		};
	}
	effects.push({
		history_state: active_route.state,
		href: active_route.href,
		payload: outcome.payload,
		target: "active_route",
		token: active_route.token,
		trigger:
			active_route.kind === "revalidation"
				? "revalidation"
				: active_route.kind,
		type: "prepare_route",
	});
	return {
		effects,
		kind: "preparing_active_route",
		model: {
			...model,
			active_route: {
				...active_route,
				phase: "preparing",
			},
		} as Core4Model,
	};
}

function accept_prefetch_response(
	model: Core4Model,
	prefetch: PrefetchSlot,
	outcome: RouteResponseOutcome,
): RouteResponseTransition | undefined {
	const effects: Core4Effect[] = [];
	const build_skew_report = route_outcome_build_skew_report(outcome);
	if (build_skew_report) {
		append_build_skew_notification(
			effects,
			model,
			build_skew_report.response,
			{
				kind: "route",
				ok: build_skew_report.response.ok,
				requestedHref: build_skew_report.requested_href,
				status: build_skew_report.response.status,
				trigger: "prefetch",
			},
		);
	}
	if (outcome.kind !== "data") {
		effects.push({
			token: prefetch.token,
			type: "release_route_work",
		});
		return {
			effects,
			kind: outcome.kind === "build_skew" ? "build_skew" : "failed",
			model: {
				...model,
				prefetch: null,
			},
		};
	}
	effects.push({
		history_state: model.browser?.state,
		href: prefetch.href,
		payload: outcome.payload,
		target: "prefetch",
		token: prefetch.token,
		trigger: "prefetch",
		type: "prepare_route",
	});
	return {
		effects,
		kind: "preparing_prefetch",
		model: {
			...model,
			prefetch: {
				href: prefetch.href,
				phase: "preparing",
				token: prefetch.token,
			},
		},
	};
}

function begin_same_document_navigation(
	model: Core4Model,
	request: NavigationRequest,
	target_href: string,
): Core4Transition<"same_document"> {
	const current = model.current;
	if (!current) {
		return settle_navigation_without_route(model, request.public_call_ids);
	}
	const source = request.source ?? "navigate";
	const target_hash = normalized_hash_from_href(target_href);
	const current_hash = normalized_hash_from_href(current.position.href);
	if (
		source !== "popstate" &&
		target_hash === current_hash &&
		!request.replace
	) {
		const effects: Core4Effect[] = [
			{
				scroll: top_scroll_state(),
				type: "apply_scroll",
			},
		];
		append_navigation_settlement(effects, request.public_call_ids, false);
		return {
			effects,
			kind: "same_document",
			model,
		};
	}
	if (model.active_route?.kind === "revalidation") {
		return retarget_browser_during_revalidation(
			model,
			request,
			target_href,
		);
	}
	const route = {
		...current.route,
		href: target_href,
		historyState: request.state,
	} satisfies RouteState;
	const sequenced = take_sequence(model);
	const scroll =
		scroll_for_href_hash(target_href) ??
		(source === "popstate"
			? (request.restored_scroll ?? top_scroll_state())
			: top_scroll_state());
	const active_route: ActiveRouteSlot =
		source === "popstate"
			? {
					browser_key: request.browser_key,
					href: target_href,
					kind: "popstate",
					phase: "publishing",
					public_call_ids: request.public_call_ids,
					redirect_count: 0,
					restored_scroll: request.restored_scroll,
					replace: true,
					scroll_to_top: true,
					sequence: sequenced.sequence,
					skip_work_indicator: true,
					source,
					state: request.state,
					token: request.token,
				}
			: {
					browser_key: request.browser_key,
					href: target_href,
					kind: "navigation",
					phase: "publishing",
					public_call_ids: request.public_call_ids,
					redirect_count: 0,
					restored_scroll: request.restored_scroll,
					replace: request.replace,
					scroll_to_top: true,
					sequence: sequenced.sequence,
					skip_work_indicator: request.skip_work_indicator,
					source: navigation_active_route_source(source),
					state: request.state,
					token: request.token,
				};
	const prepared: PreparedRoute = {
		css_bundles: [],
		deps: [],
		render_payload: null,
		route,
	};
	const published = begin_route_publication(
		{
			...sequenced.model,
			active_route,
		} as Core4Model,
		active_route,
		prepared,
		{
			did_navigate: source === "popstate" || target_hash !== current_hash,
			hooks: { kind: "none" },
			save_current_scroll:
				source !== "popstate" && target_hash !== current_hash,
			scroll: {
				kind: "apply",
				scroll,
				target_route_id: route_scroll_target_id(route),
			},
			use_view_transition: false,
		},
	);
	return {
		effects: published.effects,
		kind: "same_document",
		model: published.model,
	};
}

function retarget_browser_during_revalidation(
	model: Core4Model,
	request: NavigationRequest,
	target_href: string,
): Core4Transition<"same_document"> {
	const current = model.current;
	if (!current) {
		return settle_navigation_without_route(model, request.public_call_ids);
	}
	const route = {
		...current.route,
		historyState: request.state,
		href: target_href,
	} satisfies RouteState;
	const position: BrowserPosition = {
		href: target_href,
		key: request.browser_key,
		state: request.state,
	};
	const target_hash = normalized_hash_from_href(target_href);
	const current_hash = normalized_hash_from_href(current.position.href);
	const did_navigate =
		request.source === "popstate" || target_hash !== current_hash;
	const effects: Core4Effect[] = [
		{
			history:
				request.source === "popstate"
					? { kind: "none" }
					: {
							href: position.href,
							kind: request.replace ? "replace" : "push",
							state: position.state,
						},
			position,
			type: "apply_history",
		},
	];
	const scroll =
		scroll_for_href_hash(target_href) ??
		(request.source === "popstate"
			? (request.restored_scroll ?? top_scroll_state())
			: top_scroll_state());
	if (scroll) {
		effects.push({
			scroll,
			type: "apply_scroll",
		});
	}
	append_navigation_settlement(
		effects,
		request.public_call_ids,
		did_navigate,
	);
	return {
		effects,
		kind: "same_document",
		model: {
			...model,
			browser: position,
			current: {
				position,
				route,
				sequence: current.sequence,
			},
		},
	};
}

function retarget_active_navigation(
	model: Core4Model,
	request: NavigationRequest,
	target_href: string,
): NavigationStartTransition | undefined {
	const active_route = model.active_route;
	if (
		!active_route ||
		active_route.kind === "boot" ||
		active_route.kind === "revalidation" ||
		!same_document_href(active_route.href, target_href)
	) {
		return undefined;
	}
	const effects: Core4Effect[] = [];
	const source = navigation_active_route_source(request.source);
	const same_intent =
		active_route.kind === "navigation" &&
		active_route.href === target_href &&
		active_route.replace === request.replace &&
		active_route.scroll_to_top === request.scroll_to_top &&
		active_route.skip_work_indicator === request.skip_work_indicator &&
		active_route.source === source &&
		active_route.state === request.state;
	if (!same_intent) {
		settle_active_route_as_not_navigated(effects, active_route);
	}
	if (same_intent) {
		return {
			effects,
			kind: "started",
			model: {
				...model,
				active_route: {
					...active_route,
					public_call_ids: active_route.public_call_ids.concat(
						request.public_call_ids,
					),
				},
			} as Core4Model,
		};
	}
	const next_active_route: NavigationActiveRouteSlot = {
		browser_key: request.browser_key,
		href: target_href,
		kind: "navigation",
		phase: active_route.phase,
		public_call_ids: request.public_call_ids,
		redirect_count: active_route.redirect_count,
		restored_scroll: active_route.restored_scroll,
		replace: request.replace,
		scroll_to_top: request.scroll_to_top,
		sequence: active_route.sequence,
		skip_work_indicator: request.skip_work_indicator,
		source,
		state: request.state,
		token: active_route.token,
	};
	return {
		effects,
		kind: "started",
		model: {
			...model,
			active_route: next_active_route,
		} as Core4Model,
	};
}

function promote_prefetch(
	model: Core4Model,
	request: NavigationRequest,
	target_href: string,
): Core4Transition<"promoted_prefetch"> | undefined {
	if (
		!model.prefetch ||
		!same_document_href(model.prefetch.href, target_href)
	) {
		return undefined;
	}
	const sequenced = take_sequence(model);
	const active_route: ActiveRouteSlot = {
		browser_key: request.browser_key,
		href: target_href,
		kind: "navigation",
		phase:
			model.prefetch.phase === "prepared"
				? "preparing"
				: model.prefetch.phase,
		public_call_ids: request.public_call_ids,
		redirect_count: 0,
		restored_scroll: request.restored_scroll,
		replace: request.replace,
		scroll_to_top: request.scroll_to_top,
		sequence: sequenced.sequence,
		skip_work_indicator: request.skip_work_indicator,
		source: navigation_active_route_source(request.source),
		state: request.state,
		token:
			model.prefetch.phase === "prepared"
				? request.token
				: model.prefetch.token,
	};
	if (model.prefetch.phase === "prepared") {
		const published = begin_route_publication(
			{
				...sequenced.model,
				active_route: {
					...active_route,
					phase: "publishing",
				},
				prefetch: null,
			} as Core4Model,
			{
				...active_route,
				phase: "publishing",
			},
			model.prefetch.prepared,
		);
		return {
			effects: published.effects,
			kind: "promoted_prefetch",
			model: published.model,
		};
	}
	return {
		effects: [],
		kind: "promoted_prefetch",
		model: {
			...sequenced.model,
			active_route,
			prefetch: null,
		} as Core4Model,
	};
}

function redirect_active_route(
	model: Core4Model,
	active_route: ActiveRouteSlot,
	href: string,
	effects: Core4Effect[] = [],
): Core4Transition<"failed" | "redirecting"> | undefined {
	const target_href = resolve_href(href, active_route.href);
	if (!target_href) {
		return fail_active_route(model, active_route, false, effects);
	}
	if (
		!is_http_href(target_href) ||
		!same_origin(target_href, active_route.href)
	) {
		if (is_http_href(target_href)) {
			effects.push({
				href: target_href,
				type: "hard_redirect",
			});
		}
		settle_active_route_as_not_navigated(effects, active_route);
		effects.push({
			token: active_route.token,
			type: "release_route_work",
		});
		return {
			effects,
			kind: "redirecting",
			model: finish_active_route(model, active_route),
		};
	}
	if (
		model.current &&
		same_document_href(target_href, model.current.route.href)
	) {
		settle_active_route_as_not_navigated(effects, active_route);
		effects.push({
			token: active_route.token,
			type: "release_route_work",
		});
		return {
			effects,
			kind: "redirecting",
			model: finish_active_route(model, active_route),
		};
	}
	let redirected: ActiveRouteSlot;
	if (active_route.kind === "boot") {
		redirected = {
			...active_route,
			href: target_href,
			phase: "fetching",
			redirect_count: active_route.redirect_count + 1,
			source: null,
		};
	} else {
		redirected = {
			...active_route,
			href: target_href,
			kind: "navigation",
			phase: "fetching",
			redirect_count: active_route.redirect_count + 1,
			source: "redirect",
		};
	}
	effects.push({
		client_build_id: model.client_build_id,
		href: target_href,
		token: active_route.token,
		trigger: redirected.kind,
		type: "fetch_route",
	});
	return {
		effects,
		kind: "redirecting",
		model: {
			...model,
			active_route: redirected,
		} as Core4Model,
	};
}

function fail_active_route(
	model: Core4Model,
	active_route: ActiveRouteSlot,
	retryable: boolean,
	effects: Core4Effect[] = [],
): Core4Transition<"failed"> {
	settle_active_route_as_not_navigated(effects, active_route);
	effects.push({
		token: active_route.token,
		type: "release_route_work",
	});
	const failed_model = finish_active_route(model, active_route);
	let next_model = failed_model;
	if (active_route.kind === "revalidation") {
		next_model = retryable
			? schedule_refresh_retry(failed_model, effects)
			: finish_refresh_with_result(
					failed_model,
					{ ok: false, reason: "max_retries_exhausted" },
					effects,
				);
	}
	return {
		effects,
		kind: "failed",
		model: next_model,
	};
}

function finish_active_route(
	model: Core4Model,
	active_route: ActiveRouteSlot,
): Core4Model {
	let refresh = model.refresh;
	if (refresh.kind === "running" && active_route.kind === "revalidation") {
		refresh = {
			attempt: refresh.attempt,
			demand: refresh.demand,
			kind: "pending",
		};
	}
	return {
		...model,
		active_route:
			model.active_route?.token === active_route.token
				? null
				: model.active_route,
		publication:
			model.publication?.token === active_route.token
				? null
				: model.publication,
		refresh,
	} as Core4Model;
}

function supersede_active_route(model: Core4Model): Core4Transition<"cleared"> {
	const active_route = model.active_route;
	if (!active_route) {
		return {
			effects: [],
			kind: "cleared",
			model,
		};
	}
	const effects: Core4Effect[] = [
		{
			token: active_route.token,
			type: "abort_route_work",
		},
	];
	settle_active_route_as_not_navigated(effects, active_route);
	return {
		effects,
		kind: "cleared",
		model: finish_active_route(model, active_route),
	};
}

function settle_active_route_as_not_navigated(
	effects: Core4Effect[],
	active_route: ActiveRouteSlot,
): void {
	if (
		active_route.kind !== "boot" &&
		active_route.public_call_ids.length > 0
	) {
		append_navigation_settlement(
			effects,
			active_route.public_call_ids,
			false,
		);
	}
}

function schedule_refresh_retry(
	model: Core4Model,
	effects: Core4Effect[],
): Core4Model {
	if (model.refresh.kind !== "pending") {
		return model;
	}
	if (model.refresh.attempt + 1 >= CORE4_REVALIDATION_MAX_RETRIES) {
		return finish_refresh_with_result(
			model,
			{ ok: false, reason: "max_retries_exhausted" },
			effects,
		);
	}
	const timer_id = `core4-refresh-${model.sequence + 1}` as TimerID;
	const attempt = model.refresh.attempt + 1;
	effects.push({
		id: timer_id,
		ms: refresh_retry_delay_ms(attempt),
		type: "start_refresh_timer",
	});
	return {
		...model,
		refresh: {
			attempt,
			demand: model.refresh.demand,
			kind: "retrying",
			timer_id,
		},
	} as Core4Model;
}

function finish_refresh_with_result(
	model: Core4Model,
	result: RevalidationResult,
	effects: Core4Effect[],
): Core4Model {
	const demand = refresh_demand(model.refresh);
	if (!demand) {
		return model;
	}
	if (
		model.refresh.kind === "debouncing" ||
		model.refresh.kind === "retrying"
	) {
		effects.push({
			id: model.refresh.timer_id,
			type: "clear_refresh_timer",
		});
	}
	if (demand.waiters.length > 0) {
		effects.push({
			ids: demand.waiters.map((waiter) => waiter.id),
			result,
			type: "settle_refresh_calls",
		});
	}
	return {
		...model,
		refresh: { kind: "idle" },
	} as Core4Model;
}

function settle_refresh_if_publication_is_fresh(
	model: Core4Model,
	publication: PublicationSlot,
	effects: Core4Effect[],
): Core4Model {
	const demand = refresh_demand(model.refresh);
	if (!demand) {
		return model;
	}
	if (publication.plan.route_sequence < demand.after_sequence) {
		return model;
	}
	if (
		!same_document_href(
			publication.plan.next.route.href,
			publication.plan.position.href,
		)
	) {
		return model;
	}
	return finish_refresh_with_result(model, { ok: true }, effects);
}

function refresh_demand(refresh: RefreshSlot): RefreshDemand | null {
	if (refresh.kind === "idle") {
		return null;
	}
	return refresh.demand;
}

function publication_reason(active_route: ActiveRouteSlot): PublicationReason {
	if (active_route.kind === "boot") {
		return "boot";
	}
	if (active_route.kind === "popstate") {
		return "popstate";
	}
	if (active_route.kind === "revalidation") {
		return "revalidation";
	}
	return "navigation";
}

function publication_position(
	model: Core4Model,
	active_route: ActiveRouteSlot,
): BrowserPosition {
	if (active_route.kind === "revalidation" && model.browser) {
		return model.browser;
	}
	return {
		href: active_route.href,
		key: active_route.browser_key,
		state: active_route.state,
	};
}

function publication_history(
	active_route: ActiveRouteSlot,
	position: BrowserPosition,
): PublicationHistoryAction {
	if (active_route.kind !== "navigation") {
		return { kind: "none" };
	}
	return {
		href: position.href,
		kind: active_route.replace ? "replace" : "push",
		state: position.state,
	};
}

function publication_hooks(
	model: Core4Model,
	active_route: ActiveRouteSlot,
): PublicationHookPlan {
	const trigger = publication_reason(active_route);
	if (trigger === "boot" || !model.current) {
		return { kind: "none" };
	}
	return {
		kind: "run",
		trigger,
	};
}

function publication_uses_view_transition(
	model: Core4Model,
	active_route: ActiveRouteSlot,
): boolean {
	return (
		model.use_view_transitions &&
		active_route.kind !== "boot" &&
		active_route.kind !== "revalidation"
	);
}

function publication_saves_current_scroll(
	active_route: ActiveRouteSlot,
): boolean {
	return active_route.kind === "navigation";
}

function publication_scroll(
	active_route: ActiveRouteSlot,
	prepared: PreparedRoute,
): PublicationScrollPlan {
	if (active_route.kind === "boot") {
		const scroll =
			active_route.restored_scroll ??
			scroll_for_href_hash(active_route.href);
		if (!scroll) {
			return { kind: "none" };
		}
		return {
			kind: "apply",
			scroll,
			target_route_id: route_scroll_target_id(prepared.route),
		};
	}
	if (
		active_route.kind !== "navigation" &&
		active_route.kind !== "popstate"
	) {
		return { kind: "none" };
	}
	const scroll =
		scroll_for_href_hash(active_route.href) ??
		(active_route.kind === "popstate"
			? (active_route.restored_scroll ?? top_scroll_state())
			: active_route.scroll_to_top === false
				? null
				: top_scroll_state());
	if (!scroll) {
		return { kind: "none" };
	}
	return {
		kind: "apply",
		scroll,
		target_route_id: route_scroll_target_id(prepared.route),
	};
}

function route_scroll_target_id(route: RouteState): string {
	const idx = route.matches.length - 1;
	const pattern = route.matches[idx]?.pattern ?? "";
	return `${idx}${route_id_separator}${pattern}`;
}

function scroll_for_href_hash(href: string): Core4ScrollState | null {
	try {
		const hash = new URL(href).hash;
		if (normalize_hash(hash).length === 0) {
			return null;
		}
		return { hash };
	} catch {
		return null;
	}
}

function top_scroll_state(): Core4ScrollState {
	return { x: 0, y: 0 };
}

function normalized_hash_from_href(href: string): string {
	try {
		return normalize_hash(new URL(href).hash);
	} catch {
		return "";
	}
}

function normalize_hash(hash: string): string {
	const without_prefix = hash.startsWith("#") ? hash.slice(1) : hash;
	if (without_prefix.length === 0) {
		return without_prefix;
	}
	try {
		return decodeURIComponent(without_prefix);
	} catch {
		return without_prefix;
	}
}

function settle_navigation_without_route(
	model: Core4Model,
	public_call_ids: readonly PublicCallID[],
): Core4Transition<"same_document"> {
	const effects: Core4Effect[] = [];
	append_navigation_settlement(effects, public_call_ids, false);
	return {
		effects,
		kind: "same_document",
		model,
	};
}

function append_navigation_settlement(
	effects: Core4Effect[],
	ids: readonly PublicCallID[],
	did_navigate: boolean,
): void {
	if (ids.length === 0) {
		return;
	}
	effects.push({
		ids,
		result: { didNavigate: did_navigate },
		type: "settle_navigation_calls",
	});
}

function take_sequence(model: Core4Model): {
	model: Core4Model;
	sequence: number;
} {
	const sequence = model.sequence + 1;
	return {
		model: {
			...model,
			sequence,
		},
		sequence,
	};
}

function refresh_retry_delay_ms(attempt: number): number {
	return Math.min(
		CORE4_REVALIDATION_BACKOFF_CAP_MS,
		CORE4_REVALIDATION_BACKOFF_BASE_MS * 2 ** Math.max(0, attempt - 1),
	);
}

function route_response_build_skew_report(
	input: RouteResponseClassificationInput,
): RouteBuildSkewReport | undefined {
	if (
		input.owner.kind === "stale" ||
		input.response.headers.get(X_VORMA_BUILD_SKEW) ===
			VORMA_PROTOCOL_ENABLED
	) {
		return undefined;
	}
	const response = build_skew_response_facts(input.response);
	if (!response.server_build_id) {
		return undefined;
	}
	let default_behavior: BuildSkewDefaultBehavior = "notifyOnly";
	if (input.owner.kind === "prefetch") {
		default_behavior = input.response.ok ? "notifyOnly" : "dropResponse";
	} else {
		const redirect_href = response_redirect_href(
			input.response,
			input.requested_href,
		);
		if (
			redirect_href &&
			is_http_href(redirect_href) &&
			!same_origin(redirect_href, input.requested_href)
		) {
			default_behavior =
				input.owner.active_route.kind === "revalidation"
					? "dropResponse"
					: "hardReload";
		}
	}
	return {
		default_behavior,
		requested_href: input.requested_href,
		response,
	};
}

function route_outcome_build_skew_report(
	outcome: RouteResponseOutcome,
): RouteBuildSkewReport | undefined {
	if (outcome.kind === "build_skew") {
		return {
			default_behavior: outcome.default_behavior,
			requested_href: outcome.href,
			response: outcome.response,
		};
	}
	return outcome.build_skew_report;
}

function api_response_build_skew_report(
	input: APIResponseClassificationInput,
): APISubmissionBuildSkewReport | undefined {
	const response = build_skew_response_facts(input.response);
	if (!response.server_build_id) {
		return undefined;
	}
	const redirect_href = response_redirect_href(
		input.response,
		input.requested_href,
	);
	return {
		default_behavior:
			redirect_href &&
			is_http_href(redirect_href) &&
			!same_origin(redirect_href, input.requested_href)
				? "hardReload"
				: "notifyOnly",
		response,
	};
}

function build_skew_response_facts(
	response: Core4ResponseFacts,
): BuildSkewResponseFacts {
	return {
		ok: response.ok,
		server_build_id: response.headers.get(BUILD_ID_HEADER) ?? "",
		status: response.status,
	};
}

function route_build_skew_triggering_response(
	model: Core4Model,
	active_route: ActiveRouteSlot,
	requested_href: string,
	response: BuildSkewResponseFacts,
): BuildSkewTriggeringResponse {
	if (active_route.kind === "revalidation") {
		return {
			kind: "route",
			ok: response.ok,
			requestedHref: requested_href,
			revalidationReason:
				refresh_demand(model.refresh)?.reason ?? "manual",
			status: response.status,
			trigger: "revalidation",
		};
	}
	return {
		kind: "route",
		ok: response.ok,
		requestedHref: requested_href,
		status: response.status,
		trigger: active_route.kind === "popstate" ? "popstate" : "navigation",
	};
}

function append_build_skew_notification(
	effects: Core4Effect[],
	model: Core4Model,
	response: BuildSkewResponseFacts,
	triggering_response: BuildSkewTriggeringResponse,
): void {
	if (
		!model.current ||
		!response.server_build_id ||
		response.server_build_id === model.client_build_id
	) {
		return;
	}
	effects.push({
		notification: {
			activeClientBuildID: model.client_build_id,
			currentRouteState: model.current.route,
			currentWorkState: derive_core4_work_state(model),
			serverBuildID: response.server_build_id,
			triggeringResponse: triggering_response,
		},
		type: "notify_build_skew",
	});
}

function response_redirect_href(
	response: Core4ResponseFacts,
	requested_href: string,
): string | null {
	const soft_redirect = response.headers.get(X_CLIENT_REDIRECT);
	if (soft_redirect) {
		return resolve_href(soft_redirect, requested_href);
	}
	if (
		response.redirected &&
		response.url.length > 0 &&
		response.url !== requested_href
	) {
		return resolve_href(response.url, requested_href);
	}
	return null;
}

function resolve_href(href: string, base_href: string): string | null {
	try {
		return new URL(href, base_href).href;
	} catch {
		return null;
	}
}

function is_http_href(href: string): boolean {
	try {
		const protocol = new URL(href).protocol;
		return protocol === "http:" || protocol === "https:";
	} catch {
		return false;
	}
}

function same_origin(left_href: string, right_href: string): boolean {
	try {
		return new URL(left_href).origin === new URL(right_href).origin;
	} catch {
		return false;
	}
}

function same_document_href(left_href: string, right_href: string): boolean {
	try {
		const left = new URL(left_href);
		const right = new URL(right_href);
		left.hash = "";
		right.hash = "";
		return left.href === right.href;
	} catch {
		return left_href === right_href;
	}
}

function is_abort_error(error: unknown): boolean {
	return (
		error instanceof DOMException &&
		(error.name === abort_error_name || error.message === api_aborted_error)
	);
}

function new_abort_error(): DOMException {
	return new DOMException(api_aborted_error, abort_error_name);
}

function to_error_string(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}
	return String(error);
}

const CORE4_REFRESH_DEBOUNCE_MS = 8;
const CORE4_RELOAD_SCROLL_MAX_AGE_MS = 3333;
const CORE4_MAX_SCROLL_ENTRIES = 50;
const REVALIDATION_OK: RevalidationResult = { ok: true };
let active_focus_cleanup: (() => void) | null = null;

export function create_client_core4(
	_app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: CommitFn,
	test_options?: Core4TestOptions,
): Result<ClientCore> {
	const registry_result = createPatternRegistry({
		dynamicParamPrefixRune: ":",
		explicitIndexSegment: "_index",
		splatSegmentRune: "*",
	});
	if (!registry_result.ok) {
		return R.err(
			`Failed to create pattern registry: ${registry_result.err}`,
		);
	}
	const runtime = new Core4Runtime(registry_result.val, commit, test_options);
	return R.ok(runtime.client_core());
}

class Core4Runtime {
	readonly pattern_registry: PatternRegistry;
	readonly commit: CommitFn;
	readonly test_options: Core4TestOptions | undefined;
	readonly route_abort_controllers = new Map<Core4Token, AbortController>();
	readonly api_abort_controllers = new Map<Core4Token, AbortController>();
	readonly navigation_waiters = new Map<
		PublicCallID,
		Deferred<{ didNavigate: boolean }>
	>();
	readonly refresh_waiters = new Map<
		PublicCallID,
		Deferred<RevalidationResult>
	>();
	readonly refresh_timers = new Map<TimerID, number>();
	readonly client_loader_prefetches = new Map<
		Core4Token,
		Core4ClientLoaderPrefetch[]
	>();
	readonly api_waiters = new Map<
		Core4Token,
		{
			deferred: Deferred<Core4APIResult<unknown>>;
			revalidation: Promise<RevalidationResult>;
		}
	>();
	readonly module_cache = new Map<string, Record<string, unknown>>();
	readonly route_loaders = new Map<string, Core4ClientLoader>();
	readonly search_schemas = new Map<string, unknown>();
	readonly work_indicator = create_core4_work_indicator();
	readonly hard_redirect: (url: string) => void;
	readonly reload_page: () => void;
	model: Core4Model = create_core4_model({ client_build_id: "" });
	client_build_id = "";
	deployment_id = "";
	boot_waiter: Deferred<Result<void>> | null = null;
	default_error_boundary:
		| ((props: { error: unknown }) => unknown)
		| undefined;
	current_render_state: RouteRenderState | null = null;
	last_activity_ms = Date.now();
	user_on_build_skew_detected:
		| ((event: BuildSkewNotification) => void)
		| undefined;
	user_on_route_update:
		| ((
				route: RouteState,
				previous_route: RouteState | null,
				reason: RouteUpdateReason,
		  ) => void)
		| undefined;
	user_on_work_update: ((work: WorkState) => void) | undefined;
	focus_cleanup: (() => void) | null = null;
	next_id = 0;
	pumping_revalidation = false;
	last_work_json = JSON.stringify(derive_core4_work_state(this.model));
	work_indicator_options: WorkIndicatorOptions | undefined;

	constructor(
		pattern_registry: PatternRegistry,
		commit: CommitFn,
		test_options: Core4TestOptions | undefined,
	) {
		this.pattern_registry = pattern_registry;
		this.commit = commit;
		this.test_options = test_options;
		this.hard_redirect =
			test_options?.hard_redirect ??
			((url: string): void => {
				window.location.assign(url);
			});
		this.reload_page =
			test_options?.reload ??
			((): void => {
				window.location.reload();
			});
	}

	client_core(): ClientCore {
		return {
			boot: (options): Promise<Result<void>> => {
				return this.boot(options);
			},
			defineView: <T = any>(input: {
				beforeRouteCommit?: BeforeRouteCommitFn;
				beforeRouteYield?: BeforeRouteYieldFn;
				clientLoader?: (props: any) => Promise<T>;
				component: (props: any) => any;
				errorBoundary?: (props: { error: unknown }) => any;
				pattern: string;
				runClientLoaderOnHMR?: boolean;
			}): ViewDefinition & { __phantom_client_loader_data?: T } => {
				return this.defineView<T>(input);
			},
			getClientBuildID: (): string => {
				return this.client_build_id;
			},
			getRootEl: (): HTMLElement => {
				return this.getRootEl();
			},
			getRouteState: (): RouteState => {
				return this.getRouteState();
			},
			getWorkState: (): WorkState => {
				return this.getWorkState();
			},
			get_default_error_boundary: ():
				| ((props: { error: unknown }) => any)
				| undefined => {
				return this.default_error_boundary as
					| ((props: { error: unknown }) => any)
					| undefined;
			},
			navigate: (href, options): Promise<{ didNavigate: boolean }> => {
				return this.navigate(href, options);
			},
			revalidate: (): Promise<RevalidationResult> => {
				return this.revalidate();
			},
			save_current_scroll: (): void => {
				this.save_current_scroll();
			},
			start_prefetch: (href): void => {
				this.start_prefetch(href);
			},
			stop_prefetch: (href): void => {
				this.stop_prefetch(href);
			},
			submit_inner: <T = unknown>(
				url: string | URL,
				request_init?: RequestInit,
				options?: {
					apiRouteKind?: APIRouteKind;
					dedupeKey?: string;
					revalidate?: boolean;
					skipWorkIndicator?: boolean;
				},
			): Promise<Core4APIResult<T>> => {
				return this.submit_inner<T>(url, request_init, options);
			},
			workIndicator: this.work_indicator.indicator,
		};
	}

	new_token(prefix: string): Core4Token {
		this.next_id++;
		return `core4-${prefix}-${this.next_id}` as Core4Token;
	}

	new_public_call_id(prefix: string): PublicCallID {
		this.next_id++;
		return `core4-call-${prefix}-${this.next_id}` as PublicCallID;
	}

	new_history_key(): BrowserKey {
		this.next_id++;
		return `core4-history-${this.next_id.toString(36)}` as BrowserKey;
	}

	new_timer_id(prefix: string): TimerID {
		this.next_id++;
		return `core4-timer-${prefix}-${this.next_id}` as TimerID;
	}

	accept_transition(
		transition: Core4Transition | undefined,
		options?: { before_effects?: () => void },
	): boolean {
		if (!transition) {
			return false;
		}
		this.model = transition.model;
		this.notify_work_update();
		options?.before_effects?.();
		for (const effect of transition.effects) {
			this.run_effect(effect);
		}
		this.pump_revalidation();
		return true;
	}

	fail_boot(error: string): void {
		if (!this.boot_waiter) {
			return;
		}
		this.boot_waiter.resolve(R.err(error));
		this.boot_waiter = null;
	}

	run_effect(effect: Core4Effect): void {
		if (effect.type === "hard_redirect") {
			this.hard_redirect(effect.href);
			return;
		}
		if (effect.type === "notify_build_skew") {
			this.user_on_build_skew_detected?.(effect.notification);
			return;
		}
		if (effect.type === "abort_route_work") {
			this.route_abort_controllers.get(effect.token)?.abort();
			this.route_abort_controllers.delete(effect.token);
			this.abort_client_loader_prefetches(effect.token);
			return;
		}
		if (effect.type === "release_route_work") {
			this.route_abort_controllers.delete(effect.token);
			this.abort_client_loader_prefetches(effect.token);
			return;
		}
		if (effect.type === "abort_api_submission") {
			this.api_abort_controllers.get(effect.token)?.abort();
			this.api_abort_controllers.delete(effect.token);
			return;
		}
		if (effect.type === "release_api_submission") {
			this.api_abort_controllers.delete(effect.token);
			return;
		}
		if (effect.type === "fetch_route") {
			void this.run_route_fetch(effect);
			return;
		}
		if (effect.type === "prepare_route") {
			void this.run_route_prepare(effect);
			return;
		}
		if (effect.type === "publish_route") {
			if (!effect.plan.next.render_payload) {
				void this.run_publication(effect.plan);
				return;
			}
			queueMicrotask(() => {
				void this.run_publication(effect.plan);
			});
			return;
		}
		if (effect.type === "settle_navigation_calls") {
			for (const id of effect.ids) {
				this.navigation_waiters.get(id)?.resolve(effect.result);
				this.navigation_waiters.delete(id);
			}
			return;
		}
		if (effect.type === "settle_refresh_calls") {
			for (const id of effect.ids) {
				this.refresh_waiters.get(id)?.resolve(effect.result);
				this.refresh_waiters.delete(id);
			}
			return;
		}
		if (effect.type === "settle_api_submission") {
			this.settle_api_submission(effect.token, effect.result);
			return;
		}
		if (effect.type === "start_refresh_timer") {
			this.clear_refresh_timer(effect.id);
			const timer = window.setTimeout(() => {
				this.refresh_timers.delete(effect.id);
				this.accept_transition(
					fire_refresh_timer(this.model, effect.id),
				);
			}, effect.ms);
			this.refresh_timers.set(effect.id, timer);
			return;
		}
		if (effect.type === "clear_refresh_timer") {
			this.clear_refresh_timer(effect.id);
			return;
		}
		if (effect.type === "apply_scroll") {
			this.apply_core4_scroll(effect.scroll);
			return;
		}
		if (effect.type === "apply_history") {
			this.apply_history_action(effect.history, effect.position);
			return;
		}
		if (effect.type === "fetch_api") {
			void this.run_api_fetch(effect);
			return;
		}
		const _exhaustive: never = effect;
		return _exhaustive;
	}

	clear_refresh_timer(id: TimerID): void {
		const timer = this.refresh_timers.get(id);
		if (timer === undefined) {
			return;
		}
		this.refresh_timers.delete(id);
		window.clearTimeout(timer);
	}

	pump_revalidation(): void {
		if (this.pumping_revalidation) {
			return;
		}
		this.pumping_revalidation = true;
		try {
			const transition = begin_pending_revalidation(this.model, {
				token: this.new_token("revalidation"),
			});
			if (transition) {
				this.accept_transition(transition);
			}
		} finally {
			this.pumping_revalidation = false;
		}
	}

	start_deferred_redirect(): void {
		if (!this.model.deferred_api_redirect || !this.model.browser) {
			return;
		}
		this.accept_transition(
			begin_deferred_api_redirect(this.model, {
				browser_key: this.new_history_key(),
				state: undefined,
				token: this.new_token("deferred-api-redirect"),
			}),
		);
	}

	async run_route_fetch(
		effect: Extract<Core4Effect, { type: "fetch_route" }>,
	): Promise<void> {
		const controller = new AbortController();
		this.route_abort_controllers.set(effect.token, controller);
		this.abort_client_loader_prefetches(effect.token);
		this.client_loader_prefetches.set(
			effect.token,
			this.start_client_loader_prefetches(effect, controller.signal),
		);
		try {
			const result = await this.fetch_route_payload(
				effect,
				controller.signal,
			);
			if (controller.signal.aborted) {
				return;
			}
			const response_facts = result.response
				? this.response_facts(result.response)
				: undefined;
			const owner = route_response_owner(this.model, effect.token);
			const outcome = response_facts
				? classify_route_response({
						owner,
						payload:
							result.kind === "data" ? result.payload : undefined,
						requested_href: effect.href,
						response: response_facts,
						token: effect.token,
					})
				: classify_route_failure({
						owner,
						retryable: true,
						token: effect.token,
					});
			const transition = accept_route_response(this.model, outcome);
			this.accept_transition(transition);
			if (effect.trigger === "boot" && outcome.kind !== "data") {
				this.fail_boot("Initial route request failed");
			}
		} catch {
			if (controller.signal.aborted) {
				return;
			}
			const transition = accept_route_response(
				this.model,
				classify_route_failure({
					owner: route_response_owner(this.model, effect.token),
					retryable: effect.trigger === "revalidation",
					token: effect.token,
				}),
			);
			this.accept_transition(transition);
			if (effect.trigger === "boot") {
				this.fail_boot("Initial route request failed");
			}
		}
	}

	async fetch_route_payload(
		effect: Extract<Core4Effect, { type: "fetch_route" }>,
		signal: AbortSignal,
	): Promise<Core4FetchResult> {
		const url = new URL(effect.href);
		url.searchParams.set(VORMA_JSON_KEY, effect.client_build_id);
		if (effect.trigger === "revalidation" && this.deployment_id) {
			url.searchParams.set(
				VERCEL_DPL_QUERY_PARAM_KEY,
				this.deployment_id,
			);
		}
		const response = await fetch(url, {
			headers: { [X_ACCEPTS_CLIENT_REDIRECT]: VORMA_PROTOCOL_ENABLED },
			signal,
		});
		if (!response.ok) {
			return { kind: "failed", response };
		}
		try {
			const payload = this.decode_route_payload(
				await response.json(),
				new URL(effect.href),
			);
			return { kind: "data", payload, response };
		} catch {
			return { kind: "failed", response };
		}
	}

	async run_route_prepare(
		effect: Extract<Core4Effect, { type: "prepare_route" }>,
	): Promise<void> {
		const controller =
			this.route_abort_controllers.get(effect.token) ??
			new AbortController();
		this.route_abort_controllers.set(effect.token, controller);
		try {
			const prepared = await this.prepare_route_payload(
				effect.payload,
				effect.trigger === "popstate" ? "navigation" : effect.trigger,
				effect.payload.routes.length > 0
					? effect.payload.routes[0]!.pattern
					: "",
				effect.href,
				effect.history_state,
				effect.token,
				controller.signal,
			);
			if (controller.signal.aborted || !prepared) {
				this.accept_transition(
					accept_route_preparation(this.model, {
						kind: "aborted",
						token: effect.token,
					}),
				);
				if (effect.trigger === "boot") {
					this.fail_boot("Initial route preparation was canceled");
				}
				this.route_abort_controllers.delete(effect.token);
				return;
			}
			const transition = accept_route_preparation(this.model, {
				kind: "prepared",
				prepared,
				token: effect.token,
			});
			this.accept_transition(transition);
		} catch {
			const transition = accept_route_preparation(this.model, {
				kind: "failed",
				retryable: effect.trigger === "revalidation",
				token: effect.token,
			});
			this.accept_transition(transition);
			if (effect.trigger === "boot") {
				this.fail_boot("Initial route preparation failed");
			}
		}
	}

	async prepare_route_payload(
		payload: RoutePayload,
		trigger: "boot" | "navigation" | "prefetch" | "revalidation",
		_fallback_pattern: string,
		href: string,
		history_state: unknown,
		token: Core4Token,
		signal: AbortSignal,
	): Promise<PreparedRoute | null> {
		preload_css([...payload.css_bundles]);
		preload_modules([...payload.deps]);
		const modules = await this.import_route_modules(payload, signal);
		if (signal.aborted) {
			return null;
		}
		if (trigger === "boot") {
			const active_route = this.model.active_route;
			if (active_route?.kind === "boot" && active_route.token === token) {
				const provisional_render_state = this.build_render_state(
					payload,
					modules,
					payload.routes.map(() => {
						return undefined;
					}),
				);
				this.accept_transition(
					accept_boot_provisional_route(this.model, {
						position: {
							href,
							key: active_route.browser_key,
							state: history_state,
						},
						route: this.route_state_from_render_state(
							provisional_render_state,
							href,
							history_state,
						),
						token,
					}),
				);
			}
		}
		const loader_results = await this.run_client_loaders_after_response(
			payload,
			modules,
			trigger,
			href,
			history_state,
			token,
			signal,
		);
		this.client_loader_prefetches.delete(token);
		if (signal.aborted) {
			return null;
		}
		await wait_for_css([...payload.css_bundles], signal);
		if (signal.aborted) {
			return null;
		}
		const render_state = this.build_render_state(
			payload,
			modules,
			loader_results,
		);
		return {
			css_bundles: payload.css_bundles,
			deps: payload.deps,
			render_payload: {
				css_bundles: payload.css_bundles,
				deps: payload.deps,
				meta_head_els: payload.meta_head_els as readonly HeadEl[],
				render_state,
				rest_head_els: payload.rest_head_els as readonly HeadEl[],
				title: payload.title,
			} satisfies Core4PreparedRenderPayload,
			route: this.route_state_from_render_state(
				render_state,
				href,
				history_state,
			),
		};
	}

	route_state_from_render_state(
		render_state: RouteRenderState,
		href: string,
		history_state: unknown,
	): RouteState {
		return {
			clientBuildID: render_state.client_build_id,
			error: render_state.error,
			historyState: history_state,
			href,
			matches: render_state.entries.map((entry) => {
				return {
					clientLoaderData: entry.client_loader_data,
					input: entry.input,
					loaderData: entry.loader_data,
					pattern: entry.pattern,
				};
			}),
			params: render_state.params,
			splatValues: [...render_state.splat_values],
		};
	}

	async import_route_modules(
		payload: RoutePayload,
		signal: AbortSignal,
	): Promise<Map<string, Record<string, unknown>>> {
		const modules = new Map<string, Record<string, unknown>>();
		for (const route of payload.routes) {
			if (signal.aborted) {
				return modules;
			}
			if (!route.module_url) {
				modules.set(route.module_url, {});
				continue;
			}
			const cache_key = this.normalize_core4_module_url(route.module_url);
			let module = this.module_cache.get(cache_key);
			if (!module) {
				module = (await import(
					/* @vite-ignore */ route.module_url
				)) as Record<string, unknown>;
				this.module_cache.set(cache_key, module);
			}
			modules.set(route.module_url, module);
			const view = module.default as ViewDefinition | undefined;
			if (view?.client_loader) {
				this.route_loaders.set(route.pattern, view.client_loader);
				registerPattern(this.pattern_registry, route.pattern);
			}
		}
		return modules;
	}

	async run_client_loaders_after_response(
		payload: RoutePayload,
		modules: Map<string, Record<string, unknown>>,
		trigger: "boot" | "navigation" | "prefetch" | "revalidation",
		href: string,
		history_state: unknown,
		token: Core4Token,
		signal: AbortSignal,
	): Promise<Array<{ data: unknown } | { error: unknown } | undefined>> {
		const prefetches = this.client_loader_prefetches.get(token) ?? [];
		const by_pattern = new Map<string, Core4ClientLoaderPrefetch>();
		for (const prefetch of prefetches) {
			by_pattern.set(prefetch.pattern, prefetch);
		}
		const known_matches = payload.routes.map((route) => {
			return { input: route.input, pattern: route.pattern };
		});
		const error_idx = payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		const abort_later: Array<(() => void) | null> = [];
		const results: Array<Promise<unknown>> = [];
		const retained_prefetches = new Set<Core4ClientLoaderPrefetch>();
		for (let idx = 0; idx < payload.routes.length; idx++) {
			const route = payload.routes[idx]!;
			const prefetch = by_pattern.get(route.pattern);
			if (error_idx !== -1 && idx >= error_idx) {
				prefetch?.abort();
				abort_later.push(null);
				results.push(Promise.resolve(undefined));
				continue;
			}
			if (prefetch) {
				prefetch.resolve_server_state(
					this.build_client_loader_server_state(payload, idx),
				);
				retained_prefetches.add(prefetch);
				abort_later.push(prefetch.abort);
				results.push(prefetch.result_promise);
				continue;
			}
			const module = modules.get(route.module_url);
			const view = module?.default as ViewDefinition | undefined;
			const loader =
				view?.client_loader ?? this.route_loaders.get(route.pattern);
			if (!loader) {
				abort_later.push(null);
				results.push(Promise.resolve(undefined));
				continue;
			}
			const loader_controller = new AbortController();
			abort_later.push(() => {
				loader_controller.abort(new_abort_error());
			});
			if (signal.aborted) {
				loader_controller.abort(new_abort_error());
			} else {
				signal.addEventListener(
					"abort",
					() => {
						loader_controller.abort(new_abort_error());
					},
					{ once: true },
				);
			}
			results.push(
				(loader as Core4ClientLoader)({
					historyState: history_state,
					href,
					input: route.input,
					knownMatches: known_matches,
					loaderData: route.loader_data,
					params: payload.params,
					pattern: route.pattern,
					serverPromise: Promise.resolve(
						this.build_client_loader_server_state(payload, idx),
					),
					signal: loader_controller.signal,
					splatValues: [...payload.splat_values],
					trigger,
				}),
			);
		}
		for (const prefetch of prefetches) {
			if (!retained_prefetches.has(prefetch)) {
				prefetch.abort();
			}
		}
		const settled = await Promise.allSettled(
			results.map((result, idx) => {
				return result.catch((error) => {
					if (!is_abort_error(error)) {
						for (
							let abort_idx = idx + 1;
							abort_idx < abort_later.length;
							abort_idx++
						) {
							abort_later[abort_idx]?.();
						}
					}
					throw error;
				});
			}),
		);
		const output: Array<
			{ data: unknown } | { error: unknown } | undefined
		> = [];
		for (const result of settled) {
			if (result.status === "fulfilled") {
				output.push(
					result.value === undefined
						? undefined
						: { data: result.value },
				);
				continue;
			}
			if (is_abort_error(result.reason)) {
				output.push(undefined);
				break;
			}
			output.push({ error: to_error_string(result.reason) });
			break;
		}
		return output;
	}

	build_render_state(
		payload: RoutePayload,
		modules: Map<string, Record<string, unknown>>,
		loader_results: Array<
			{ data: unknown } | { error: unknown } | undefined
		>,
	): RouteRenderState {
		const server_error_idx = payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		const client_error_idx = loader_results.findIndex((result) => {
			return !!result && "error" in result;
		});
		let error: RouteErrorState | null = null;
		if (server_error_idx !== -1) {
			error = {
				error: payload.routes[server_error_idx]!.server_error,
				idx: server_error_idx,
				source: "server",
			};
		} else if (client_error_idx !== -1) {
			error = {
				error: (loader_results[client_error_idx] as { error: unknown })
					.error,
				idx: client_error_idx,
				source: "clientLoader",
			};
		}
		return {
			client_build_id: payload.server_build_id || this.client_build_id,
			entries: payload.routes.map((route, idx) => {
				const loader_result = loader_results[idx];
				return {
					client_loader_data:
						loader_result && "data" in loader_result
							? loader_result.data
							: undefined,
					input: route.input,
					loader_data: route.loader_data,
					module: modules.get(route.module_url) ?? {},
					module_url: route.module_url,
					pattern: route.pattern,
				};
			}),
			error,
			history_state: undefined,
			params: payload.params,
			splat_values: [...payload.splat_values],
		};
	}

	build_client_loader_server_state(
		payload: RoutePayload,
		idx: number,
	): Core4ClientLoaderServerState {
		const server_error_idx = payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		return {
			clientBuildID: payload.server_build_id || this.client_build_id,
			loaderData: payload.routes[idx]?.loader_data,
			matches: payload.routes.map((route) => {
				return {
					input: route.input,
					loaderData: route.loader_data,
					pattern: route.pattern,
				};
			}),
			outermostServerError:
				server_error_idx === -1
					? null
					: {
							error: payload.routes[server_error_idx]!
								.server_error,
							idx: server_error_idx,
						},
		};
	}

	start_client_loader_prefetches(
		effect: Extract<Core4Effect, { type: "fetch_route" }>,
		signal: AbortSignal,
	): Core4ClientLoaderPrefetch[] {
		if (effect.trigger === "boot") {
			return [];
		}
		const url = new URL(effect.href);
		let match = findNestedMatches(this.pattern_registry, url.pathname);
		if (!match) {
			const segments = url.pathname.split("/").filter(Boolean);
			for (let idx = segments.length; idx >= 0; idx--) {
				const partial =
					idx === 0 ? "/" : `/${segments.slice(0, idx).join("/")}`;
				match = findNestedMatches(this.pattern_registry, partial);
				if (match) {
					break;
				}
			}
		}
		if (!match) {
			return [];
		}
		const known_matches = match.matches.map((matched) => {
			const pattern = matched.registeredPattern.originalPattern;
			return {
				input: parseSearchParams(
					this.search_schemas.get(pattern),
					url.searchParams,
				),
				pattern,
			};
		});
		const input_by_pattern = new Map(
			known_matches.map((known_match) => {
				return [known_match.pattern, known_match.input] as const;
			}),
		);
		const trigger =
			effect.trigger === "prefetch"
				? "prefetch"
				: effect.trigger === "revalidation"
					? "revalidation"
					: "navigation";
		const history_state =
			this.model.active_route?.token === effect.token
				? this.model.active_route.state
				: this.model.browser?.state;
		const prefetches: Core4ClientLoaderPrefetch[] = [];
		for (const matched of match.matches) {
			const pattern = matched.registeredPattern.originalPattern;
			const loader = this.route_loaders.get(pattern);
			if (!loader) {
				continue;
			}
			prefetches.push(
				this.start_client_loader_prefetch({
					history_state,
					href: effect.href,
					input: input_by_pattern.get(pattern),
					known_matches,
					loader,
					params: match.params,
					pattern,
					signal,
					splat_values: [...match.splatValues],
					trigger,
				}),
			);
		}
		return prefetches;
	}

	start_client_loader_prefetch(input: {
		history_state: unknown;
		href: string;
		input: unknown;
		known_matches: ClientLoaderKnownMatch[];
		loader: Core4ClientLoader;
		params: Record<string, string>;
		pattern: string;
		signal: AbortSignal;
		splat_values: string[];
		trigger: "navigation" | "prefetch" | "revalidation";
	}): Core4ClientLoaderPrefetch {
		let resolve_server_state!: (
			server_state: Core4ClientLoaderServerState,
		) => void;
		let reject_server_state!: (error: unknown) => void;
		const server_promise = new Promise<Core4ClientLoaderServerState>(
			(resolve, reject) => {
				resolve_server_state = resolve;
				reject_server_state = reject;
			},
		);
		server_promise.catch(() => {});
		const loader_controller = new AbortController();
		const abort = (): void => {
			loader_controller.abort(new_abort_error());
			reject_server_state(new_abort_error());
		};
		if (input.signal.aborted) {
			abort();
		} else {
			input.signal.addEventListener("abort", abort, { once: true });
		}
		const result_promise = input.loader({
			historyState: input.history_state,
			href: input.href,
			input: input.input,
			knownMatches: input.known_matches,
			loaderData: undefined,
			params: input.params,
			pattern: input.pattern,
			serverPromise: server_promise,
			signal: loader_controller.signal,
			splatValues: input.splat_values,
			trigger: input.trigger,
		});
		result_promise.catch(() => {});
		return {
			abort,
			pattern: input.pattern,
			resolve_server_state,
			result_promise,
		};
	}

	abort_client_loader_prefetches(token: Core4Token): void {
		const prefetches = this.client_loader_prefetches.get(token);
		if (!prefetches) {
			return;
		}
		this.client_loader_prefetches.delete(token);
		for (const prefetch of prefetches) {
			prefetch.abort();
		}
	}

	async run_publication(plan: PublicationPlan): Promise<void> {
		const payload = plan.next
			.render_payload as Core4PreparedRenderPayload | null;
		if (!payload) {
			await this.execute_publication_transaction(plan, null);
			return;
		}
		const publish = async (): Promise<void> => {
			await this.execute_publication_transaction(plan, payload);
		};
		const view_transition =
			plan.use_view_transition && "startViewTransition" in document
				? (
						document as Document & {
							startViewTransition?: (
								callback: () => void | Promise<void>,
							) => {
								finished?: Promise<void>;
								updateCallbackDone?: Promise<void>;
							};
						}
					).startViewTransition
				: undefined;
		if (!view_transition) {
			await publish();
			return;
		}
		const transition = view_transition.call(document, publish);
		await (transition.updateCallbackDone ?? transition.finished);
		if (transition.finished) {
			await transition.finished;
		}
	}

	async execute_publication_transaction(
		plan: PublicationPlan,
		payload: Core4PreparedRenderPayload | null,
	): Promise<void> {
		const current_render_state = this.current_render_state;
		if (!payload && plan.reason === "boot") {
			this.fail_publication_transaction(
				plan,
				"Initial route publication was missing render state",
			);
			return;
		}
		if (!payload && !current_render_state) {
			this.fail_publication_transaction(plan);
			return;
		}
		const next_render_state_source =
			payload?.render_state ?? current_render_state;
		if (!next_render_state_source) {
			this.fail_publication_transaction(plan);
			return;
		}
		if (plan.hooks.kind === "run") {
			try {
				await this.run_publication_hooks(plan);
			} catch {
				this.fail_publication_transaction(plan);
				return;
			}
		}
		if (plan.save_current_scroll) {
			this.save_current_scroll();
		}
		this.apply_publication_history(plan);
		if (payload) {
			apply_head_and_title(
				payload.title,
				[...payload.meta_head_els],
				[...payload.rest_head_els],
			);
			apply_css_bundles([...payload.css_bundles]);
			preload_modules([...payload.deps]);
		}
		const committed = commit_publication(this.model, {
			token: plan.token,
		});
		if (!committed) {
			this.route_abort_controllers.delete(plan.token);
			return;
		}
		this.accept_transition(committed);
		const next_render_state: RouteRenderState = {
			...next_render_state_source,
			history_state: plan.position.state,
		};
		this.current_render_state = next_render_state;
		const scroll_intent = this.publication_scroll_intent(plan.scroll);
		const client_commit: ClientCommit = {
			route_render: {
				scroll_intent,
				state: next_render_state,
			},
			work: {
				...derive_core4_work_state(this.model),
				navigation: null,
			},
		};
		const previous_route = plan.previous?.route ?? null;
		if (
			!previous_route ||
			!jsonDeepEquals(previous_route, plan.next.route)
		) {
			client_commit.route_update = {
				previous_route,
				reason: plan.reason,
				route: plan.next.route,
			};
		}
		this.commit(client_commit);
		if (client_commit.route_update) {
			this.user_on_route_update?.(
				client_commit.route_update.route,
				client_commit.route_update.previous_route,
				client_commit.route_update.reason,
			);
		}
		if (scroll_intent) {
			this.apply_core4_scroll(scroll_intent.scroll);
		}
		if (payload) {
			this.last_activity_ms = Date.now();
		}
		this.accept_transition(settle_publication(this.model, plan.token));
		if (plan.reason === "boot" && this.boot_waiter) {
			this.boot_waiter.resolve(R.ok(undefined));
			this.boot_waiter = null;
			this.start_deferred_redirect();
		}
	}

	fail_publication_transaction(
		plan: PublicationPlan,
		boot_error?: string,
	): void {
		if (boot_error) {
			this.fail_boot(boot_error);
		}
		this.accept_transition(fail_publication(this.model, plan.token));
	}

	async run_publication_hooks(plan: PublicationPlan): Promise<void> {
		if (!this.current_render_state || plan.hooks.kind !== "run") {
			return;
		}
		const next_payload = plan.next
			.render_payload as Core4PreparedRenderPayload | null;
		if (!next_payload) {
			return;
		}
		const current = this.model.current?.route ?? plan.previous?.route;
		if (!current) {
			return;
		}
		const next = plan.next.route;
		const hooks: Array<BeforeRouteCommitFn | BeforeRouteYieldFn> = [];
		for (const entry of this.current_render_state.entries) {
			const view = entry.module.default as ViewDefinition | undefined;
			if (view?.before_route_yield) {
				hooks.push(view.before_route_yield);
			}
		}
		for (const entry of next_payload.render_state.entries) {
			const view = entry.module.default as ViewDefinition | undefined;
			if (view?.before_route_commit) {
				hooks.push(view.before_route_commit);
			}
		}
		await Promise.all(
			hooks.map((hook) => {
				return hook({
					current,
					next,
					signal:
						this.route_abort_controllers.get(plan.token)?.signal ??
						new AbortController().signal,
					trigger:
						plan.hooks.kind === "run"
							? plan.hooks.trigger
							: "navigation",
				});
			}),
		);
	}

	apply_publication_history(plan: PublicationPlan): void {
		this.apply_history_action(plan.history, plan.position);
	}

	apply_history_action(
		history: PublicationHistoryAction,
		position: BrowserPosition,
	): void {
		if (history.kind === "none") {
			return;
		}
		const next_state = {
			[HISTORY_KEY_FIELD]: position.key,
			[HISTORY_USER_STATE_FIELD]: position.state,
		};
		if (history.kind === "replace") {
			const existing = window.history.state;
			const base =
				existing && typeof existing === "object" ? existing : {};
			window.history.replaceState(
				{ ...(base as object), ...next_state },
				"",
				history.href,
			);
		} else {
			window.history.pushState(next_state, "", history.href);
		}
	}

	publication_scroll_intent(
		scroll: PublicationPlan["scroll"],
	): ScrollIntent | undefined {
		if (scroll.kind === "none") {
			return undefined;
		}
		return {
			scroll: scroll.scroll,
			target_route_id: scroll.target_route_id,
		};
	}

	async run_api_fetch(
		effect: Extract<Core4Effect, { type: "fetch_api" }>,
	): Promise<void> {
		const controller = new AbortController();
		this.api_abort_controllers.set(effect.token, controller);
		let dispatched = true;
		try {
			const response = await fetch(effect.href, {
				...effect.request_init,
				method: effect.method,
				signal: controller.signal,
			});
			const response_facts = this.response_facts(response);
			const data = await this.read_api_response_data(response);
			this.accept_transition(
				accept_api_submission_outcome(
					this.model,
					classify_api_response({
						browser_key: this.new_history_key(),
						data,
						navigation_token: this.new_token("api-redirect"),
						requested_href: effect.href,
						response: response_facts,
						state: undefined,
						submission:
							this.model.submissions[effect.token] ?? null,
						token: effect.token,
					}),
				),
			);
		} catch (error) {
			if (controller.signal.aborted) {
				this.accept_transition(
					accept_api_submission_outcome(
						this.model,
						classify_api_runtime_failure({
							dispatched,
							error: api_aborted_error,
							kind: "aborted",
							submission:
								this.model.submissions[effect.token] ?? null,
							token: effect.token,
						}),
					),
				);
				return;
			}
			this.accept_transition(
				accept_api_submission_outcome(
					this.model,
					classify_api_runtime_failure({
						dispatched,
						error: is_abort_error(error)
							? api_aborted_error
							: error instanceof Error
								? error.message
								: String(error),
						kind: "network_error",
						submission:
							this.model.submissions[effect.token] ?? null,
						token: effect.token,
					}),
				),
			);
		}
	}

	async read_api_response_data(response: Response): Promise<unknown> {
		if (response.status === 204) {
			return undefined;
		}
		const content_type = response.headers
			.get(CONTENT_TYPE_HEADER)
			?.toLowerCase();
		if (content_type?.includes("json")) {
			return response.json();
		}
		const text = await response.text();
		return text.length === 0 ? undefined : text;
	}

	settle_api_submission(
		token: Core4Token,
		result: APISubmissionResult,
	): void {
		const waiter = this.api_waiters.get(token);
		if (!waiter) {
			return;
		}
		this.api_waiters.delete(token);
		if (result.success) {
			waiter.deferred.resolve({
				data: result.data,
				response: result.response as Response,
				revalidationPromise: waiter.revalidation,
				success: true,
			});
			return;
		}
		waiter.deferred.resolve({
			error: result.error,
			response: result.response as Response | undefined,
			revalidationPromise: waiter.revalidation,
			success: false,
		});
	}

	notify_work_update(): void {
		const work = derive_core4_work_state(this.model);
		const next_json = JSON.stringify(work);
		this.sync_work_indicator(derive_core4_work_projection(this.model));
		if (next_json === this.last_work_json) {
			return;
		}
		this.last_work_json = next_json;
		this.user_on_work_update?.(work);
		this.commit({ work });
	}

	sync_work_indicator(projection: readonly Core4WorkProjection[]): void {
		const options = this.work_indicator_options;
		if (!options) {
			this.work_indicator.set_vorma_active(false);
			return;
		}
		for (const work of projection) {
			if (work.kind === "prefetch") {
				continue;
			}
			if (
				work.kind === "navigation" &&
				options.skipNavigations !== true &&
				!work.skip_work_indicator
			) {
				this.work_indicator.set_vorma_active(true);
				return;
			}
			if (
				work.kind === "revalidation" &&
				options.skipRevalidations !== true &&
				!work.skip_work_indicator
			) {
				this.work_indicator.set_vorma_active(true);
				return;
			}
			if (
				work.kind === "apiRequest" &&
				options.skipAPIRequests !== true &&
				!work.skip_work_indicator
			) {
				this.work_indicator.set_vorma_active(true);
				return;
			}
		}
		this.work_indicator.set_vorma_active(false);
	}

	async boot(options: ClientOptions): Promise<Result<void>> {
		if (this.model.phase !== "booting") {
			this.work_indicator_options = options.workIndicator;
			this.work_indicator.configure(options.workIndicator);
			return R.ok(undefined);
		}
		const payload_el = document.getElementById(DATA_SCRIPT_ID);
		if (!payload_el) {
			return R.err(`Missing element: #${DATA_SCRIPT_ID}`);
		}
		let raw_payload: unknown;
		try {
			raw_payload = JSON.parse(payload_el.textContent ?? "{}");
		} catch (error) {
			return R.err(
				error instanceof Error ? error.message : String(error),
			);
		}
		const raw_record = raw_payload as Record<string, unknown>;
		this.client_build_id = String(
			(raw_record.ClientBuildID as string) ?? "",
		);
		this.deployment_id = String((raw_record.DeploymentID as string) ?? "");
		this.user_on_route_update = options.onRouteUpdate;
		this.user_on_work_update = options.onWorkUpdate;
		this.user_on_build_skew_detected = options.onBuildSkewDetected;
		this.default_error_boundary = options.defaultErrorBoundary;
		this.work_indicator_options = options.workIndicator;
		this.work_indicator.configure(options.workIndicator);
		this.ensure_history_key();
		try {
			window.history.scrollRestoration = "manual";
		} catch {}
		const browser = this.read_browser_position();
		const boot_init = initialize_core4_boot(this.model, {
			browser,
			client_build_id: this.client_build_id,
			use_view_transitions: options.useViewTransitions ?? false,
		});
		if (!boot_init) {
			return R.err("Cannot initialize Vorma boot state");
		}
		this.accept_transition(boot_init);
		this.boot_waiter = make_deferred<Result<void>>();
		this.accept_transition(
			begin_boot(this.model, {
				browser,
				payload: this.decode_route_payload(
					raw_payload,
					this.current_url(),
				),
				restored_scroll: this.read_reload_scroll(),
				token: this.new_token("boot"),
			}),
		);
		const result = await this.boot_waiter.promise;
		if (!result.ok) {
			return result;
		}
		if (options.render) {
			await options.render();
		}
		this.install_browser_listeners(options);
		await this.install_dev_hmr();
		this.pump_revalidation();
		return result;
	}

	async install_dev_hmr(): Promise<void> {
		if (!import.meta.env.DEV) {
			return;
		}
		const { install_core4_dev_hmr } = await import("./dev.ts");
		install_core4_dev_hmr(this);
	}

	accept_hmr_route_state(route: RouteState): void {
		this.accept_transition(accept_hmr_route_update(this.model, route));
	}

	install_browser_listeners(options: ClientOptions): void {
		window.addEventListener("popstate", () => {
			void this.handle_popstate();
		});
		window.addEventListener("beforeunload", () => {
			this.write_reload_scroll();
		});
		if (this.focus_cleanup) {
			this.focus_cleanup();
			this.focus_cleanup = null;
		}
		if (active_focus_cleanup) {
			active_focus_cleanup();
			active_focus_cleanup = null;
		}
		if (!options.revalidateOnWindowFocus) {
			return;
		}
		const focus_revalidation_options =
			typeof options.revalidateOnWindowFocus === "object"
				? options.revalidateOnWindowFocus
				: null;
		const stale_ms = focus_revalidation_options?.staleTimeMS ?? 5000;
		const skip_work_indicator =
			focus_revalidation_options?.skipWorkIndicator === true;
		const on_focus = (): void => {
			if (document.visibilityState !== "visible") {
				return;
			}
			const work = derive_core4_work_state(this.model);
			if (
				work.navigation ||
				work.revalidation ||
				work.apiRequests.length > 0
			) {
				return;
			}
			if (Date.now() - this.last_activity_ms < stale_ms) {
				return;
			}
			this.accept_transition(
				request_revalidation(this.model, {
					reason: "windowFocus",
					skip_work_indicator,
					timer_id: this.new_timer_id("refresh"),
				}),
			);
		};
		window.addEventListener("focus", on_focus);
		window.addEventListener("visibilitychange", on_focus);
		this.focus_cleanup = (): void => {
			window.removeEventListener("focus", on_focus);
			window.removeEventListener("visibilitychange", on_focus);
		};
		active_focus_cleanup = this.focus_cleanup;
	}

	async navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	): Promise<{ didNavigate: boolean }> {
		if (this.model.phase !== "ready" || !this.model.browser) {
			throw new Error("Vorma not booted");
		}
		const id = this.new_public_call_id("navigation");
		const deferred = make_deferred<{ didNavigate: boolean }>();
		const accepted = this.accept_transition(
			begin_navigation(this.model, {
				browser_key: this.new_history_key(),
				href: String(href),
				public_call_ids: [id],
				replace: options?.replace ?? false,
				scroll_to_top: options?.scrollToTop,
				skip_work_indicator: options?.skipWorkIndicator ?? false,
				state: options?.state,
				token: this.new_token("navigation"),
			}),
			{
				before_effects: () => {
					this.navigation_waiters.set(id, deferred);
				},
			},
		);
		if (!accepted) {
			return { didNavigate: false };
		}
		return deferred.promise;
	}

	async revalidate(): Promise<RevalidationResult> {
		if (this.model.phase !== "ready") {
			throw new Error("Vorma not booted");
		}
		const id = this.new_public_call_id("refresh");
		const deferred = make_deferred<RevalidationResult>();
		const accepted = this.accept_transition(
			request_revalidation(this.model, {
				reason: "manual",
				skip_work_indicator: false,
				timer_id: this.new_timer_id("refresh"),
				waiter_id: id,
			}),
			{
				before_effects: () => {
					this.refresh_waiters.set(id, deferred);
				},
			},
		);
		if (!accepted) {
			return REVALIDATION_OK;
		}
		return deferred.promise;
	}

	async submit_inner<T = unknown>(
		url: string | URL,
		request_init?: RequestInit,
		options?: {
			apiRouteKind?: APIRouteKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	): Promise<Core4APIResult<T>> {
		if (!this.model.current || !this.model.browser) {
			throw new Error("Vorma not booted");
		}
		const href = String(url);
		const method = (request_init?.method ?? "GET").toUpperCase().trim();
		const route_kind =
			options?.apiRouteKind ??
			(method === "GET" || method === "HEAD" ? "query" : "mutation");
		const should_revalidate =
			options?.revalidate ?? route_kind === "mutation";
		const api_token = this.new_token("api");
		const api_deferred = make_deferred<Core4APIResult<unknown>>();
		let refresh_promise: Promise<RevalidationResult> =
			Promise.resolve(REVALIDATION_OK);
		let refresh_deferred: Deferred<RevalidationResult> | undefined;
		let refresh_waiter_id: PublicCallID | undefined;
		if (should_revalidate && this.model.phase === "ready") {
			refresh_waiter_id = this.new_public_call_id("api-refresh");
			refresh_deferred = make_deferred<RevalidationResult>();
			refresh_promise = refresh_deferred.promise;
		}
		const accepted = this.accept_transition(
			begin_api_submission(this.model, {
				dedupe_key: options?.dedupeKey ?? null,
				href,
				key: options?.dedupeKey ?? api_token,
				method,
				refresh_waiter_id,
				request_init: this.normalize_api_request_init(
					method,
					request_init,
				),
				route_kind,
				should_revalidate,
				skip_work_indicator: options?.skipWorkIndicator ?? false,
				token: api_token,
			}),
			{
				before_effects: () => {
					if (refresh_waiter_id && refresh_deferred) {
						this.refresh_waiters.set(
							refresh_waiter_id,
							refresh_deferred,
						);
					}
					this.api_waiters.set(api_token, {
						deferred: api_deferred,
						revalidation: refresh_promise,
					});
				},
			},
		);
		if (!accepted) {
			refresh_deferred?.resolve(REVALIDATION_OK);
			return {
				error: "API submission was not accepted.",
				revalidationPromise: refresh_promise,
				success: false,
			};
		}
		return api_deferred.promise as Promise<Core4APIResult<T>>;
	}

	normalize_api_request_init(
		method: string,
		request_init: RequestInit | undefined,
	): RequestInit {
		const headers = new Headers(request_init?.headers);
		if (this.deployment_id) {
			headers.set(VERCEL_X_DEPLOYMENT_ID, this.deployment_id);
		}
		headers.set(X_ACCEPTS_CLIENT_REDIRECT, VORMA_PROTOCOL_ENABLED);
		const body = request_init?.body;
		const is_get = method === "GET" || method === "HEAD";
		const should_json =
			!is_get &&
			body &&
			typeof body === "object" &&
			!(body instanceof ReadableStream) &&
			!(body instanceof FormData) &&
			!(body instanceof URLSearchParams) &&
			!(body instanceof Blob) &&
			!(body instanceof ArrayBuffer) &&
			!ArrayBuffer.isView(body);
		const next_init: RequestInit = {
			...request_init,
			headers,
			method,
		};
		if (is_get) {
			delete next_init.body;
		} else if (should_json) {
			next_init.body = JSON.stringify(body);
			if (!headers.has(CONTENT_TYPE_HEADER)) {
				headers.set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE);
			}
		}
		return next_init;
	}

	getRouteState(): RouteState {
		if (!this.model.current) {
			throw new Error("Vorma not booted");
		}
		return this.model.current.route;
	}

	getWorkState(): WorkState {
		if (!this.model.current) {
			throw new Error("Vorma not booted");
		}
		return derive_core4_work_state(this.model);
	}

	getRootEl(): HTMLElement {
		const existing = document.getElementById(VORMA_ROOT_EL_ID);
		if (existing) {
			return existing;
		}
		const root = document.createElement("div");
		root.id = VORMA_ROOT_EL_ID;
		document.body.insertBefore(root, document.body.firstChild);
		return root;
	}

	defineView<T = unknown>(input: {
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		clientLoader?: (props: any) => Promise<T>;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		pattern: string;
		runClientLoaderOnHMR?: boolean;
	}): ViewDefinition & { __phantom_client_loader_data?: T } {
		registerPattern(this.pattern_registry, input.pattern);
		if (input.clientLoader) {
			this.route_loaders.set(
				input.pattern,
				input.clientLoader as Core4ClientLoader,
			);
		} else {
			this.route_loaders.delete(input.pattern);
		}
		if (import.meta.env.DEV) {
			void import("./dev.ts").then(({ configure_core4_dev_hmr_view }) => {
				configure_core4_dev_hmr_view(
					this,
					input.pattern,
					input.runClientLoaderOnHMR === true,
				);
			});
		}
		return {
			before_route_commit: input.beforeRouteCommit,
			before_route_yield: input.beforeRouteYield,
			client_loader: input.clientLoader,
			component: input.component,
			error_boundary: input.errorBoundary,
			pattern: input.pattern,
		};
	}

	start_prefetch(href: string): void {
		if (!this.model.browser) {
			return;
		}
		this.accept_transition(
			begin_prefetch(this.model, {
				href,
				token: this.new_token("prefetch"),
			}),
		);
	}

	stop_prefetch(href: string): void {
		this.accept_transition(cancel_prefetch(this.model, href));
	}

	async handle_popstate(): Promise<void> {
		if (!this.model.browser) {
			return;
		}
		const previous = this.model.browser;
		const next = this.read_browser_position();
		if (previous.key && previous.key !== next.key) {
			this.save_scroll_for_key(previous.key, this.get_scroll_pos());
		}
		const id = this.new_public_call_id("popstate");
		const deferred = make_deferred<{ didNavigate: boolean }>();
		const accepted = this.accept_transition(
			begin_popstate(this.model, {
				browser: next,
				public_call_ids: [id],
				restored_scroll: this.get_scroll_for_key(next.key),
				token: this.new_token("popstate"),
			}),
			{
				before_effects: () => {
					this.navigation_waiters.set(id, deferred);
				},
			},
		);
		if (!accepted) {
			return;
		}
		const result = await deferred.promise;
		if (
			!result.didNavigate &&
			!this.model.active_route &&
			!this.route_snapshot_matches_browser()
		) {
			this.reload_page();
		}
	}

	route_snapshot_matches_browser(): boolean {
		return (
			!!this.model.current &&
			!!this.model.browser &&
			this.model.current.position.href === this.model.browser.href &&
			this.model.current.position.key === this.model.browser.key
		);
	}

	current_url(): URL {
		return new URL(window.location.href);
	}

	ensure_history_key(): void {
		const state = window.history.state;
		if (state && typeof state === "object" && HISTORY_KEY_FIELD in state) {
			return;
		}
		const base = state && typeof state === "object" ? state : {};
		window.history.replaceState(
			{
				...(base as object),
				[HISTORY_KEY_FIELD]: this.new_history_key(),
			},
			"",
			this.current_url().href,
		);
	}

	read_browser_position(): BrowserPosition {
		const state = window.history.state;
		const key =
			state && typeof state === "object" && HISTORY_KEY_FIELD in state
				? String((state as Record<string, unknown>)[HISTORY_KEY_FIELD])
				: "";
		const user_state =
			state && typeof state === "object"
				? (state as Record<string, unknown>)[HISTORY_USER_STATE_FIELD]
				: undefined;
		return {
			href: this.current_url().href,
			key: key as BrowserKey,
			state: user_state,
		};
	}

	decode_route_payload(raw: unknown, url: URL): RoutePayload {
		const data = raw as Record<string, any>;
		const patterns: string[] = data.MatchedPatterns ?? [];
		const schemas: unknown[] = Array.isArray(data.SearchSchemas)
			? data.SearchSchemas
			: [];
		const loaders: unknown[] = data.LoadersData ?? [];
		const imports: string[] = data.ImportURLs ?? [];
		const error_idx: number | null = data.OutermostServerErrIdx ?? null;
		const error_value = data.OutermostServerErr;
		let title: string | undefined;
		if (data.Title?.dangerousInnerHTML !== undefined) {
			const textarea = document.createElement("textarea");
			textarea.innerHTML = data.Title.dangerousInnerHTML;
			title = textarea.value;
		}
		return {
			css_bundles: data.CSSBundles ?? [],
			deps: data.Deps ?? [],
			meta_head_els: data.MetaHeadEls ?? [],
			params: data.Params ?? {},
			rest_head_els: data.RestHeadEls ?? [],
			routes: patterns.map((pattern, idx) => {
				const schema = schemas[idx];
				this.search_schemas.set(pattern, schema);
				return {
					input: parseSearchParams(schema, url.searchParams),
					loader_data: loaders[idx],
					module_url: imports[idx] ?? "",
					pattern,
					server_error:
						error_idx !== null && error_idx === idx
							? error_value
							: undefined,
				};
			}),
			server_build_id: String(data.ClientBuildID ?? this.client_build_id),
			splat_values: data.SplatValues ?? [],
			title,
		};
	}

	response_facts(response: Response): Core4ResponseFacts {
		return {
			headers: response.headers,
			ok: response.ok,
			raw_response: response,
			redirected: response.redirected,
			status: response.status,
			status_text: response.statusText,
			url: response.url,
		};
	}

	normalize_core4_module_url(url: string): string {
		return new URL(url, window.location.href).pathname;
	}

	get_scroll_pos(): { x: number; y: number } {
		return { x: window.scrollX, y: window.scrollY };
	}

	read_scroll_entries(): Array<[string, { x: number; y: number }]> {
		try {
			const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
			const parsed = raw ? JSON.parse(raw) : [];
			if (!Array.isArray(parsed)) {
				return [];
			}
			return parsed.filter(
				(entry): entry is [string, { x: number; y: number }] => {
					return (
						Array.isArray(entry) &&
						entry.length === 2 &&
						typeof entry[0] === "string" &&
						typeof entry[1]?.x === "number" &&
						typeof entry[1]?.y === "number"
					);
				},
			);
		} catch {
			return [];
		}
	}

	save_scroll_for_key(key: string, position: { x: number; y: number }): void {
		const entries = this.read_scroll_entries().filter((entry) => {
			return entry[0] !== key;
		});
		entries.push([key, position]);
		if (entries.length > CORE4_MAX_SCROLL_ENTRIES) {
			entries.splice(0, entries.length - CORE4_MAX_SCROLL_ENTRIES);
		}
		try {
			sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
		} catch {}
	}

	get_scroll_for_key(key: string): ScrollState | undefined {
		for (const [entry_key, position] of this.read_scroll_entries()) {
			if (entry_key === key) {
				return position;
			}
		}
		return undefined;
	}

	save_current_scroll(): void {
		if (this.model.browser?.key) {
			this.save_scroll_for_key(
				this.model.browser.key,
				this.get_scroll_pos(),
			);
		}
	}

	read_reload_scroll(): ScrollState | undefined {
		try {
			const raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
			if (!raw) {
				return undefined;
			}
			sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
			const parsed = JSON.parse(raw);
			if (
				typeof parsed?.x === "number" &&
				typeof parsed?.y === "number" &&
				typeof parsed?.unix === "number" &&
				typeof parsed?.href === "string" &&
				Date.now() - parsed.unix <= CORE4_RELOAD_SCROLL_MAX_AGE_MS &&
				same_document_href(parsed.href, this.current_url().href)
			) {
				return { x: parsed.x, y: parsed.y };
			}
		} catch {}
		return undefined;
	}

	write_reload_scroll(): void {
		try {
			sessionStorage.setItem(
				SCROLL_STORAGE_RELOAD_KEY,
				JSON.stringify({
					...this.get_scroll_pos(),
					href: this.current_url().href,
					unix: Date.now(),
				}),
			);
		} catch {}
	}

	apply_core4_scroll(scroll: ScrollState | undefined): void {
		if (!scroll) {
			return;
		}
		if ("hash" in scroll) {
			const raw = scroll.hash.startsWith("#")
				? scroll.hash.slice(1)
				: scroll.hash;
			let id = raw;
			try {
				id = decodeURIComponent(raw);
			} catch {}
			document.getElementById(id)?.scrollIntoView();
			return;
		}
		const scroll_to =
			this.test_options?.scroll_to ??
			((x: number, y: number): void => {
				window.scrollTo(x, y);
			});
		scroll_to(scroll.x, scroll.y);
	}
}
function make_deferred<T>(): Deferred<T> {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((inner_resolve) => {
		resolve = inner_resolve;
	});
	return { promise, resolve };
}

function create_core4_work_indicator(): Core4WorkIndicatorController {
	let options: WorkIndicatorOptions | undefined;
	let visible = false;
	let show_timer: number | undefined;
	let hide_timer: number | undefined;
	const tokens = new Set<symbol>();
	let release_vorma: (() => void) | undefined;

	function clear_timer(timer: number | undefined): undefined {
		if (timer !== undefined) {
			window.clearTimeout(timer);
		}
		return undefined;
	}

	function sync(): void {
		const current = options;
		if (!current) {
			show_timer = clear_timer(show_timer);
			hide_timer = clear_timer(hide_timer);
			return;
		}
		if (tokens.size > 0) {
			hide_timer = clear_timer(hide_timer);
			if (visible || show_timer !== undefined) {
				return;
			}
			show_timer = window.setTimeout(() => {
				show_timer = undefined;
				if (!options || tokens.size === 0 || visible) {
					return;
				}
				options.start();
				visible = true;
			}, current.startDelayMS ?? 12);
			return;
		}
		show_timer = clear_timer(show_timer);
		if (!visible || hide_timer !== undefined) {
			return;
		}
		hide_timer = window.setTimeout(() => {
			hide_timer = undefined;
			if (!options || tokens.size > 0 || !visible) {
				return;
			}
			options.stop();
			visible = false;
		}, current.stopDelayMS ?? 12);
	}

	function begin(): () => void {
		const token = Symbol("core4-work");
		let released = false;
		tokens.add(token);
		sync();
		return () => {
			if (released) {
				return;
			}
			released = true;
			tokens.delete(token);
			sync();
		};
	}

	return {
		configure: (next_options): void => {
			if (visible && options && options !== next_options) {
				options.stop();
				visible = false;
			}
			show_timer = clear_timer(show_timer);
			hide_timer = clear_timer(hide_timer);
			options = next_options;
			sync();
		},
		indicator: {
			isActive: (): boolean => {
				return tokens.size > 0;
			},
			track: <T>(promise: PromiseLike<T>): Promise<T> => {
				const release = begin();
				return Promise.resolve(promise).finally(release);
			},
		},
		set_vorma_active: (active): void => {
			if (active) {
				release_vorma ??= begin();
				return;
			}
			release_vorma?.();
			release_vorma = undefined;
		},
	};
}
