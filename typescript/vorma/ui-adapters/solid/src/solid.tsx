/// <reference types="vite/client" />

import {
	Show,
	createMemo,
	createSignal,
	onCleanup,
	onMount,
	type Accessor,
	type JSX,
	type ValidComponent,
} from "solid-js";
import { Dynamic } from "solid-js/web";
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
import {
	buildInitialRouteOutletStoreState,
	buildNavigationLinkAnchorRenderProps,
	buildTypedAdapterRouteComponentMountProps,
	createRouteOutletAdapterSyncHost,
	createTypedAdapterLinkFactory,
	createTypedAdapterValueHookFactories,
	formatOutermostErrorForRendering,
	renderRouteOutletAdapterRenderModel,
	resolveRouteOutletAdapterRenderModel,
	type RouteOutletStoreState,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
	type VormaLinkPropsBase,
	type VormaTypedAdapterAddClientLoaderProps,
} from "vorma/client/__internal";

/////////////////////////////////////////////////////////////////////
/////// Store
/////////////////////////////////////////////////////////////////////

const [routeOutletStoreState, setRouteOutletStoreState] = createSignal<
	RouteOutletStoreState | undefined
>(undefined);

function getRouteOutletStoreStateOrInitialize(): RouteOutletStoreState {
	const currentStoreState = routeOutletStoreState();
	if (currentStoreState !== undefined) {
		return currentStoreState;
	}
	const initialStoreState = buildInitialRouteOutletStoreState();
	setRouteOutletStoreState(initialStoreState);
	return initialStoreState;
}

const routeOutletAdapterSyncHost = createRouteOutletAdapterSyncHost({
	getCurrentStoreState: getRouteOutletStoreStateOrInitialize,
	applyNextStoreState: (nextStoreState) => {
		if (nextStoreState !== routeOutletStoreState()) {
			setRouteOutletStoreState(nextStoreState);
		}
	},
});

function readLoadersData(): unknown[] {
	return getRouteOutletStoreStateOrInitialize().navigation.loadersData;
}

function readClientLoadersData(): unknown[] {
	return getRouteOutletStoreStateOrInitialize().navigation.clientLoadersData;
}

function readRouterData() {
	return getRouteOutletStoreStateOrInitialize().navigation.routerData;
}

function readMatchedPatterns(): string[] {
	return getRouteOutletStoreStateOrInitialize().routeOutletBranchInputState
		.matchedPatterns;
}

function readOutermostError(): unknown {
	return getRouteOutletStoreStateOrInitialize().navigation.outermostError;
}

function readRouteOutletBranchInputState() {
	return getRouteOutletStoreStateOrInitialize().routeOutletBranchInputState;
}

export const loadersData = readLoadersData;
export const clientLoadersData = readClientLoadersData;
export const routerData = readRouterData;
export const location = () => getRouteOutletStoreStateOrInitialize().location;

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
		return createMemo(() => readRouterData());
	}) as UseRouterDataFunction<App, true>;
}

function createSolidTypedAdapterValueHookFactories<
	App extends ExtractApp<VormaAppConfig>,
