import {
	createEffect,
	createMemo,
	createSignal,
	onCleanup,
	onMount,
	Show,
	untrack,
	type JSX,
	type ValidComponent,
} from "solid-js";
import { Dynamic, render as renderSolid } from "solid-js/web";
import {
	buildInitialRouteOutletStoreState,
	buildRouteOutletBranchState,
	buildTypedAdapterRoutePropsWithInternalRouteInstanceToken,
	createRouteOutletRuntimeListenerInitializer,
	createTypedAdapterRouteInstanceToken,
	markTypedAdapterRouteInstanceTokenActive,
	markTypedAdapterRouteInstanceTokenDisposed,
	resolveRouteOutletBranchRenderState,
	shouldRemountRouteOutletComponentMount,
	syncRouteOutletStoreStateFromRuntime,
	type RouteOutletBranchInputState,
	type RouteOutletStoreState,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

type StoreState = RouteOutletStoreState;
type RouteOutletBranchInputStateValue = RouteOutletBranchInputState;

const initialStoreState = buildInitialRouteOutletStoreState();
const [navigationState, setNavigationState] = createSignal(
	initialStoreState.navigation,
);
const [routeOutletBranchInputStateSignal, setRouteOutletBranchInputState] =
	createSignal(initialStoreState.routeOutletBranchInputState);
const [locationState, setLocationState] = createSignal(
	initialStoreState.location,
);

const loadersData = () => navigationState().loadersData;
const clientLoadersData = () => navigationState().clientLoadersData;
const routerData = () => navigationState().routerData;
const outermostError = () => navigationState().outermostError;

export { clientLoadersData, loadersData, routerData };

const location = () => locationState();

export { location };

function getCurrentStoreState(): StoreState {
	return {
		navigation: navigationState(),
		routeOutletBranchInputState: routeOutletBranchInputStateSignal(),
		location: locationState(),
	};
}

function applyNextStoreState(nextStoreState: StoreState): void {
	const previousStoreState = getCurrentStoreState();
	if (nextStoreState !== previousStoreState) {
		if (nextStoreState.navigation !== previousStoreState.navigation) {
			setNavigationState(nextStoreState.navigation);
		}
		if (
			nextStoreState.routeOutletBranchInputState !==
			previousStoreState.routeOutletBranchInputState
		) {
			setRouteOutletBranchInputState(
				nextStoreState.routeOutletBranchInputState,
			);
		}
		if (nextStoreState.location !== previousStoreState.location) {
			setLocationState(nextStoreState.location);
		}
	}
}

function syncStoreState(): void {
	syncRouteOutletStoreStateFromRuntime({
		getCurrentStoreState,
		applyNextStoreState,
	});
}

const initUIListeners = createRouteOutletRuntimeListenerInitializer({
	syncStoreState,
});

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

type VormaRouteComponentMountProps = {
	getCurrentRouteKey: () => string;
	getCurrentRouteComponent: () => ValidComponent | undefined;
	getMatchedPatterns: () => readonly string[];
	getLoadersData: () => readonly unknown[];
	getClientLoadersData: () => readonly unknown[];
	idx: number;
	Outlet: (localProps?: Record<string, any>) => JSX.Element;
};

type VormaErrorBranchMountProps = {
	getIsErrorIdx: () => boolean;
	getErrorComponent: () => ValidComponent | undefined;
	getCurrentError: () => unknown;
};

function VormaErrorBranchMount(props: VormaErrorBranchMountProps): JSX.Element {
	const [mountContainerEl, setMountContainerEl] = createSignal<
		HTMLSpanElement | undefined
	>(undefined);
	let disposeMountedErrorBranch: (() => void) | undefined;

	createEffect(() => {
		const mountContainer = mountContainerEl();
		if (!mountContainer) {
			return;
		}

		const isError = props.getIsErrorIdx();
		const ErrorComp = props.getErrorComponent();
		const currentError = props.getCurrentError();

		disposeMountedErrorBranch?.();
		disposeMountedErrorBranch = undefined;
		mountContainer.textContent = "";

		if (!isError) {
			return;
		}

		disposeMountedErrorBranch = renderSolid(() => {
			if (ErrorComp) {
				return (
					<Dynamic
						component={ErrorComp as ValidComponent}
						error={currentError}
					/>
				);
			}
			return `Error: ${currentError || "unknown"}`;
		}, mountContainer);
	});

	onCleanup(() => {
		disposeMountedErrorBranch?.();
	});

	return <span ref={setMountContainerEl} style={{ display: "contents" }} />;
}

function VormaRouteComponentMount(
	props: VormaRouteComponentMountProps,
): JSX.Element {
	const [mountContainerEl, setMountContainerEl] = createSignal<
		HTMLSpanElement | undefined
	>(undefined);
	const [mountedRouteComponent, setMountedRouteComponent] = createSignal<
		ValidComponent | undefined
	>(undefined);
	let disposeMountedRouteComponent: (() => void) | undefined;
	let mountedRouteInstanceToken: unknown | undefined;
	let previousObservedRouteComponent: ValidComponent | undefined;

	function mountRouteComponentIntoContainer(
		mountContainer: HTMLSpanElement,
	): void {
		const routeKeyAtMount = untrack(() => props.getCurrentRouteKey());
		const matchedPatternsAtMount = untrack(() =>
			props.getMatchedPatterns(),
		);
		const loadersDataAtMount = untrack(() => props.getLoadersData());
		const clientLoadersDataAtMount = untrack(() =>
			props.getClientLoadersData(),
		);
		mountedRouteInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: props.idx,
			routeKey: routeKeyAtMount,
			matchedPatterns: matchedPatternsAtMount,
			loadersData: loadersDataAtMount,
			clientLoadersData: clientLoadersDataAtMount,
		});
		markTypedAdapterRouteInstanceTokenActive({
			routeInstanceToken: mountedRouteInstanceToken,
		});
		const routeInstanceTokenForMountedComponent = mountedRouteInstanceToken;
		disposeMountedRouteComponent = renderSolid(() => {
			const CurrentComp = mountedRouteComponent();
			if (!CurrentComp) {
				return <></>;
			}
			return (
				<Dynamic
					component={CurrentComp as ValidComponent}
					idx={props.idx}
					Outlet={props.Outlet}
					{...buildTypedAdapterRoutePropsWithInternalRouteInstanceToken(
						{
							routeInstanceToken:
								routeInstanceTokenForMountedComponent,
						},
					)}
				/>
			);
		}, mountContainer);
	}

	function disposeMountedRouteComponentAndToken(): void {
		if (mountedRouteInstanceToken !== undefined) {
			markTypedAdapterRouteInstanceTokenDisposed({
				routeInstanceToken: mountedRouteInstanceToken,
			});
			mountedRouteInstanceToken = undefined;
		}
		disposeMountedRouteComponent?.();
		disposeMountedRouteComponent = undefined;
	}

	createEffect((previousRouteKey: string | undefined) => {
		const nextRouteKey = props.getCurrentRouteKey();
		const nextRouteComponent = props.getCurrentRouteComponent();
		const mountContainer = mountContainerEl();
		const shouldRemountMountedRouteComponent =
			shouldRemountRouteOutletComponentMount({
				idx: props.idx,
				previousRouteKey,
				nextRouteKey,
				previousRouteComponent: previousObservedRouteComponent,
				nextRouteComponent,
			});
		previousObservedRouteComponent = nextRouteComponent;
		if (!mountContainer) {
			return nextRouteKey;
		}

		if (!disposeMountedRouteComponent) {
			setMountedRouteComponent(() => {
				return nextRouteComponent;
			});
			mountRouteComponentIntoContainer(mountContainer);
			return nextRouteKey;
		}

		if (shouldRemountMountedRouteComponent) {
			disposeMountedRouteComponentAndToken();
			setMountedRouteComponent(() => {
				return nextRouteComponent;
			});
			mountRouteComponentIntoContainer(mountContainer);
			return nextRouteKey;
		}

		return nextRouteKey;
	}, undefined);

	onCleanup(() => {
		disposeMountedRouteComponentAndToken();
	});

	return <span ref={setMountContainerEl} style={{ display: "contents" }} />;
}

