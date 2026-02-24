import {
	type ComponentType,
	type JSX,
	useLayoutEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
} from "react";
import {
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchState,
	createRouteOutletRuntimeListenerInitializer,
	resolveRouteOutletBranchRenderState,
	type RouteOutletStoreState,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

type StoreState = RouteOutletStoreState;

let state = buildInitialRouteOutletStoreState();
const listeners = new Set<() => void>();

const store = {
	getSnapshot: (): StoreState => state,
	subscribe(listener: () => void): () => void {
		listeners.add(listener);
		return () => {
			listeners.delete(listener);
		};
	},
	setState(updater: (prev: StoreState) => StoreState): void {
		const nextState = updater(state);
		if (nextState !== state) {
			state = nextState;
			listeners.forEach((listener) => {
				listener();
			});
		}
	},
};

function syncStoreState(): void {
	store.setState((previousStoreState) => {
		return buildNextRouteOutletStoreStateFromRuntime(previousStoreState);
	});
}

function useStoreSelector<T>(selector: (state: StoreState) => T): T {
	return useSyncExternalStore(
		store.subscribe,
		() => selector(store.getSnapshot()),
		() => selector(store.getSnapshot()),
	);
}

export function useLoadersData(): any {
	return useStoreSelector((storeState) => storeState.navigation.loadersData);
}

export function useClientLoadersData(): any {
	return useStoreSelector(
		(storeState) => storeState.navigation.clientLoadersData,
	);
}

export function useRouterData() {
	return useStoreSelector((storeState) => storeState.navigation.routerData);
}

export function useLocation() {
	return useStoreSelector((storeState) => storeState.location);
}

const initUIListeners = createRouteOutletRuntimeListenerInitializer({
	syncStoreState,
});

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

type VormaOutletProps = {
	idx: number;
	Outlet: (localProps: Record<string, any> | undefined) => JSX.Element;
};

type VormaErrorBoundaryProps = {
	error: unknown;
};

export function VormaRootOutlet(props: { idx?: number }): JSX.Element {
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

	const routeOutletBranchInputState = useStoreSelector(
		(storeState) => storeState.routeOutletBranchInputState,
	);
	const outermostError = useStoreSelector(
		(storeState) => storeState.navigation.outermostError,
	);
	const routeOutletBranchState = buildRouteOutletBranchState({
		navigationState: routeOutletBranchInputState,
		idx,
	});
	const routeOutletBranchRenderState = resolveRouteOutletBranchRenderState({
		branchState: routeOutletBranchState,
	});

	const Outlet = useMemo(() => {
		return (localProps: Record<string, any> | undefined) => {
			return (
				<VormaRootOutlet
					{...passthroughPropsRef.current}
					{...localProps}
					idx={idx + 1}
				/>
			);
		};
	}, [idx, routeOutletBranchRenderState.nextRouteKey]);

	switch (routeOutletBranchRenderState.renderKind) {
		case "error": {
			const ErrorComp = routeOutletBranchRenderState.errorComponent as
				| ComponentType<VormaErrorBoundaryProps>
				| undefined;
			if (ErrorComp) {
				return <ErrorComp error={outermostError} />;
			}
			return <>{`Error: ${outermostError || "unknown"}`}</>;
		}
		case "fallback":
			return <Outlet key={routeOutletBranchRenderState.nextRouteKey} />;
		case "empty":
			return <></>;
		case "component": {
			const CurrentComp =
				routeOutletBranchRenderState.currentComponent as
					| ComponentType<VormaOutletProps>
					| undefined;
			if (!CurrentComp) {
				return <></>;
			}
			return (
				<CurrentComp
					key={routeOutletBranchRenderState.currentRouteKey}
					idx={idx}
					Outlet={Outlet}
				/>
			);
		}
	}
}
