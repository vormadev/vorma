import { Data, Effect, Ref } from "effect";
import {
	getIsModifiedNavigationClick,
	getIsPrimaryNavigationClick,
} from "vorma/kit/url";
import type { LinkNavFns } from "../make_link_props.ts";
import { make_runtime_lifecycle } from "./runtime_lifecycle.ts";
import {
	type BrowserTimerRuntime,
	make_browser_timer_runtime,
	type TimerCancel,
} from "./timer_runtime.ts";

const LINK_CLICK_FAILED_MESSAGE = "Vorma: Link click failed";
const LINK_POINTER_DOWN_FAILED_MESSAGE =
	"Vorma: Link pointerdown navigation failed";
const NATIVE_CLICK_EVENT = "click";
const POINTER_TYPE_MOUSE = "mouse";
const POINTER_TYPE_PEN = "pen";
const POINTER_TYPE_TOUCH = "touch";

export type LinkNavigationPhase = "click" | "pointerdown";

export class LinkNavigationFailed extends Data.TaggedError(
	"LinkNavigationFailed",
)<{
	readonly error: unknown;
	readonly href: string;
	readonly phase: LinkNavigationPhase;
}> {}

export type LinkNavigationEvent = {
	readonly altKey?: boolean;
	readonly button?: number;
	readonly ctrlKey?: boolean;
	readonly currentTarget?: {
		addEventListener?: (
			event_name: string,
			listener: (event: Event) => void,
			options?: AddEventListenerOptions,
		) => void;
	};
	readonly defaultPrevented?: boolean;
	readonly metaKey?: boolean;
	readonly pointerType?: string;
	readonly preventDefault?: () => void;
	readonly shiftKey?: boolean;
};

export type LinkNavigationOptions = {
	readonly href: string;
	readonly replace?: boolean;
	readonly scroll_to_top?: boolean;
	readonly skip_work_indicator?: boolean;
	readonly state?: unknown;
};

export type LinkClickInput = LinkNavigationOptions & {
	readonly event: LinkNavigationEvent;
	readonly nav: LinkNavFns;
	readonly target_attr?: string;
};

export type LinkPointerDownInput = LinkNavigationOptions & {
	readonly event: LinkNavigationEvent;
	readonly nav: LinkNavFns;
	readonly target_attr?: string;
};

export type LinkPrefetchIntent = {
	start: Effect.Effect<void>;
	stop: Effect.Effect<void>;
};

export type LinkPrefetchInput = {
	readonly delay_ms: number;
	readonly href: string;
	readonly nav: Pick<LinkNavFns, "start_prefetch" | "stop_prefetch">;
};

export type LinkIntentRuntime = {
	click_navigation: (
		input: LinkClickInput,
	) => Effect.Effect<void, LinkNavigationFailed>;
	is_touch_active: Effect.Effect<boolean>;
	make_prefetch_intent: (
		input: LinkPrefetchInput,
	) => Effect.Effect<LinkPrefetchIntent>;
	pointer_down_navigation: (
		input: LinkPointerDownInput,
	) => Effect.Effect<void, LinkNavigationFailed>;
	report_navigation_failure: (
		message: string,
		failure: LinkNavigationFailed,
	) => Effect.Effect<void>;
	shutdown: Effect.Effect<void>;
};

export type LinkIntentRuntimeOptions = {
	readonly log_error?: (message: string, error: unknown) => void;
	readonly timer_runtime?: BrowserTimerRuntime;
};

function can_intercept_navigation(
	event: LinkNavigationEvent,
	target_attr: string | undefined,
): boolean {
	if (event.defaultPrevented) {
		return false;
	}
	if (getIsModifiedNavigationClick(event)) {
		return false;
	}
	if (!getIsPrimaryNavigationClick(event)) {
		return false;
	}
	if (target_attr && target_attr !== "" && target_attr !== "_self") {
		return false;
	}
	return true;
}

