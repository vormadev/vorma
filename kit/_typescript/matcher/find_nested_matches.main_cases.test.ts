import { describe, expect, it } from "vitest";
import { findNestedMatches } from "./find_nested_matches.ts";
import {
	createPatternRegistry,
	registerPattern,
} from "./register.ts";
import {
	NestedPatterns,
	NestedScenarios,
	differentOptsToTest,
	equalParams,
	equalSplat,
	modifyPatternsToOpts,
} from "./find_nested_matches.test.helpers.ts";

describe("TestFindAllMatches", () => {
	for (const opts of differentOptsToTest) {
		describe(`with options ${JSON.stringify(opts)}`, () => {
			const registry = createPatternRegistry(opts);

			for (const p of modifyPatternsToOpts(
				NestedPatterns,
				"_index",
				opts,
			)) {
				registerPattern(registry, p);
			}

			for (const tc of NestedScenarios) {
				it(tc.Path, () => {
					const results = findNestedMatches(registry, tc.Path);

					if (!results) {
						if (tc.ExpectedMatches.length > 0) {
							throw new Error(`Expected results for ${tc.Path}`);
						}
						return;
					}

					if (!equalParams(tc.Params, results.params)) {
						throw new Error(
							`Expected params ${JSON.stringify(tc.Params)}, got ${JSON.stringify(results.params)}`,
						);
					}
					if (!equalSplat(tc.SplatValues, results.splatValues)) {
						throw new Error(
							`Expected splat values ${JSON.stringify(tc.SplatValues)}, got ${JSON.stringify(results.splatValues)}`,
						);
					}

					const actualMatches = results.matches;
					const errors: string[] = [];

					// Check if there's a failure
					const expectedCount = tc.ExpectedMatches.length;
					const actualCount = actualMatches.length;

					let fail =
						(!results && expectedCount > 0) ||
						expectedCount !== actualCount;

					// Compare each matched pattern
					for (
						let i = 0;
						i < Math.max(expectedCount, actualCount);
						i++
					) {
						if (i < expectedCount && i < actualCount) {
							const expected = tc.ExpectedMatches[i];
							const actual = actualMatches[i];

							if (
								expected !==
								actual?.registeredPattern.normalizedPattern
							) {
								fail = true;
								break;
							}
						} else {
							fail = true;
							break;
						}
					}

					// Only output errors if a failure occurred
					if (fail) {
						errors.push(`\n===== Path: "${tc.Path}" =====`);

						// Expected matches exist but got none
						if (!results && expectedCount > 0) {
							errors.push("Expected matches but got none.");
						}

						// Length mismatch
						if (expectedCount !== actualCount) {
							errors.push(
								`Expected ${expectedCount} matches, got ${actualCount}`,
							);
						}

						// Always output all expected and actual matches for debugging
						errors.push("Expected Matches:");
						for (let i = 0; i < tc.ExpectedMatches.length; i++) {
							const expected = tc.ExpectedMatches[i];
							errors.push(`  [${i}] {Pattern: "${expected}"}`);
						}

						errors.push("Actual Matches:");
						for (let i = 0; i < actualMatches.length; i++) {
							const actual = actualMatches[i];
							errors.push(
								`  [${i}] {Pattern: "${actual?.registeredPattern.normalizedPattern}"}`,
							);
						}

						// Print only if something went wrong
						throw new Error(errors.join("\n"));
					}
				});
			}
		});
	}
});
