import { computed, signal } from "@preact/signals";
import { h, type ComponentType } from "preact";
import { useLayoutEffect, useMemo, useRef } from "preact/hooks";
import { addLocationListener, addRouteChangeListener } from "vorma/client";
import {
	applyScrollState,
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchState,
	type RouteOutletBranchInputState,
	type RouteOutletStoreState,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

type StoreState = RouteOutletStoreState;

type RouteOutletBranchInputStateValue = RouteOutletBranchInputState;

const storeState = signal<StoreState>(buildInitialRouteOutletStoreState());

const loadersData = computed(() => storeState.value.navigation.loadersData);
const clientLoadersData = computed(
	() => storeState.value.navigation.clientLoadersData,
);
const routerData = computed(() => storeState.value.navigation.routerData);
const outermostError = computed(
	() => storeState.value.navigation.outermostError,
);
const routeOutletBranchInputState = computed<RouteOutletBranchInputStateValue>(
	() => storeState.value.routeOutletBranchInputState,
);

export { clientLoadersData, loadersData, routerData };

export const location = computed(() => storeState.value.location);

function syncStoreState(): void {
	const previousStoreState = storeState.value;
	const nextStoreState =
		buildNextRouteOutletStoreStateFromRuntime(previousStoreState);
	if (nextStoreState !== previousStoreState) {
		storeState.value = nextStoreState;
	}
}

let isInited = false;

function initUIListeners(): void {
	if (isInited) {
		return;
	}
	isInited = true;

	addRouteChangeListener((event) => {
		syncStoreState();
		window.requestAnimationFrame(() => {
			applyScrollState(event.detail.__scrollState);
		});
	});

	addLocationListener(() => {
		syncStoreState();
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
		syncStoreState();
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
