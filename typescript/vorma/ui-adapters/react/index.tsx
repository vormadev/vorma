import type { ComponentProps, ComponentType, JSX, MouseEvent } from "react";
import {
	memo,
	useLayoutEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
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
import type {
	RouteOutletStoreState,
	TypedAdapterLinkDefaultProps,
	TypedAdapterLinkProps,
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

let state = buildInitialRouteOutletStoreState();
const listeners = new Set<() => void>();

const store = {
	getSnapshot: (): RouteOutletStoreState => state,
	subscribe(listener: () => void): () => void {
		listeners.add(listener);
		return () => {
			listeners.delete(listener);
		};
	},
	setState(
		updater: (prev: RouteOutletStoreState) => RouteOutletStoreState,
	): void {
		const nextState = updater(state);
		if (nextState !== state) {
			state = nextState;
			listeners.forEach((listener) => {
				listener();
			});
		}
	},
};

function useStoreSelector<T>(selector: (state: RouteOutletStoreState) => T): T {
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

const routeOutletAdapterSyncHost = createRouteOutletAdapterSyncHost({
	getCurrentStoreState: store.getSnapshot,
	applyNextStoreState: (nextStoreState) => {
		store.setState(() => nextStoreState);
	},
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
	matchedPattern: string;
	Outlet: (localProps: Record<string, any> | undefined) => JSX.Element;
};

function VormaRouteComponentMount(
	props: VormaRouteComponentMountProps,
): JSX.Element {
	const typedAdapterRouteMountProps = useMemo(() => {
		return buildTypedAdapterRouteComponentMountProps({
			routePropsIndex: props.idx,
			matchedPattern: props.matchedPattern,
		});
	}, [props.idx, props.matchedPattern]);

	return (
		<props.CurrentComp
			idx={props.idx}
			Outlet={props.Outlet}
			{...typedAdapterRouteMountProps}
		/>
	);
}

export function VormaRootOutlet(props: { idx?: number }): JSX.Element {
	const idx = props.idx ?? 0;
	const passthroughPropsRef = useRef(props);
	passthroughPropsRef.current = props;

	useLayoutEffect(() => {
		routeOutletAdapterSyncHost.syncRootOutletMount({
			idx,
		});
	}, [idx]);

	const routeOutletBranchInputState = useStoreSelector(
		(storeState) => storeState.routeOutletBranchInputState,
	);
	const outermostErrorFromStore = useStoreSelector(
		(storeState) => storeState.navigation.outermostError,
	);
	const routeOutletRenderModel = resolveRouteOutletAdapterRenderModel({
		routeOutletBranchInputState,
		outermostError: outermostErrorFromStore,
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
	}, [idx, routeOutletRenderModel.nextRouteKey]);

	return renderRouteOutletAdapterRenderModel<JSX.Element>({
		routeOutletRenderModel,
		renderErrorWithBoundary: ({ errorComponent, outermostError }) => {
			const ErrorComp =
				errorComponent as ComponentType<VormaErrorBoundaryProps>;
			return <ErrorComp error={outermostError} />;
		},
		renderErrorWithoutBoundary: ({ outermostError }) => {
			return <>{`Error: ${outermostError || "unknown"}`}</>;
		},
		renderComponent: ({
			currentComponent,
			currentRouteKey,
			matchedPattern,
		}) => {
			const CurrentComp =
				currentComponent as ComponentType<VormaOutletProps>;
			return (
				<VormaRouteComponentMount
					key={currentRouteKey}
					CurrentComp={CurrentComp}
					idx={idx}
					matchedPattern={matchedPattern}
					Outlet={Outlet}
				/>
			);
		},
		renderMissingComponent: () => <></>,
		renderFallback: ({ nextRouteKey }) => <Outlet key={nextRouteKey} />,
		renderEmpty: () => <></>,
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
	return useRouterData as UseRouterDataFunction<App, false>;
}

function createReactTypedAdapterValueHookFactories<
	App extends ExtractApp<VormaAppConfig>,
>() {
	return createTypedAdapterValueHookFactories<App>({
		useLoadersData,
		useMatchedPatterns: () => useRouterData().matchedPatterns,
		useClientLoadersData,
		useMemoizedPatternLoaderData: ({
			resolvePatternLoaderData,
			dependencies,
		}) => {
			return useMemo(resolvePatternLoaderData, dependencies);
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
/////// LINK APIs
/////////////////////////////////////////////////////////////////////

export function VormaLink(
	props: ComponentProps<"a"> &
		VormaLinkPropsBase<
			MouseEvent<HTMLAnchorElement, globalThis.MouseEvent>
		>,
) {
	const anchorRenderProps = buildNavigationLinkAnchorRenderProps(props);

	return (
		<a
			data-external={anchorRenderProps.dataExternal}
			{...(anchorRenderProps.safeAnchorProps as any)}
			onPointerEnter={anchorRenderProps.onPointerEnter}
			onFocus={anchorRenderProps.onFocus}
			onPointerLeave={anchorRenderProps.onPointerLeave}
			onBlur={anchorRenderProps.onBlur}
			onTouchCancel={anchorRenderProps.onTouchCancel}
			onClick={anchorRenderProps.onClick}
		>
			{props.children}
		</a>
	);
}

type TypedVormaLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<
	App,
	Pattern,
	ComponentProps<"a">,
	MouseEvent<HTMLAnchorElement, globalThis.MouseEvent>
>;

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		ComponentProps<"a">,
		MouseEvent<HTMLAnchorElement, globalThis.MouseEvent>
	>,
) {
	type App = ExtractApp<C>;

	const TypedLink = createTypedAdapterLinkFactory<
		App,
		ComponentProps<"a">,
		MouseEvent<HTMLAnchorElement, globalThis.MouseEvent>,
		JSX.Element
	>({
		vormaAppConfig,
		defaultProps,
		renderTypedLink: ({ resolveTypedLinkProps }) => {
			const resolvedProps = resolveTypedLinkProps();
			return (
				<VormaLink
					{...resolvedProps.linkProps}
					href={resolvedProps.href}
					state={resolvedProps.state}
				/>
			);
		},
	});

	const MemoizedTypedLink = memo(TypedLink) as <
		Pattern extends VormaLoaderPattern<App>,
	>(
		props: TypedVormaLinkProps<App, Pattern>,
	) => JSX.Element;

	(MemoizedTypedLink as any).displayName = (TypedLink as any).displayName;

	return MemoizedTypedLink;
}
