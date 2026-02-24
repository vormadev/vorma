import type { PatternRegistry } from "vorma/kit/matcher/register";
import type {
	NavigateProps,
	NavigationStateManager,
} from "../core/navigation/types.ts";
import type { VormaAppConfig } from "./helpers.ts";

/**
 * Serialized head element shape transferred between server/runtime boundaries.
 */
export type HeadEl = {
	tag?: string;
	attributesKnownSafe?: Record<string, string>;
	booleanAttributes?: Array<string>;
	dangerousInnerHTML?: string;
};

type Meta = {
	title: HeadEl | null | undefined;
	metaHeadEls: Array<HeadEl> | null | undefined;
	restHeadEls: Array<HeadEl> | null | undefined;
};

type RouteDataState = {
	outermostServerError?: string;
	outermostServerErrorIdx?: number;

	matchedPatterns: Array<string>;
	loadersData: Array<unknown>;
	importURLs: Array<string>;
	exportKeys: Array<string>;
	errorExportKeys: string[];
	hasRootData: boolean;

	params: Record<string, string>;
	splatValues: Array<string>;
};

type RuntimeRouteState = RouteDataState & {
	outermostClientError?: string;
	outermostClientErrorIdx?: number;
	outermostError?: string;
	outermostErrorIdx?: number;

	buildID: string;
	rootElementID?: string;

	activeComponents: Array<unknown> | null;
	activeErrorBoundary?: unknown;
};

/**
 * Canonical route-data payload shape consumed by adapters and runtime helpers.
 */
export type GetRouteDataOutput = RouteDataState &
	Meta & {
		deps: Array<string>;
		cssBundles: Array<string>;
	};

/**
 * Process-wide symbol key used to store runtime state on globalThis.
 */
export const VORMA_SYMBOL = Symbol.for("__vorma_internal__");

/**
 * Contract for route-level error boundary components.
 */
export type RouteErrorComponent = (props: { error: string }) => unknown;

/**
 * Data returned from server loaders and awaited by client loaders.
 */
export type ClientLoaderAwaitedServerData<RD, LD> = {
	matchedPatterns: string[];
	loaderData: LD;
	rootData: RD;
	buildID: string;
};

/**
 * Client loader wait function contract, keyed by route pattern.
 * Wait functions are speculative: the runtime may invoke them before final
 * navigation ownership checks complete, and their result may be discarded if
 * that navigation is superseded. Implementations should treat side effects as
 * idempotent and honor `signal` for cancellation-aware cleanup.
 */
export type PatternWaitFn = (props: {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<ClientLoaderAwaitedServerData<unknown, unknown>>;
	signal: AbortSignal;
}) => Promise<unknown>;

/**
 * Global runtime state container mounted on `globalThis[VORMA_SYMBOL]`.
 */
export type VormaClientGlobal = RuntimeRouteState & {
	isDev: boolean;
	viteDevURL: string;
	publicPathPrefix: string;
	isTouchDevice: boolean;
	patternToWaitFnMap: Record<string, PatternWaitFn>;
	clientLoadersData: Array<unknown>;
	defaultErrorBoundary: RouteErrorComponent;
	useViewTransitions: boolean;
	deploymentID: string;
	vormaAppConfig: VormaAppConfig;
	routeManifestURL: string;
	routeManifest: Record<string, number> | undefined;
	clientModuleMap: Record<
		string,
		{
			importURL: string;
			exportKey: string;
			errorExportKey: string;
		}
	>;
	patternRegistry: PatternRegistry;
};

type VormaGlobalThis = typeof globalThis & {
	[VORMA_SYMBOL]: VormaClientGlobal;
};

/**
 * Returns typed `get`/`set` accessors for the shared global runtime state.
 */
export function __getVormaClientGlobal() {
	const dangerousGlobalThis = globalThis as VormaGlobalThis;
	function get<K extends keyof VormaClientGlobal>(key: K) {
		return dangerousGlobalThis[VORMA_SYMBOL][key] as VormaClientGlobal[K];
	}
	function set<
		K extends keyof VormaClientGlobal,
		V extends VormaClientGlobal[K],
	>(key: K, value: V) {
		dangerousGlobalThis[VORMA_SYMBOL][key] = value;
	}
	return { get, set };
}

export const __vormaClientGlobal = __getVormaClientGlobal();

/**
 * Returns router data snapshot for application consumption.
 */
export function getRouterData<
	T = unknown,
	P extends Record<string, string> = Record<string, string>,
>() {
	const rootData = (
		__vormaClientGlobal.get("hasRootData")
			? __vormaClientGlobal.get("loadersData")[0]
			: null
	) as T;
	return {
		buildID: __vormaClientGlobal.get("buildID") || "",
		matchedPatterns: __vormaClientGlobal.get("matchedPatterns") || [],
		splatValues: __vormaClientGlobal.get("splatValues") || [],
		params: (__vormaClientGlobal.get("params") || {}) as P,
		rootData,
	};
}

export type ClientRuntimeRenderState = Pick<
	VormaClientGlobal,
	| "loadersData"
	| "clientLoadersData"
	| "outermostError"
	| "outermostErrorIdx"
	| "activeComponents"
	| "activeErrorBoundary"
	| "importURLs"
	| "exportKeys"
>;

/**
 * Returns render-state fields required by route outlet runtime reconciliation.
 */
export function getClientRuntimeRenderState(): ClientRuntimeRenderState {
	return {
		loadersData: __vormaClientGlobal.get("loadersData"),
		clientLoadersData: __vormaClientGlobal.get("clientLoadersData"),
		outermostError: __vormaClientGlobal.get("outermostError"),
		outermostErrorIdx: __vormaClientGlobal.get("outermostErrorIdx"),
		activeComponents: __vormaClientGlobal.get("activeComponents"),
		activeErrorBoundary: __vormaClientGlobal.get("activeErrorBoundary"),
		importURLs: __vormaClientGlobal.get("importURLs"),
		exportKeys: __vormaClientGlobal.get("exportKeys"),
	};
}

/**
 * Registers a client loader wait function for a route pattern.
 */
export function setClientLoaderWaitFn(
	pattern: string,
	waitFn: PatternWaitFn,
): void {
	const currentMap = __vormaClientGlobal.get("patternToWaitFnMap") || {};
	__vormaClientGlobal.set("patternToWaitFnMap", {
		...currentMap,
		[pattern]: waitFn,
	});
}

/**
 * Minimal navigation runtime surface exposed to modules that should not depend
 * on the full navigation manager implementation.
 */
export type NavigationStateAccess = Pick<
	NavigationStateManager,
	"navigate" | "removeNavigation" | "getNavigations"
> & {
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

let navigationStateAccess: NavigationStateAccess | null = null;

/**
 * Sets the shared navigation runtime access object.
 */
export function setNavigationStateAccess(access: NavigationStateAccess): void {
	navigationStateAccess = access;
}

/**
 * Returns the shared navigation runtime access object.
 */
export function getNavigationStateAccess(): NavigationStateAccess {
	if (!navigationStateAccess) {
		throw new Error("Navigation state access has not been initialized.");
	}
	return navigationStateAccess;
}