>() {
	return createTypedAdapterValueHookFactories<App>({
		useLoadersData: readLoadersData,
		useMatchedPatterns: readMatchedPatterns,
		useClientLoadersData: readClientLoadersData,
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
		routeProps: VormaRouteProps<App, Pattern>,
	): Accessor<VormaLoaderOutput<App, Pattern>> {
		return createMemo(() => {
			return useLoaderDataValue(routeProps);
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
		return createMemo(() => {
			return usePatternLoaderDataValue(pattern);
		});
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
		ResultData,
	>(
		addClientLoaderProps: VormaTypedAdapterAddClientLoaderProps<
			App,
			Pattern,
			LoaderData,
			ResultData
		>,
	) {
		const useClientLoaderDataValue =
			addClientLoaderValue(addClientLoaderProps);
		const useClientLoaderData = (
			routeProps?: VormaRouteProps<App, Pattern>,
		): Accessor<ResultData | undefined> => {
			return createMemo(() => {
				if (routeProps === undefined) {
					return useClientLoaderDataValue();
				}
				return useClientLoaderDataValue(routeProps);
			});
		};
		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Accessor<ResultData>;
			(): Accessor<ResultData | undefined>;
		};
	};
}

/////////////////////////////////////////////////////////////////////
/////// Link
/////////////////////////////////////////////////////////////////////

type SolidLinkEvent = Event;

export function VormaLink(
	linkProps: JSX.AnchorHTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<SolidLinkEvent>,
) {
	const anchorRenderProps = createMemo(() => {
		return buildNavigationLinkAnchorRenderProps(linkProps);
	});
	const safeAnchorProps = createMemo(() => {
		return anchorRenderProps()
			.safeAnchorProps as JSX.AnchorHTMLAttributes<HTMLAnchorElement>;
	});
	return (
		<a
			data-external={anchorRenderProps().dataExternal}
			{...safeAnchorProps()}
			onPointerEnter={anchorRenderProps().onPointerEnter}
			onFocus={anchorRenderProps().onFocus}
			onPointerLeave={anchorRenderProps().onPointerLeave}
			onBlur={anchorRenderProps().onBlur}
			onTouchCancel={anchorRenderProps().onTouchCancel}
			onClick={anchorRenderProps().onClick}
		>
			{linkProps.children}
		</a>
	);
}

type SolidTypedLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<
	App,
	Pattern,
	JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
	SolidLinkEvent
>;

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
		SolidLinkEvent
	>,
) {
	type App = ExtractApp<C>;
	return createTypedAdapterLinkFactory<
		App,
		JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
		SolidLinkEvent,
		JSX.Element
	>({
		vormaAppConfig,
		defaultProps,
		renderTypedLink: (renderProps) => {
			const resolvedTypedLinkProps = createMemo(() => {
				return renderProps.resolveTypedLinkProps();
			});
			return (
				<VormaLink
					{...(resolvedTypedLinkProps()
						.linkProps as JSX.AnchorHTMLAttributes<HTMLAnchorElement>)}
					href={resolvedTypedLinkProps().href}
					state={resolvedTypedLinkProps().state}
				/>
			);
		},
	}) as <Pattern extends VormaLoaderPattern<App>>(
		linkProps: SolidTypedLinkProps<App, Pattern>,
	) => JSX.Element;
}

/////////////////////////////////////////////////////////////////////
/////// Root Outlet
/////////////////////////////////////////////////////////////////////

function RouteOutletComponentMount(props: {
	currentRouteMountKey: string;
	currentRouteComponent: unknown;
	currentMatchedPattern: string;
	routeIndex: number;
	Outlet: (localProps?: Record<string, unknown>) => JSX.Element;
}): JSX.Element {
	const mountedRouteOutletElement = createMemo(() => {
		const currentRouteMountKey = props.currentRouteMountKey;
		const currentRouteComponent =
			props.currentRouteComponent as ValidComponent;
		const routeIndex = props.routeIndex;
		const Outlet = props.Outlet;
		const routeComponentMountProps =
			buildTypedAdapterRouteComponentMountProps({
				routePropsIndex: routeIndex,
				matchedPattern: props.currentMatchedPattern,
			});
		return (
			<Show when={currentRouteMountKey} keyed>
				<Dynamic
					component={currentRouteComponent}
					idx={routeIndex}
					Outlet={Outlet}
					{...routeComponentMountProps}
				/>
			</Show>
		);
	});
	return <>{mountedRouteOutletElement()}</>;
}

function areRouteOutletRenderModelsEquivalent(
	previousModel: ReturnType<typeof resolveRouteOutletAdapterRenderModel>,
	nextModel: ReturnType<typeof resolveRouteOutletAdapterRenderModel>,
): boolean {
	if (previousModel.renderKind !== nextModel.renderKind) {
		return false;
	}
	if (previousModel.renderKind === "component") {
		return (
			nextModel.renderKind === "component" &&
			previousModel.currentRouteMountKey ===
				nextModel.currentRouteMountKey &&
			previousModel.currentComponent === nextModel.currentComponent
		);
	}
	if (previousModel.renderKind === "error") {
		return (
			nextModel.renderKind === "error" &&
			previousModel.currentRouteKey === nextModel.currentRouteKey &&
			previousModel.nextRouteKey === nextModel.nextRouteKey &&
			previousModel.errorComponent === nextModel.errorComponent &&
			previousModel.outermostError === nextModel.outermostError
		);
	}
	if (previousModel.renderKind === "missing-component") {
		return (
			nextModel.renderKind === "missing-component" &&
			previousModel.currentRouteKey === nextModel.currentRouteKey &&
			previousModel.nextRouteKey === nextModel.nextRouteKey
		);
	}
	if (previousModel.renderKind === "fallback") {
		return (
			nextModel.renderKind === "fallback" &&
			previousModel.nextRouteKey === nextModel.nextRouteKey
		);
	}
	return nextModel.renderKind === "empty";
}

export function VormaRootOutlet(
	rootOutletProps: { idx?: number } & Record<string, unknown>,
): JSX.Element {
	const routeIndex = rootOutletProps.idx ?? 0;
	getRouteOutletStoreStateOrInitialize();

	onMount(() => {
		if (routeIndex !== 0) {
			return;
		}
		routeOutletAdapterSyncHost.syncRootOutletMount({
			idx: routeIndex,
		});
		onCleanup(() => {
			routeOutletAdapterSyncHost.syncRootOutletUnmount();
		});
	});

	const routeOutletRenderModel = createMemo(
		() => {
			return resolveRouteOutletAdapterRenderModel({
				routeOutletBranchInputState: readRouteOutletBranchInputState(),
				outermostError: readOutermostError(),
				idx: routeIndex,
			});
		},
		undefined,
		{
			equals: areRouteOutletRenderModelsEquivalent,
		},
	);
	const Outlet = createMemo(() => {
		const nextRouteKey = routeOutletRenderModel().nextRouteKey;
		void nextRouteKey;
		return (localProps?: Record<string, unknown>): JSX.Element => {
			return (
				<VormaRootOutlet
					{...rootOutletProps}
					{...localProps}
					idx={routeIndex + 1}
				/>
			);
		};
	});
	const renderedRootOutletBranch = createMemo(() => {
		return renderRouteOutletAdapterRenderModel<JSX.Element>({
			routeOutletRenderModel: routeOutletRenderModel(),
			renderErrorWithBoundary: (props) => {
				return (
					<Dynamic
						component={props.errorComponent as ValidComponent}
						error={props.outermostError}
					/>
				);
			},
			renderErrorWithoutBoundary: (props) => {
				return (
					<>
						{formatOutermostErrorForRendering(props.outermostError)}
					</>
				);
			},
			renderComponent: (props) => {
				return (
					<RouteOutletComponentMount
						currentRouteMountKey={props.currentRouteMountKey}
						currentRouteComponent={props.currentComponent}
						currentMatchedPattern={props.matchedPattern}
						routeIndex={routeIndex}
						Outlet={Outlet()}
					/>
				);
			},
			renderMissingComponent: () => {
				return <></>;
			},
			renderFallback: (props) => {
				return (
					<Show when={props.nextRouteKey} keyed>
						{Outlet()()}
					</Show>
				);
			},
			renderEmpty: () => {
				return <></>;
			},
		});
	});

	return <>{renderedRootOutletBranch()}</>;
}
