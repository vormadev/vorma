import { describe, expect, it } from "vitest";
import { isSkipEligibilityViolated } from "./skip_server_fetch_rules.ts";
import type { SkipCheckContext } from "./skip_server_fetch_types.ts";

function buildContext(targetHref: string): SkipCheckContext {
	return {
		routeManifest: { "/items": 1 },
		patternRegistry: {},
		patternToWaitFnMap: {},
		clientModuleMap: {
			"/items": {
				importURL: "/items.js",
				exportKey: "default",
				errorExportKey: "",
			},
		},
		currentMatchedPatterns: ["/items"],
		currentParams: {},
		currentSplatValues: [],
		currentLoadersData: [{}],
		url: new URL(targetHref),
		matchResult: {
			matches: [
				{
					registeredPattern: {
						originalPattern: "/items",
						normalizedSegments: [],
						lastSegType: "static",
					},
				},
			],
			params: {},
			splatValues: [],
		},
	};
}

describe("skip server fetch eligibility", () => {
	it("treats query order changes as changed for skip gating", () => {
		window.history.replaceState({}, "", "/items?a=1&b=2");
		const ctx = buildContext("http://localhost:3000/items?b=2&a=1");

		expect(isSkipEligibilityViolated(ctx)).toBe(true);
	});

	it("allows skip gating when query string is exactly unchanged", () => {
		window.history.replaceState({}, "", "/items?a=1&b=2");
		const ctx = buildContext("http://localhost:3000/items?a=1&b=2");

		expect(isSkipEligibilityViolated(ctx)).toBe(false);
	});
});
