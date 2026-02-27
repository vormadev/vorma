import { computed, signal } from "@preact/signals";
import type { ComponentType, HTMLAttributes, TargetedMouseEvent } from "preact";
import { h } from "preact";
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
import type {
	RouteOutletBranchInputState,
	RouteOutletStoreState,
	TypedAdapterLinkDefaultProps,
	VormaLinkPropsBase,
} from "vorma/client/__internal";
import {
	buildInitialRouteOutletStoreState,
	buildNavigationLinkAnchorRenderProps,
	buildTypedAdapterRouteComponentMountProps,
	createRouteOutletAdapterSyncHost,
	createTypedAdapterLinkFactory,
	createTypedAdapterValueHookFactories,
	renderRouteOutletAdapterRenderModel,
	resolveRouteOutletAdapterRenderModel,
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

const storeState = signal<RouteOutletStoreState>(
	buildInitialRouteOutletStoreState(),
);

const loadersData = computed(() => storeState.value.navigation.loadersData);
const clientLoadersData = computed(
	() => storeState.value.navigation.clientLoadersData,
);
const routerData = computed(() => storeState.value.navigation.routerData);
const outermostError = computed(
	() => storeState.value.navigation.outermostError,
);
const routeOutletBranchInputState = computed<RouteOutletBranchInputState>(
	() => storeState.value.routeOutletBranchInputState,
);

export { clientLoadersData, loadersData, routerData };

export const location = computed(() => storeState.value.location);

const routeOutletAdapterSyncHost = createRouteOutletAdapterSyncHost({
	getCurrentStoreState: () => storeState.value,
	applyNextStoreState: (nextStoreState) => {
		storeState.value = nextStoreState;
	},
});

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

type VormaOutletProps = {
	idx: number;
	Outlet: (localProps: Record<string, any> | undefined) => JSX.Element | null;
};

type VormaErrorBoundaryProps = {
	error: unknown;
};

type VormaRouteComponentMountProps = {
	CurrentComp: ComponentType<VormaOutletProps>;
	idx: number;
	matchedPattern: string;
	Outlet: (localProps: Record<string, any> | undefined) => JSX.Element | null;
};

function VormaRouteComponentMount(props: VormaRouteComponentMountProps) {
	const typedAdapterRouteMountProps = useMemo(() => {
		return buildTypedAdapterRouteComponentMountProps({
			routePropsIndex: props.idx,
			matchedPattern: props.matchedPattern,
		});
	}, [props.idx, props.matchedPattern]);

	return h(props.CurrentComp, {
		idx: props.idx,
		Outlet: props.Outlet,
		...typedAdapterRouteMountProps,
	});
}

export function VormaRootOutlet(props: { idx?: number }): h.JSX.Element | null {
	const idx = props.idx ?? 0;
	const passthroughPropsRef = useRef(props);
	passthroughPropsRef.current = props;

	useLayoutEffect(() => {
		routeOutletAdapterSyncHost.syncRootOutletMount({
			idx,
		});
	}, [idx]);

	const routeOutletRenderModel = resolveRouteOutletAdapterRenderModel({
		routeOutletBranchInputState: routeOutletBranchInputState.value,
		outermostError: outermostError.value,
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
	}, [idx, routeOutletRenderModel.nextRouteKey]);

	return renderRouteOutletAdapterRenderModel<h.JSX.Element | null>({
		routeOutletRenderModel,
		renderErrorWithBoundary: ({ errorComponent, outermostError }) => {
			return h(errorComponent as ComponentType<VormaErrorBoundaryProps>, {
				error: outermostError,
			});
		},
		renderErrorWithoutBoundary: ({ outermostError }) => {
			return h("div", {}, `Error: ${outermostError || "unknown"}`);
		},
		renderComponent: ({
			currentComponent,
			currentRouteKey,
			matchedPattern,
		}) => {
			return h(VormaRouteComponentMount, {
				key: currentRouteKey,
				CurrentComp:
					currentComponent as ComponentType<VormaOutletProps>,
				idx,
				matchedPattern,
				Outlet,
			});
		},
		renderMissingComponent: () => null,
		renderFallback: ({ nextRouteKey }) => {
			return h(Outlet, {
				key: nextRouteKey,
			});
		},
		renderEmpty: () => null,
	});
}

/////////////////////////////////////////////////////////////////////
/////// TYPED HOOKS
/////////////////////////////////////////////////////////////////////

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
		useMatchedPatterns: () => routerData.value.matchedPatterns,
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
/////// LINK APIs
/////////////////////////////////////////////////////////////////////

export function VormaLink(
	props: HTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<TargetedMouseEvent<HTMLAnchorElement>>,
) {
	const anchorRenderProps = buildNavigationLinkAnchorRenderProps(props);

	return h(
		"a",
		{
			"data-external": anchorRenderProps.dataExternal,
			...(anchorRenderProps.safeAnchorProps as any),
			onPointerEnter: anchorRenderProps.onPointerEnter,
			onFocus: anchorRenderProps.onFocus,
			onPointerLeave: anchorRenderProps.onPointerLeave,
			onBlur: anchorRenderProps.onBlur,
			onTouchCancel: anchorRenderProps.onTouchCancel,
			onClick: anchorRenderProps.onClick,
		},
		props.children,
	);
}

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		HTMLAttributes<HTMLAnchorElement>,
		TargetedMouseEvent<HTMLAnchorElement>
	>,
) {
	type App = ExtractApp<C>;

	return createTypedAdapterLinkFactory<
		App,
		HTMLAttributes<HTMLAnchorElement>,
		TargetedMouseEvent<HTMLAnchorElement>,
		h.JSX.Element
	>({
		vormaAppConfig,
		defaultProps,
		renderTypedLink: ({ resolveTypedLinkProps }) => {
			const resolvedProps = resolveTypedLinkProps();
			return h(VormaLink, {
				...resolvedProps.linkProps,
				href: resolvedProps.href,
				state: resolvedProps.state,
			});
		},
	});
}
