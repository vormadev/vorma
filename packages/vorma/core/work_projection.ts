import type { NavigationSource, WorkProjection } from "./client_core_types.ts";
import type { RefreshWorkView } from "./revalidation_scheduler.ts";
import type { WorkState } from "./work_state.ts";

/*
Pure projections from explicit work sources to WorkState/WorkProjection.
The core assembles one WorkSources value per derivation; everything the
projections may consider is named here instead of being read ad hoc from
router internals.
*/

export type WorkSources = {
	// In-flight navigation, if the active fetch is a nav.
	nav: {
		href: string;
		replace: boolean;
		source: NavigationSource;
		skip_work_indicator?: boolean;
	} | null;
	// In-flight revalidation, if the active fetch is a revalidation.
	active_revalidation: {
		attempt: number;
		skip_work_indicator?: boolean;
	} | null;
	// Timer-free scheduler state.
	refresh: RefreshWorkView;
	// skip_work_indicator carried by the outstanding demand, if any.
	refresh_demand_skip_work_indicator?: boolean;
	// Whether a pending demand counts as visible work: nothing in flight
	// will satisfy it and nothing else is active.
	refresh_pending_visible: boolean;
	// Concurrent API submissions in insertion order.
	submissions: Array<{
		key: string;
		method: string;
		href: string;
		skip_work_indicator?: boolean;
	}>;
	// Pending prefetch, if one is still preparing.
	prefetch: { href: string } | null;
};

export function derive_work_projection(sources: WorkSources): WorkProjection[] {
	const work: WorkProjection[] = [];

	if (sources.nav) {
		work.push({
			kind: "navigation",
			skip_work_indicator: sources.nav.skip_work_indicator,
		});
	}

	const pending_revalidation =
		sources.refresh.kind !== "idle" &&
		(sources.refresh.kind === "debouncing" ||
			sources.refresh.kind === "retrying" ||
			sources.refresh_pending_visible);
	if (sources.active_revalidation || pending_revalidation) {
		work.push({
			kind: "revalidation",
			skip_work_indicator: sources.active_revalidation
				? sources.active_revalidation.skip_work_indicator
				: sources.refresh_demand_skip_work_indicator,
		});
	}

	for (const s of sources.submissions) {
		work.push({
			kind: "apiRequest",
			skip_work_indicator: s.skip_work_indicator,
		});
	}

	if (sources.prefetch) {
		work.push({
			kind: "prefetch",
		});
	}

	return work;
}

export function derive_work_state(sources: WorkSources): WorkState {
	let revalidation: WorkState["revalidation"] = null;
	if (sources.active_revalidation) {
		revalidation = {
			status: "running",
			attempt: sources.active_revalidation.attempt,
		};
	} else if (sources.refresh.kind === "debouncing") {
		revalidation = { status: "debouncing", attempt: 0 };
	} else if (sources.refresh.kind === "retrying") {
		revalidation = {
			status: "retrying",
			attempt: sources.refresh.attempt,
		};
	}

	return {
		navigation: sources.nav
			? {
					href: sources.nav.href,
					replace: sources.nav.replace,
					source: sources.nav.source,
				}
			: null,
		revalidation,
		prefetch: sources.prefetch ? { href: sources.prefetch.href } : null,
		apiRequests: sources.submissions.map((s) => {
			return {
				key: s.key,
				method: s.method,
				href: s.href,
			};
		}),
	};
}
