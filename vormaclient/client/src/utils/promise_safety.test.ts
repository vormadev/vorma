import { describe, expect, it } from "vitest";
import { observePromiseRejection } from "./promise_safety.ts";

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
