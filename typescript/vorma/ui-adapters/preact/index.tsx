/// <reference types="vite/client" />

import { computed, signal } from "@preact/signals";
import {
	h,
	type ComponentType,
	type HTMLAttributes,
	type TargetedMouseEvent,
} from "preact";
import { useLayoutEffect, useMemo, useRef } from "preact/hooks";
import type { JSX } from "preact/jsx-runtime";
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

const routeOutletStoreState = signal<RouteOutletStoreState | undefined>(
	undefined,
);

function getRouteOutletStoreStateOrInitialize(): RouteOutletStoreState {
	if (routeOutletStoreState.value !== undefined) {
		return routeOutletStoreState.value;
	}
	const initialStoreState = buildInitialRouteOutletStoreState();
	routeOutletStoreState.value = initialStoreState;
	return initialStoreState;
}

const routeOutletAdapterSyncHost = createRouteOutletAdapterSyncHost({
	getCurrentStoreState: getRouteOutletStoreStateOrInitialize,
	applyNextStoreState: (nextStoreState) => {
		if (nextStoreState !== routeOutletStoreState.value) {
			routeOutletStoreState.value = nextStoreState;
		}
	},
});

const loadersData = computed(
	() => getRouteOutletStoreStateOrInitialize().navigation.loadersData,
);
const clientLoadersData = computed(
	() => getRouteOutletStoreStateOrInitialize().navigation.clientLoadersData,
);
const routerData = computed(
	() => getRouteOutletStoreStateOrInitialize().navigation.routerData,
);
const matchedPatterns = computed(
	() =>
		getRouteOutletStoreStateOrInitialize().routeOutletBranchInputState
			.matchedPatterns,
);
const outermostError = computed(
	() => getRouteOutletStoreStateOrInitialize().navigation.outermostError,
);
const routeOutletBranchInputState = computed(
	() => getRouteOutletStoreStateOrInitialize().routeOutletBranchInputState,
);

export { clientLoadersData, loadersData, routerData };
export const location = computed(
	() => getRouteOutletStoreStateOrInitialize().location,
);

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
	return (() => {
		return routerData.value;
	}) as UseRouterDataFunction<App, false>;
}

function createPreactTypedAdapterValueHookFactories<
	App extends ExtractApp<VormaAppConfig>,
>() {
	return createTypedAdapterValueHookFactories<App>({
		useLoadersData: () => loadersData.value,
		useMatchedPatterns: () => matchedPatterns.value,
		useClientLoadersData: () => clientLoadersData.value,
	});
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return createPreactTypedAdapterValueHookFactories<App>().makeUseLoaderDataHook();
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return createPreactTypedAdapterValueHookFactories<App>().makeUsePatternLoaderDataHook();
}

export function makeTypedAddClientLoader<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return createPreactTypedAdapterValueHookFactories<App>().makeAddClientLoaderHook();
}

/////////////////////////////////////////////////////////////////////
/////// Link
/////////////////////////////////////////////////////////////////////

type PreactLinkEvent = TargetedMouseEvent<HTMLAnchorElement>;

export function VormaLink(
	linkProps: HTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<PreactLinkEvent>,
) {
	const anchorRenderProps = buildNavigationLinkAnchorRenderProps(linkProps);
	return h(
		"a",
		{
			"data-external": anchorRenderProps.dataExternal,
			...(anchorRenderProps.safeAnchorProps as HTMLAttributes<HTMLAnchorElement>),
			onPointerEnter: anchorRenderProps.onPointerEnter,
			onFocus: anchorRenderProps.onFocus,
			onPointerLeave: anchorRenderProps.onPointerLeave,
			onBlur: anchorRenderProps.onBlur,
			onTouchCancel: anchorRenderProps.onTouchCancel,
			onClick: anchorRenderProps.onClick,
		},
		linkProps.children,
	);
}

type PreactTypedLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<
	App,
	Pattern,
	HTMLAttributes<HTMLAnchorElement>,
	PreactLinkEvent
>;

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		HTMLAttributes<HTMLAnchorElement>,
		PreactLinkEvent
	>,
) {
	type App = ExtractApp<C>;
	return createTypedAdapterLinkFactory<
		App,
		HTMLAttributes<HTMLAnchorElement>,
		PreactLinkEvent,
		JSX.Element
	>({
		vormaAppConfig,
		defaultProps,
		renderTypedLink: (renderProps) => {
			const resolvedTypedLinkProps = renderProps.resolveTypedLinkProps();
			return h(VormaLink, {
				...resolvedTypedLinkProps.linkProps,
				href: resolvedTypedLinkProps.href,
				state: resolvedTypedLinkProps.state,
			});
		},
	}) as <Pattern extends VormaLoaderPattern<App>>(
		linkProps: PreactTypedLinkProps<App, Pattern>,
	) => JSX.Element;
}

/////////////////////////////////////////////////////////////////////
/////// Root Outlet
/////////////////////////////////////////////////////////////////////

type RouteOutletComponentProps = {
	idx: number;
	Outlet: (localProps?: Record<string, unknown>) => JSX.Element | null;
} & Record<string, unknown>;

type RouteErrorBoundaryProps = {
	error: unknown;
};

function RouteOutletComponentMount(props: {
	currentComponent: unknown;
	currentRouteMountKey: string;
	matchedPattern: string;
	routeIndex: number;
	Outlet: (localProps?: Record<string, unknown>) => JSX.Element | null;
}) {
	const CurrentComponent =
		props.currentComponent as ComponentType<RouteOutletComponentProps>;
	return h(CurrentComponent, {
		key: props.currentRouteMountKey,
		idx: props.routeIndex,
		Outlet: props.Outlet,
		...buildTypedAdapterRouteComponentMountProps({
			routePropsIndex: props.routeIndex,
			matchedPattern: props.matchedPattern,
		}),
	});
}

export function VormaRootOutlet(
	rootOutletProps: { idx?: number } & Record<string, unknown>,
): JSX.Element | null {
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

	const routeOutletRenderModel = resolveRouteOutletAdapterRenderModel({
		routeOutletBranchInputState: routeOutletBranchInputState.value,
		outermostError: outermostError.value,
		idx: routeIndex,
	});
	const Outlet = useMemo(() => {
		return (localProps?: Record<string, unknown>) => {
			return h(VormaRootOutlet, {
				...passthroughPropsRef.current,
				...localProps,
				idx: routeIndex + 1,
			});
		};
	}, [routeIndex, routeOutletRenderModel.nextRouteKey]);

	return renderRouteOutletAdapterRenderModel<JSX.Element | null>({
		routeOutletRenderModel,
		renderErrorWithBoundary: (props) => {
			const ErrorBoundaryComponent =
				props.errorComponent as ComponentType<RouteErrorBoundaryProps>;
			return h(ErrorBoundaryComponent, {
				error: props.outermostError,
			});
		},
		renderErrorWithoutBoundary: (props) => {
			return h(
				"span",
				{},
				`Error: ${String(props.outermostError ?? "unknown")}`,
			);
		},
		renderComponent: (props) => {
			return h(RouteOutletComponentMount, {
				key: props.currentRouteMountKey,
				currentComponent: props.currentComponent,
				currentRouteMountKey: props.currentRouteMountKey,
				matchedPattern: props.matchedPattern,
				routeIndex,
				Outlet,
			});
		},
		renderMissingComponent: () => {
			return null;
		},
		renderFallback: (props) => {
			return h(Outlet, {
				key: props.nextRouteKey,
			});
		},
		renderEmpty: () => {
			return null;
		},
	});
}
