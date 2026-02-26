import { jsonDeepEquals } from "vorma/kit/json";
import {
	getClientRuntimeRenderState,
	getRouterData,
	type ClientRuntimeRenderState,
} from "../app/context.ts";
import { getRuntimeLocationState } from "../platform/location.ts";
import { syncTypedAdapterRouteInstanceStoreFromNavigationState } from "./typed_adapter_helpers_runtime.ts";

type RouteOutletRouterDataState = ReturnType<typeof getRouterData>;

type RouteOutletRuntimeRenderState = Pick<
	ClientRuntimeRenderState,
	| "loadersData"
	| "clientLoadersData"
	| "outermostError"
	| "outermostErrorIdx"
	| "activeComponents"
	| "activeErrorBoundary"
	| "importURLs"
	| "exportKeys"
>;

export type RouteOutletNavigationState = {
	loadersData: RouteOutletRuntimeRenderState["loadersData"];
	clientLoadersData: RouteOutletRuntimeRenderState["clientLoadersData"];
	routerData: RouteOutletRouterDataState;
	outermostError: RouteOutletRuntimeRenderState["outermostError"];
	outermostErrorIdx: RouteOutletRuntimeRenderState["outermostErrorIdx"];
	activeComponents: RouteOutletRuntimeRenderState["activeComponents"];
	activeErrorBoundary: RouteOutletRuntimeRenderState["activeErrorBoundary"];
	importURLs: RouteOutletRuntimeRenderState["importURLs"];
	exportKeys: RouteOutletRuntimeRenderState["exportKeys"];
};

export type RouteOutletLocationState = ReturnType<
	typeof getRuntimeLocationState
>;

export type RouteOutletBranchState = {
	currentRouteKey: string;
	nextRouteKey: string;
	isErrorIdx: boolean;
	currentComponent: unknown;
	errorComponent: unknown;
	shouldFallbackOutlet: boolean;
};

type RouteOutletBranchRouteKeys = Pick<
	RouteOutletBranchState,
	"currentRouteKey" | "nextRouteKey"
>;

export type RouteOutletBranchRenderState =
	| (RouteOutletBranchRouteKeys & {
			renderKind: "error";
			errorComponent: unknown;
	  })
	| (RouteOutletBranchRouteKeys & {
			renderKind: "component";
			currentComponent: unknown;
	  })
	| (RouteOutletBranchRouteKeys & {
			renderKind: "fallback";
	  })
	| (RouteOutletBranchRouteKeys & {
			renderKind: "empty";
	  });

export type RouteOutletBranchInputState = Pick<
	RouteOutletNavigationState,
	| "outermostErrorIdx"
	| "activeComponents"
	| "activeErrorBoundary"
	| "importURLs"
	| "exportKeys"
> & {
	loaderCount: number;
	matchedPatterns: readonly string[];
};

export type RouteOutletStoreState = {
	navigation: RouteOutletNavigationState;
	routeOutletBranchInputState: RouteOutletBranchInputState;
	location: RouteOutletLocationState;
};

function canonicalizeWithEqualityCheck<T>(props: {
	previousValue: T;
	nextValue: T;
	isEqual: (firstValue: T, secondValue: T) => boolean;
}): T {
	const { previousValue, nextValue, isEqual } = props;
	if (isEqual(previousValue, nextValue)) {
		return previousValue;
	}
	return nextValue;
}

function areRouteOutletNavigationStatesReferenceEqual(props: {
	previousNavigationState: RouteOutletNavigationState;
	nextNavigationState: RouteOutletNavigationState;
}): boolean {
	const { previousNavigationState, nextNavigationState } = props;
	return (
		nextNavigationState.loadersData ===
			previousNavigationState.loadersData &&
		nextNavigationState.clientLoadersData ===
			previousNavigationState.clientLoadersData &&
		nextNavigationState.routerData === previousNavigationState.routerData &&
		nextNavigationState.outermostError ===
			previousNavigationState.outermostError &&
		nextNavigationState.outermostErrorIdx ===
			previousNavigationState.outermostErrorIdx &&
		nextNavigationState.activeComponents ===
			previousNavigationState.activeComponents &&
		nextNavigationState.activeErrorBoundary ===
			previousNavigationState.activeErrorBoundary &&
		nextNavigationState.importURLs === previousNavigationState.importURLs &&
		nextNavigationState.exportKeys === previousNavigationState.exportKeys
	);
}

