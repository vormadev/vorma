import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { debounce } from "./debounce.ts";

describe("debounce", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
	});

	it("delays the call by the specified time and resolves with return value", async () => {
		const spy = vi.fn(
			(props: { firstNumber: number; secondNumber: number }) =>
				props.firstNumber + props.secondNumber,
		);
		const debounced = debounce(spy, 100);

		const resultPromise = debounced({
			firstNumber: 1,
			secondNumber: 2,
		});
		// not called immediately
		expect(spy).not.toHaveBeenCalled();

		// midway through delay
		vi.advanceTimersByTime(50);
		expect(spy).not.toHaveBeenCalled();

		// after full delay
		vi.advanceTimersByTime(50);
		await expect(resultPromise).resolves.toBe(3);

		expect(spy).toHaveBeenCalledTimes(1);
		expect(spy).toHaveBeenCalledWith({
			firstNumber: 1,
			secondNumber: 2,
		});
	});

	it("only the last call within the delay window executes", async () => {
		const spy = vi.fn((x: number) => x * 2);
		const debounced = debounce(spy, 100);

		// first call
		const _p1 = debounced(5);
		// before it fires, call again
		vi.advanceTimersByTime(50);
		const p2 = debounced(6);

		// advance past delay
		vi.advanceTimersByTime(100);

		// only the second promise should resolve
		await expect(p2).resolves.toBe(12);

		// ensure original call was cancelled
		expect(spy).toHaveBeenCalledTimes(1);
		expect(spy).toHaveBeenCalledWith(6);
	});

	it("maintains correct `this` if wrapped in a method", async () => {
		const obj = {
			value: 10,
			getValue(add: number) {
				return this.value + add;
			},
		};
		// wrap as a method
		const debounced = debounce(obj.getValue.bind(obj), 30);

		const p = debounced(5);
		vi.advanceTimersByTime(30);
		await expect(p).resolves.toBe(15);
	});

	it("rejects when the debounced function throws", async () => {
		const boom = new Error("boom");
		const spy = vi.fn(() => {
			throw boom;
		});
		const debounced = debounce(spy, 20);

		const p = debounced();
		vi.advanceTimersByTime(20);

		await expect(p).rejects.toThrow("boom");
		expect(spy).toHaveBeenCalledTimes(1);
	});
});
