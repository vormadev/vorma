import type { Accessor, JSX, ValidComponent } from "solid-js";
import {
	createEffect,
	createMemo,
	createSignal,
	onCleanup,
	onMount,
	Show,
	splitProps,
	untrack,
} from "solid-js";
import { Dynamic, render } from "solid-js/web";
import type {
	ExtractApp,
	UseRouterDataFunction,
	VormaAppBase,
	VormaAppConfig,
	VormaLoaderOutput,
	VormaLoaderPattern,
	VormaRouteGeneric,
	VormaRoutePropsGeneric,
} from "vorma/client";
import type {
	RouteOutletBranchInputState,
	RouteOutletStoreState,
	TypedAdapterLinkDefaultProps,
	VormaLinkPropsBase,
	VormaTypedAdapterAddClientLoaderProps,
} from "vorma/client/__internal";
import {
	buildInitialRouteOutletStoreState,
	buildTypedAdapterRouteComponentMountProps,
	createRouteOutletAdapterSyncHost,
	createTypedAdapterLinkFactory,
	createTypedAdapterValueHookFactories,
	makeFinalLinkProps,
	navigationInternalLinkPropKeysForAnchors,
	resolveRouteOutletAdapterRenderModel,
	shouldRemountRouteOutletComponentMount,
} from "vorma/client/__internal";

export type VormaRouteProps<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = string,
> = VormaRoutePropsGeneric<JSX.Element, App, Pattern>;

export type VormaRoute<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = string,
> = VormaRouteGeneric<JSX.Element, App, Pattern>;

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

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

function getCurrentStoreState(): RouteOutletStoreState {
	return {
		navigation: navigationState(),
		routeOutletBranchInputState: routeOutletBranchInputStateSignal(),
		location: locationState(),
	};
}

function applyNextStoreState(nextStoreState: RouteOutletStoreState): void {
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

const routeOutletAdapterSyncHost = createRouteOutletAdapterSyncHost({
	getCurrentStoreState,
	applyNextStoreState,
});

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

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

		disposeMountedErrorBranch = render(() => {
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

type VormaRouteComponentMountProps = {
	getCurrentRouteKey: () => string;
	getCurrentRouteComponent: () => ValidComponent | undefined;
	getCurrentMatchedPattern: () => string;
	idx: number;
	Outlet: (localProps?: Record<string, any>) => JSX.Element;
};

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
		const matchedPatternAtMount = untrack(() =>
			props.getCurrentMatchedPattern(),
		);
		const typedAdapterRouteMountProps =
			buildTypedAdapterRouteComponentMountProps({
				routePropsIndex: props.idx,
				matchedPattern: matchedPatternAtMount,
			});
		disposeMountedRouteComponent = render(() => {
			const CurrentComp = mountedRouteComponent();
			if (!CurrentComp) {
				return <></>;
			}
			return (
				<Dynamic
					component={CurrentComp as ValidComponent}
					idx={props.idx}
					Outlet={props.Outlet}
					{...typedAdapterRouteMountProps}
				/>
			);
		}, mountContainer);
	}

	function disposeMountedRouteComponentAndToken(): void {
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
			disposeMountedNextOutlet = render(() => {
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
		routeOutletAdapterSyncHost.syncRootOutletMount({
			idx,
		});
	});

	const routeOutletBranchInputState = createMemo<RouteOutletBranchInputState>(
		() => {
			return routeOutletBranchInputStateSignal();
		},
	);
	const routeOutletRenderModel = createMemo(() => {
		return resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: routeOutletBranchInputState(),
			outermostError: outermostError(),
			idx,
		});
	});
	const isErrorIdx = createMemo(() => {
		return routeOutletRenderModel().renderKind === "error";
	});
	const currentRouteComponent = createMemo<ValidComponent | undefined>(() => {
		const currentRenderModel = routeOutletRenderModel();
		if (currentRenderModel.renderKind !== "component") {
			return undefined;
		}
		return currentRenderModel.currentComponent as
			| ValidComponent
			| undefined;
	});
	const currentRouteKey = createMemo(() => {
		return routeOutletRenderModel().currentRouteKey;
	});
	const nextOutletRouteKey = createMemo(() => {
		return routeOutletRenderModel().nextRouteKey;
	});
	const shouldFallbackOutlet = createMemo(() => {
		return routeOutletRenderModel().renderKind === "fallback";
	});
	const errorComponent = createMemo<ValidComponent | undefined>(() => {
		const currentRenderModel = routeOutletRenderModel();
		if (currentRenderModel.renderKind !== "error") {
			return undefined;
		}
		return currentRenderModel.errorComponent as ValidComponent | undefined;
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
					getCurrentMatchedPattern={() => {
						const currentRenderModel = routeOutletRenderModel();
						return currentRenderModel.renderKind === "component"
							? currentRenderModel.matchedPattern
							: "";
					}}
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
				getCurrentError={() => routeOutletRenderModel().outermostError}
			/>
		</>
	);
}

/////////////////////////////////////////////////////////////////////
/////// TYPED HOOKS
/////////////////////////////////////////////////////////////////////

export function makeTypedUseRouterData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return (() => routerData) as UseRouterDataFunction<App, true>;
}

