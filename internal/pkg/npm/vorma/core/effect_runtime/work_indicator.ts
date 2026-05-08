import { Effect, Ref } from "effect";
import type { WorkIndicator, WorkIndicatorOptions } from "./client_contract.ts";

export const WORK_INDICATOR_DEFAULT_SHOW_DELAY_MS = 12;
export const WORK_INDICATOR_DEFAULT_HIDE_DELAY_MS = 12;

export type WorkIndicatorRuntime = {
	indicator: WorkIndicator;
	configure: (
		options: WorkIndicatorOptions | undefined,
	) => Effect.Effect<void>;
	set_vorma_active: (active: boolean) => Effect.Effect<void>;
	begin: Effect.Effect<Effect.Effect<void>>;
	is_active: Effect.Effect<boolean>;
	shutdown: Effect.Effect<void>;
};

type TimerHandle = ReturnType<typeof window.setTimeout>;

type WorkIndicatorState = {
	active_count: number;
	options: WorkIndicatorOptions | undefined;
	visible: boolean;
	show_timer: TimerHandle | null;
	hide_timer: TimerHandle | null;
};

export function make_work_indicator(): Effect.Effect<
	WorkIndicatorRuntime,
	never
> {
	return Effect.gen(function* () {
		const state_ref = yield* Ref.make<WorkIndicatorState>({
			active_count: 0,
			options: undefined,
			visible: false,
			show_timer: null,
			hide_timer: null,
		});

		const sync = sync_indicator(state_ref);
		let release_vorma_work: Effect.Effect<void> | null = null;
		const begin = Effect.gen(function* () {
			let released = false;
			yield* Ref.update(state_ref, (state) => {
				return {
					...state,
					active_count: state.active_count + 1,
				};
			});
			yield* sync;
			return Effect.gen(function* () {
				if (released) {
					return;
				}
				released = true;
				yield* Ref.update(state_ref, (state) => {
					return {
						...state,
						active_count: Math.max(0, state.active_count - 1),
					};
				});
				yield* sync;
			});
		});

		const set_vorma_active: WorkIndicatorRuntime["set_vorma_active"] = (
			active,
		) => {
			if (active) {
				if (release_vorma_work) {
					return Effect.void;
				}
				return Effect.gen(function* () {
					release_vorma_work = yield* begin;
				});
			}
			if (!release_vorma_work) {
				return sync;
			}
			const release = release_vorma_work;
			release_vorma_work = null;
			return release;
		};

		const configure: WorkIndicatorRuntime["configure"] = (options) => {
			return Effect.gen(function* () {
				const previous = yield* Ref.get(state_ref);
				clear_timer(previous.show_timer);
				clear_timer(previous.hide_timer);
				if (
					previous.visible &&
					previous.options &&
					previous.options !== options
				) {
					previous.options.hide();
				}
				yield* Ref.set(state_ref, {
					...previous,
					options,
					visible: previous.visible && previous.options === options,
					show_timer: null,
					hide_timer: null,
				});
				yield* sync;
			});
		};

		const shutdown = Effect.gen(function* () {
			const state = yield* Ref.get(state_ref);
			clear_timer(state.show_timer);
			clear_timer(state.hide_timer);
			if (state.visible && state.options) {
				state.options.hide();
			}
			release_vorma_work = null;
			yield* Ref.set(state_ref, {
				active_count: 0,
				options: undefined,
				visible: false,
				show_timer: null,
				hide_timer: null,
			});
		});

		return {
			indicator: {
				track: <T>(promise: PromiseLike<T>): Promise<T> => {
					const release = Effect.runSync(begin);
					return Promise.resolve(promise).finally(() => {
						Effect.runSync(release);
					});
				},
				isActive: (): boolean => {
					return Effect.runSync(
						Ref.get(state_ref).pipe(
							Effect.map((state) => {
								return state.active_count > 0;
							}),
						),
					);
				},
			},
			configure,
			set_vorma_active,
			begin,
			is_active: Ref.get(state_ref).pipe(
				Effect.map((state) => {
					return state.active_count > 0;
				}),
			),
			shutdown,
		};
	});
}

function sync_indicator(
	state_ref: Ref.Ref<WorkIndicatorState>,
): Effect.Effect<void> {
	return Effect.gen(function* () {
		const state = yield* Ref.get(state_ref);
		const options = state.options;
		if (!options) {
			clear_timer(state.show_timer);
			clear_timer(state.hide_timer);
			yield* Ref.set(state_ref, {
				...state,
				show_timer: null,
				hide_timer: null,
			});
			return;
		}
		if (state.active_count > 0) {
			clear_timer(state.hide_timer);
			if (state.visible || state.show_timer) {
				yield* Ref.set(state_ref, {
					...state,
					hide_timer: null,
				});
				return;
			}
			const show_timer = window.setTimeout(() => {
				Effect.runSync(show_indicator(state_ref));
			}, options.showDelayMS ?? WORK_INDICATOR_DEFAULT_SHOW_DELAY_MS);
			yield* Ref.set(state_ref, {
				...state,
				hide_timer: null,
				show_timer,
			});
			return;
		}
		clear_timer(state.show_timer);
		if (state.hide_timer) {
			yield* Ref.set(state_ref, {
				...state,
				show_timer: null,
			});
			return;
		}
		const hide_timer = window.setTimeout(() => {
			Effect.runSync(hide_indicator(state_ref));
		}, options.hideDelayMS ?? WORK_INDICATOR_DEFAULT_HIDE_DELAY_MS);
		yield* Ref.set(state_ref, {
			...state,
			hide_timer,
			show_timer: null,
		});
	});
}

function show_indicator(
	state_ref: Ref.Ref<WorkIndicatorState>,
): Effect.Effect<void> {
	return Effect.gen(function* () {
		const state = yield* Ref.get(state_ref);
		if (!state.options || state.active_count === 0 || state.visible) {
			yield* Ref.set(state_ref, {
				...state,
				show_timer: null,
			});
			return;
		}
		state.options.show();
		yield* Ref.set(state_ref, {
			...state,
			visible: true,
			show_timer: null,
		});
	});
}

function hide_indicator(
	state_ref: Ref.Ref<WorkIndicatorState>,
): Effect.Effect<void> {
	return Effect.gen(function* () {
		const state = yield* Ref.get(state_ref);
		if (!state.options || state.active_count > 0) {
			yield* Ref.set(state_ref, {
				...state,
				hide_timer: null,
			});
			return;
		}
		state.options.hide();
		yield* Ref.set(state_ref, {
			...state,
			hide_timer: null,
			visible: false,
		});
	});
}

function clear_timer(timer: TimerHandle | null): void {
	if (timer) {
		window.clearTimeout(timer);
	}
}
