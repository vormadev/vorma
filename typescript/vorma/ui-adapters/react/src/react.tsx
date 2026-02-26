import {
	useLayoutEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
	type ComponentType,
	type JSX,
} from "react";
import {
	buildInitialRouteOutletStoreState,
	buildRouteOutletBranchState,
	buildTypedAdapterRoutePropsWithInternalRouteInstanceToken,
	createRouteOutletRuntimeListenerInitializer,
	createTypedAdapterRouteInstanceToken,
	markTypedAdapterRouteInstanceTokenActive,
	markTypedAdapterRouteInstanceTokenDisposed,
	resolveRouteOutletBranchRenderState,
	syncRouteOutletStoreStateFromRuntime,
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
	syncRouteOutletStoreStateFromRuntime({
		getCurrentStoreState: store.getSnapshot,
		applyNextStoreState: (nextStoreState) => {
			store.setState(() => nextStoreState);
		},
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

type VormaRouteComponentMountProps = {
	CurrentComp: ComponentType<VormaOutletProps>;
	idx: number;
	routeKey: string;
	Outlet: (localProps: Record<string, any> | undefined) => JSX.Element;
};

function VormaRouteComponentMount(
	props: VormaRouteComponentMountProps,
): JSX.Element {
	const routeInstanceTokenRef = useRef<unknown>(undefined);
	if (routeInstanceTokenRef.current === undefined) {
		const currentNavigationState = store.getSnapshot().navigation;
		routeInstanceTokenRef.current = createTypedAdapterRouteInstanceToken({
			routePropsIndex: props.idx,
			routeKey: props.routeKey,
			matchedPatterns: currentNavigationState.routerData.matchedPatterns,
			loadersData: currentNavigationState.loadersData,
			clientLoadersData: currentNavigationState.clientLoadersData,
		});
	}

	useLayoutEffect(() => {
		const routeInstanceToken = routeInstanceTokenRef.current;
		markTypedAdapterRouteInstanceTokenActive({
			routeInstanceToken,
		});
		return () => {
			markTypedAdapterRouteInstanceTokenDisposed({
				routeInstanceToken,
			});
		};
	}, []);

	return (
		<props.CurrentComp
			idx={props.idx}
			Outlet={props.Outlet}
			{...buildTypedAdapterRoutePropsWithInternalRouteInstanceToken({
				routeInstanceToken: routeInstanceTokenRef.current,
			})}
		/>
	);
}

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
				<VormaRouteComponentMount
					key={routeOutletBranchRenderState.currentRouteKey}
					CurrentComp={CurrentComp}
					idx={idx}
					routeKey={routeOutletBranchRenderState.currentRouteKey}
					Outlet={Outlet}
				/>
			);
		}
	}
}