function createSolidTypedAdapterValueHookFactories<
	App extends ExtractApp<VormaAppConfig>,
>() {
	return createTypedAdapterValueHookFactories<App>({
		useLoadersData: loadersData,
		useMatchedPatterns: () => routerData().matchedPatterns,
		useClientLoadersData: clientLoadersData,
	});
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	const useLoaderDataValue =
		createSolidTypedAdapterValueHookFactories<App>().makeUseLoaderDataHook();
	return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRouteProps<App, Pattern>,
	): Accessor<VormaLoaderOutput<App, Pattern>> {
		return createMemo<VormaLoaderOutput<App, Pattern>>(() => {
			return useLoaderDataValue(props);
		});
	};
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	const usePatternLoaderDataValue =
		createSolidTypedAdapterValueHookFactories<App>().makeUsePatternLoaderDataHook();
	return function usePatternLoaderData<
		Pattern extends VormaLoaderPattern<App>,
	>(pattern: Pattern): Accessor<VormaLoaderOutput<App, Pattern> | undefined> {
		const loaderData = createMemo<
			VormaLoaderOutput<App, Pattern> | undefined
		>(() => {
			return usePatternLoaderDataValue(pattern);
		});
		return loaderData;
	};
}

export function makeTypedAddClientLoader<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	const addClientLoaderValue =
		createSolidTypedAdapterValueHookFactories<App>().makeAddClientLoaderHook();
	return function addClientLoader<
		Pattern extends VormaLoaderPattern<App>,
		LoaderData extends VormaLoaderOutput<App, Pattern>,
		T = any,
	>(
		props: VormaTypedAdapterAddClientLoaderProps<
			App,
			Pattern,
			LoaderData,
			T
		>,
	) {
		const useClientLoaderDataValue = addClientLoaderValue(props);
		type Res = Awaited<ReturnType<ReturnType<typeof addClientLoaderValue>>>;

		const useClientLoaderData = (
			routeProps?: VormaRouteProps<App, Pattern>,
		): Accessor<Res | undefined> => {
			return createMemo<Res | undefined>(() => {
				if (routeProps === undefined) {
					return useClientLoaderDataValue();
				}
				return useClientLoaderDataValue(routeProps);
			});
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Accessor<Res>;
			(): Accessor<Res | undefined>;
		};
	};
}

/////////////////////////////////////////////////////////////////////
/////// LINK APIs
/////////////////////////////////////////////////////////////////////

export function VormaLink(
	props: JSX.AnchorHTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<Event>,
) {
	const finalLinkProps = createMemo(() => makeFinalLinkProps<Event>(props));
	const [, rest] = splitProps(props, [
		...navigationInternalLinkPropKeysForAnchors,
	] as Array<keyof typeof props>);

	return (
		<a
			data-external={finalLinkProps().dataExternal}
			{...rest}
			onPointerEnter={finalLinkProps().onPointerEnter}
			onFocus={finalLinkProps().onFocus}
			onPointerLeave={finalLinkProps().onPointerLeave}
			onBlur={finalLinkProps().onBlur}
			onTouchCancel={finalLinkProps().onTouchCancel}
			onClick={finalLinkProps().onClick}
		>
			{props.children}
		</a>
	);
}

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
		Event
	>,
) {
	type App = ExtractApp<C>;

	return createTypedAdapterLinkFactory<
		App,
		JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
		Event,
		JSX.Element
	>({
		vormaAppConfig,
		defaultProps,
		renderTypedLink: ({ resolveTypedLinkProps }) => {
			const resolvedProps = createMemo(resolveTypedLinkProps);
			return (
				<VormaLink
					{...(resolvedProps()
						.linkProps as JSX.AnchorHTMLAttributes<HTMLAnchorElement>)}
					href={resolvedProps().href}
					state={resolvedProps().state}
				/>
			);
		},
	});
}
