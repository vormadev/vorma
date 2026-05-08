import { Deferred, Effect, Fiber } from "effect";
import { describe, expect, it } from "vitest";
import {
	make_revalidation_coordinator,
	REVALIDATION_BACKOFF_BASE_MS,
	REVALIDATION_DEBOUNCE_MS,
	RevalidationAttemptFailed,
	RevalidationBuildSkew,
	type RevalidationAttemptInput,
} from "./effect_runtime/revalidation_coordinator.ts";
import type { ScheduleMS } from "./effect_runtime/timer_runtime.ts";

function run_effect<A>(program: Effect.Effect<A, never, never>): Promise<A> {
	return Effect.runPromise(program);
}

function drain(): Effect.Effect<void> {
	return Effect.gen(function* () {
		for (let i = 0; i < 10; i++) {
			yield* Effect.yieldNow;
		}
	});
}

type TestScheduledTimer = {
	delay_ms: number;
	action: Effect.Effect<void>;
	canceled: boolean;
};

function make_test_scheduler() {
	const timers: TestScheduledTimer[] = [];
	const schedule_ms: ScheduleMS = (delay_ms, action) => {
		return Effect.sync(() => {
			const timer = { delay_ms, action, canceled: false };
			timers.push(timer);
			return Effect.sync(() => {
				timer.canceled = true;
			});
		});
	};
	const pending_delays = (): number[] => {
		return timers
			.filter((timer) => {
				return !timer.canceled;
			})
			.map((timer) => {
				return timer.delay_ms;
			});
	};
	const run_next = Effect.gen(function* () {
		while (timers.length > 0) {
			const timer = timers.shift()!;
			if (!timer.canceled) {
				yield* timer.action;
				return;
			}
		}
	});
	return { schedule_ms, pending_delays, run_next };
}

describe("ccc Effect revalidation experiment", () => {
	it("debounces requests and resolves all coalesced waiters from one attempt", async () => {
		const calls: RevalidationAttemptInput[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const scheduler = make_test_scheduler();
					const coordinator = yield* make_revalidation_coordinator({
						schedule_ms: scheduler.schedule_ms,
						run: (input) => {
							return Effect.sync(() => {
								calls.push(input);
							});
						},
					});

					const first = yield* Effect.forkChild(
						coordinator.request("manual", { debounce: true }),
					);
					const second = yield* Effect.forkChild(
						coordinator.request("windowFocus", {
							debounce: true,
						}),
					);

					yield* Effect.yieldNow;
					yield* Effect.sync(() => {
						expect(scheduler.pending_delays()).toEqual([
							REVALIDATION_DEBOUNCE_MS,
						]);
						expect(calls).toHaveLength(0);
					});

					yield* scheduler.run_next;
					yield* drain();
					const first_result = yield* Fiber.join(first);
					const second_result = yield* Fiber.join(second);

					return { first_result, second_result };
				}),
			),
		);

		expect(result.first_result).toEqual({ ok: true });
		expect(result.second_result).toEqual({ ok: true });
		expect(calls).toHaveLength(1);
		expect(calls[0]).toMatchObject({
			attempt: 1,
			reason: "windowFocus",
		});
	});

	it("retries with scheduled exponential backoff", async () => {
		const calls: RevalidationAttemptInput[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const scheduler = make_test_scheduler();
					const coordinator = yield* make_revalidation_coordinator({
						schedule_ms: scheduler.schedule_ms,
						run: (input) => {
							return Effect.gen(function* () {
								yield* Effect.sync(() => {
									calls.push(input);
								});
								if (calls.length < 3) {
									return yield* Effect.fail(
										new RevalidationAttemptFailed({
											error: "route failed",
										}),
									);
								}
							});
						},
					});

					const fiber = yield* Effect.forkChild(
						coordinator.request("apiRequest"),
					);

					yield* drain();
					yield* Effect.sync(() => {
						expect(calls.map((call) => call.attempt)).toEqual([1]);
					});

					yield* Effect.sync(() => {
						expect(scheduler.pending_delays()).toEqual([
							REVALIDATION_BACKOFF_BASE_MS,
						]);
						expect(calls.map((call) => call.attempt)).toEqual([1]);
					});

					yield* scheduler.run_next;
					yield* drain();
					yield* Effect.sync(() => {
						expect(calls.map((call) => call.attempt)).toEqual([
							1, 2,
						]);
					});

					yield* Effect.sync(() => {
						expect(scheduler.pending_delays()).toEqual([
							REVALIDATION_BACKOFF_BASE_MS * 2,
						]);
					});
					yield* scheduler.run_next;
					yield* drain();
					return yield* Fiber.join(fiber);
				}),
			),
		);

		expect(result).toEqual({ ok: true });
		expect(calls.map((call) => call.attempt)).toEqual([1, 2, 3]);
		expect(calls.map((call) => call.reason)).toEqual([
			"apiRequest",
			"retry",
			"retry",
		]);
	});

	it("settles all waiters immediately on build skew", async () => {
		const calls: RevalidationAttemptInput[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const coordinator = yield* make_revalidation_coordinator({
						run: (input) => {
							return Effect.sync(() => {
								calls.push(input);
							}).pipe(
								Effect.andThen(
									Effect.fail(new RevalidationBuildSkew()),
								),
							);
						},
					});

					return yield* coordinator.request("manual");
				}),
			),
		);

		expect(result).toEqual({ ok: false, reason: "build_skew" });
		expect(calls).toHaveLength(1);
	});

	it("does not let an older active run satisfy newer demand", async () => {
		const calls: RevalidationAttemptInput[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const release_first = yield* Deferred.make<void>();
					const coordinator = yield* make_revalidation_coordinator({
						run: (input) => {
							return Effect.gen(function* () {
								yield* Effect.sync(() => {
									calls.push(input);
								});
								if (calls.length === 1) {
									yield* Deferred.await(release_first);
								}
							});
						},
					});

					const first = yield* Effect.forkChild(
						coordinator.request("manual"),
					);
					yield* drain();
					yield* Effect.sync(() => {
						expect(calls).toHaveLength(1);
					});

					const second = yield* Effect.forkChild(
						coordinator.request("apiRequest"),
					);
					yield* Effect.yieldNow;
					yield* Deferred.succeed(release_first, undefined);
					yield* drain();

					const first_result = yield* Fiber.join(first);
					const second_result = yield* Fiber.join(second);

					return { first_result, second_result };
				}),
			),
		);

		expect(result.first_result).toEqual({ ok: true });
		expect(result.second_result).toEqual({ ok: true });
		expect(calls.map((call) => call.seq)).toEqual([2, 4]);
		expect(calls.map((call) => call.reason)).toEqual([
			"manual",
			"apiRequest",
		]);
	});
});
