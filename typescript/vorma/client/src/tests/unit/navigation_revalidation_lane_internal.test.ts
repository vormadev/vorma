import { describe, expect, it, vi } from "vitest";
import type { NavigateProps } from "../../../src/runtime.ts";
import {
	buildRevalidationLaneRuntimeCommandPlan,
	createDeterministicRevalidationLane,
	reduceRevalidationLaneEvent,
} from "../../runtime.ts";

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

describe("deterministic revalidation lane reducer and command planning", () => {
	it("plans start-pass commands when no pass is in flight", () => {
		const executionPlan = reduceRevalidationLaneEvent({
			state: {
				hasInFlightPass: false,
				isTrailingEligible: false,
				inFlightTargetMatchesCurrentHref: true,
			},
			event: {
				type: "revalidation_requested",
			},
		});
		expect(executionPlan).toEqual({
			type: "start_new_pass",
		});
		expect(
			buildRevalidationLaneRuntimeCommandPlan({
				executionPlan,
			}),
		).toEqual({
			commands: [{ type: "start_pass" }],
			terminalResult: { type: "return_started_pass" },
		});
	});

	it("plans mismatch restart commands in deterministic order", () => {
		const executionPlan = reduceRevalidationLaneEvent({
			state: {
				hasInFlightPass: true,
				isTrailingEligible: false,
				inFlightTargetMatchesCurrentHref: false,
			},
			event: {
				type: "revalidation_requested",
			},
		});
		expect(executionPlan).toEqual({
			type: "restart_after_target_mismatch",
		});
		expect(
			buildRevalidationLaneRuntimeCommandPlan({
				executionPlan,
			}),
		).toEqual({
			commands: [
				{ type: "notify_in_flight_target_mismatch" },
				{ type: "clear_queued_trailing_request" },
				{ type: "start_pass" },
			],
			terminalResult: { type: "return_started_pass" },
		});
	});

	it("plans in-flight reuse before trailing eligibility opens", () => {
		const executionPlan = reduceRevalidationLaneEvent({
			state: {
				hasInFlightPass: true,
				isTrailingEligible: false,
				inFlightTargetMatchesCurrentHref: true,
			},
			event: {
				type: "revalidation_requested",
			},
		});
		expect(executionPlan).toEqual({
			type: "reuse_in_flight_pass",
		});
		expect(
			buildRevalidationLaneRuntimeCommandPlan({
				executionPlan,
			}),
		).toEqual({
			commands: [],
			terminalResult: { type: "return_in_flight_pass" },
		});
	});

	it("plans trailing-pass scheduling when the trailing window is open", () => {
		const executionPlan = reduceRevalidationLaneEvent({
			state: {
				hasInFlightPass: true,
				isTrailingEligible: true,
				inFlightTargetMatchesCurrentHref: true,
			},
			event: {
				type: "revalidation_requested",
			},
		});
		expect(executionPlan).toEqual({
			type: "schedule_trailing_pass",
		});
		expect(
			buildRevalidationLaneRuntimeCommandPlan({
				executionPlan,
			}),
		).toEqual({
			commands: [{ type: "schedule_trailing_pass" }],
			terminalResult: { type: "return_scheduled_trailing_pass" },
		});
	});
});

describe("deterministic revalidation lane internals", () => {
	it("starts a single pass when idle", async () => {
		let currentHref = "http://localhost:3000/one";
		let hasInFlightPass = false;
		let inFlightTargetUrl: string | null = null;
		const onInFlightTargetMismatch = vi.fn();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => currentHref,
			getHasInFlightPass: () => hasInFlightPass,
			getInFlightTargetUrl: () => inFlightTargetUrl,
			onInFlightTargetMismatch,
		});
		const navigateSinglePass = vi.fn(async (props: NavigateProps) => {
			inFlightTargetUrl = props.href;
			hasInFlightPass = true;
			return {
				didNavigate: true,
			};
		});

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
		let hasInFlightPass = false;
		let inFlightTargetUrl: string | null = null;
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => "http://localhost:3000/same",
			getHasInFlightPass: () => hasInFlightPass,
			getInFlightTargetUrl: () => inFlightTargetUrl,
			onInFlightTargetMismatch: vi.fn(),
		});
		const navigateSinglePass = vi.fn((props: NavigateProps) => {
			inFlightTargetUrl = props.href;
			hasInFlightPass = true;
			return deferred.promise.finally(() => {
				hasInFlightPass = false;
			});
		});

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
		let hasInFlightPass = false;
		let inFlightTargetUrl: string | null = null;
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => "http://localhost:3000/trailing",
			getHasInFlightPass: () => hasInFlightPass,
			getInFlightTargetUrl: () => inFlightTargetUrl,
			onInFlightTargetMismatch: vi.fn(),
		});
		const navigateSinglePass = vi
			.fn()
			.mockImplementationOnce((props: NavigateProps) => {
				inFlightTargetUrl = props.href;
				hasInFlightPass = true;
				return firstDeferred.promise.finally(() => {
					hasInFlightPass = false;
				});
			})
			.mockImplementationOnce((props: NavigateProps) => {
				inFlightTargetUrl = props.href;
				hasInFlightPass = true;
				return secondDeferred.promise.finally(() => {
					hasInFlightPass = false;
				});
			});

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
		let hasInFlightPass = false;
		let inFlightTargetUrl: string | null = null;
		const firstDeferred = createDeferred<{ didNavigate: boolean }>();
		const mismatchResultPromise = Promise.resolve({ didNavigate: true });
		const onInFlightTargetMismatch = vi.fn();
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => currentHref,
			getHasInFlightPass: () => hasInFlightPass,
			getInFlightTargetUrl: () => inFlightTargetUrl,
			onInFlightTargetMismatch,
		});
		const navigateSinglePass = vi
			.fn()
			.mockImplementationOnce((props: NavigateProps) => {
				inFlightTargetUrl = props.href;
				hasInFlightPass = true;
				return firstDeferred.promise.finally(() => {
					hasInFlightPass = false;
				});
			})
			.mockImplementationOnce((props: NavigateProps) => {
				inFlightTargetUrl = props.href;
				hasInFlightPass = true;
				return mismatchResultPromise.finally(() => {
					hasInFlightPass = false;
				});
			});

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
		let hasInFlightPass = false;
		let inFlightTargetUrl: string | null = null;
		const runLane = createDeterministicRevalidationLane({
			getCurrentHref: () => "http://localhost:3000/clear",
			getHasInFlightPass: () => hasInFlightPass,
			getInFlightTargetUrl: () => inFlightTargetUrl,
			onInFlightTargetMismatch: vi.fn(),
		});
		const navigateSinglePass = vi.fn((props: NavigateProps) => {
			inFlightTargetUrl = props.href;
			hasInFlightPass = true;
			return firstDeferred.promise.finally(() => {
				hasInFlightPass = false;
			});
		});

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
