import { Effect, Ref } from "effect";

export const WINDOW_EVENT_BEFOREUNLOAD = "beforeunload";
export const WINDOW_EVENT_FOCUS = "focus";
export const WINDOW_EVENT_POPSTATE = "popstate";

export type RuntimeLifecycle = {
	add_finalizer: (finalizer: Effect.Effect<void>) => Effect.Effect<void>;
	listen_window: <K extends keyof WindowEventMap>(
		event_name: K,
		listener: (event: WindowEventMap[K]) => void,
	) => Effect.Effect<void>;
	shutdown: Effect.Effect<void>;
};

type LifecycleState = {
	closed: boolean;
	finalizers: ReadonlyArray<Effect.Effect<void>>;
};

export function make_runtime_lifecycle(): Effect.Effect<
	RuntimeLifecycle,
	never
> {
	return Effect.gen(function* () {
		const state_ref = yield* Ref.make<LifecycleState>({
			closed: false,
			finalizers: [],
		});

		const add_finalizer: RuntimeLifecycle["add_finalizer"] = (
			finalizer,
		) => {
			return Effect.gen(function* () {
				const immediate = yield* Ref.modify(state_ref, (state) => {
					if (state.closed) {
						return [finalizer, state];
					}
					return [
						null,
						{
							closed: false,
							finalizers: [finalizer, ...state.finalizers],
						},
					];
				});
				if (immediate) {
					yield* run_finalizer(immediate);
				}
			});
		};

		const listen_window: RuntimeLifecycle["listen_window"] = (
			event_name,
			listener,
		) => {
			return Effect.gen(function* () {
				const event_listener = listener as EventListener;
				yield* Effect.sync(() => {
					window.addEventListener(event_name, event_listener);
				});
				yield* add_finalizer(
					Effect.sync(() => {
						window.removeEventListener(event_name, event_listener);
					}),
				);
			});
		};

		const shutdown = Effect.gen(function* () {
			const finalizers = yield* Ref.modify(state_ref, (state) => {
				if (state.closed) {
					return [
						[],
						{
							closed: true,
							finalizers: [],
						},
					];
				}
				return [
					state.finalizers,
					{
						closed: true,
						finalizers: [],
					},
				];
			});
			yield* Effect.forEach(
				finalizers,
				(finalizer) => {
					return run_finalizer(finalizer);
				},
				{ discard: true },
			);
		});

		return {
			add_finalizer,
			listen_window,
			shutdown,
		};
	});
}

function run_finalizer(finalizer: Effect.Effect<void>): Effect.Effect<void> {
	return finalizer.pipe(
		Effect.catchAllCause(() => {
			return Effect.void;
		}),
	);
}
