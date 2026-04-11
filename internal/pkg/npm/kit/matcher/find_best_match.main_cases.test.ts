import { describe, expect, it } from "vitest";
import {
	differentOptsToTest,
	getTestCases,
	modifyPatternsToOpts,
	NOT_FOUND,
} from "./find_best_match.test.helpers.ts";
import { findBestMatch } from "./find_best_match.ts";
import { createPatternRegistry, registerPattern } from "./register.ts";

describe("FindBestMatch", () => {
	for (const opts of differentOptsToTest) {
		for (const tt of getTestCases()) {
			describe(`with options ${JSON.stringify(opts)}`, () => {
				it(tt.name, () => {
					const registry_res = createPatternRegistry(opts);
					if (!registry_res.ok) {
						throw new Error(
							`createPatternRegistry() error: ${registry_res.err}`,
						);
					}
					const registry = registry_res.val;
					for (const pattern of modifyPatternsToOpts(
						tt.patterns,
						"",
						opts,
					)) {
						registerPattern(registry, pattern);
					}

					const match = findBestMatch(registry, tt.path);
					const wantMatch = tt.wantPattern !== NOT_FOUND;

					if (wantMatch && match === null) {
						throw new Error(
							`FindBestMatch() match for ${tt.path} = null -- want ${tt.wantPattern}`,
						);
					}

					if (!wantMatch) {
						if (match !== null) {
							throw new Error(
								`FindBestMatch() match for ${tt.path} = ${match.registeredPattern?.normalizedPattern} -- want null`,
							);
						}
						return;
					}

					expect(match!.registeredPattern!.normalizedPattern).toBe(
						tt.wantPattern,
					);

					// Compare params, allowing null == empty map
					if (
						tt.wantParams === null &&
						Object.keys(match!.params).length > 0
					) {
						throw new Error(
							`FindBestMatch() params = ${JSON.stringify(match!.params)}, want null`,
						);
					} else if (tt.wantParams !== null) {
						expect(match!.params).toEqual(tt.wantParams);
					}

					// Compare splat segments
					if (tt.wantSplatSegments === null) {
						expect(match!.splatValues).toEqual([]);
					} else {
						expect(match!.splatValues).toEqual(
							tt.wantSplatSegments,
						);
					}
				});
			});
		}
	}
});
