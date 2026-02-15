import { jsonDeepEquals } from "vorma/kit/json";
import {
	getClientRuntimeRenderState,
	getRouterData,
	type ClientRuntimeRenderState,
} from "../app/context.ts";
import { getLocation } from "../client.ts";

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

export type RouteOutletLocationState = ReturnType<typeof getLocation>;

export type RouteOutletBranchState = {
	currentRouteKey: string;
	nextRouteKey: string;
	isErrorIdx: boolean;
	currentComponent: unknown;
	errorComponent: unknown;
	shouldFallbackOutlet: boolean;
};

export type RouteOutletBranchInputState = Pick<
	RouteOutletNavigationState,
	| "outermostErrorIdx"
	| "activeComponents"
	| "activeErrorBoundary"
	| "importURLs"
	| "exportKeys"
> & {
	loaderCount: number;
};

function canonicalizeWithJsonDeepEquals<T>(props: {
	previousValue: T;
	nextValue: T;
}): T {
	const { previousValue, nextValue } = props;
	if (jsonDeepEquals(previousValue, nextValue)) {
		return previousValue;
	}
	return nextValue;
}

function canonicalizeWithObjectIdentity<T>(props: {
	previousValue: T;
	nextValue: T;
}): T {
	const { previousValue, nextValue } = props;
	if (Object.is(previousValue, nextValue)) {
		return previousValue;
	}
	return nextValue;
}

function buildCurrentRouteOutletNavigationStateFromRuntime(): RouteOutletNavigationState {
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
	return buildCurrentRouteOutletNavigationStateFromRuntime();
}

export function buildNextRouteOutletNavigationState(
	previousNavigationState: RouteOutletNavigationState,
): RouteOutletNavigationState {
	const nextNavigationStateRaw =
		buildCurrentRouteOutletNavigationStateFromRuntime();

	const nextNavigationState: RouteOutletNavigationState = {
		loadersData: canonicalizeWithJsonDeepEquals({
			previousValue: previousNavigationState.loadersData,
			nextValue: nextNavigationStateRaw.loadersData,
		}),
		clientLoadersData: canonicalizeWithJsonDeepEquals({
			previousValue: previousNavigationState.clientLoadersData,
			nextValue: nextNavigationStateRaw.clientLoadersData,
		}),
		routerData: canonicalizeWithJsonDeepEquals({
			previousValue: previousNavigationState.routerData,
			nextValue: nextNavigationStateRaw.routerData,
		}),
		outermostError: canonicalizeWithObjectIdentity({
			previousValue: previousNavigationState.outermostError,
			nextValue: nextNavigationStateRaw.outermostError,
		}),
		outermostErrorIdx: canonicalizeWithObjectIdentity({
			previousValue: previousNavigationState.outermostErrorIdx,
			nextValue: nextNavigationStateRaw.outermostErrorIdx,
		}),
		activeComponents: canonicalizeWithJsonDeepEquals({
			previousValue: previousNavigationState.activeComponents,
			nextValue: nextNavigationStateRaw.activeComponents,
		}),
		activeErrorBoundary: canonicalizeWithObjectIdentity({
			previousValue: previousNavigationState.activeErrorBoundary,
			nextValue: nextNavigationStateRaw.activeErrorBoundary,
		}),
		importURLs: canonicalizeWithJsonDeepEquals({
			previousValue: previousNavigationState.importURLs,
			nextValue: nextNavigationStateRaw.importURLs,
		}),
		exportKeys: canonicalizeWithJsonDeepEquals({
			previousValue: previousNavigationState.exportKeys,
			nextValue: nextNavigationStateRaw.exportKeys,
		}),
	};

	if (
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
	) {
		return previousNavigationState;
	}

	return nextNavigationState;
}

export function buildCurrentRouteOutletLocationState(): RouteOutletLocationState {
	return getLocation();
}

export function areRouteOutletLocationsEqual(
	firstLocationState: RouteOutletLocationState,
	secondLocationState: RouteOutletLocationState,
): boolean {
	return (
		Object.is(firstLocationState.pathname, secondLocationState.pathname) &&
		Object.is(firstLocationState.search, secondLocationState.search) &&
		Object.is(firstLocationState.hash, secondLocationState.hash) &&
		Object.is(firstLocationState.state, secondLocationState.state)
	);
}

export function buildRouteOutletRouteKey(props: {
	importURLs: ReadonlyArray<string> | null | undefined;
	exportKeys: ReadonlyArray<string> | null | undefined;
	idx: number;
}): string {
	const importURL = props.importURLs?.[props.idx] || "";
	const exportKey = props.exportKeys?.[props.idx] || "";
	return `${importURL}|${exportKey}`;
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
	};
}

export function areRouteOutletBranchInputsEqualByIdentity(
	firstInputState: RouteOutletBranchInputState,
	secondInputState: RouteOutletBranchInputState,
): boolean {
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
		firstInputState.exportKeys === secondInputState.exportKeys
	);
}

export function buildRouteOutletBranchState(props: {
	navigationState: RouteOutletBranchInputState;
	idx: number;
}): RouteOutletBranchState {
	const { navigationState, idx } = props;
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
			importURLs: navigationState.importURLs,
			exportKeys: navigationState.exportKeys,
			idx,
		}),
		nextRouteKey: buildRouteOutletRouteKey({
			importURLs: navigationState.importURLs,
			exportKeys: navigationState.exportKeys,
			idx: idx + 1,
		}),
		isErrorIdx,
		currentComponent,
		errorComponent,
		shouldFallbackOutlet,
	};
}
