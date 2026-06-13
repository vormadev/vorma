import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Deferred } from "./client_core_types.ts";
import {
	MAX_REVALIDATION_RETRIES,
	REVALIDATION_BACKOFF_BASE_MS,
	REVALIDATION_BACKOFF_CAP_MS,
	REVALIDATION_DEBOUNCE_MS,
	create_revalidation_scheduler,
	revalidation_build_skew,
	revalidation_exhausted,
	revalidation_ok,
	type RevalidationRun,
	type RevalidationScheduler,
} from "./revalidation_scheduler.ts";
import type { RevalidationResult } from "./types.ts";

function make_deferred<T>(): Deferred<T> {
	let resolve!: (v: T) => void;
	const promise = new Promise<T>((r) => {
		resolve = r;
	});
	return { promise, resolve };
}

type Harness = {
	scheduler: RevalidationScheduler;
	runs: RevalidationRun[];
	work_updates: () => number;
	set_ready: (ready: boolean) => void;
	set_active_seq: (seq: number | null) => void;
};

function make_harness(): Harness {
	let seq = 0;
	let ready = true;
	let active_seq: number | null = null;
	const runs: RevalidationRun[] = [];
	let work_updates = 0;
	const scheduler = create_revalidation_scheduler({
		next_seq: () => {
			seq++;
			return seq;
		},
		is_ready: () => ready,
		active_seq: () => active_seq,
		start_revalidation: (run) => {
			runs.push(run);
		},
		on_work_update: () => {
			work_updates++;
		},
	});
	return {
		scheduler,
		runs,
		work_updates: () => work_updates,
		set_ready: (r) => {
			ready = r;
		},
		set_active_seq: (s) => {
			active_seq = s;
		},
	};
}

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
});

describe("require", () => {
	it("records a pending demand and runs it on maybe_revalidate", () => {
		const h = make_harness();
		h.scheduler.require("manual");

		expect(h.scheduler.demand()?.reason).toBe("manual");
		expect(h.runs).toHaveLength(0);

		h.scheduler.maybe_revalidate();
		expect(h.runs).toEqual([
			{ attempt: 1, reason: "manual", skip_work_indicator: false },
		]);
	});

	it("debounces before becoming runnable", () => {
		const h = make_harness();
		h.scheduler.require("manual", undefined, true);

		h.scheduler.maybe_revalidate();
		expect(h.runs).toHaveLength(0);
		expect(h.scheduler.work_view()).toEqual({ kind: "debouncing" });

		vi.advanceTimersByTime(REVALIDATION_DEBOUNCE_MS);
		expect(h.runs).toHaveLength(1);
	});

	it("a newer demand collapses the debounce window and carries waiters over", async () => {
		const h = make_harness();
		const first = make_deferred<RevalidationResult>();
		h.scheduler.require("manual", first, true);
		const second = make_deferred<RevalidationResult>();
		h.scheduler.require("apiRequest", second, true);

		vi.advanceTimersByTime(REVALIDATION_DEBOUNCE_MS);
		expect(h.runs).toHaveLength(1);
		expect(h.runs[0]?.reason).toBe("apiRequest");

		h.scheduler.mark_fresh(100);
		await expect(first.promise).resolves.toEqual(revalidation_ok);
		await expect(second.promise).resolves.toEqual(revalidation_ok);
	});

	it("skip_work_indicator only survives when every merged demand skips", () => {
		const h = make_harness();
		h.scheduler.require("apiRequest", undefined, undefined, true);
		expect(h.scheduler.demand()?.skip_work_indicator).toBe(true);

		h.scheduler.require("apiRequest", undefined, undefined, undefined);
		expect(h.scheduler.demand()?.skip_work_indicator).toBe(false);

		h.scheduler.require("apiRequest", undefined, undefined, true);
		expect(h.scheduler.demand()?.skip_work_indicator).toBe(false);
	});
});

describe("mark_fresh", () => {
	it("ignores fetches that started before the demand", () => {
		const h = make_harness();
		h.scheduler.require("manual");
		const demand_seq = h.scheduler.demand()!.after_seq;

		h.scheduler.mark_fresh(demand_seq);
		expect(h.scheduler.demand()).not.toBeNull();

		h.scheduler.mark_fresh(demand_seq + 1);
		expect(h.scheduler.demand()).toBeNull();
	});

	it("resolves waiters ok", async () => {
		const h = make_harness();
		const waiter = make_deferred<RevalidationResult>();
		h.scheduler.require("manual", waiter);
		h.scheduler.mark_fresh(100);
		await expect(waiter.promise).resolves.toEqual(revalidation_ok);
	});
});

