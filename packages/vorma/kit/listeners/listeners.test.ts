// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { addOnWindowFocusListener } from "./listeners.ts";

describe("addOnWindowFocusListener", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
	});

	it("runs the callback after the focus debounce delay", () => {
		const callback = vi.fn();
		const cleanup = addOnWindowFocusListener(callback);
		try {
			window.dispatchEvent(new Event("focus"));
			vi.advanceTimersByTime(29);
			expect(callback).not.toHaveBeenCalled();

			vi.advanceTimersByTime(1);
			expect(callback).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("cancels pending debounced focus callbacks on cleanup", () => {
		const callback = vi.fn();
		const cleanup = addOnWindowFocusListener(callback);

		window.dispatchEvent(new Event("focus"));
		cleanup();
		vi.advanceTimersByTime(30);

		expect(callback).not.toHaveBeenCalled();
	});
});
