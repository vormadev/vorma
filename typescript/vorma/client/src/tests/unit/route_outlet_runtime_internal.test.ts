import { beforeEach, describe, expect, it, vi } from "vitest";

const {
	getClientRuntimeRenderStateMock,
	getRouterDataMock,
	getRuntimeLocationStateMock,
} = vi.hoisted(() => {
	return {
		getClientRuntimeRenderStateMock: vi.fn(),
		getRouterDataMock: vi.fn(),
		getRuntimeLocationStateMock: vi.fn(),
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

vi.mock("../../platform/location.ts", async (importOriginal) => {
	const actual =
		await importOriginal<typeof import("../../platform/location.ts")>();
	return {
		...actual,
		getRuntimeLocationState: getRuntimeLocationStateMock,
	};
});

import {
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchState,
	buildRouteOutletRouteKey,
	resolveRouteOutletBranchRenderState,
	shouldRemountRouteOutletComponentMount,
	syncRouteOutletStoreStateFromRuntime,
} from "../../ui/route_outlet_runtime.ts";

beforeEach(() => {
	getClientRuntimeRenderStateMock.mockReset();
	getRouterDataMock.mockReset();
	getRuntimeLocationStateMock.mockReset();

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
	getRuntimeLocationStateMock.mockReturnValue({
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

	it("resolves branch render state for component, fallback, and error branches", () => {
		const componentBranchState = buildRouteOutletBranchState({
			navigationState: {
				loaderCount: 2,
				outermostErrorIdx: undefined,
				activeComponents: [() => "A", () => "B"],
				activeErrorBoundary: undefined,
				importURLs: ["/routes/a.tsx", "/routes/b.tsx"],
				exportKeys: ["Route", "Route"],
			},
			idx: 0,
		});
		expect(
			resolveRouteOutletBranchRenderState({
				branchState: componentBranchState,
			}),
		).toMatchObject({
			renderKind: "component",
			currentRouteKey: componentBranchState.currentRouteKey,
			nextRouteKey: componentBranchState.nextRouteKey,
		});

		const fallbackBranchState = buildRouteOutletBranchState({
			navigationState: {
				loaderCount: 2,
				outermostErrorIdx: undefined,
				activeComponents: [undefined, () => "Child"],
				activeErrorBoundary: undefined,
				importURLs: ["/routes/a.tsx", "/routes/b.tsx"],
				exportKeys: ["Route", "Route"],
			},
			idx: 0,
		});
		expect(
			resolveRouteOutletBranchRenderState({
				branchState: fallbackBranchState,
			}),
		).toEqual({
			renderKind: "fallback",
			currentRouteKey: fallbackBranchState.currentRouteKey,
			nextRouteKey: fallbackBranchState.nextRouteKey,
		});

		const errorBoundary = () => "ErrorBoundary";
		const errorBranchState = buildRouteOutletBranchState({
			navigationState: {
				loaderCount: 1,
				outermostErrorIdx: 0,
				activeComponents: [() => "Root"],
				activeErrorBoundary: errorBoundary,
				importURLs: ["/routes/root.tsx"],
				exportKeys: ["Route"],
			},
			idx: 0,
		});
		expect(
			resolveRouteOutletBranchRenderState({
				branchState: errorBranchState,
			}),
		).toEqual({
			renderKind: "error",
			errorComponent: errorBoundary,
			currentRouteKey: errorBranchState.currentRouteKey,
			nextRouteKey: errorBranchState.nextRouteKey,
		});
	});

	it("remount policy remounts on component identity change and root key changes only", () => {
		const previousComponent = () => "A";
		const nextComponent = () => "B";

		expect(
			shouldRemountRouteOutletComponentMount({
				idx: 1,
				previousRouteKey: "key-a",
				nextRouteKey: "key-a",
				previousRouteComponent: previousComponent,
				nextRouteComponent: nextComponent,
			}),
		).toBe(true);

		expect(
			shouldRemountRouteOutletComponentMount({
				idx: 0,
				previousRouteKey: "key-a",
				nextRouteKey: "key-b",
				previousRouteComponent: previousComponent,
				nextRouteComponent: previousComponent,
			}),
		).toBe(true);

		expect(
			shouldRemountRouteOutletComponentMount({
				idx: 1,
				previousRouteKey: "key-a",
				nextRouteKey: "key-b",
				previousRouteComponent: previousComponent,
				nextRouteComponent: previousComponent,
			}),
		).toBe(false);
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
		getRuntimeLocationStateMock.mockReturnValue({
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
		getRuntimeLocationStateMock.mockReturnValue({
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

	it("reuses previous branch snapshot when navigation changes only in non-branch fields", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();
		const nextLocationState = previousStoreState.location.state;

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
			buildID: "2",
			matchedPatterns: ["/"],
			splatValues: [],
			params: {},
			rootData: { root: true },
		});
		getRuntimeLocationStateMock.mockReturnValue({
			pathname: "/",
			search: "",
			hash: "",
			state: nextLocationState,
		});

		const nextStoreState =
			buildNextRouteOutletStoreStateFromRuntime(previousStoreState);

		expect(nextStoreState.navigation).not.toBe(
			previousStoreState.navigation,
		);
		expect(nextStoreState.routeOutletBranchInputState).toBe(
			previousStoreState.routeOutletBranchInputState,
		);
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
		getRuntimeLocationStateMock.mockReturnValue({
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

	it("sync helper skips apply callback when runtime store snapshot is unchanged", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();
		const applyNextStoreState = vi.fn();

		const syncedStoreState = syncRouteOutletStoreStateFromRuntime({
			getCurrentStoreState: () => previousStoreState,
			applyNextStoreState,
		});

		expect(syncedStoreState).toBe(previousStoreState);
		expect(applyNextStoreState).not.toHaveBeenCalled();
	});

	it("sync helper applies next snapshot when runtime store snapshot changes", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();
		const applyNextStoreState = vi.fn();

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
		getRuntimeLocationStateMock.mockReturnValue({
			pathname: "/",
			search: "",
			hash: "",
			state: previousStoreState.location.state,
		});

		const syncedStoreState = syncRouteOutletStoreStateFromRuntime({
			getCurrentStoreState: () => previousStoreState,
			applyNextStoreState,
		});

		expect(syncedStoreState).not.toBe(previousStoreState);
		expect(applyNextStoreState).toHaveBeenCalledTimes(1);
		expect(applyNextStoreState).toHaveBeenCalledWith(syncedStoreState);
	});
});
