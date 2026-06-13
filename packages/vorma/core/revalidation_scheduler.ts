import type { Deferred, RevalidationReason } from "./client_core_types.ts";
import type { RevalidationResult } from "./types.ts";

/*
Owns the `refresh` base fact: the outstanding view-data demand and its retry
timing. The scheduler decides WHEN a revalidation must run and what its
demand facts are; actually fetching is the core's job, requested through
`start_revalidation`. Ordering is by seq, never wall clock: a route fetch
satisfies a demand only if its seq is greater than the demand's `after_seq`.
*/

export const REVALIDATION_DEBOUNCE_MS = 8;
export const MAX_REVALIDATION_RETRIES = 8;
export const REVALIDATION_BACKOFF_BASE_MS = 500;
export const REVALIDATION_BACKOFF_CAP_MS = 30000;

export const revalidation_ok: RevalidationResult = { ok: true };
export const revalidation_build_skew: RevalidationResult = {
	ok: false,
	reason: "build_skew",
};
export const revalidation_exhausted: RevalidationResult = {
	ok: false,
	reason: "max_retries_exhausted",
};

export type RefreshWaiter = Deferred<RevalidationResult>;

export type RefreshDemand = {
	after_seq: number;
	reason: RevalidationReason;
	skip_work_indicator?: boolean;
	waiters: RefreshWaiter[];
};

export type RefreshState =
	| { kind: "idle" }
	| { kind: "pending"; demand: RefreshDemand; attempt: number }
	| {
			kind: "debouncing";
			demand: RefreshDemand;
			timer: ReturnType<typeof setTimeout>;
	  }
	| {
			kind: "retrying";
			demand: RefreshDemand;
			attempt: number;
			timer: ReturnType<typeof setTimeout>;
	  };

// Timer-free view of scheduler state for work projection.
export type RefreshWorkView =
	| { kind: "idle" }
	| { kind: "pending"; attempt: number }
	| { kind: "debouncing" }
	| { kind: "retrying"; attempt: number };

export type RevalidationRun = {
	attempt: number;
	reason: RevalidationReason;
	skip_work_indicator?: boolean;
};

export interface RevalidationSchedulerDeps {
	// Monotonic counter shared with route fetches.
	next_seq(): number;
	// Whether the router has finished booting.
	is_ready(): boolean;
	// Seq of the in-flight nav/revalidation, or null when none is active.
	active_seq(): number | null;
	// Launch one revalidation fetch with these demand facts.
	start_revalidation(run: RevalidationRun): void;
	// Work state may have changed (retry scheduled or demand exhausted).
	on_work_update(): void;
}

export interface RevalidationScheduler {
	// Record a fresh demand, merging waiters from any outstanding demand.
	require(
		reason: RevalidationReason,
		waiter?: RefreshWaiter,
		debounce?: boolean,
		skip_work_indicator?: boolean,
	): void;
	// A route fetch with this seq published; satisfy the demand if it covers it.
	mark_fresh(seq: number): void;
	// Build skew detected during revalidation; resolve waiters accordingly.
	mark_build_skew(): void;
	// Run a pending demand now if nothing blocks it.
	maybe_revalidate(): void;
	// Outstanding demand, if any.
	demand(): RefreshDemand | null;
	// Whether the in-flight fetch will satisfy the outstanding demand.
	active_will_refresh(): boolean;
	// Timer-free state view for work projection.
	work_view(): RefreshWorkView;
}