export function buildCurrentRouteOutletNavigationState(): RouteOutletNavigationState {
	const runtimeRenderState = getClientRuntimeRenderState();
	return {
		loadersData: runtimeRenderState.loadersData,
		clientLoadersData: runtimeRenderState.clientLoadersData,
		routerData: getRouterData(),
		outermostError: runtimeRenderState.outermostError,
		outermostErrorIdx: runtimeRenderState.outermostErrorIdx,
		activeComponents: runtimeRenderState.activeComponents,
		activeErrorBoundary: runtimeRenderState.activeErrorBoundary,
		importURLs: runtimeRenderState.importURLs,
		exportKeys: runtimeRenderState.exportKeys,
	};
}

export function buildInitialRouteOutletNavigationState(): RouteOutletNavigationState {
	return buildCurrentRouteOutletNavigationState();
}

export function buildNextRouteOutletNavigationState(
	previousNavigationState: RouteOutletNavigationState,
): RouteOutletNavigationState {
	const nextNavigationStateRaw = buildCurrentRouteOutletNavigationState();

	const nextNavigationState: RouteOutletNavigationState = {
		loadersData: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.loadersData,
			nextValue: nextNavigationStateRaw.loadersData,
			isEqual: jsonDeepEquals,
		}),
		clientLoadersData: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.clientLoadersData,
			nextValue: nextNavigationStateRaw.clientLoadersData,
			isEqual: jsonDeepEquals,
		}),
		routerData: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.routerData,
			nextValue: nextNavigationStateRaw.routerData,
			isEqual: jsonDeepEquals,
		}),
		outermostError: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.outermostError,
			nextValue: nextNavigationStateRaw.outermostError,
			isEqual: Object.is,
		}),
		outermostErrorIdx: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.outermostErrorIdx,
			nextValue: nextNavigationStateRaw.outermostErrorIdx,
			isEqual: Object.is,
		}),
		activeComponents: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.activeComponents,
			nextValue: nextNavigationStateRaw.activeComponents,
			isEqual: jsonDeepEquals,
		}),
		activeErrorBoundary: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.activeErrorBoundary,
			nextValue: nextNavigationStateRaw.activeErrorBoundary,
			isEqual: Object.is,
		}),
		importURLs: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.importURLs,
			nextValue: nextNavigationStateRaw.importURLs,
			isEqual: jsonDeepEquals,
		}),
		exportKeys: canonicalizeWithEqualityCheck({
			previousValue: previousNavigationState.exportKeys,
			nextValue: nextNavigationStateRaw.exportKeys,
			isEqual: jsonDeepEquals,
		}),
	};

	if (
		areRouteOutletNavigationStatesReferenceEqual({
			previousNavigationState,
			nextNavigationState,
		})
	) {
		return previousNavigationState;
	}

	return nextNavigationState;
}

export function areRouteOutletLocationsEqual(props: {
	firstLocationState: RouteOutletLocationState;
	secondLocationState: RouteOutletLocationState;
}): boolean {
	const { firstLocationState, secondLocationState } = props;
	return (
		Object.is(firstLocationState.pathname, secondLocationState.pathname) &&
		Object.is(firstLocationState.search, secondLocationState.search) &&
		Object.is(firstLocationState.hash, secondLocationState.hash) &&
		Object.is(firstLocationState.state, secondLocationState.state)
	);
}

export function buildCurrentRouteOutletLocationState(): RouteOutletLocationState {
	return getRuntimeLocationState();
}

export function buildRouteOutletRouteKey(props: {
	importURLs: ReadonlyArray<string> | null | undefined;
	exportKeys: ReadonlyArray<string> | null | undefined;
	matchedPatterns: ReadonlyArray<string> | null | undefined;
	idx: number;
}): string {
	const importURL = props.importURLs?.[props.idx] || "";
	const exportKey = props.exportKeys?.[props.idx] || "";
	const matchedPattern = props.matchedPatterns?.[props.idx] || "";
	return JSON.stringify([props.idx, importURL, exportKey, matchedPattern]);
}