describe("mark_build_skew", () => {
	it("resolves waiters with build_skew and clears the demand", async () => {
		const h = make_harness();
		const waiter = make_deferred<RevalidationResult>();
		h.scheduler.require("manual", waiter);
		h.scheduler.mark_build_skew();
		await expect(waiter.promise).resolves.toEqual(revalidation_build_skew);
		expect(h.scheduler.demand()).toBeNull();
	});
});

describe("maybe_revalidate gating", () => {
	it("does nothing before boot completes", () => {
		const h = make_harness();
		h.set_ready(false);
		h.scheduler.require("manual");
		h.scheduler.maybe_revalidate();
		expect(h.runs).toHaveLength(0);
	});

	it("does nothing while any fetch is active", () => {
		const h = make_harness();
		h.scheduler.require("manual");
		h.set_active_seq(1);
		h.scheduler.maybe_revalidate();
		expect(h.runs).toHaveLength(0);
	});

	it("active_will_refresh is true only for fetches newer than the demand", () => {
		const h = make_harness();
		h.scheduler.require("manual");
		const demand_seq = h.scheduler.demand()!.after_seq;

		h.set_active_seq(demand_seq);
		expect(h.scheduler.active_will_refresh()).toBe(false);

		h.set_active_seq(demand_seq + 1);
		expect(h.scheduler.active_will_refresh()).toBe(true);
	});
});

describe("retry backoff", () => {
	function fail_current_run(h: Harness): void {
		// The core reports failure by leaving the demand outstanding and
		// calling maybe_revalidate again once the active fetch settles.
		h.scheduler.maybe_revalidate();
	}

	it("retries with exponential backoff up to the cap, then exhausts", async () => {
		const h = make_harness();
		const waiter = make_deferred<RevalidationResult>();
		h.scheduler.require("manual", waiter);

		h.scheduler.maybe_revalidate();
		expect(h.runs).toHaveLength(1);

		const expected_delays: number[] = [];
		for (let attempt = 1; attempt < MAX_REVALIDATION_RETRIES; attempt++) {
			expected_delays.push(
				Math.min(
					REVALIDATION_BACKOFF_BASE_MS * Math.pow(2, attempt - 1),
					REVALIDATION_BACKOFF_CAP_MS,
				),
			);
		}

		for (const [i, delay] of expected_delays.entries()) {
			fail_current_run(h);
			expect(h.scheduler.work_view()).toEqual({
				kind: "retrying",
				attempt: i + 2,
			});
			vi.advanceTimersByTime(delay);
			expect(h.runs).toHaveLength(i + 2);
			expect(h.runs[i + 1]?.reason).toBe("retry");
		}

		fail_current_run(h);
		await expect(waiter.promise).resolves.toEqual(revalidation_exhausted);
		expect(h.scheduler.demand()).toBeNull();
	});

	it("a retry timer that fires while a fetch is active does not start a run", () => {
		const h = make_harness();
		h.scheduler.require("manual");
		h.scheduler.maybe_revalidate();
		expect(h.runs).toHaveLength(1);

		h.scheduler.maybe_revalidate();
		expect(h.scheduler.work_view().kind).toBe("retrying");

		h.set_active_seq(50);
		vi.advanceTimersByTime(REVALIDATION_BACKOFF_BASE_MS);
		expect(h.runs).toHaveLength(1);

		// Once the active fetch settles, the skipped attempt re-enters
		// backoff rather than firing immediately.
		h.set_active_seq(null);
		h.scheduler.maybe_revalidate();
		expect(h.scheduler.work_view()).toEqual({ kind: "retrying", attempt: 3 });
		vi.advanceTimersByTime(REVALIDATION_BACKOFF_BASE_MS * 2);
		expect(h.runs).toHaveLength(2);
		expect(h.runs[1]?.reason).toBe("retry");
	});

	it("notifies work state when a retry is scheduled and when exhausted", () => {
		const h = make_harness();
		h.scheduler.require("manual");
		h.scheduler.maybe_revalidate();
		expect(h.work_updates()).toBe(0);

		h.scheduler.maybe_revalidate();
		expect(h.work_updates()).toBe(1);
	});
});
