import { createPatternRegistry } from "vorma/kit/matcher/register";

export const DIST_TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

type DistGlobalRecord = Record<PropertyKey, unknown>;

export type DistTestVormaInternal = {
	buildID: string;
	matchedPatterns: string[];
	loadersData: unknown[];
	importURLs: string[];
	exportKeys: string[];
	errorExportKeys: string[];
	hasRootData: boolean;
	params: Record<string, string>;
	splatValues: string[];
	activeComponents: unknown[] | null;
	activeErrorBoundary: unknown;
	outermostServerError: unknown;
	outermostClientError: unknown;
	outermostServerErrorIdx: number | undefined;
	outermostClientErrorIdx: number | undefined;
	outermostError: unknown;
	outermostErrorIdx: number | undefined;
	isDev: boolean;
	viteDevURL: string;
	publicPathPrefix: string;
	isTouchDevice: boolean;
	patternToWaitFnMap: Record<string, unknown>;
	clientLoadersData: unknown[];
	defaultErrorBoundary: () => null;
	useViewTransitions: boolean;
	deploymentID: string;
	vormaAppConfig: typeof DIST_TEST_VORMA_APP_CONFIG;
	routeManifestURL: string;
	routeManifest: unknown;
	patternRegistry: unknown;
	runtimeRouteSnapshot?: Record<string, unknown>;
};

export function installDistTestVormaGlobal(): DistTestVormaInternal {
	const patternRegistry = createPatternRegistry({
		dynamicParamPrefixRune: DIST_TEST_VORMA_APP_CONFIG.loadersDynamicRune,
		splatSegmentRune: DIST_TEST_VORMA_APP_CONFIG.loadersSplatRune,
		explicitIndexSegment:
			DIST_TEST_VORMA_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
	});

	const globals = {
		buildID: "1",
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		activeComponents: [],
		activeErrorBoundary: undefined,
		outermostServerError: undefined,
		outermostClientError: undefined,
		outermostServerErrorIdx: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchDevice: false,
		patternToWaitFnMap: {},
		clientLoadersData: [],
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: DIST_TEST_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry,
		runtimeRouteSnapshot: {
			buildID: "1",
			matchedPatterns: [],
			loadersData: [],
			importURLs: [],
			exportKeys: [],
			errorExportKeys: [],
			hasRootData: false,
			params: {},
			splatValues: [],
			activeComponents: [],
			activeErrorBoundary: undefined,
			outermostServerError: undefined,
			outermostClientError: undefined,
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			rootElementID: undefined,
			clientLoadersData: [],
		},
	};

	const symbol = Symbol.for("__vorma_internal__");
	const globalRecord = globalThis as DistGlobalRecord;
	globalRecord[symbol] = globals;
	return globals;
}
