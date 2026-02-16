import { describe, expect, it } from "vitest";
import { findBestMatch } from "./find_best_match.ts";
import { createPatternRegistry, registerPattern } from "./register.ts";

describe("FindBestMatchAdditionalScenarios", () => {
	it("should not match /settings/account with registered patterns", () => {
		const registry = createPatternRegistry({
			explicitIndexSegment: "_index",
		});

		registerPattern(registry, "/");
		registerPattern(registry, "/:slug");
		registerPattern(registry, "/_index");
		registerPattern(registry, "/app");

		const path = "/settings/account";
		const match = findBestMatch(registry, path);

		expect(match).toBe(null);
	});
});
