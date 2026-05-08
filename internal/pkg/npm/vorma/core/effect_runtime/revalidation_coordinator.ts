import { Data, Deferred, Duration, Effect, Fiber, Queue, Ref } from "effect";
import type { RevalidationResult } from "../types.ts";

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
	readonly fiber: Fiber.RuntimeFiber<void, never>;
};

type SleepRun = {
	readonly attempt: number;
	readonly reason: RevalidationReason;
	readonly skipWorkIndicator: boolean;
	readonly fiber: Fiber.RuntimeFiber<void, never>;
};

type Model = {
	readonly nextSeq: number;
	readonly waiters: ReadonlyArray<Waiter>;
	readonly active: ActiveRun | null;
	readonly sleeper: SleepRun | null;
};

type Command =
	| {
			readonly _tag: "Request";
			readonly reason: Exclude<RevalidationReason, "retry">;
			readonly debounce: boolean;
			readonly skipWorkIndicator: boolean;
			readonly deferred: Deferred.Deferred<RevalidationResult>;
	  }
	| {
			readonly _tag: "RunNow";
			readonly attempt: number;
			readonly reason: RevalidationReason;
	  }
	| {
			readonly _tag: "AttemptSucceeded";
			readonly seq: number;
	  }
	| {
			readonly _tag: "AttemptFailed";
			readonly seq: number;
			readonly attempt: number;
			readonly reason: RevalidationReason;
	  }
	| {
			readonly _tag: "AttemptBuildSkew";
			readonly seq: number;
	  }
	| {
			readonly _tag: "Cancel";
			readonly result: RevalidationResult;
	  }
	| {
			readonly _tag: "Shutdown";
	  };

export function make_revalidation_coordinator(
	options: RevalidationCoordinatorOptions,
): Effect.Effect<RevalidationCoordinator, never> {
	return Effect.gen(function* () {
		const queue = yield* Queue.unbounded<Command>();
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
			return Fiber.interruptFork(current.sleeper.fiber).pipe(
				Effect.asVoid,
			);
		};

		const cancel_active = (current: Model): Effect.Effect<void> => {
			if (!current.active) {
				return Effect.void;
			}
			return Fiber.interruptFork(current.active.fiber).pipe(
				Effect.asVoid,
			);
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
		): Effect.Effect<SleepRun> => {
			return Effect.gen(function* () {
				const fiber = yield* Effect.forkDaemon(
					Effect.sleep(Duration.millis(delayMS)).pipe(
						Effect.andThen(
							Queue.offer(queue, {
								_tag: "RunNow",
								attempt,
								reason,
							}),
						),
						Effect.catchAll(() => {
							return Effect.void;
						}),
						Effect.asVoid,
					),
				);
				return { attempt, reason, skipWorkIndicator, fiber };
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
				const fiber = yield* Effect.forkDaemon(
					options
						.run({ seq, attempt, reason, skipWorkIndicator })
						.pipe(
							Effect.andThen(
								Queue.offer(queue, {
									_tag: "AttemptSucceeded",
									seq,
								}),
							),
							Effect.catchTag("RevalidationBuildSkew", () => {
								return Queue.offer(queue, {
									_tag: "AttemptBuildSkew",
									seq,
								});
							}),
							Effect.catchAll(() => {
								return Queue.offer(queue, {
									_tag: "AttemptFailed",
									seq,
									attempt,
									reason,
								});
							}),
							Effect.asVoid,
						),
				);
				yield* set_model({
					...current,
					nextSeq: seq,
					sleeper: null,
					active: { seq, attempt, reason, skipWorkIndicator, fiber },
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
				);
				yield* set_model({
					...current,
					active: null,
					sleeper,
				});
			});
		};

		const on_request = (
			command: Extract<Command, { _tag: "Request" }>,
		): Effect.Effect<void> => {
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

		const on_cancel = (
			command: Extract<Command, { _tag: "Cancel" }>,
		): Effect.Effect<void> => {
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

		const command_effect = (command: Command): Effect.Effect<void> => {
			switch (command._tag) {
				case "Request": {
					return on_request(command);
				}
				case "RunNow": {
					return start_attempt(command.attempt, command.reason);
				}
				case "AttemptSucceeded": {
					return after_success(command.seq);
				}
				case "AttemptFailed": {
					return after_failure(command.seq, command.attempt);
				}
				case "AttemptBuildSkew": {
					return after_build_skew(command.seq);
				}
				case "Cancel": {
					return on_cancel(command);
				}
				case "Shutdown": {
					return Effect.gen(function* () {
						const current = yield* Ref.get(model);
						yield* cancel_sleep(current);
						yield* cancel_active(current);
						yield* Queue.shutdown(queue);
					});
				}
			}
		};

		const actor = Queue.take(queue).pipe(
			Effect.flatMap(command_effect),
			Effect.forever,
			Effect.ensuring(
				Effect.gen(function* () {
					const current = yield* Ref.get(model);
					yield* cancel_sleep(current);
					yield* cancel_active(current);
					yield* complete_waiters(
						current.waiters,
						REVALIDATION_EXHAUSTED,
					);
				}),
			),
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
		const actor_fiber = yield* Effect.forkDaemon(actor);

		const snapshot = Ref.get(model).pipe(
			Effect.map((current): RevalidationCoordinatorSnapshot => {
				return model_snapshot(current);
			}),
		);

		return {
			request: (reason, request_options) => {
				return Effect.gen(function* () {
					const waiter = yield* Deferred.make<RevalidationResult>();
					yield* on_request({
						_tag: "Request",
						reason,
						debounce: request_options?.debounce === true,
						skipWorkIndicator:
							request_options?.skipWorkIndicator === true,
						deferred: waiter,
					});
					return yield* Deferred.await(waiter);
				});
			},
			cancel: (result) => {
				return on_cancel({
					_tag: "Cancel",
					result: result ?? REVALIDATION_OK,
				});
			},
			snapshot,
			shutdown: Queue.offer(queue, { _tag: "Shutdown" }).pipe(
				Effect.andThen(Fiber.join(actor_fiber)),
				Effect.asVoid,
				Effect.catchAll(() => {
					return Effect.void;
				}),
			),
		};
	});
}
