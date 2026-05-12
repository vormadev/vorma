import type { Effect, WorkState } from "./events.ts";
import type {
	ActiveRoute,
	BrowserPosition,
	Config,
	CurrentRoute,
	Model,
	NavigationIntent,
	PreparedRoute,
	RefreshDemand,
	RefreshSlot,
	RouteRenderPlanState,
	RouteState,
	ScrollIntent,
	ScrollState,
	Submission,
	WorkIndicatorState,
} from "./model.ts";

export const MAX_REDIRECTS = 10;
export const REVALIDATION_DEBOUNCE_MS = 8;
export const MAX_REVALIDATION_RETRIES = 8;
export const REVALIDATION_BACKOFF_BASE_MS = 500;
export const REVALIDATION_BACKOFF_CAP_MS = 30_000;

export const REVALIDATION_OK = { ok: true } as const;
export const REVALIDATION_BUILD_SKEW = {
	ok: false,
	reason: "build_skew",
} as const;
export const REVALIDATION_EXHAUSTED = {
	ok: false,
	reason: "max_retries_exhausted",
} as const;

/////////////////////////////////////////////////////////////////////
/////// URL
/////////////////////////////////////////////////////////////////////

export function normalize_url(href: string, base: string): string | null {
	try {
		return new URL(href, base).href;
	} catch {
		return null;
	}
}

export function is_http_url(href: string): boolean {
	try {
		const protocol = new URL(href).protocol;
		return protocol === "http:" || protocol === "https:";
	} catch {
		return false;
	}
}

export function is_same_origin(href: string, base: string): boolean {
	try {
		return new URL(href).origin === new URL(base).origin;
	} catch {
		return false;
	}
}

export function is_same_document(a: string, b: string): boolean {
	try {
		const u_a = new URL(a);
		const u_b = new URL(b);
		u_a.hash = "";
		u_b.hash = "";
		return u_a.href === u_b.href;
	} catch {
		return a === b;
	}
}

export function url_hash_normalized(href: string): string {
	try {
		const raw = new URL(href).hash;
		const stripped = raw.startsWith("#") ? raw.slice(1) : raw;
		if (stripped.length === 0) {
			return "";
		}
		try {
			return decodeURIComponent(stripped);
		} catch {
			return stripped;
		}
	} catch {
		return "";
	}
}

export function url_has_hash(href: string): boolean {
	return url_hash_normalized(href).length > 0;
}

/////////////////////////////////////////////////////////////////////
/////// Refresh Demand
/////////////////////////////////////////////////////////////////////

export function merge_refresh_demand(
	previous: RefreshDemand | null,
	next: RefreshDemand,
): RefreshDemand {
	if (!previous) {
		return next;
	}
	return {
		after_sequence: next.after_sequence,
		reason: next.reason,
		skip_work_indicator:
			previous.skip_work_indicator && next.skip_work_indicator,
		waiters: previous.waiters.concat(next.waiters),
	};
}

export function refresh_demand_of(slot: RefreshSlot): RefreshDemand | null {
	if (slot.kind === "idle") {
		return null;
	}
	return slot.demand;
}

export function refresh_backoff_ms(attempt: number): number {
	if (attempt <= 0) {
		return 0;
	}
	return Math.min(
		REVALIDATION_BACKOFF_CAP_MS,
		REVALIDATION_BACKOFF_BASE_MS * 2 ** (attempt - 1),
	);
}

/////////////////////////////////////////////////////////////////////
/////// Scroll
/////////////////////////////////////////////////////////////////////

export function scroll_intent_for(
	prepared: PreparedRoute,
	scroll: ScrollState,
): ScrollIntent {
	const idx = prepared.matches.length - 1;
	const pattern = prepared.matches[idx]?.pattern ?? "";
	return { scroll, target_route_id: `${idx}:${pattern}` };
}

export function scroll_for_navigation(
	nav: NavigationIntent,
	prepared_url: string,
): ScrollState | null {
	if (nav.is_initial) {
		if (nav.popstate_restored_scroll) {
			return nav.popstate_restored_scroll;
		}
		if (url_has_hash(prepared_url)) {
			return { hash: new URL(prepared_url).hash };
		}
		return null;
	}
	if (nav.is_popstate) {
		if (url_has_hash(prepared_url)) {
			return { hash: new URL(prepared_url).hash };
		}
		return nav.popstate_restored_scroll ?? { x: 0, y: 0 };
	}
	if (url_has_hash(prepared_url)) {
		return { hash: new URL(prepared_url).hash };
	}
	if (nav.scroll_to_top === false) {
		return null;
	}
	return { x: 0, y: 0 };
}

/////////////////////////////////////////////////////////////////////
/////// Route State
/////////////////////////////////////////////////////////////////////

export function route_state_from_prepared(
	prepared: PreparedRoute,
	href: string,
	history_state: unknown,
): RouteState {
	return {
		href,
		historyState: history_state,
		clientBuildID: prepared.client_build_id,
		params: prepared.params,
		splatValues: [...prepared.splat_values],
		matches: prepared.matches.map((m) => {
			return {
				pattern: m.pattern,
				input: m.input,
				loaderData: m.loader_data,
				clientLoaderData: m.client_loader_data,
			};
		}),
		error: prepared.error,
	};
}

export function render_plan_from_prepared(
	prepared: PreparedRoute,
	history_state: unknown,
): RouteRenderPlanState {
	return {
		entries: prepared.matches.map((m) => {
			return {
				pattern: m.pattern,
				input: m.input,
				module_url: m.module_url,
				loader_data: m.loader_data,
				client_loader_data: m.client_loader_data,
			};
		}),
		error: prepared.error,
		params: prepared.params,
		splat_values: [...prepared.splat_values],
		client_build_id: prepared.client_build_id,
		history_state,
	};
}