type VormaNextOutletMountProps = {
	getNextOutletRouteKey: () => string;
	passthroughProps: Record<string, any>;
	localProps?: Record<string, any>;
	nextIdx: number;
};

function VormaNextOutletMount(props: VormaNextOutletMountProps): JSX.Element {
	const [mountContainerEl, setMountContainerEl] = createSignal<
		HTMLSpanElement | undefined
	>(undefined);
	let disposeMountedNextOutlet: (() => void) | undefined;

	createEffect((previousNextOutletRouteKey: string | undefined) => {
		const nextOutletRouteKey = props.getNextOutletRouteKey();
		const mountContainer = mountContainerEl();
		if (
			nextOutletRouteKey === previousNextOutletRouteKey &&
			mountContainer &&
			disposeMountedNextOutlet
		) {
			return previousNextOutletRouteKey;
		}

		disposeMountedNextOutlet?.();
		disposeMountedNextOutlet = undefined;

		if (mountContainer) {
			disposeMountedNextOutlet = renderSolid(() => {
				return (
					<VormaRootOutlet
						{...props.passthroughProps}
						{...props.localProps}
						idx={props.nextIdx}
					/>
				);
			}, mountContainer);
		}

		return nextOutletRouteKey;
	}, undefined);

	onCleanup(() => {
		disposeMountedNextOutlet?.();
	});

	return <span ref={setMountContainerEl} style={{ display: "contents" }} />;
}

