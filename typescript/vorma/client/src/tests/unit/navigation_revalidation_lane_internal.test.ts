import { describe, expect, it, vi } from "vitest";
import { createDeterministicRevalidationLane } from "../../core/navigation/runtime_revalidation_lane.ts";
import type { NavigateProps } from "../../core/navigation/types.ts";

type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (reason?: unknown) => void;
};

function createDeferred<T>(): Deferred<T> {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

describe("deterministic revalidation lane internals", () => {
	it("starts a single pass when idle", async () => {
		let currentHref = "http://localhost:3000/one";
		const onInFlightTargetMismatch = vi.fn();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => currentHref,
			onInFlightTargetMismatch,
		});
		const navigateSinglePass = vi.fn(async (_props: NavigateProps) => ({
			didNavigate: true,
		}));

		const result = await runLane.runRevalidation({ navigateSinglePass });

		expect(result).toEqual({ didNavigate: true });
		expect(navigateSinglePass).toHaveBeenCalledTimes(1);
		expect(navigateSinglePass).toHaveBeenCalledWith({
			href: currentHref,
			navigationType: "revalidation",
		});
		expect(onInFlightTargetMismatch).not.toHaveBeenCalled();
	});

	it("reuses in-flight promise before trailing window opens", async () => {
		const deferred = createDeferred<{ didNavigate: boolean }>();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => "http://localhost:3000/same",
			onInFlightTargetMismatch: vi.fn(),
		});
		const navigateSinglePass = vi.fn(() => deferred.promise);

		const firstPromise = runLane.runRevalidation({ navigateSinglePass });
		const secondPromise = runLane.runRevalidation({ navigateSinglePass });

		expect(secondPromise).toBe(firstPromise);
		expect(navigateSinglePass).toHaveBeenCalledTimes(1);

		deferred.resolve({ didNavigate: true });
		await expect(firstPromise).resolves.toEqual({ didNavigate: true });
	});

	it("queues at most one trailing pass once trailing window opens", async () => {
		const firstDeferred = createDeferred<{ didNavigate: boolean }>();
		const secondDeferred = createDeferred<{ didNavigate: boolean }>();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => "http://localhost:3000/trailing",
			onInFlightTargetMismatch: vi.fn(),
		});
		const navigateSinglePass = vi
			.fn()
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockImplementationOnce(() => secondDeferred.promise);

		const firstPromise = runLane.runRevalidation({ navigateSinglePass });
		await Promise.resolve();

		const trailingPromiseA = runLane.runRevalidation({
			navigateSinglePass,
		});
		const trailingPromiseB = runLane.runRevalidation({
			navigateSinglePass,
		});

		expect(trailingPromiseA).toBe(trailingPromiseB);
		expect(trailingPromiseA).not.toBe(firstPromise);
		expect(navigateSinglePass).toHaveBeenCalledTimes(1);

		firstDeferred.resolve({ didNavigate: true });
		await expect(firstPromise).resolves.toEqual({ didNavigate: true });
		expect(navigateSinglePass).toHaveBeenCalledTimes(2);

		secondDeferred.resolve({ didNavigate: true });
		await expect(trailingPromiseA).resolves.toEqual({ didNavigate: true });
	});

	it("starts a new pass immediately when in-flight target mismatches current href", async () => {
		let currentHref = "http://localhost:3000/first";
		const firstDeferred = createDeferred<{ didNavigate: boolean }>();
		const mismatchResultPromise = Promise.resolve({ didNavigate: true });
		const onInFlightTargetMismatch = vi.fn();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => currentHref,
			onInFlightTargetMismatch,
		});
		const navigateSinglePass = vi
			.fn()
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockImplementationOnce(() => mismatchResultPromise);

		const firstPromise = runLane.runRevalidation({ navigateSinglePass });
		currentHref = "http://localhost:3000/second";

		const mismatchPromise = runLane.runRevalidation({ navigateSinglePass });

		await expect(mismatchPromise).resolves.toEqual({ didNavigate: true });
		expect(onInFlightTargetMismatch).toHaveBeenCalledTimes(1);
		expect(navigateSinglePass).toHaveBeenCalledTimes(2);
		expect(navigateSinglePass).toHaveBeenNthCalledWith(2, {
			href: "http://localhost:3000/second",
			navigationType: "revalidation",
		});

		firstDeferred.resolve({ didNavigate: true });
		await expect(firstPromise).resolves.toEqual({ didNavigate: true });
	});

	it("clears queued trailing request without running a second pass", async () => {
		const firstDeferred = createDeferred<{ didNavigate: boolean }>();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => "http://localhost:3000/clear",
			onInFlightTargetMismatch: vi.fn(),
		});
		const navigateSinglePass = vi.fn(() => firstDeferred.promise);

		const firstPromise = runLane.runRevalidation({ navigateSinglePass });
		await Promise.resolve();
		const trailingPromise = runLane.runRevalidation({ navigateSinglePass });

		runLane.clearQueuedTrailingRequest();
		await expect(trailingPromise).resolves.toEqual({ didNavigate: false });

		firstDeferred.resolve({ didNavigate: true });
		await expect(firstPromise).resolves.toEqual({ didNavigate: true });
		expect(navigateSinglePass).toHaveBeenCalledTimes(1);
	});
});
