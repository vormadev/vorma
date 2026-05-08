import { Deferred, Effect, Fiber, Queue, Ref } from "effect";
import { jsonDeepEquals } from "vorma/kit/json";
import type { ClientCommit, CommitFn, WorkState } from "./client_contract.ts";

export const WORK_NAVIGATION_SOURCE_NAVIGATE = "navigate";
export const WORK_NAVIGATION_SOURCE_POPSTATE = "popstate";
export const WORK_NAVIGATION_SOURCE_REDIRECT = "redirect";
export const WORK_REVALIDATION_STATUS_DEBOUNCING = "debouncing";
export const WORK_REVALIDATION_STATUS_RUNNING = "running";
export const WORK_REVALIDATION_STATUS_RETRYING = "retrying";

const COMMAND_SET_NAVIGATION = "SetNavigation";
const COMMAND_SET_REVALIDATION = "SetRevalidation";
const COMMAND_SET_PREFETCH = "SetPrefetch";
const COMMAND_SET_API_REQUESTS = "SetAPIRequests";
const COMMAND_BEGIN_API_REQUEST = "BeginAPIRequest";
const COMMAND_END_API_REQUEST = "EndAPIRequest";
const COMMAND_EMIT_CURRENT = "EmitCurrent";
const COMMAND_SHUTDOWN = "Shutdown";

export type WorkNavigation = NonNullable<WorkState["navigation"]>;
export type WorkRevalidation = NonNullable<WorkState["revalidation"]>;
export type WorkPrefetch = NonNullable<WorkState["prefetch"]>;
export type WorkAPIRequest = WorkState["apiRequests"][number];
export type WorkIndicatorActivity = {
	navigation: boolean;
	revalidation: boolean;
	apiRequests: boolean;
};
export type WorkNavigationInput = WorkNavigation & {
	skipWorkIndicator?: boolean;
};
export type WorkAPIRequestInput = WorkAPIRequest & {
	skipWorkIndicator?: boolean;
};

export type WorkStateActor = {
	set_navigation: (
		navigation: WorkNavigationInput | null,
	) => Effect.Effect<void>;
	set_revalidation: (
		revalidation: WorkRevalidation | null,
	) => Effect.Effect<void>;
	set_prefetch: (prefetch: WorkPrefetch | null) => Effect.Effect<void>;
	set_api_requests: (
		requests: ReadonlyArray<WorkAPIRequestInput>,
	) => Effect.Effect<void>;
	begin_api_request: (request: WorkAPIRequestInput) => Effect.Effect<void>;
	end_api_request: (key: string) => Effect.Effect<void>;
	emit_current: Effect.Effect<void>;
	snapshot: Effect.Effect<WorkState>;
	indicator_activity: Effect.Effect<WorkIndicatorActivity>;
	shutdown: Effect.Effect<void>;
};

export type WorkStateActorOptions = {
	commit?: CommitFn;
	on_update?: (work: WorkState) => Effect.Effect<void>;
	on_indicator_update?: (
		activity: WorkIndicatorActivity,
	) => Effect.Effect<void>;
};

type Model = {
	readonly state: WorkState;
	readonly last_emitted: WorkState;
	readonly navigation_skip_work_indicator: boolean;
	readonly api_request_skip_work_indicators: ReadonlyMap<string, boolean>;
	readonly last_indicator_activity: WorkIndicatorActivity;
};

type Command =
	| {
			readonly _tag: typeof COMMAND_SET_NAVIGATION;
			readonly navigation: WorkNavigationInput | null;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_SET_REVALIDATION;
			readonly revalidation: WorkRevalidation | null;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_SET_PREFETCH;
			readonly prefetch: WorkPrefetch | null;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_SET_API_REQUESTS;
			readonly requests: ReadonlyArray<WorkAPIRequestInput>;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_BEGIN_API_REQUEST;
			readonly request: WorkAPIRequestInput;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_END_API_REQUEST;
			readonly key: string;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_EMIT_CURRENT;
			readonly ack: Deferred.Deferred<void>;
	  }
	| {
			readonly _tag: typeof COMMAND_SHUTDOWN;
	  };

const EMPTY_WORK_STATE: WorkState = {
	navigation: null,
	revalidation: null,
	prefetch: null,
	apiRequests: [],
};

