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

describe("TestFindAllMatchesAdditionalScenarios", () => {
	const testCases = [
		{
			name: "Invalid match with unhandled segment",
			patterns: ["/", "/:slug", "/_index", "/app"],
			path: "/settings/account",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "Deeper Invalid 'Almost' Match",
			patterns: ["/dashboard/customers"],
			path: "/dashboard/customers/reports",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "Splat as the Only Full Match",
			patterns: ["/files/*", "/files/images"],
			path: "/files/documents/report.pdf",
			expectMatch: true,
			expectedMatches: ["/files/*"],
		},
		{
			name: "Index Segment Edge Case with Extra Segment",
			patterns: ["/articles/_index"],
			path: "/articles/some-topic",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "No Root Fallback for Multi-Segment Path",
			patterns: ["/"],
			path: "/some/random/path",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "A",
			patterns: ["/"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/"],
		},
		{
			name: "B",
			patterns: ["/*"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/*"],
		},
		{
			name: "C",
			patterns: ["/_index"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/_index"],
		},
		{
			name: "AB",
			patterns: ["/", "/*"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/", "/*"],
		},
		{
			name: "AC",
			patterns: ["/", "/_index"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/", "/_index"],
		},
		{
			name: "BC",
			patterns: ["/*", "/_index"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/_index"],
		},
		{
			name: "ABC",
			patterns: ["/", "/*", "/_index"],
			path: "/",
			expectMatch: true,
			expectedMatches: ["/", "/_index"],
		},
		{
			name: "A-docs",
			patterns: ["/"],
			path: "/docs",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "B-docs",
			patterns: ["/*"],
			path: "/docs",
			expectMatch: true,
			expectedMatches: ["/*"],
		},
		{
			name: "C-docs",
			patterns: ["/_index"],
			path: "/docs",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "AB-docs",
			patterns: ["/", "/*"],
			path: "/docs",
			expectMatch: true,
			expectedMatches: ["/", "/*"],
		},
		{
			name: "AC-docs",
			patterns: ["/", "/_index"],
			path: "/docs",
			expectMatch: false,
			expectedMatches: [],
		},
		{
			name: "BC-docs",
			patterns: ["/*", "/_index"],
			path: "/docs",
			expectMatch: true,
			expectedMatches: ["/*"],
		},
		{
			name: "ABC-docs",
			patterns: ["/", "/*", "/_index"],
			path: "/docs",
			expectMatch: true,
			expectedMatches: ["/", "/*"],
		},
	];

	for (const tc of testCases) {
		it(tc.name, () => {
			const registry = createPatternRegistry({
				explicitIndexSegment: "_index",
			});
			for (const p of tc.patterns) {
				registerPattern(registry, p);
			}

			const results = findNestedMatches(registry, tc.path);

			expect(!!results).toBe(tc.expectMatch);

			// If no match was expected, ensure the results are truly empty.
			if (!tc.expectMatch) {
				if (results !== null && results.matches.length !== 0) {
					throw new Error(
						`Expected no matches for path ${tc.path}, but got ${results.matches.length} matches`,
					);
				}
			}

			// If a match was expected, check the specific patterns that matched
			if (tc.expectMatch) {
				if (results === null || results.matches.length === 0) {
					throw new Error(
						`Expected matches for path ${tc.path}, but got none`,
					);
				} else if (
					tc.expectedMatches &&
					tc.expectedMatches.length > 0
				) {
					// Check that we got the expected patterns
					const actualPatterns = results.matches.map(
						(m) => m.registeredPattern.originalPattern,
					);

					if (actualPatterns.length !== tc.expectedMatches.length) {
						throw new Error(
							`Path ${tc.path}: expected ${tc.expectedMatches.length} matches ${JSON.stringify(tc.expectedMatches)}, got ${actualPatterns.length} matches ${JSON.stringify(actualPatterns)}`,
						);
					} else {
						for (let i = 0; i < tc.expectedMatches.length; i++) {
							if (actualPatterns[i] !== tc.expectedMatches[i]) {
								throw new Error(
									`Path ${tc.path}: at position [${i}], expected ${tc.expectedMatches[i]}, got ${actualPatterns[i]}`,
								);
							}
						}
					}
				}
			}
		});
	}
});
