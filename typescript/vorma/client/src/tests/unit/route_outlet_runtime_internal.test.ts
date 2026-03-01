import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchState,
	buildRouteOutletRouteKey,
	resolveRouteOutletBranchRenderState,
	setRuntimeRouteSnapshot,
	shouldRemountRouteOutletComponentMount,
	syncRouteOutletStoreStateFromRuntime,
	VORMA_SYMBOL,
} from "../../runtime.ts";

function installRouteOutletRuntimeGlobalState() {
	(globalThis as any)[VORMA_SYMBOL] = {
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: {
			actionsRouterMountRoot: "/api/",
			actionsDynamicRune: ":",
			actionsSplatRune: "*",
			loadersDynamicRune: ":",
			loadersSplatRune: "*",
			loadersExplicitIndexSegmentIdentifier: "_index",
		},
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry: undefined,
		runtimeRouteSnapshot: {
			buildID: "1",
			matchedPatterns: ["/"],
			loadersData: [{ root: true }],
			importURLs: ["/routes/root.tsx"],
			exportKeys: ["Route"],
			errorExportKeys: [""],
			hasRootData: true,
			params: {},
			splatValues: [],
			activeComponents: ["RootComponent"],
			activeErrorBoundary: undefined,
			rootElementID: undefined,
			outermostServerError: undefined,
			outermostClientError: undefined,
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			clientLoadersData: [{ client: true }],
		},
	};
}

beforeEach(() => {
	installRouteOutletRuntimeGlobalState();
	window.history.replaceState({ from: "default" }, "", "/");
});

describe("route outlet runtime internals", () => {
	it("builds distinct route keys when import and export parts contain delimiters", () => {
		const firstKey = buildRouteOutletRouteKey({
			importURLs: ["/routes/a|b.tsx"],
			exportKeys: ["Route"],
			matchedPatterns: ["/a"],
			idx: 0,
		});
		const secondKey = buildRouteOutletRouteKey({
			importURLs: ["/routes/a.tsx"],
			exportKeys: ["b|Route"],
			matchedPatterns: ["/a"],
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
				matchedPatterns: ["/a", "/b"],
			},
			idx: 0,
		});

		expect(branchState.currentRouteKey).not.toBe(branchState.nextRouteKey);
		expect(branchState.currentRouteKey).toBe(
			JSON.stringify([0, "/routes/a|b.tsx", "Route", "/a"]),
		);
		expect(branchState.nextRouteKey).toBe(
			JSON.stringify([1, "/routes/a.tsx", "b|Route", "/b"]),
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
				matchedPatterns: [],
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
				matchedPatterns: ["/a", "/b"],
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
				matchedPatterns: ["/a", "/b"],
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
				matchedPatterns: ["/"],
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

	it("remount policy remounts on component identity change and any key change", () => {
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
		).toBe(true);
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
			matchedPatterns: ["/a"],
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
					matchedPatterns: ["/a"],
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
					matchedPatterns: ["/a"],
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
			matchedPatterns: ["/"],
		});
		expect(initialState.location).toEqual({
			pathname: "/",
			search: "",
			hash: "",
			state: null,
		});
	});

	it("reuses previous store state when runtime snapshots are equivalent", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();

		const nextStoreState =
			buildNextRouteOutletStoreStateFromRuntime(previousStoreState);

		expect(nextStoreState).toBe(previousStoreState);
	});

	it("updates navigation and branch snapshots when runtime navigation changes", () => {
		const previousStoreState = buildInitialRouteOutletStoreState();
		setRuntimeRouteSnapshot({
			...(globalThis as any)[VORMA_SYMBOL].runtimeRouteSnapshot,
			loadersData: [{ root: true }, { child: true }],
			clientLoadersData: [{ client: true }],
			activeComponents: ["RootComponent", "ChildComponent"],
			importURLs: ["/routes/root.tsx", "/routes/child.tsx"],
			exportKeys: ["Route", "Route"],
			matchedPatterns: ["/", "/child"],
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
		setRuntimeRouteSnapshot({
			...(globalThis as any)[VORMA_SYMBOL].runtimeRouteSnapshot,
			buildID: "2",
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
		window.history.replaceState(
			nextLocationState,
			"",
			"/docs?tab=usage#section",
		);

		const nextStoreState =
			buildNextRouteOutletStoreStateFromRuntime(previousStoreState);

		expect(nextStoreState.location).not.toBe(previousStoreState.location);
		expect(nextStoreState.location).toEqual({
			pathname: "/docs",
			search: "?tab=usage",
			hash: "#section",
			state: null,
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
		setRuntimeRouteSnapshot({
			...(globalThis as any)[VORMA_SYMBOL].runtimeRouteSnapshot,
			loadersData: [{ root: true }, { child: true }],
			clientLoadersData: [{ client: true }],
			activeComponents: ["RootComponent", "ChildComponent"],
			importURLs: ["/routes/root.tsx", "/routes/child.tsx"],
			exportKeys: ["Route", "Route"],
			matchedPatterns: ["/", "/child"],
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