/////////////////////////////////////////////////////////////////////
/////// Work State
/////////////////////////////////////////////////////////////////////

export function derive_work_state(model: Model): WorkState {
	if (model.phase !== "ready" && model.phase !== "booting") {
		return {
			navigation: null,
			revalidation: null,
			prefetch: null,
			apiRequests: [],
		};
	}

	const active_route = model.active_route;
	const prefetch = model.phase === "ready" ? model.prefetch : null;
	const refresh: RefreshSlot =
		model.phase === "ready" ? model.refresh : { kind: "idle" };

	let navigation: WorkState["navigation"] = null;
	let revalidation: WorkState["revalidation"] = null;

	if (
		active_route &&
		active_route.intent.kind === "navigation" &&
		active_route.phase !== "publishing" &&
		!active_route.intent.nav.is_initial
	) {
		const nav = active_route.intent.nav;
		navigation = {
			href: active_route.url,
			replace: nav.replace,
			source: nav.source,
		};
	}

	if (active_route?.intent.kind === "revalidation") {
		revalidation = {
			status: "running",
			attempt: active_route.intent.reval.attempt,
		};
	} else if (refresh.kind === "debouncing") {
		revalidation = { status: "debouncing", attempt: 0 };
	} else if (refresh.kind === "retrying") {
		revalidation = { status: "retrying", attempt: refresh.attempt };
	} else if (refresh.kind === "pending" && !active_route) {
		revalidation = { status: "running", attempt: refresh.attempt };
	} else if (refresh.kind === "running" && !active_route) {
		revalidation = { status: "running", attempt: refresh.attempt };
	}

	return {
		navigation,
		revalidation,
		prefetch:
			prefetch && prefetch.phase !== "prepared"
				? { href: prefetch.url }
				: null,
		apiRequests: Object.values(model.submissions)
			.filter((s): s is Submission => !!s)
			.map((s) => {
				return {
					key: s.dedupe_key ?? s.token,
					method: s.method,
					href: s.href,
				};
			}),
	};
}

export function derive_work_indicator(
	model: Model,
	work: WorkState,
): WorkIndicatorState {
	if (model.phase !== "ready" && model.phase !== "booting") {
		return { should_be_active: false };
	}
	const opts: Config["work_indicator"] = model.config.work_indicator;
	if (!opts) {
		return { should_be_active: false };
	}

	const active_route = model.active_route;

	if (
		work.navigation &&
		opts.skipNavigations !== true &&
		active_route?.intent.kind === "navigation" &&
		!active_route.intent.nav.is_initial &&
		!active_route.intent.nav.skip_work_indicator
	) {
		return { should_be_active: true };
	}

	if (work.revalidation && opts.skipRevalidations !== true) {
		const demand_skip =
			model.phase === "ready"
				? refresh_demand_of(model.refresh)?.skip_work_indicator
				: undefined;
		const reval_skip =
			active_route?.intent.kind === "revalidation"
				? active_route.intent.reval.skip_work_indicator
				: demand_skip;
		if (!reval_skip) {
			return { should_be_active: true };
		}
	}

	if (opts.skipAPIRequests !== true) {
		for (const s of Object.values(model.submissions)) {
			if (s && !s.skip_work_indicator) {
				return { should_be_active: true };
			}
		}
	}

	return { should_be_active: false };
}

/////////////////////////////////////////////////////////////////////
/////// Finalize
/////////////////////////////////////////////////////////////////////

export function finalize(
	model: Model,
	effects: readonly Effect[],
	previous_work_indicator: WorkIndicatorState,
): { model: Model; effects: readonly Effect[] } {
	if (model.phase !== "ready" && model.phase !== "booting") {
		return { model, effects };
	}
	const work = derive_work_state(model);
	const indicator = derive_work_indicator(model, work);
	const final_effects: Effect[] = [...effects];
	if (
		indicator.should_be_active !== previous_work_indicator.should_be_active
	) {
		final_effects.push({
			type: "sync_work_indicator",
			should_be_active: indicator.should_be_active,
		});
	}
	final_effects.push({ type: "notify_work_update", work });
	const next_model: Model =
		model.phase === "ready"
			? { ...model, work_indicator: indicator }
			: { ...model, work_indicator: indicator };
	return { model: next_model, effects: final_effects };
}

/////////////////////////////////////////////////////////////////////
/////// Browser Position
/////////////////////////////////////////////////////////////////////

export function browser_position_matches(
	browser: BrowserPosition,
	current: CurrentRoute | null,
): boolean {
	if (!current) {
		return false;
	}
	return (
		current.position.href === browser.href &&
		current.position.key === browser.key
	);
}

/////////////////////////////////////////////////////////////////////
/////// Settlement
/////////////////////////////////////////////////////////////////////

export function settle_navigation_calls_unsuccessful(
	route: ActiveRoute,
): Effect[] {
	if (route.intent.kind !== "navigation") {
		return [];
	}
	return route.intent.public_calls.map((call_id) => {
		return {
			type: "settle_navigation_call",
			call_id,
			result: { didNavigate: false },
		};
	});
}

export function settle_refresh_waiters(
	demand: RefreshDemand,
	result:
		| { ok: true }
		| { ok: false; reason: "build_skew" | "max_retries_exhausted" },
): Effect[] {
	return demand.waiters.map((w) => {
		return {
			type: "settle_revalidate_call",
			call_id: w.call_id,
			result,
		};
	});
}