export function create_revalidation_scheduler(
	deps: RevalidationSchedulerDeps,
): RevalidationScheduler {
	let refresh: RefreshState = { kind: "idle" };

	function demand(): RefreshDemand | null {
		if (refresh.kind === "idle") {
			return null;
		}
		return refresh.demand;
	}

	function clear(): void {
		if (refresh.kind === "debouncing" || refresh.kind === "retrying") {
			clearTimeout(refresh.timer);
		}
		refresh = { kind: "idle" };
	}

	function require(
		reason: RevalidationReason,
		waiter?: RefreshWaiter,
		debounce?: boolean,
		skip_work_indicator?: boolean,
	): void {
		const previous_demand = demand();
		const waiters = previous_demand?.waiters ?? [];
		clear();
		if (waiter) {
			waiters.push(waiter);
		}
		const next_demand: RefreshDemand = {
			after_seq: deps.next_seq(),
			reason,
			skip_work_indicator:
				(previous_demand?.skip_work_indicator ?? true) &&
				skip_work_indicator === true,
			waiters,
		};

		if (debounce) {
			let next_refresh!: Extract<RefreshState, { kind: "debouncing" }>;
			const timer = setTimeout(() => {
				if (refresh !== next_refresh) {
					return;
				}
				refresh = { kind: "pending", demand: next_demand, attempt: 0 };
				maybe_revalidate();
			}, REVALIDATION_DEBOUNCE_MS);
			next_refresh = { kind: "debouncing", demand: next_demand, timer };
			refresh = next_refresh;
		} else {
			refresh = { kind: "pending", demand: next_demand, attempt: 0 };
		}
	}

	function mark_fresh(seq: number): void {
		const current_demand = demand();
		if (!current_demand) {
			return;
		}
		if (seq <= current_demand.after_seq) {
			return;
		}
		const waiters = current_demand.waiters;
		clear();
		for (const w of waiters) {
			w.resolve(revalidation_ok);
		}
	}

	function mark_build_skew(): void {
		const current_demand = demand();
		if (!current_demand) {
			return;
		}
		const waiters = current_demand.waiters;
		clear();
		for (const w of waiters) {
			w.resolve(revalidation_build_skew);
		}
	}

	function active_will_refresh(): boolean {
		const current_demand = demand();
		const active_seq = deps.active_seq();
		return (
			active_seq !== null &&
			current_demand !== null &&
			active_seq > current_demand.after_seq
		);
	}

	function maybe_revalidate(): void {
		if (!deps.is_ready()) {
			return;
		}
		if (refresh.kind === "idle") {
			return;
		}
		if (refresh.kind === "debouncing") {
			return;
		}
		if (active_will_refresh() || deps.active_seq() !== null) {
			return;
		}
		if (refresh.kind === "retrying") {
			return;
		}

		const current_demand = refresh.demand;
		const attempt = refresh.attempt;
		if (attempt >= MAX_REVALIDATION_RETRIES) {
			clear();
			for (const w of current_demand.waiters) {
				w.resolve(revalidation_exhausted);
			}
			deps.on_work_update();
			return;
		}

		const delay =
			attempt === 0
				? 0
				: Math.min(
						REVALIDATION_BACKOFF_BASE_MS * Math.pow(2, attempt - 1),
						REVALIDATION_BACKOFF_CAP_MS,
					);

		if (delay === 0) {
			refresh = { kind: "pending", demand: current_demand, attempt: attempt + 1 };
			deps.start_revalidation({
				attempt: attempt + 1,
				reason: current_demand.reason,
				skip_work_indicator: current_demand.skip_work_indicator,
			});
		} else {
			const retry_demand: RefreshDemand = {
				...current_demand,
				reason: "retry",
			};
			let next_refresh!: Extract<RefreshState, { kind: "retrying" }>;
			const timer = setTimeout(() => {
				if (refresh !== next_refresh) {
					return;
				}
				refresh = {
					kind: "pending",
					demand: retry_demand,
					attempt: attempt + 1,
				};
				if (deps.active_seq() !== null) {
					return;
				}
				deps.start_revalidation({
					attempt: attempt + 1,
					reason: retry_demand.reason,
					skip_work_indicator: retry_demand.skip_work_indicator,
				});
			}, delay);
			next_refresh = {
				kind: "retrying",
				demand: retry_demand,
				attempt: attempt + 1,
				timer,
			};
			refresh = next_refresh;
			deps.on_work_update();
		}
	}

	function work_view(): RefreshWorkView {
		if (refresh.kind === "pending") {
			return { kind: "pending", attempt: refresh.attempt };
		}
		if (refresh.kind === "debouncing") {
			return { kind: "debouncing" };
		}
		if (refresh.kind === "retrying") {
			return { kind: "retrying", attempt: refresh.attempt };
		}
		return { kind: "idle" };
	}

	return {
		require,
		mark_fresh,
		mark_build_skew,
		maybe_revalidate,
		demand,
		active_will_refresh,
		work_view,
	};
}
