import { describe, expect, it } from "vitest";
import { create_core6_deferred } from "./deferred.ts";

describe("core6 deferred settlement", () => {
	it("resolves exactly once", async () => {
		const deferred = create_core6_deferred<string>();

		expect(deferred.settled()).toBe(false);
		expect(deferred.resolve("first")).toBe(true);
		expect(deferred.resolve("second")).toBe(false);
		expect(deferred.reject(new Error("late"))).toBe(false);

		await expect(deferred.promise).resolves.toBe("first");
		expect(deferred.settled()).toBe(true);
	});

	it("rejects exactly once", async () => {
		const deferred = create_core6_deferred<string>();
		const error = new Error("failed");

		expect(deferred.reject(error)).toBe(true);
		expect(deferred.resolve("late")).toBe(false);

		await expect(deferred.promise).rejects.toBe(error);
		expect(deferred.settled()).toBe(true);
	});

	it("adopts promised resolution values", async () => {
		const deferred = create_core6_deferred<string>();

		expect(deferred.resolve(Promise.resolve("async"))).toBe(true);

		await expect(deferred.promise).resolves.toBe("async");
	});
});
