import type { PatternRegistry } from "vorma/kit/matcher/register";
import type { VormaAppConfig } from "./helpers.ts";
import type {
	NavigateProps,
	NavigationStateManager,
} from "../core/navigation/types.ts";

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

	activeComponents: Array<unknown> | null;
	activeErrorBoundary?: unknown;
};

export type GetRouteDataOutput = RouteDataState &
	Meta & {
		deps: Array<string>;
		cssBundles: Array<string>;
	};

export const VORMA_SYMBOL = Symbol.for("__vorma_internal__");

export type RouteErrorComponent = (props: { error: string }) => unknown;

export type ClientLoaderAwaitedServerData<RD, LD> = {
	matchedPatterns: string[];
	loaderData: LD;
	rootData: RD;
	buildID: string;
};

export type PatternWaitFn = (props: {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<ClientLoaderAwaitedServerData<unknown, unknown>>;
	signal: AbortSignal;
}) => Promise<unknown>;

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

export function getRouterData<
	T = unknown,
	P extends Record<string, string> = Record<string, string>,
>() {
	const rootData = (__vormaClientGlobal.get("hasRootData")
		? __vormaClientGlobal.get("loadersData")[0]
		: null) as T;
	return {
		buildID: __vormaClientGlobal.get("buildID") || "",
		matchedPatterns: __vormaClientGlobal.get("matchedPatterns") || [],
		splatValues: __vormaClientGlobal.get("splatValues") || [],
		params: (__vormaClientGlobal.get("params") || {}) as P,
		rootData,
	};
}

export type NavigationStateAccess = Pick<
	NavigationStateManager,
	"navigate" | "removeNavigation" | "getNavigations"
> & {
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

let navigationStateAccess: NavigationStateAccess | null = null;

export function setNavigationStateAccess(access: NavigationStateAccess): void {
	navigationStateAccess = access;
}

export function getNavigationStateAccess(): NavigationStateAccess {
	if (!navigationStateAccess) {
		throw new Error("Navigation state access has not been initialized.");
	}
	return navigationStateAccess;
}
