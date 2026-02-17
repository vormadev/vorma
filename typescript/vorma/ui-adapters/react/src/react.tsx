import {
	type ComponentType,
	type JSX,
	useLayoutEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
} from "react";
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
	type RouteOutletLocationState,
	type RouteOutletNavigationState,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

type NavigationState = RouteOutletNavigationState;
type RouteOutletBranchInputStateValue = RouteOutletBranchInputState;
type LocationState = RouteOutletLocationState;

type StoreState = {
	navigation: NavigationState;
	routeOutletBranchInputState: RouteOutletBranchInputStateValue;
	location: LocationState;
};

function buildInitialStoreState(): StoreState {
	const initialNavigationState = buildInitialRouteOutletNavigationState();
	return {
		navigation: initialNavigationState,
		routeOutletBranchInputState: buildRouteOutletBranchInputState(
			initialNavigationState,
		),
		location: buildCurrentRouteOutletLocationState(),
	};
}

let state = buildInitialStoreState();
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

function syncNavigationState(): void {
	store.setState((previousStoreState) => {
		const nextNavigationState = buildNextRouteOutletNavigationState(
			previousStoreState.navigation,
		);
		if (nextNavigationState === previousStoreState.navigation) {
			return previousStoreState;
		}
		const nextRouteOutletBranchInputStateRaw =
			buildRouteOutletBranchInputState(nextNavigationState);
		const nextRouteOutletBranchInputState =
			areRouteOutletBranchInputsEqualByIdentity({
				firstInputState: previousStoreState.routeOutletBranchInputState,
				secondInputState: nextRouteOutletBranchInputStateRaw,
			})
				? previousStoreState.routeOutletBranchInputState
				: nextRouteOutletBranchInputStateRaw;
		return {
			...previousStoreState,
			navigation: nextNavigationState,
			routeOutletBranchInputState: nextRouteOutletBranchInputState,
		};
	});
}

function syncLocationState(): void {
	store.setState((previousStoreState) => {
		const nextLocationState = buildCurrentRouteOutletLocationState();
		if (
			areRouteOutletLocationsEqual({
				firstLocationState: previousStoreState.location,
				secondLocationState: nextLocationState,
			})
		) {
			return previousStoreState;
		}
		return {
			...previousStoreState,
			location: nextLocationState,
		};
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

let isInited = false;

function initUIListeners(): void {
	if (isInited) {
		return;
	}
	isInited = true;

	addRouteChangeListener((event) => {
		syncNavigationState();
		window.requestAnimationFrame(() => {
			applyScrollState(event.detail.__scrollState);
		});
	});

	addLocationListener(() => {
		syncLocationState();
	});
}

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
		syncNavigationState();
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
	}, [idx, routeOutletBranchState.nextRouteKey]);

	const CurrentComp = routeOutletBranchState.currentComponent as
		| ComponentType<VormaOutletProps>
		| undefined;
	const ErrorComp = routeOutletBranchState.errorComponent as
		| ComponentType<VormaErrorBoundaryProps>
		| undefined;

	if (routeOutletBranchState.isErrorIdx) {
		if (ErrorComp) {
			return <ErrorComp error={outermostError} />;
		}
		return <>{`Error: ${outermostError || "unknown"}`}</>;
	}

	if (!CurrentComp) {
		if (routeOutletBranchState.shouldFallbackOutlet) {
			return <Outlet key={routeOutletBranchState.nextRouteKey} />;
		}
		return <></>;
	}

	return (
		<CurrentComp
			key={routeOutletBranchState.currentRouteKey}
			idx={idx}
			Outlet={Outlet}
		/>
	);
}
