import { Data, Deferred, Duration, Effect, Fiber, Ref } from "effect";
import type { RevalidationResult } from "../types.ts";
import type { ScheduleMS, SleepMS, TimerCancel } from "./timer_runtime.ts";

export type RevalidationReason =
	| "manual"
	| "retry"
	| "apiRequest"
	| "windowFocus";

export const REVALIDATION_DEBOUNCE_MS = 8;
export const MAX_REVALIDATION_RETRIES = 8;
export const REVALIDATION_BACKOFF_BASE_MS = 500;
export const REVALIDATION_BACKOFF_CAP_MS = 30_000;

const REVALIDATION_OK: RevalidationResult = { ok: true };
const REVALIDATION_BUILD_SKEW: RevalidationResult = {
	ok: false,
	reason: "build_skew",
};
const REVALIDATION_EXHAUSTED: RevalidationResult = {
	ok: false,
	reason: "max_retries_exhausted",
};

export class RevalidationAttemptFailed extends Data.TaggedError(
	"RevalidationAttemptFailed",
)<{
	readonly error: unknown;
}> {}

export class RevalidationBuildSkew extends Data.TaggedError(
	"RevalidationBuildSkew",
)<{}> {}

export type RevalidationAttemptInput = {
	seq: number;
	attempt: number;
	reason: RevalidationReason;
	skipWorkIndicator: boolean;
};

export type RevalidationCoordinatorOptions = {
	run: (
		input: RevalidationAttemptInput,
	) => Effect.Effect<void, RevalidationAttemptFailed | RevalidationBuildSkew>;
	schedule_ms?: ScheduleMS;
	sleep_ms?: SleepMS;
	debounceMS?: number;
	maxRetries?: number;
	backoffBaseMS?: number;
	backoffCapMS?: number;
	on_snapshot_change?: (
		snapshot: RevalidationCoordinatorSnapshot,
	) => Effect.Effect<void>;
};

export type RevalidationCoordinator = {
	request: (
		reason: Exclude<RevalidationReason, "retry">,
		options?: { debounce?: boolean; skipWorkIndicator?: boolean },
	) => Effect.Effect<RevalidationResult>;
	request_started: (
		reason: Exclude<RevalidationReason, "retry">,
		options?: { debounce?: boolean; skipWorkIndicator?: boolean },
	) => Effect.Effect<Effect.Effect<RevalidationResult>>;
	cancel: (result?: RevalidationResult) => Effect.Effect<void>;
	snapshot: Effect.Effect<RevalidationCoordinatorSnapshot>;
	shutdown: Effect.Effect<void>;
};

export type RevalidationCoordinatorSnapshot = {
	phase: "idle" | "debouncing" | "running" | "retrying";
	waiterCount: number;
	activeSeq: number | null;
	nextSeq: number;
	attempt: number;
	reason: RevalidationReason | null;
	skipWorkIndicator: boolean;
};

type Waiter = {
	readonly afterSeq: number;
	readonly reason: Exclude<RevalidationReason, "retry">;
	readonly skipWorkIndicator: boolean;
	readonly deferred: Deferred.Deferred<RevalidationResult>;
};

type ActiveRun = {
	readonly seq: number;
	readonly attempt: number;
	readonly reason: RevalidationReason;
	readonly skipWorkIndicator: boolean;
	readonly fiber: Fiber.Fiber<void, never> | null;
};

type SleepRun = {
	readonly attempt: number;
	readonly reason: RevalidationReason;
	readonly skipWorkIndicator: boolean;
	readonly cancel: TimerCancel;
};

type Model = {
	readonly nextSeq: number;
	readonly waiters: ReadonlyArray<Waiter>;
	readonly active: ActiveRun | null;
	readonly sleeper: SleepRun | null;
};

