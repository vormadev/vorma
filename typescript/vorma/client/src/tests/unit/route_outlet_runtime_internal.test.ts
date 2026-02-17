import { describe, expect, it } from "vitest";
import {
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildRouteOutletBranchState,
	buildRouteOutletRouteKey,
} from "../../ui/route_outlet_runtime.ts";

describe("route outlet runtime internals", () => {
	it("builds distinct route keys when import and export parts contain delimiters", () => {
		const firstKey = buildRouteOutletRouteKey({
			importURLs: ["/routes/a|b.tsx"],
			exportKeys: ["Route"],
			idx: 0,
		});
		const secondKey = buildRouteOutletRouteKey({
			importURLs: ["/routes/a.tsx"],
			exportKeys: ["b|Route"],
			idx: 0,
		});

		expect(firstKey).not.toBe(secondKey);
	});

	it("propagates collision-safe keys through branch state", () => {
		const branchState = buildRouteOutletBranchState({
			navigationState: {
				loaderCount: 2,
				outermostErrorIdx: undefined,
				activeComponents: [() => "A", () => "B"],
				activeErrorBoundary: undefined,
				importURLs: ["/routes/a|b.tsx", "/routes/a.tsx"],
				exportKeys: ["Route", "b|Route"],
			},
			idx: 0,
		});

		expect(branchState.currentRouteKey).not.toBe(branchState.nextRouteKey);
		expect(branchState.currentRouteKey).toBe(
			JSON.stringify([0, "/routes/a|b.tsx", "Route"]),
		);
		expect(branchState.nextRouteKey).toBe(
			JSON.stringify([1, "/routes/a.tsx", "b|Route"]),
		);
	});

	it("keeps branch keys distinct when route metadata is missing", () => {
		const branchState = buildRouteOutletBranchState({
			navigationState: {
				loaderCount: 2,
				outermostErrorIdx: undefined,
				activeComponents: [undefined, () => "Child"],
				activeErrorBoundary: undefined,
				importURLs: [],
				exportKeys: [],
			},
			idx: 0,
		});

		expect(branchState.currentRouteKey).not.toBe(branchState.nextRouteKey);
	});

	it("compares location state by pathname/search/hash and state identity", () => {
		const sharedState = { from: "test" };
		expect(
			areRouteOutletLocationsEqual({
				firstLocationState: {
					pathname: "/docs",
					search: "?tab=api",
					hash: "#intro",
					state: sharedState,
				},
				secondLocationState: {
					pathname: "/docs",
					search: "?tab=api",
					hash: "#intro",
					state: sharedState,
				},
			}),
		).toBe(true);

		expect(
			areRouteOutletLocationsEqual({
				firstLocationState: {
					pathname: "/docs",
					search: "?tab=api",
					hash: "#intro",
					state: sharedState,
				},
				secondLocationState: {
					pathname: "/docs",
					search: "?tab=api",
					hash: "#usage",
					state: sharedState,
				},
			}),
		).toBe(false);
	});

	it("compares branch input snapshots by identity-sensitive fields", () => {
		const activeComponents = [() => "A"];
		const importURLs = ["/routes/a.tsx"];
		const exportKeys = ["Route"];
		const firstInputState = {
			loaderCount: 1,
			outermostErrorIdx: undefined,
			activeComponents,
			activeErrorBoundary: undefined,
			importURLs,
			exportKeys,
		};

		expect(
			areRouteOutletBranchInputsEqualByIdentity({
				firstInputState,
				secondInputState: {
					loaderCount: 1,
					outermostErrorIdx: undefined,
					activeComponents,
					activeErrorBoundary: undefined,
					importURLs,
					exportKeys,
				},
			}),
		).toBe(true);

		expect(
			areRouteOutletBranchInputsEqualByIdentity({
				firstInputState,
				secondInputState: {
					loaderCount: 1,
					outermostErrorIdx: undefined,
					activeComponents: [...activeComponents],
					activeErrorBoundary: undefined,
					importURLs,
					exportKeys,
				},
			}),
		).toBe(false);
	});
});
