import { describe, expect, it } from "vitest";
import { findNestedMatches } from "./find_nested_matches.ts";
import { createPatternRegistry, registerPattern } from "./register.ts";

describe("TestPartialMatchingWithGaps", () => {
	it("should match parent and deeply nested route without intermediate routes", () => {
		const registry = createPatternRegistry({
			explicitIndexSegment: "_index",
		});

		// Register only the parent and the deeply nested route
		// NOT registering /bob/larry or /bob/larry/susan
		registerPattern(registry, "/bob");
		registerPattern(registry, "/bob/larry/susan/jeff");

		// Try to match the full path
		const results = findNestedMatches(registry, "/bob/larry/susan/jeff");

		expect(results).not.toBeNull();
		expect(results?.matches).toHaveLength(2);

		// Check that we got the right patterns
		const patterns = results!.matches.map(
			(m) => m.registeredPattern.originalPattern,
		);
		expect(patterns).toContain("/bob");
		expect(patterns).toContain("/bob/larry/susan/jeff");
	});

	it("should not match intermediate paths that aren't registered", () => {
		const registry = createPatternRegistry({
			explicitIndexSegment: "_index",
		});

		registerPattern(registry, "/bob");
		registerPattern(registry, "/bob/larry/susan/jeff");

		// Try to match an intermediate path
		const results = findNestedMatches(registry, "/bob/larry");

		// This should NOT find a match because /bob/larry isn't registered
		// and /bob/larry/susan/jeff doesn't match
		expect(results).toBeNull();
	});
});
