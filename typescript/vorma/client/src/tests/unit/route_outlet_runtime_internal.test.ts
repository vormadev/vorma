import { beforeEach, describe, expect, it, vi } from "vitest";

const { getClientRuntimeRenderStateMock, getRouterDataMock, getLocationMock } =
	vi.hoisted(() => {
		return {
			getClientRuntimeRenderStateMock: vi.fn(),
			getRouterDataMock: vi.fn(),
			getLocationMock: vi.fn(),
		};
	});

vi.mock("../../app/context.ts", async (importOriginal) => {
	const actual =
		await importOriginal<typeof import("../../app/context.ts")>();
	return {
		...actual,
		getClientRuntimeRenderState: getClientRuntimeRenderStateMock,
		getRouterData: getRouterDataMock,
	};
});

vi.mock("../../client.ts", async (importOriginal) => {
	const actual = await importOriginal<typeof import("../../client.ts")>();
	return {
		...actual,
		getLocation: getLocationMock,
	};
});

import {
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchState,
	buildRouteOutletRouteKey,
} from "../../ui/route_outlet_runtime.ts";

beforeEach(() => {
	getClientRuntimeRenderStateMock.mockReset();
	getRouterDataMock.mockReset();
	getLocationMock.mockReset();

	const sharedLocationState = { from: "default" };
	getClientRuntimeRenderStateMock.mockReturnValue({
		loadersData: [{ root: true }],
		clientLoadersData: [{ client: true }],
		outermostError: undefined,
		outermostErrorIdx: undefined,
		activeComponents: ["RootComponent"],
		activeErrorBoundary: undefined,
		importURLs: ["/routes/root.tsx"],
		exportKeys: ["Route"],
	});
	getRouterDataMock.mockReturnValue({
		buildID: "1",
		matchedPatterns: ["/"],
		splatValues: [],
		params: {},
		rootData: { root: true },
	});
	getLocationMock.mockReturnValue({
		pathname: "/",
		search: "",
		hash: "",
		state: sharedLocationState,
	});
});

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

	it("builds initial store state from current runtime snapshots", () => {
		const initialState = buildInitialRouteOutletStoreState();

		expect(initialState.navigation).toEqual({
			loadersData: [{ root: true }],
			clientLoadersData: [{ client: true }],
			routerData: {
				buildID: "1",
				matchedPatterns: ["/"],
				splatValues: [],
				params: {},
				rootData: { root: true },
			},
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: ["RootComponent"],
			activeErrorBoundary: undefined,
			importURLs: ["/routes/root.tsx"],
			exportKeys: ["Route"],
		});
		expect(initialState.routeOutletBranchInputState).toEqual({
			loaderCount: 1,
			outermostErrorIdx: undefined,
			activeComponents: ["RootComponent"],
			activeErrorBoundary: undefined,
			importURLs: ["/routes/root.tsx"],
			exportKeys: ["Route"],
		});
		expect(initialState.location).toEqual({
			pathname: "/",
			search: "",
			hash: "",
			state: { from: "default" },
		});
	});

	it("reuses previous store state when runtime snapshots are equivalent", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();

		getClientRuntimeRenderStateMock.mockReturnValue({
			loadersData: [{ root: true }],
			clientLoadersData: [{ client: true }],
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: ["RootComponent"],
			activeErrorBoundary: undefined,
			importURLs: ["/routes/root.tsx"],
			exportKeys: ["Route"],
		});
		getRouterDataMock.mockReturnValue({
			buildID: "1",
			matchedPatterns: ["/"],
			splatValues: [],
			params: {},
			rootData: { root: true },
		});
		getLocationMock.mockReturnValue({
			pathname: "/",
			search: "",
			hash: "",
			state: previousStoreState.location.state,
		});

		const nextStoreState =
			buildNextRouteOutletStoreStateFromRuntime(previousStoreState);

		expect(nextStoreState).toBe(previousStoreState);
	});

	it("updates navigation and branch snapshots when runtime navigation changes", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();
		const nextLocationState = previousStoreState.location.state;

		getClientRuntimeRenderStateMock.mockReturnValue({
			loadersData: [{ root: true }, { child: true }],
			clientLoadersData: [{ client: true }],
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: ["RootComponent", "ChildComponent"],
			activeErrorBoundary: undefined,
			importURLs: ["/routes/root.tsx", "/routes/child.tsx"],
			exportKeys: ["Route", "Route"],
		});
		getRouterDataMock.mockReturnValue({
			buildID: "1",
			matchedPatterns: ["/", "/child"],
			splatValues: [],
			params: {},
			rootData: { root: true },
		});
		getLocationMock.mockReturnValue({
			pathname: "/",
			search: "",
			hash: "",
			state: nextLocationState,
		});

		const nextStoreState =
			buildNextRouteOutletStoreStateFromRuntime(previousStoreState);

		expect(nextStoreState).not.toBe(previousStoreState);
		expect(nextStoreState.navigation).not.toBe(
			previousStoreState.navigation,
		);
		expect(nextStoreState.routeOutletBranchInputState).not.toBe(
			previousStoreState.routeOutletBranchInputState,
		);
		expect(nextStoreState.routeOutletBranchInputState.loaderCount).toBe(2);
		expect(nextStoreState.location).toBe(previousStoreState.location);
	});

	it("updates location snapshot when pathname/search/hash/state changes", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();
		const nextLocationState = { from: "changed" };

		getClientRuntimeRenderStateMock.mockReturnValue({
			loadersData: [{ root: true }],
			clientLoadersData: [{ client: true }],
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: ["RootComponent"],
			activeErrorBoundary: undefined,
			importURLs: ["/routes/root.tsx"],
			exportKeys: ["Route"],
		});
		getRouterDataMock.mockReturnValue({
			buildID: "1",
			matchedPatterns: ["/"],
			splatValues: [],
			params: {},
			rootData: { root: true },
		});
		getLocationMock.mockReturnValue({
			pathname: "/docs",
			search: "?tab=usage",
			hash: "#section",
			state: nextLocationState,
		});

		const nextStoreState =
			buildNextRouteOutletStoreStateFromRuntime(previousStoreState);

		expect(nextStoreState.location).not.toBe(previousStoreState.location);
		expect(nextStoreState.location).toEqual({
			pathname: "/docs",
			search: "?tab=usage",
			hash: "#section",
			state: nextLocationState,
		});
		expect(nextStoreState.navigation).toBe(previousStoreState.navigation);
		expect(nextStoreState.routeOutletBranchInputState).toBe(
			previousStoreState.routeOutletBranchInputState,
		);
	});
});
