import { batch, computed, signal } from "@preact/signals";
import { h, type ComponentType } from "preact";
import { useLayoutEffect, useMemo, useRef } from "preact/hooks";
import { addLocationListener, addRouteChangeListener } from "vorma/client";
import {
	applyScrollState,
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildCurrentRouteOutletLocationState,
	buildInitialRouteOutletNavigationState,
	buildNextRouteOutletNavigationState,
	buildRouteOutletBranchInputState,
	buildRouteOutletBranchState,
	type RouteOutletBranchInputState,
	type RouteOutletNavigationState,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

type NavigationState = RouteOutletNavigationState;
type RouteOutletBranchInputStateValue = RouteOutletBranchInputState;

const initialNavigationState = buildInitialRouteOutletNavigationState();

const loadersData = signal(initialNavigationState.loadersData);
const clientLoadersData = signal(initialNavigationState.clientLoadersData);
const routerData = signal(initialNavigationState.routerData);
const outermostError = signal(initialNavigationState.outermostError);
const outermostErrorIdx = signal(initialNavigationState.outermostErrorIdx);
const activeComponents = signal(initialNavigationState.activeComponents);
const activeErrorBoundary = signal(initialNavigationState.activeErrorBoundary);
const importURLs = signal(initialNavigationState.importURLs);
const exportKeys = signal(initialNavigationState.exportKeys);
const routeOutletBranchInputState = signal<RouteOutletBranchInputStateValue>(
	buildRouteOutletBranchInputState(initialNavigationState),
);

export { clientLoadersData, loadersData, routerData };

const locationState = signal(buildCurrentRouteOutletLocationState());
export const location = computed(() => locationState.value);

function readNavigationSignals(): NavigationState {
	return {
		loadersData: loadersData.value,
		clientLoadersData: clientLoadersData.value,
		routerData: routerData.value,
		outermostError: outermostError.value,
		outermostErrorIdx: outermostErrorIdx.value,
		activeComponents: activeComponents.value,
		activeErrorBoundary: activeErrorBoundary.value,
		importURLs: importURLs.value,
		exportKeys: exportKeys.value,
	};
}

function syncNavigationSignals(): void {
	const previousNavigationState = readNavigationSignals();
	const nextNavigationState = buildNextRouteOutletNavigationState(
		previousNavigationState,
	);
	if (nextNavigationState === previousNavigationState) {
		return;
	}
	const previousRouteOutletBranchInputState =
		routeOutletBranchInputState.value;
	const nextRouteOutletBranchInputStateRaw =
		buildRouteOutletBranchInputState(nextNavigationState);
	const nextRouteOutletBranchInputState =
		areRouteOutletBranchInputsEqualByIdentity({
			firstInputState: previousRouteOutletBranchInputState,
			secondInputState: nextRouteOutletBranchInputStateRaw,
		})
			? previousRouteOutletBranchInputState
			: nextRouteOutletBranchInputStateRaw;

	batch(() => {
		if (
			nextNavigationState.loadersData !==
			previousNavigationState.loadersData
		) {
			loadersData.value = nextNavigationState.loadersData;
		}
		if (
			nextNavigationState.clientLoadersData !==
			previousNavigationState.clientLoadersData
		) {
			clientLoadersData.value = nextNavigationState.clientLoadersData;
		}
		if (
			nextNavigationState.routerData !==
			previousNavigationState.routerData
		) {
			routerData.value = nextNavigationState.routerData;
		}
		if (
			!Object.is(
				nextNavigationState.outermostError,
				previousNavigationState.outermostError,
			)
		) {
			outermostError.value = nextNavigationState.outermostError;
		}
		if (
			!Object.is(
				nextNavigationState.outermostErrorIdx,
				previousNavigationState.outermostErrorIdx,
			)
		) {
			outermostErrorIdx.value = nextNavigationState.outermostErrorIdx;
		}
		if (
			nextNavigationState.activeComponents !==
			previousNavigationState.activeComponents
		) {
			activeComponents.value = nextNavigationState.activeComponents;
		}
		if (
			!Object.is(
				nextNavigationState.activeErrorBoundary,
				previousNavigationState.activeErrorBoundary,
			)
		) {
			activeErrorBoundary.value = nextNavigationState.activeErrorBoundary;
		}
		if (
			nextNavigationState.importURLs !==
			previousNavigationState.importURLs
		) {
			importURLs.value = nextNavigationState.importURLs;
		}
		if (
			nextNavigationState.exportKeys !==
			previousNavigationState.exportKeys
		) {
			exportKeys.value = nextNavigationState.exportKeys;
		}
		if (
			nextRouteOutletBranchInputState !==
			previousRouteOutletBranchInputState
		) {
			routeOutletBranchInputState.value = nextRouteOutletBranchInputState;
		}
	});
}

function syncLocationSignal(): void {
	const nextLocationState = buildCurrentRouteOutletLocationState();
	if (
		!areRouteOutletLocationsEqual({
			firstLocationState: locationState.value,
			secondLocationState: nextLocationState,
		})
	) {
		locationState.value = nextLocationState;
	}
}

let isInited = false;

function initUIListeners(): void {
	if (isInited) {
		return;
	}
	isInited = true;

	addRouteChangeListener((event) => {
		syncNavigationSignals();
		window.requestAnimationFrame(() => {
			applyScrollState(event.detail.__scrollState);
		});
	});

	addLocationListener(() => {
		syncLocationSignal();
	});
}

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

type VormaOutletProps = {
	idx: number;
	Outlet: (
		localProps: Record<string, any> | undefined,
	) => h.JSX.Element | null;
};

type VormaErrorBoundaryProps = {
	error: unknown;
};

export function VormaRootOutlet(props: { idx?: number }): h.JSX.Element | null {
	const idx = props.idx ?? 0;
	const isInitialRootRenderRef = useRef(true);
	const passthroughPropsRef = useRef(props);
	passthroughPropsRef.current = props;

	useLayoutEffect(() => {
		if (idx !== 0 || !isInitialRootRenderRef.current) {
			return;
		}
		initUIListeners();
		isInitialRootRenderRef.current = false;
		syncNavigationSignals();
	}, [idx]);

	const routeOutletBranchState = buildRouteOutletBranchState({
		navigationState: routeOutletBranchInputState.value,
		idx,
	});

	const Outlet = useMemo(() => {
		return (localProps: Record<string, any> | undefined) => {
			return h(VormaRootOutlet, {
				...passthroughPropsRef.current,
				...localProps,
				idx: idx + 1,
			});
		};
	}, [idx, routeOutletBranchState.nextRouteKey]);

	const CurrentComp = routeOutletBranchState.currentComponent as
		| ComponentType<VormaOutletProps>
		| undefined;
	const ErrorComp = routeOutletBranchState.errorComponent as
		| ComponentType<VormaErrorBoundaryProps>
		| undefined;

	if (routeOutletBranchState.isErrorIdx) {
		if (ErrorComp) {
			return h(ErrorComp, { error: outermostError.value });
		}
		return h("div", {}, `Error: ${outermostError.value || "unknown"}`);
	}

	if (!CurrentComp) {
		if (routeOutletBranchState.shouldFallbackOutlet) {
			return h(Outlet, { key: routeOutletBranchState.nextRouteKey });
		}
		return null;
	}

	return h(CurrentComp, {
		key: routeOutletBranchState.currentRouteKey,
		idx,
		Outlet,
	});
}