export function make_revalidation_coordinator(
	options: RevalidationCoordinatorOptions,
): Effect.Effect<RevalidationCoordinator, never> {
	return Effect.gen(function* () {
		const model = yield* Ref.make<Model>({
			nextSeq: 0,
			waiters: [],
			active: null,
			sleeper: null,
		});

		const cfg = {
			debounceMS: options.debounceMS ?? REVALIDATION_DEBOUNCE_MS,
			maxRetries: options.maxRetries ?? MAX_REVALIDATION_RETRIES,
			backoffBaseMS:
				options.backoffBaseMS ?? REVALIDATION_BACKOFF_BASE_MS,
			backoffCapMS: options.backoffCapMS ?? REVALIDATION_BACKOFF_CAP_MS,
		};
		const sleep_ms =
			options.sleep_ms ??
			((delay_ms: number) => {
				return Effect.sleep(Duration.millis(delay_ms));
			});
		const schedule_ms =
			options.schedule_ms ??
			((delay_ms: number, action: Effect.Effect<void>) => {
				return Effect.gen(function* () {
					const fiber = yield* Effect.forkChild(
						sleep_ms(delay_ms).pipe(Effect.andThen(action)),
						{ startImmediately: true },
					);
					return Fiber.interrupt(fiber).pipe(Effect.asVoid);
				});
			});

		const backoff_ms = (attempt: number): number => {
			return Math.min(
				cfg.backoffBaseMS * 2 ** Math.max(0, attempt - 2),
				cfg.backoffCapMS,
			);
		};

		const skip_work_indicator_from_waiters = (
			waiters: ReadonlyArray<Waiter>,
		): boolean => {
			return (
				waiters.length > 0 &&
				waiters.every((waiter) => {
					return waiter.skipWorkIndicator;
				})
			);
		};

		const model_snapshot = (
			current: Model,
		): RevalidationCoordinatorSnapshot => {
			return {
				phase: current.active
					? "running"
					: current.sleeper
						? current.sleeper.reason === "retry"
							? "retrying"
							: "debouncing"
						: "idle",
				waiterCount: current.waiters.length,
				activeSeq: current.active?.seq ?? null,
				nextSeq: current.nextSeq,
				attempt:
					current.active?.attempt ?? current.sleeper?.attempt ?? 0,
				reason:
					current.active?.reason ?? current.sleeper?.reason ?? null,
				skipWorkIndicator:
					current.active?.skipWorkIndicator ??
					current.sleeper?.skipWorkIndicator ??
					false,
			};
		};

		const emit_snapshot = (current: Model): Effect.Effect<void> => {
			if (!options.on_snapshot_change) {
				return Effect.void;
			}
			return options.on_snapshot_change(model_snapshot(current));
		};

		const set_model = (next: Model): Effect.Effect<void> => {
			return Ref.set(model, next).pipe(
				Effect.andThen(emit_snapshot(next)),
			);
		};

		const cancel_sleep = (current: Model): Effect.Effect<void> => {
			if (!current.sleeper) {
				return Effect.void;
			}
			return current.sleeper.cancel;
		};

		const cancel_active = (current: Model): Effect.Effect<void> => {
			if (!current.active?.fiber) {
				return Effect.void;
			}
			return Fiber.interrupt(current.active.fiber).pipe(Effect.asVoid);
		};

		const complete_waiters = (
			waiters: ReadonlyArray<Waiter>,
			result: RevalidationResult,
		): Effect.Effect<void> => {
			return Effect.forEach(
				waiters,
				(waiter) => {
					return Deferred.succeed(waiter.deferred, result);
				},
				{ discard: true },
			);
		};

		const schedule_sleep = (
			attempt: number,
			reason: RevalidationReason,
			delayMS: number,
			skipWorkIndicator: boolean,
			wake: Effect.Effect<void>,
		): Effect.Effect<SleepRun> => {
			return Effect.gen(function* () {
				const cancel = yield* schedule_ms(delayMS, wake);
				return { attempt, reason, skipWorkIndicator, cancel };
			});
		};

		const schedule_debounce = (
			reason: RevalidationReason,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				yield* cancel_sleep(current);
				const sleeper = yield* schedule_sleep(
					1,
					reason,
					cfg.debounceMS,
					skip_work_indicator_from_waiters(current.waiters),
					start_attempt(1, reason),
				);
				yield* set_model({ ...current, sleeper });
			});
		};

		const start_attempt = (
			attempt: number,
			reason: RevalidationReason,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.waiters.length === 0 || current.active) {
					return;
				}
				const skipWorkIndicator = skip_work_indicator_from_waiters(
					current.waiters,
				);
				yield* cancel_sleep(current);
				const seq = current.nextSeq + 1;
				const attempt_effect = options
					.run({ seq, attempt, reason, skipWorkIndicator })
					.pipe(
						Effect.andThen(after_success(seq)),
						Effect.catchTag("RevalidationBuildSkew", () => {
							return after_build_skew(seq);
						}),
						Effect.catch(() => {
							return after_failure(seq, attempt);
						}),
						Effect.asVoid,
					);
				yield* set_model({
					...current,
					nextSeq: seq,
					sleeper: null,
					active: {
						seq,
						attempt,
						reason,
						skipWorkIndicator,
						fiber: null,
					},
				});
				const fiber = yield* Effect.forkDetach(attempt_effect, {
					startImmediately: true,
				});
				yield* Ref.update(model, (current_after_start) => {
					if (current_after_start.active?.seq !== seq) {
						return current_after_start;
					}
					return {
						...current_after_start,
						active: {
							...current_after_start.active,
							fiber,
						},
					};
				});
			});
		};

		const after_success = (seq: number): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.seq !== seq) {
					return;
				}
				const satisfied = current.waiters.filter((waiter) => {
					return waiter.afterSeq < seq;
				});
				const remaining = current.waiters.filter((waiter) => {
					return waiter.afterSeq >= seq;
				});
				yield* set_model({
					...current,
					waiters: remaining,
					active: null,
				});
				yield* complete_waiters(satisfied, REVALIDATION_OK);
				if (remaining.length > 0) {
					yield* start_attempt(
						1,
						remaining[remaining.length - 1]?.reason ?? "manual",
					);
				}
			});
		};

		const after_build_skew = (seq: number): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.seq !== seq) {
					return;
				}
				yield* set_model({
					...current,
					waiters: [],
					active: null,
				});
				yield* complete_waiters(
					current.waiters,
					REVALIDATION_BUILD_SKEW,
				);
			});
		};

		const after_failure = (
			seq: number,
			attempt: number,
		): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				if (current.active?.seq !== seq) {
					return;
				}
				if (attempt >= cfg.maxRetries) {
					yield* set_model({
						...current,
						waiters: [],
						active: null,
					});
					yield* complete_waiters(
						current.waiters,
						REVALIDATION_EXHAUSTED,
					);
					return;
				}
				const sleeper = yield* schedule_sleep(
					attempt + 1,
					"retry",
					backoff_ms(attempt + 1),
					skip_work_indicator_from_waiters(current.waiters),
					start_attempt(attempt + 1, "retry"),
				);
				yield* set_model({
					...current,
					active: null,
					sleeper,
				});
			});
		};

		const on_request = (command: {
			readonly reason: Exclude<RevalidationReason, "retry">;
			readonly debounce: boolean;
			readonly skipWorkIndicator: boolean;
			readonly deferred: Deferred.Deferred<RevalidationResult>;
		}): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const afterSeq = current.nextSeq + 1;
				const next = {
					...current,
					nextSeq: afterSeq,
					waiters: [
						...current.waiters,
						{
							afterSeq,
							reason: command.reason,
							skipWorkIndicator: command.skipWorkIndicator,
							deferred: command.deferred,
						},
					],
				};
				yield* Ref.set(model, next);
				if (next.active) {
					return;
				}
				if (command.debounce) {
					yield* schedule_debounce(command.reason);
					return;
				}
				yield* start_attempt(1, command.reason);
			});
		};

		const on_cancel = (command: {
			readonly result: RevalidationResult;
		}): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				yield* cancel_sleep(current);
				yield* cancel_active(current);
				yield* set_model({
					...current,
					waiters: [],
					active: null,
					sleeper: null,
				});
				yield* complete_waiters(current.waiters, command.result);
			});
		};

		const snapshot = Ref.get(model).pipe(
			Effect.map((current): RevalidationCoordinatorSnapshot => {
				return model_snapshot(current);
			}),
		);

		const request_started: RevalidationCoordinator["request_started"] = (
			reason,
			request_options,
		) => {
			return Effect.gen(function* () {
				const waiter = yield* Deferred.make<RevalidationResult>();
				yield* on_request({
					reason,
					debounce: request_options?.debounce === true,
					skipWorkIndicator:
						request_options?.skipWorkIndicator === true,
					deferred: waiter,
				});
				return Deferred.await(waiter);
			});
		};

		return {
			request: (reason, request_options) => {
				return Effect.gen(function* () {
					const await_result = yield* request_started(
						reason,
						request_options,
					);
					return yield* await_result;
				});
			},
			request_started,
			cancel: (result) => {
				return on_cancel({
					result: result ?? REVALIDATION_OK,
				});
			},
			snapshot,
			shutdown: Effect.gen(function* () {
				const current = yield* Ref.get(model);
				yield* cancel_sleep(current);
				yield* cancel_active(current);
				yield* complete_waiters(
					current.waiters,
					REVALIDATION_EXHAUSTED,
				);
			}).pipe(
				Effect.catch(() => {
					return Effect.void;
				}),
			),
		};
	});
}
