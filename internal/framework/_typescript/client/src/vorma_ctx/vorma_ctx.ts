import type { PatternRegistry } from "vorma/kit/matcher/register";
import type { VormaAppConfig } from "../vorma_app_helpers/vorma_app_helpers.ts";

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

type shared = {
	outermostServerError?: string;
	outermostClientError?: string;
	outermostServerErrorIdx?: number;
	outermostClientErrorIdx?: number;
	outermostError?: string; // derived from above
	outermostErrorIdx?: number; // derived from above

	matchedPatterns: Array<string>;
	loadersData: Array<any>;
	importURLs: Array<string>;
	exportKeys: Array<string>;
	errorExportKeys: string[];
	hasRootData: boolean;

	params: Record<string, string>;
	splatValues: Array<string>;

	buildID: string;

	activeComponents: Array<any> | null;
	activeErrorBoundary?: any;
};

export type GetRouteDataOutput = Omit<shared, "buildID"> &
	Meta & {
		deps: Array<string>;
		cssBundles: Array<string>;
	};

export const VORMA_SYMBOL = Symbol.for("__vorma_internal__");

export type RouteErrorComponent = (props: { error: string }) => any;

export type ClientLoaderAwaitedServerData<RD, LD> = {
	matchedPatterns: string[];
	loaderData: LD;
	rootData: RD;
	buildID: string;
};

export type VormaClientGlobal = shared & {
	isDev: boolean;
	viteDevURL: string;
	publicPathPrefix: string;
	isTouchDevice: boolean;
	patternToWaitFnMap: Record<
		string,
		(props: {
			params: Record<string, string>;
			splatValues: string[];
			serverDataPromise: Promise<ClientLoaderAwaitedServerData<any, any>>;
			signal: AbortSignal;
		}) => Promise<any>
	>;
	clientLoadersData: Array<any>;
	defaultErrorBoundary: RouteErrorComponent;
	useViewTransitions: boolean;
	deploymentID: string;
	vormaAppConfig: VormaAppConfig;
	// SSR'd
	routeManifestURL: string;
	// Fetched at startup -- fine because progressive enhancement
	// and not needed until any given route's second navigation
	// anyway
	routeManifest: Record<string, number> | undefined;
	// built up as we navigate
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

export function __getVormaClientGlobal() {
	const dangerousGlobalThis = globalThis as any;
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

// to debug ctx in browser, paste this:
// const vorma_ctx = window[Symbol.for("__vorma_internal__")];

export function getRouterData<
	T = any,
	P extends Record<string, string> = Record<string, string>,
>() {
	const rootData: T = __vormaClientGlobal.get("hasRootData")
		? __vormaClientGlobal.get("loadersData")[0]
		: null;
	return {
		buildID: __vormaClientGlobal.get("buildID") || "",
		matchedPatterns: __vormaClientGlobal.get("matchedPatterns") || [],
		splatValues: __vormaClientGlobal.get("splatValues") || [],
		params: (__vormaClientGlobal.get("params") || {}) as P,
		rootData,
	};
}