export function VormaRootOutlet(
	props: { idx?: number } & Record<string, any>,
): JSX.Element {
	const idx = props.idx ?? 0;

	onMount(() => {
		if (idx !== 0) {
			return;
		}
		initUIListeners();
		syncStoreState();
	});

	const routeOutletBranchInputState =
		createMemo<RouteOutletBranchInputStateValue>(() => {
			return routeOutletBranchInputStateSignal();
		});
	const routeOutletBranchState = createMemo(() => {
		return buildRouteOutletBranchState({
			navigationState: routeOutletBranchInputState(),
			idx,
		});
	});
	const routeOutletBranchRenderState = createMemo(() => {
		return resolveRouteOutletBranchRenderState({
			branchState: routeOutletBranchState(),
		});
	});
	const isErrorIdx = createMemo(() => {
		return routeOutletBranchRenderState().renderKind === "error";
	});
	const currentRouteComponent = createMemo<ValidComponent | undefined>(() => {
		const branchRenderState = routeOutletBranchRenderState();
		if (branchRenderState.renderKind !== "component") {
			return undefined;
		}
		return branchRenderState.currentComponent as ValidComponent | undefined;
	});
	const currentRouteKey = createMemo(() => {
		return routeOutletBranchRenderState().currentRouteKey;
	});
	const nextOutletRouteKey = createMemo(() => {
		return routeOutletBranchRenderState().nextRouteKey;
	});
	const shouldFallbackOutlet = createMemo(() => {
		return routeOutletBranchRenderState().renderKind === "fallback";
	});
	const errorComponent = createMemo<ValidComponent | undefined>(() => {
		const branchRenderState = routeOutletBranchRenderState();
		if (branchRenderState.renderKind !== "error") {
			return undefined;
		}
		return branchRenderState.errorComponent as ValidComponent | undefined;
	});

	const Outlet = (localProps?: Record<string, any>): JSX.Element => {
		return (
			<VormaNextOutletMount
				getNextOutletRouteKey={nextOutletRouteKey}
				passthroughProps={props}
				localProps={localProps}
				nextIdx={idx + 1}
			/>
		);
	};

	return (
		<>
			<Show when={currentRouteComponent()}>
				<VormaRouteComponentMount
					getCurrentRouteKey={currentRouteKey}
					getCurrentRouteComponent={currentRouteComponent}
					getMatchedPatterns={() => routerData().matchedPatterns}
					getLoadersData={loadersData}
					getClientLoadersData={clientLoadersData}
					idx={idx}
					Outlet={Outlet}
				/>
			</Show>
			<Show when={!currentRouteComponent() && shouldFallbackOutlet()}>
				<Outlet />
			</Show>
			<VormaErrorBranchMount
				getIsErrorIdx={isErrorIdx}
				getErrorComponent={errorComponent}
				getCurrentError={outermostError}
			/>
		</>
	);
}
