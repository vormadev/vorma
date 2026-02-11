import { afterEach, describe, expect, it, vi } from "vitest";
import {
	isArrayBufferView,
	isInstanceOfGlobal,
	observePromiseRejection,
} from "../../platform/safety.ts";

describe("global_constructors", () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it("returns false instead of throwing when constructors are unavailable", () => {
		vi.stubGlobal("Blob", undefined as any);
		vi.stubGlobal("FormData", undefined as any);
		vi.stubGlobal("URLSearchParams", undefined as any);
		vi.stubGlobal("ReadableStream", undefined as any);
		vi.stubGlobal("ArrayBuffer", undefined as any);

		expect(isInstanceOfGlobal({}, "Blob")).toBe(false);
		expect(isInstanceOfGlobal({}, "FormData")).toBe(false);
		expect(isInstanceOfGlobal({}, "URLSearchParams")).toBe(false);
		expect(isInstanceOfGlobal({}, "ReadableStream")).toBe(false);
		expect(isInstanceOfGlobal({}, "ArrayBuffer")).toBe(false);
		expect(isArrayBufferView({})).toBe(false);
	});

	it("detects matching instances when constructors exist", () => {
		const params = new URLSearchParams("a=1");
		const bytes = new Uint8Array([1, 2, 3]);

		expect(isInstanceOfGlobal(params, "URLSearchParams")).toBe(true);
		expect(isInstanceOfGlobal(bytes.buffer, "ArrayBuffer")).toBe(true);
		expect(isArrayBufferView(bytes)).toBe(true);
	});
});

describe("promise safety helpers", () => {
	it("preserves resolved promise values", async () => {
		const guarded = observePromiseRejection(Promise.resolve("ok"));
		await expect(guarded).resolves.toBe("ok");
	});

	it("preserves rejection behavior for awaiters", async () => {
		const error = new Error("boom");
		const guarded = observePromiseRejection(Promise.reject(error));
		await expect(guarded).rejects.toBe(error);
	});
});
