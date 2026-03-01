/// <reference types="vite/client" />

import {
	memo,
	useLayoutEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
	type ComponentProps,
	type ComponentType,
	type FocusEvent,
	type JSX,
	type MouseEvent,
	type PointerEvent,
	type TouchEvent,
} from "react";
import type {
	ExtractApp,
	UseRouterDataFunction,
	VormaAppBase,
	VormaAppConfig,
	VormaLoaderPattern,
	VormaRouteGeneric,
	VormaRoutePropsGeneric,
} from "vorma/client";
import {
	buildNavigationLinkAnchorRenderProps,
	buildInitialRouteOutletStoreState,
	buildTypedAdapterRouteComponentMountProps,
	createRouteOutletAdapterSyncHost,
	createTypedAdapterLinkFactory,
	createTypedAdapterValueHookFactories,
	renderRouteOutletAdapterRenderModel,
	resolveRouteOutletAdapterRenderModel,
	type RouteOutletStoreState,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
	type VormaLinkPropsBase,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// Store
/////////////////////////////////////////////////////////////////////

type RouteOutletStoreListener = () => void;

let routeOutletStoreState: RouteOutletStoreState | undefined;
const routeOutletStoreListeners = new Set<RouteOutletStoreListener>();

function getRouteOutletStoreStateOrInitialize(): RouteOutletStoreState {
	if (routeOutletStoreState !== undefined) {
		return routeOutletStoreState;
	}
	routeOutletStoreState = buildInitialRouteOutletStoreState();
	return routeOutletStoreState;
}

function applyNextRouteOutletStoreState(props: {
	nextStoreState: RouteOutletStoreState;
}): void {
	if (props.nextStoreState === getRouteOutletStoreStateOrInitialize()) {
		return;
	}
	routeOutletStoreState = props.nextStoreState;
	routeOutletStoreListeners.forEach((listener) => {
		listener();
	});
}

function subscribeToRouteOutletStore(
	listener: RouteOutletStoreListener,
): () => void {
	routeOutletStoreListeners.add(listener);
	return () => {
		routeOutletStoreListeners.delete(listener);
	};
}

function useRouteOutletStoreSelector<Value>(props: {
	selectValue: (storeState: RouteOutletStoreState) => Value;
}): Value {
	return useSyncExternalStore(
		subscribeToRouteOutletStore,
		() => props.selectValue(getRouteOutletStoreStateOrInitialize()),
		() => props.selectValue(getRouteOutletStoreStateOrInitialize()),
	);
}

const routeOutletAdapterSyncHost = createRouteOutletAdapterSyncHost({
	getCurrentStoreState: getRouteOutletStoreStateOrInitialize,
	applyNextStoreState: (nextStoreState) => {
		applyNextRouteOutletStoreState({
			nextStoreState,
		});
	},
});

function useMatchedPatterns(): string[] {
	return useRouteOutletStoreSelector({
		selectValue: (storeState) =>
			storeState.routeOutletBranchInputState.matchedPatterns,
	});
}

export function useLoadersData(): unknown[] {
	return useRouteOutletStoreSelector({
		selectValue: (storeState) => storeState.navigation.loadersData,
	});
}

export function useClientLoadersData(): unknown[] {
	return useRouteOutletStoreSelector({
		selectValue: (storeState) => storeState.navigation.clientLoadersData,
	});
}

function useRouterDataFromStore() {
	return useRouteOutletStoreSelector({
		selectValue: (storeState) => storeState.navigation.routerData,
	});
}

export function useRouterData() {
	return useRouterDataFromStore();
}

export function useLocation() {
	return useRouteOutletStoreSelector({
		selectValue: (storeState) => storeState.location,
	});
}

/////////////////////////////////////////////////////////////////////
/////// Typed Value Hooks
/////////////////////////////////////////////////////////////////////

export type VormaRouteProps<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = VormaRoutePropsGeneric<JSX.Element, App, Pattern>;

export type VormaRoute<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = VormaRouteGeneric<JSX.Element, App, Pattern>;

export function makeTypedUseRouterData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return useRouterDataFromStore as UseRouterDataFunction<App, false>;
}

function createReactTypedAdapterValueHookFactories<
	App extends ExtractApp<VormaAppConfig>,
>() {
	return createTypedAdapterValueHookFactories<App>({
		useLoadersData,
		useMatchedPatterns: useMatchedPatterns,
		useClientLoadersData,
		useMemoizedPatternLoaderData: (props) => {
			return useMemo(props.resolvePatternLoaderData, props.dependencies);
		},
	});
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return createReactTypedAdapterValueHookFactories<App>().makeUseLoaderDataHook();
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return createReactTypedAdapterValueHookFactories<App>().makeUsePatternLoaderDataHook();
}

export function makeTypedAddClientLoader<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return createReactTypedAdapterValueHookFactories<App>().makeAddClientLoaderHook();
}

/////////////////////////////////////////////////////////////////////
/////// Link
/////////////////////////////////////////////////////////////////////

type ReactLinkEvent =
	| MouseEvent<HTMLAnchorElement, globalThis.MouseEvent>
	| PointerEvent<HTMLAnchorElement>
	| FocusEvent<HTMLAnchorElement>
	| TouchEvent<HTMLAnchorElement>;