export function make_link_intent_runtime(
	options: LinkIntentRuntimeOptions = {},
): Effect.Effect<LinkIntentRuntime, never> {
	return Effect.gen(function* () {
		const timer_runtime =
			options.timer_runtime ?? (yield* make_browser_timer_runtime());
		const lifecycle = yield* make_runtime_lifecycle();
		const is_touch_active_ref = yield* Ref.make(false);
		const log_error =
			options.log_error ??
			((message: string, error: unknown): void => {
				console.error(message, error);
			});

		const set_pointer_modality = (
			pointer_type: string,
		): Effect.Effect<void> => {
			if (pointer_type === POINTER_TYPE_TOUCH) {
				return Ref.set(is_touch_active_ref, true);
			}
			if (
				pointer_type === POINTER_TYPE_MOUSE ||
				pointer_type === POINTER_TYPE_PEN
			) {
				return Ref.set(is_touch_active_ref, false);
			}
			return Effect.void;
		};

		yield* lifecycle.listen_window_effect("touchstart", () => {
			return Ref.set(is_touch_active_ref, true);
		});
		yield* lifecycle.listen_window_effect("pointerdown", (event) => {
			return set_pointer_modality(event.pointerType);
		});
		yield* lifecycle.listen_window_effect("pointermove", (event) => {
			return set_pointer_modality(event.pointerType);
		});

		const run_navigation = (
			phase: LinkNavigationPhase,
			nav: LinkNavFns,
			input: LinkNavigationOptions,
			include_state: boolean,
		): Effect.Effect<void, LinkNavigationFailed> => {
			return Effect.tryPromise({
				try: async () => {
					const args: Parameters<LinkNavFns["navigate"]>[0] = {
						href: input.href,
						replace: input.replace,
						scrollToTop: input.scroll_to_top,
					};
					if (input.skip_work_indicator !== undefined) {
						args.skipWorkIndicator = input.skip_work_indicator;
					}
					if (include_state) {
						args.state = input.state;
					}
					await nav.navigate(args);
				},
				catch: (error) => {
					return new LinkNavigationFailed({
						error,
						href: input.href,
						phase,
					});
				},
			}).pipe(Effect.asVoid);
		};

		const make_prefetch_intent: LinkIntentRuntime["make_prefetch_intent"] =
			(input) => {
				return Effect.gen(function* () {
					const cancel_ref = yield* Ref.make<TimerCancel | null>(
						null,
					);

					const clear_timer = Effect.gen(function* () {
						const cancel = yield* Ref.get(cancel_ref);
						if (!cancel) {
							return;
						}
						yield* Ref.set(cancel_ref, null);
						yield* cancel;
					});

					const start = Effect.gen(function* () {
						yield* clear_timer;
						const cancel = yield* timer_runtime.schedule_ms(
							input.delay_ms,
							Effect.sync(() => {
								input.nav.start_prefetch(input.href);
							}),
						);
						yield* Ref.set(cancel_ref, cancel);
					});

					const stop = Effect.gen(function* () {
						yield* clear_timer;
						yield* Effect.sync(() => {
							input.nav.stop_prefetch(input.href);
						});
					});

					return { start, stop };
				});
			};

		return {
			click_navigation: (input) => {
				if (!can_intercept_navigation(input.event, input.target_attr)) {
					return Effect.void;
				}
				return Effect.sync(() => {
					input.event.preventDefault?.();
				}).pipe(
					Effect.flatMap(() => {
						return run_navigation("click", input.nav, input, true);
					}),
				);
			},
			is_touch_active: Ref.get(is_touch_active_ref),
			make_prefetch_intent,
			pointer_down_navigation: (input) => {
				if (!can_intercept_navigation(input.event, input.target_attr)) {
					return Effect.void;
				}
				const pointer_type = input.event.pointerType;
				if (
					pointer_type !== POINTER_TYPE_MOUSE &&
					pointer_type !== POINTER_TYPE_PEN
				) {
					return Effect.void;
				}
				return Effect.sync(() => {
					input.event.preventDefault?.();
					input.event.currentTarget?.addEventListener?.(
						NATIVE_CLICK_EVENT,
						(event) => {
							event.preventDefault();
						},
						{ once: true },
					);
				}).pipe(
					Effect.flatMap(() => {
						return run_navigation(
							"pointerdown",
							input.nav,
							input,
							false,
						);
					}),
				);
			},
			report_navigation_failure: (message, failure) => {
				return Effect.sync(() => {
					log_error(message, failure.error);
				});
			},
			shutdown: lifecycle.shutdown,
		};
	});
}

export { LINK_CLICK_FAILED_MESSAGE, LINK_POINTER_DOWN_FAILED_MESSAGE };