export function buildRouteOutletBranchInputState(
	navigationState: RouteOutletNavigationState,
): RouteOutletBranchInputState {
	return {
		loaderCount: navigationState.loadersData?.length ?? 0,
		outermostErrorIdx: navigationState.outermostErrorIdx,
		activeComponents: navigationState.activeComponents,
		activeErrorBoundary: navigationState.activeErrorBoundary,
		importURLs: navigationState.importURLs,
		exportKeys: navigationState.exportKeys,
		matchedPatterns: navigationState.routerData.matchedPatterns,
	};
}

export function areRouteOutletBranchInputsEqualByIdentity(props: {
	firstInputState: RouteOutletBranchInputState;
	secondInputState: RouteOutletBranchInputState;
}): boolean {
	const { firstInputState, secondInputState } = props;
	return (
		firstInputState.loaderCount === secondInputState.loaderCount &&
		Object.is(
			firstInputState.outermostErrorIdx,
			secondInputState.outermostErrorIdx,
		) &&
		firstInputState.activeComponents ===
			secondInputState.activeComponents &&
		Object.is(
			firstInputState.activeErrorBoundary,
			secondInputState.activeErrorBoundary,
		) &&
		firstInputState.importURLs === secondInputState.importURLs &&
		firstInputState.exportKeys === secondInputState.exportKeys &&
		jsonDeepEquals(
			firstInputState.matchedPatterns,
			secondInputState.matchedPatterns,
		)
	);
}

export function buildInitialRouteOutletStoreState(): RouteOutletStoreState {
	const initialNavigationState = buildInitialRouteOutletNavigationState();
	return {
		navigation: initialNavigationState,
		routeOutletBranchInputState: buildRouteOutletBranchInputState(
			initialNavigationState,
		),
		location: buildCurrentRouteOutletLocationState(),
	};
}

function resolveNextRouteOutletBranchInputState(props: {
	previousStoreState: RouteOutletStoreState;
	nextNavigationState: RouteOutletNavigationState;
}): RouteOutletBranchInputState {
	const { previousStoreState, nextNavigationState } = props;
	const previousNavigationState = previousStoreState.navigation;
	const previousRouteOutletBranchInputState =
		previousStoreState.routeOutletBranchInputState;

	if (nextNavigationState === previousNavigationState) {
		return previousRouteOutletBranchInputState;
	}

	const nextRouteOutletBranchInputState =
		buildRouteOutletBranchInputState(nextNavigationState);
	if (
		areRouteOutletBranchInputsEqualByIdentity({
			firstInputState: previousRouteOutletBranchInputState,
			secondInputState: nextRouteOutletBranchInputState,
		})
	) {
		return previousRouteOutletBranchInputState;
	}

	return nextRouteOutletBranchInputState;
}

function resolveNextRouteOutletLocationState(props: {
	previousLocationState: RouteOutletLocationState;
}): RouteOutletLocationState {
	const { previousLocationState } = props;
	const nextLocationState = buildCurrentRouteOutletLocationState();
	if (
		areRouteOutletLocationsEqual({
			firstLocationState: previousLocationState,
			secondLocationState: nextLocationState,
		})
	) {
		return previousLocationState;
	}

	return nextLocationState;
}

export function buildNextRouteOutletStoreStateFromRuntime(
	previousStoreState: RouteOutletStoreState,
): RouteOutletStoreState {
	const nextNavigationState = buildNextRouteOutletNavigationState(
		previousStoreState.navigation,
	);

	const nextRouteOutletBranchInputState =
		resolveNextRouteOutletBranchInputState({
			previousStoreState,
			nextNavigationState,
		});
	const nextLocationState = resolveNextRouteOutletLocationState({
		previousLocationState: previousStoreState.location,
	});

	if (
		nextNavigationState === previousStoreState.navigation &&
		nextRouteOutletBranchInputState ===
			previousStoreState.routeOutletBranchInputState &&
		nextLocationState === previousStoreState.location
	) {
		return previousStoreState;
	}

	return {
		navigation: nextNavigationState,
		routeOutletBranchInputState: nextRouteOutletBranchInputState,
		location: nextLocationState,
	};
}

