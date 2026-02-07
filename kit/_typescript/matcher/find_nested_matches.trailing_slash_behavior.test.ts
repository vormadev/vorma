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

describe("TestTrailingSlashBehavior", () => {
	const patterns = [
		"/",
		"/_index",
		"/about",
		"/about/location",
		"/about/hobbies",
		"/about/:id",
		"/about/*",
	];

	const testCases = [
		{
			name: "about with trailing slash",
			path: "/about/",
			expectedMatches: ["/", "/about"],
			unexpectedMatches: ["/about/:id", "/about/*"],
		},
		{
			name: "about without trailing slash",
			path: "/about",
			expectedMatches: ["/", "/about"],
			unexpectedMatches: ["/about/:id", "/about/*"],
		},
		{
			name: "about with actual id",
			path: "/about/123",
			expectedMatches: ["/", "/about/:id"],
			unexpectedMatches: ["/about/*"],
		},
		{
			name: "about location exact match",
			path: "/about/location",
			expectedMatches: ["/", "/about", "/about/location"],
			unexpectedMatches: [
				"/_index",
				"/about/:id", // exact match should take precedence
				"/about/*", // exact match should take precedence
			],
		},
		{
			name: "about with multiple segments",
			path: "/about/something/else",
			expectedMatches: [
				"/",
				"/about/*", // should catch multiple segments
			],
			unexpectedMatches: [
				"/about/:id", // only handles one segment
				"/about/location", // not an exact match
			],
		},
	];

	const registry = createPatternRegistry({ explicitIndexSegment: "_index" });
	for (const p of patterns) {
		registerPattern(registry, p);
	}

	for (const tc of testCases) {
		it(tc.name, () => {
			const results = findNestedMatches(registry, tc.path);

			if (!results) {
				throw new Error(
					`Path ${tc.path}: expected to find matches, but got none`,
				);
			}

			// Extract the patterns from results
			const actualPatterns = results!.matches;

			// Check for expected patterns
			for (const expected of tc.expectedMatches) {
				const found = actualPatterns.some(
					(actual) =>
						actual.registeredPattern.originalPattern === expected,
				);
				if (!found && expected !== "") {
					// ignore empty string check for now
					throw new Error(
						`Path ${tc.path}: expected pattern ${expected} to match, but it didn't`,
					);
				}
			}

			// Check for unexpected patterns
			for (const unexpected of tc.unexpectedMatches) {
				const found = actualPatterns.some(
					(actual) =>
						actual.registeredPattern.originalPattern === unexpected,
				);
				if (found) {
					throw new Error(
						`Path ${tc.path}: pattern ${unexpected} should NOT match, but it did`,
					);
				}
			}
		});
	}
});
