import {
	createEffect,
	createMemo,
	createSignal,
	onCleanup,
	Show,
	type JSX,
	type ValidComponent,
} from "solid-js";
import { Dynamic, render as renderSolid } from "solid-js/web";
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

function readRouteOutletBranchInputSignals(): RouteOutletBranchInputStateValue {
	return routeOutletBranchInputStateSignal();
}

function syncStoreState(): void {
	const previousStoreState: StoreState = {
		navigation: navigationState(),
		routeOutletBranchInputState: routeOutletBranchInputStateSignal(),
		location: locationState(),
	};
	const nextStoreState =
		buildNextRouteOutletStoreStateFromRuntime(previousStoreState);
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

type VormaRouteComponentMountProps = {
	getCurrentRouteKey: () => string;
	getCurrentRouteComponent: () => ValidComponent | undefined;
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
	let previousObservedRouteComponent: ValidComponent | undefined;

	function mountRouteComponentIntoContainer(
		mountContainer: HTMLSpanElement,
	): void {
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
				/>
			);
		}, mountContainer);
	}

	createEffect((previousRouteKey: string | undefined) => {
		const nextRouteKey = props.getCurrentRouteKey();
		const nextRouteComponent = props.getCurrentRouteComponent();
		const mountContainer = mountContainerEl();
		const didComponentIdentityChange =
			previousObservedRouteComponent !== undefined &&
			previousObservedRouteComponent !== nextRouteComponent;
		const shouldRemountForComponentIdentityChange =
			didComponentIdentityChange;
		const didRouteKeyChange =
			typeof previousRouteKey === "string" &&
			previousRouteKey !== nextRouteKey;
		const shouldRemountForRouteKeyChange =
			props.idx === 0 && didRouteKeyChange;
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

		if (
			shouldRemountForRouteKeyChange ||
			shouldRemountForComponentIdentityChange
		) {
			disposeMountedRouteComponent();
			disposeMountedRouteComponent = undefined;
			setMountedRouteComponent(() => {
				return nextRouteComponent;
			});
			mountRouteComponentIntoContainer(mountContainer);
			return nextRouteKey;
		}

		return nextRouteKey;
	}, undefined);

	onCleanup(() => {
		disposeMountedRouteComponent?.();
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

	if (idx === 0) {
		initUIListeners();
		syncStoreState();
	}

	const routeOutletBranchInputState =
		createMemo<RouteOutletBranchInputStateValue>(() => {
			return readRouteOutletBranchInputSignals();
		});
	const routeOutletBranchState = createMemo(() => {
		return buildRouteOutletBranchState({
			navigationState: routeOutletBranchInputState(),
			idx,
		});
	});
	const isErrorIdx = createMemo(() => {
		return routeOutletBranchState().isErrorIdx;
	});
	const currentRouteComponent = createMemo<ValidComponent | undefined>(() => {
		if (isErrorIdx()) {
			return undefined;
		}
		return routeOutletBranchState().currentComponent as
			| ValidComponent
			| undefined;
	});
	const currentRouteKey = createMemo(() => {
		return routeOutletBranchState().currentRouteKey;
	});
	const nextOutletRouteKey = createMemo(() => {
		return routeOutletBranchState().nextRouteKey;
	});
	const shouldFallbackOutlet = createMemo(() => {
		return routeOutletBranchState().shouldFallbackOutlet;
	});
	const errorComponent = createMemo<ValidComponent | undefined>(() => {
		if (!isErrorIdx()) {
			return undefined;
		}
		return routeOutletBranchState().errorComponent as
			| ValidComponent
			| undefined;
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