export function syncRouteOutletStoreStateFromRuntime(props: {
	getCurrentStoreState: () => RouteOutletStoreState;
	applyNextStoreState: (nextStoreState: RouteOutletStoreState) => void;
}): RouteOutletStoreState {
	const previousStoreState = props.getCurrentStoreState();
	const nextStoreState =
		buildNextRouteOutletStoreStateFromRuntime(previousStoreState);
	syncTypedAdapterRouteInstanceStoreFromNavigationState({
		matchedPatterns: nextStoreState.navigation.routerData.matchedPatterns,
		loadersData: nextStoreState.navigation.loadersData,
		clientLoadersData: nextStoreState.navigation.clientLoadersData,
		importURLs: nextStoreState.navigation.importURLs ?? [],
		exportKeys: nextStoreState.navigation.exportKeys ?? [],
	});
	if (nextStoreState !== previousStoreState) {
		props.applyNextStoreState(nextStoreState);
	}
	return nextStoreState;
}

export function buildRouteOutletBranchState(props: {
	navigationState: RouteOutletBranchInputState;
	idx: number;
}): RouteOutletBranchState {
	const { navigationState, idx } = props;
	const { importURLs, exportKeys } = navigationState;
	const isErrorIdx = idx === navigationState.outermostErrorIdx;
	const currentComponent = isErrorIdx
		? undefined
		: navigationState.activeComponents?.[idx];
	const errorComponent = isErrorIdx
		? navigationState.activeErrorBoundary
		: undefined;
	const loaderCount = navigationState.loaderCount;
	const shouldFallbackOutlet =
		!isErrorIdx && !currentComponent && idx + 1 < loaderCount;

	return {
		currentRouteKey: buildRouteOutletRouteKey({
			importURLs,
			exportKeys,
			matchedPatterns: navigationState.matchedPatterns,
			idx,
		}),
		nextRouteKey: buildRouteOutletRouteKey({
			importURLs,
			exportKeys,
			matchedPatterns: navigationState.matchedPatterns,
			idx: idx + 1,
		}),
		isErrorIdx,
		currentComponent,
		errorComponent,
		shouldFallbackOutlet,
	};
}

export function resolveRouteOutletBranchRenderState(props: {
	branchState: RouteOutletBranchState;
}): RouteOutletBranchRenderState {
	const { branchState } = props;
	const routeKeys: RouteOutletBranchRouteKeys = {
		currentRouteKey: branchState.currentRouteKey,
		nextRouteKey: branchState.nextRouteKey,
	};

	if (branchState.isErrorIdx) {
		return {
			renderKind: "error",
			errorComponent: branchState.errorComponent,
			...routeKeys,
		};
	}

	if (branchState.currentComponent) {
		return {
			renderKind: "component",
			currentComponent: branchState.currentComponent,
			...routeKeys,
		};
	}

	if (branchState.shouldFallbackOutlet) {
		return {
			renderKind: "fallback",
			...routeKeys,
		};
	}

	return {
		renderKind: "empty",
		...routeKeys,
	};
}

export function shouldRemountRouteOutletComponentMount(props: {
	idx: number;
	previousRouteKey: string | undefined;
	nextRouteKey: string;
	previousRouteComponent: unknown;
	nextRouteComponent: unknown;
}): boolean {
	const {
		idx,
		previousRouteKey,
		nextRouteKey,
		previousRouteComponent,
		nextRouteComponent,
	} = props;
	void idx;
	const didRouteComponentIdentityChange =
		previousRouteComponent !== undefined &&
		previousRouteComponent !== nextRouteComponent;
	if (didRouteComponentIdentityChange) {
		return true;
	}

	const didRouteKeyChange =
		typeof previousRouteKey === "string" &&
		previousRouteKey !== nextRouteKey;
	return didRouteKeyChange;
}
