import { describe, expect, it } from "vitest";
import { createPatternRegistry, registerPattern } from "./register.ts";

describe("matcher registration collision invariants", () => {
	it("treats duplicate registration of the same original pattern as idempotent", () => {
		const registry = createPatternRegistry();
		const first = registerPattern(registry, "/users/:id");
		const second = registerPattern(registry, "/users/:id");

		expect(second).toBe(first);
		expect(registry.dynamicPatterns.size).toBe(1);
	});

	it("rejects normalized collisions between dynamic and static aliases", () => {
		const registry = createPatternRegistry({
			dynamicParamPrefixRune: "$",
		});
		registerPattern(registry, "/users/$id");

		expect(() => registerPattern(registry, "/users/:id")).toThrow(
			'normalized pattern collision: "/users/:id" and "/users/$id" both normalize to "/users/:id"',
		);
	});

	it("rejects normalized collisions between splat and literal aliases", () => {
		const registry = createPatternRegistry({
			splatSegmentRune: "#",
		});
		registerPattern(registry, "/files/#");

		expect(() => registerPattern(registry, "/files/*")).toThrow(
			'normalized pattern collision: "/files/*" and "/files/#" both normalize to "/files/*"',
		);
	});
});
