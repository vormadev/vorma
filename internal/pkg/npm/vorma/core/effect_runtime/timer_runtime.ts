import { Effect } from "effect";

export type SleepMS = (delay_ms: number) => Effect.Effect<void>;
export type TimerCancel = Effect.Effect<void>;
export type ScheduleMS = (
	delay_ms: number,
	action: Effect.Effect<void>,
) => Effect.Effect<TimerCancel>;

export type BrowserTimerRuntime = {
	schedule_ms: ScheduleMS;
	sleep_ms: SleepMS;
};

export function browser_sleep_ms(delay_ms: number): Effect.Effect<void> {
	return Effect.callback<void>((resume, signal) => {
		const timeout_id: ReturnType<typeof setTimeout> = setTimeout(() => {
			resume(Effect.void);
		}, delay_ms);
		const clear_timeout = (): void => {
			clearTimeout(timeout_id);
		};
		signal.addEventListener("abort", clear_timeout, { once: true });
		return Effect.sync(() => {
			signal.removeEventListener("abort", clear_timeout);
			clear_timeout();
		});
	});
}

export function browser_schedule_ms(
	delay_ms: number,
	action: Effect.Effect<void>,
): Effect.Effect<TimerCancel> {
	return Effect.sync(() => {
		const timeout_id: ReturnType<typeof setTimeout> = setTimeout(() => {
			Effect.runSync(action);
		}, delay_ms);
		return Effect.sync(() => {
			clearTimeout(timeout_id);
		});
	});
}

export function make_browser_timer_runtime(): Effect.Effect<
	BrowserTimerRuntime,
	never
> {
	return Effect.succeed({
		schedule_ms: browser_schedule_ms,
		sleep_ms: browser_sleep_ms,
	});
}
