import { describe, expect, it } from "vitest";
import { findBestMatch } from "./find_best_match.ts";
import { createPatternRegistry, registerPattern } from "./register.ts";

describe("FindBestMatchAdditionalScenarios", () => {
	it("should not match /settings/account with registered patterns", () => {
		const registry_res = createPatternRegistry({
			explicitIndexSegment: "_index",
		});
		if (!registry_res.ok) {
			throw new Error(
				`createPatternRegistry() error: ${registry_res.err}`,
			);
		}
		const registry = registry_res.val;

		registerPattern(registry, "/");
		registerPattern(registry, "/:slug");
		registerPattern(registry, "/_index");
		registerPattern(registry, "/app");

		const path = "/settings/account";
		const match = findBestMatch(registry, path);

		expect(match).toBe(null);
	});
});
