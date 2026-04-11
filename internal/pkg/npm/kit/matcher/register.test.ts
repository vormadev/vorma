import { describe, expect, it } from "vitest";
import { createPatternRegistry, registerPattern } from "./register.ts";

describe("matcher registration collision invariants", () => {
	it("treats duplicate registration of the same original pattern as idempotent", () => {
		const registry_res = createPatternRegistry();
		if (!registry_res.ok) {
			throw new Error(
				`createPatternRegistry() error: ${registry_res.err}`,
			);
		}
		const registry = registry_res.val;
		const first_res = registerPattern(registry, "/users/:id");
		const second_res = registerPattern(registry, "/users/:id");
		if (!first_res.ok) {
			throw new Error(`registerPattern() error: ${first_res.err}`);
		}
		if (!second_res.ok) {
			throw new Error(`registerPattern() error: ${second_res.err}`);
		}
		const first = first_res.val;
		const second = second_res.val;

		expect(second).toBe(first);
		expect(registry.dynamicPatterns.size).toBe(1);
	});

	it("rejects normalized collisions between dynamic and static aliases", () => {
		const registry_res = createPatternRegistry({
			dynamicParamPrefixRune: "$",
		});
		if (!registry_res.ok) {
			throw new Error(
				`createPatternRegistry() error: ${registry_res.err}`,
			);
		}
		const registry = registry_res.val;
		registerPattern(registry, "/users/$id");

		expect(registerPattern(registry, "/users/:id").ok).toBe(false);
	});

	it("rejects normalized collisions between splat and literal aliases", () => {
		const registry_res = createPatternRegistry({
			splatSegmentRune: "#",
		});
		if (!registry_res.ok) {
			throw new Error(
				`createPatternRegistry() error: ${registry_res.err}`,
			);
		}
		const registry = registry_res.val;
		registerPattern(registry, "/files/#");

		expect(registerPattern(registry, "/files/*").ok).toBe(false);
	});
});