export function VormaLink(
	linkProps: ComponentProps<"a"> & VormaLinkPropsBase<ReactLinkEvent>,
) {
	const anchorRenderProps = buildNavigationLinkAnchorRenderProps(linkProps);
	return (
		<a
			data-external={anchorRenderProps.dataExternal}
			{...(anchorRenderProps.safeAnchorProps as ComponentProps<"a">)}
			onPointerEnter={anchorRenderProps.onPointerEnter}
			onFocus={anchorRenderProps.onFocus}
			onPointerLeave={anchorRenderProps.onPointerLeave}
			onBlur={anchorRenderProps.onBlur}
			onTouchCancel={anchorRenderProps.onTouchCancel}
			onClick={anchorRenderProps.onClick}
		>
			{linkProps.children}
		</a>
	);
}

type ReactTypedLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<App, Pattern, ComponentProps<"a">, ReactLinkEvent>;

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		ComponentProps<"a">,
		ReactLinkEvent
	>,
) {
	type App = ExtractApp<C>;
	const TypedLink = createTypedAdapterLinkFactory<
		App,
		ComponentProps<"a">,
		ReactLinkEvent,
		JSX.Element
	>({
		vormaAppConfig,
		defaultProps,
		renderTypedLink: (renderProps) => {
			const resolvedTypedLinkProps = renderProps.resolveTypedLinkProps();
			return (
				<VormaLink
					{...resolvedTypedLinkProps.linkProps}
					href={resolvedTypedLinkProps.href}
					state={resolvedTypedLinkProps.state}
				/>
			);
		},
	});
	return memo(TypedLink) as <Pattern extends VormaLoaderPattern<App>>(
		linkProps: ReactTypedLinkProps<App, Pattern>,
	) => JSX.Element;
}

/////////////////////////////////////////////////////////////////////
/////// Root Outlet
/////////////////////////////////////////////////////////////////////

type RouteOutletComponentProps = {
	idx: number;
	Outlet: (localProps?: Record<string, unknown>) => JSX.Element;
} & Record<string, unknown>;

type RouteErrorBoundaryProps = {
	error: unknown;
};

function RouteOutletComponentMount(props: {
	currentComponent: unknown;
	currentRouteMountKey: string;
	matchedPattern: string;
	routeIndex: number;
	Outlet: (localProps?: Record<string, unknown>) => JSX.Element;
}) {
	const CurrentComponent =
		props.currentComponent as ComponentType<RouteOutletComponentProps>;
	return (
		<CurrentComponent
			key={props.currentRouteMountKey}
			idx={props.routeIndex}
			Outlet={props.Outlet}
			{...buildTypedAdapterRouteComponentMountProps({
				routePropsIndex: props.routeIndex,
				matchedPattern: props.matchedPattern,
			})}
		/>
	);
}

export function VormaRootOutlet(
	rootOutletProps: { idx?: number } & Record<string, unknown>,
): JSX.Element {
	const routeIndex = rootOutletProps.idx ?? 0;
	const passthroughPropsRef = useRef(rootOutletProps);
	passthroughPropsRef.current = rootOutletProps;

	useLayoutEffect(() => {
		if (routeIndex !== 0) {
			return;
		}
		routeOutletAdapterSyncHost.syncRootOutletMount({
			idx: routeIndex,
		});
		return () => {
			routeOutletAdapterSyncHost.syncRootOutletUnmount();
		};
	}, [routeIndex]);

	const routeOutletBranchInputState = useRouteOutletStoreSelector({
		selectValue: (storeState) => storeState.routeOutletBranchInputState,
	});
	const outermostError = useRouteOutletStoreSelector({
		selectValue: (storeState) => storeState.navigation.outermostError,
	});
	const routeOutletRenderModel = resolveRouteOutletAdapterRenderModel({
		routeOutletBranchInputState,
		outermostError,
		idx: routeIndex,
	});
	const Outlet = useMemo(() => {
		return (localProps?: Record<string, unknown>) => {
			return (
				<VormaRootOutlet
					{...passthroughPropsRef.current}
					{...localProps}
					idx={routeIndex + 1}
				/>
			);
		};
	}, [routeIndex, routeOutletRenderModel.nextRouteKey]);

	return renderRouteOutletAdapterRenderModel<JSX.Element>({
		routeOutletRenderModel,
		renderErrorWithBoundary: (props) => {
			const ErrorBoundaryComponent =
				props.errorComponent as ComponentType<RouteErrorBoundaryProps>;
			return <ErrorBoundaryComponent error={props.outermostError} />;
		},
		renderErrorWithoutBoundary: (props) => {
			return <>Error: {String(props.outermostError ?? "unknown")}</>;
		},
		renderComponent: (props) => {
			return (
				<RouteOutletComponentMount
					key={props.currentRouteMountKey}
					currentComponent={props.currentComponent}
					currentRouteMountKey={props.currentRouteMountKey}
					matchedPattern={props.matchedPattern}
					routeIndex={routeIndex}
					Outlet={Outlet}
				/>
			);
		},
		renderMissingComponent: () => {
			return <></>;
		},
		renderFallback: (props) => {
			return <Outlet key={props.nextRouteKey} />;
		},
		renderEmpty: () => {
			return <></>;
		},
	});
}
