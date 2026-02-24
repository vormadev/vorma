import { computed, signal } from "@preact/signals";
import { h, type ComponentType } from "preact";
import { useLayoutEffect, useMemo, useRef } from "preact/hooks";
import {
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchState,
	createRouteOutletRuntimeListenerInitializer,
	resolveRouteOutletBranchRenderState,
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

const initUIListeners = createRouteOutletRuntimeListenerInitializer({
	syncStoreState,
});

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
	const routeOutletBranchRenderState = resolveRouteOutletBranchRenderState({
		branchState: routeOutletBranchState,
	});

	const Outlet = useMemo(() => {
		return (localProps: Record<string, any> | undefined) => {
			return h(VormaRootOutlet, {
				...passthroughPropsRef.current,
				...localProps,
				idx: idx + 1,
			});
		};
	}, [idx, routeOutletBranchRenderState.nextRouteKey]);

	switch (routeOutletBranchRenderState.renderKind) {
		case "error": {
			const ErrorComp = routeOutletBranchRenderState.errorComponent as
				| ComponentType<VormaErrorBoundaryProps>
				| undefined;
			if (ErrorComp) {
				return h(ErrorComp, { error: outermostError.value });
			}
			return h("div", {}, `Error: ${outermostError.value || "unknown"}`);
		}
		case "fallback":
			return h(Outlet, {
				key: routeOutletBranchRenderState.nextRouteKey,
			});
		case "empty":
			return null;
		case "component": {
			const CurrentComp =
				routeOutletBranchRenderState.currentComponent as
					| ComponentType<VormaOutletProps>
					| undefined;
			if (!CurrentComp) {
				return null;
			}
			return h(CurrentComp, {
				key: routeOutletBranchRenderState.currentRouteKey,
				idx,
				Outlet,
			});
		}
	}
}