const EMPTY_WORK_INDICATOR_ACTIVITY: WorkIndicatorActivity = {
	navigation: false,
	revalidation: false,
	apiRequests: false,
};

export function make_work_state_actor(
	options: WorkStateActorOptions = {},
): Effect.Effect<WorkStateActor, never> {
	return Effect.gen(function* () {
		const queue = yield* Queue.unbounded<Command>();
		const model = yield* Ref.make<Model>({
			state: EMPTY_WORK_STATE,
			last_emitted: EMPTY_WORK_STATE,
			navigation_skip_work_indicator: false,
			api_request_skip_work_indicators: new Map(),
			last_indicator_activity: EMPTY_WORK_INDICATOR_ACTIVITY,
		});

		const emit = (work: WorkState): Effect.Effect<void> => {
			const commit = options.commit;
			const on_update = options.on_update;
			return Effect.gen(function* () {
				if (commit) {
					yield* Effect.sync(() => {
						const client_commit: ClientCommit = { work };
						commit(client_commit);
					});
				}
				if (on_update) {
					yield* on_update(work);
				}
			});
		};

		const emit_indicator_activity = (
			activity: WorkIndicatorActivity,
		): Effect.Effect<void> => {
			const on_indicator_update = options.on_indicator_update;
			if (!on_indicator_update) {
				return Effect.void;
			}
			return on_indicator_update(activity);
		};

		const emit_if_changed = (): Effect.Effect<void> => {
			return Effect.gen(function* () {
				const current = yield* Ref.get(model);
				const next_indicator_activity =
					indicator_activity_from_model(current);
				const did_change_work = !jsonDeepEquals(
					current.state,
					current.last_emitted,
				);
				const did_change_indicator_activity = !jsonDeepEquals(
					next_indicator_activity,
					current.last_indicator_activity,
				);
				if (did_change_work) {
					yield* emit(current.state);
				}
				if (
					did_change_indicator_activity ||
					(did_change_work &&
						!indicator_activity_has_work(next_indicator_activity))
				) {
					yield* emit_indicator_activity(next_indicator_activity);
				}
				yield* Ref.set(model, {
					...current,
					last_emitted: did_change_work
						? current.state
						: current.last_emitted,
					last_indicator_activity: did_change_indicator_activity
						? next_indicator_activity
						: current.last_indicator_activity,
				});
			});
		};

		const update_model = (
			f: (current: Model) => Model,
		): Effect.Effect<void> => {
			return Ref.update(model, (current) => {
				return f(current);
			}).pipe(Effect.andThen(emit_if_changed()));
		};

		const replace_api_request = (
			state: WorkState,
			request: WorkAPIRequest,
		): WorkState => {
			const idx = state.apiRequests.findIndex((current) => {
				return current.key === request.key;
			});
			if (idx === -1) {
				return {
					...state,
					apiRequests: [...state.apiRequests, request],
				};
			}
			return {
				...state,
				apiRequests: state.apiRequests.map((current, current_idx) => {
					if (current_idx === idx) {
						return request;
					}
					return current;
				}),
			};
		};

		const remove_api_request = (
			state: WorkState,
			key: string,
		): WorkState => {
			return {
				...state,
				apiRequests: state.apiRequests.filter((current) => {
					return current.key !== key;
				}),
			};
		};

		const complete = (
			ack: Deferred.Deferred<void>,
		): Effect.Effect<void> => {
			return Deferred.succeed(ack, undefined);
		};

		const public_navigation = (
			navigation: WorkNavigationInput,
		): WorkNavigation => {
			return {
				href: navigation.href,
				replace: navigation.replace,
				source: navigation.source,
			};
		};

		const public_api_request = (
			request: WorkAPIRequestInput,
		): WorkAPIRequest => {
			return {
				key: request.key,
				method: request.method,
				href: request.href,
			};
		};

		const set_navigation_now = (
			navigation: WorkNavigationInput | null,
		): Effect.Effect<void> => {
			return update_model((current) => {
				return {
					...current,
					state: {
						...current.state,
						navigation: navigation
							? public_navigation(navigation)
							: null,
					},
					navigation_skip_work_indicator:
						navigation?.skipWorkIndicator === true,
				};
			});
		};

		const set_revalidation_now = (
			revalidation: WorkRevalidation | null,
		): Effect.Effect<void> => {
			return update_model((current) => {
				return {
					...current,
					state: {
						...current.state,
						revalidation,
					},
				};
			});
		};

		const set_prefetch_now = (
			prefetch: WorkPrefetch | null,
		): Effect.Effect<void> => {
			return update_model((current) => {
				return {
					...current,
					state: {
						...current.state,
						prefetch,
					},
				};
			});
		};

		const set_api_requests_now = (
			requests: ReadonlyArray<WorkAPIRequestInput>,
		): Effect.Effect<void> => {
			return update_model((current) => {
				return {
					...current,
					state: {
						...current.state,
						apiRequests: requests.map(public_api_request),
					},
					api_request_skip_work_indicators: new Map(
						requests.map((request) => {
							return [
								request.key,
								request.skipWorkIndicator === true,
							];
						}),
					),
				};
			});
		};

		const begin_api_request_now = (
			request: WorkAPIRequestInput,
		): Effect.Effect<void> => {
			return update_model((current) => {
				const api_request_skip_work_indicators = new Map(
					current.api_request_skip_work_indicators,
				);
				api_request_skip_work_indicators.set(
					request.key,
					request.skipWorkIndicator === true,
				);
				return {
					...current,
					state: replace_api_request(
						current.state,
						public_api_request(request),
					),
					api_request_skip_work_indicators,
				};
			});
		};

		const end_api_request_now = (key: string): Effect.Effect<void> => {
			return update_model((current) => {
				const api_request_skip_work_indicators = new Map(
					current.api_request_skip_work_indicators,
				);
				api_request_skip_work_indicators.delete(key);
				return {
					...current,
					state: remove_api_request(current.state, key),
					api_request_skip_work_indicators,
				};
			});
		};

		const command_program = (command: Command): Effect.Effect<void> => {
			switch (command._tag) {
				case COMMAND_SET_NAVIGATION: {
					return set_navigation_now(command.navigation).pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_SET_REVALIDATION: {
					return set_revalidation_now(command.revalidation).pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_SET_PREFETCH: {
					return set_prefetch_now(command.prefetch).pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_SET_API_REQUESTS: {
					return set_api_requests_now(command.requests).pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_BEGIN_API_REQUEST: {
					return begin_api_request_now(command.request).pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_END_API_REQUEST: {
					return end_api_request_now(command.key).pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_EMIT_CURRENT: {
					return emit_if_changed().pipe(
						Effect.andThen(complete(command.ack)),
					);
				}
				case COMMAND_SHUTDOWN: {
					return Queue.shutdown(queue);
				}
			}
		};

		const actor = Queue.take(queue).pipe(
			Effect.flatMap(command_program),
			Effect.forever,
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);
		const actor_fiber = yield* Effect.forkDaemon(actor);

		return {
			set_navigation: set_navigation_now,
			set_revalidation: set_revalidation_now,
			set_prefetch: set_prefetch_now,
			set_api_requests: set_api_requests_now,
			begin_api_request: begin_api_request_now,
			end_api_request: end_api_request_now,
			emit_current: emit_if_changed(),
			snapshot: Ref.get(model).pipe(
				Effect.map((current) => {
					return current.state;
				}),
			),
			indicator_activity: Ref.get(model).pipe(
				Effect.map((current) => {
					return indicator_activity_from_model(current);
				}),
			),
			shutdown: Queue.offer(queue, { _tag: COMMAND_SHUTDOWN }).pipe(
				Effect.andThen(Fiber.join(actor_fiber)),
				Effect.asVoid,
				Effect.catchAll(() => {
					return Effect.void;
				}),
			),
		};
	});
}

function indicator_activity_from_model(current: Model): WorkIndicatorActivity {
	return {
		navigation:
			current.state.navigation !== null &&
			!current.navigation_skip_work_indicator,
		revalidation: current.state.revalidation !== null,
		apiRequests: current.state.apiRequests.some((request) => {
			return (
				current.api_request_skip_work_indicators.get(request.key) !==
				true
			);
		}),
	};
}

function indicator_activity_has_work(activity: WorkIndicatorActivity): boolean {
	return activity.navigation || activity.revalidation || activity.apiRequests;
}
