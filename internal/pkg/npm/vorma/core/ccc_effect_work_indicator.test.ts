import { Effect } from "effect";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { WorkIndicatorOptions } from "./effect_runtime/client_contract.ts";
import { make_work_indicator } from "./effect_runtime/work_indicator.ts";

type WorkIndicatorRenderer = WorkIndicatorOptions & {
	stop: ReturnType<typeof vi.fn>;
	start: ReturnType<typeof vi.fn>;
};

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (error: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function renderer(): WorkIndicatorRenderer {
	return {
		stop: vi.fn(() => {
			return;
		}),
		stopDelayMS: 1,
		start: vi.fn(() => {
			return;
		}),
		startDelayMS: 1,
	};
}

describe("ccc Effect work indicator experiment", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it("tracks app-owned work and reconciles delayed start/stop", async () => {
		vi.useFakeTimers();
		const config = renderer();
		const runtime = await run_effect(make_work_indicator());
		await run_effect(runtime.configure(config));
		const work = deferred<number>();

		const tracked = runtime.indicator.track(work.promise);

		expect(runtime.indicator.isActive()).toBe(true);
		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		work.resolve(42);
		await expect(tracked).resolves.toBe(42);
		expect(runtime.indicator.isActive()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("moves visible work to replacement options", async () => {
		vi.useFakeTimers();
		const first = renderer();
		const second = renderer();
		const runtime = await run_effect(make_work_indicator());
		await run_effect(runtime.configure(first));
		const work = deferred<void>();
		const tracked = runtime.indicator.track(work.promise);
		await vi.advanceTimersByTimeAsync(1);
		expect(first.start).toHaveBeenCalledTimes(1);

		await run_effect(runtime.configure(second));
		await vi.advanceTimersByTimeAsync(1);

		expect(first.stop).toHaveBeenCalledTimes(1);
		expect(second.start).toHaveBeenCalledTimes(1);
		work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);
		expect(second.stop).toHaveBeenCalledTimes(1);
	});
});
