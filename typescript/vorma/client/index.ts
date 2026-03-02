/**
 * Stable public runtime client API.
 *
 * Unstable adapter/runtime internals are exported from `vorma/client/__internal`.
 */
/**
 * Stable public runtime client API.
 *
 * Unstable adapter/runtime internals are exported from `vorma/client/__internal`.
 */
export {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	buildMutationURL,
	buildQueryURL,
	defaultErrorBoundary,
	getBuildID,
	getHistoryInstance,
	getLocation,
	getRootEl,
	getRouterData,
	getStatus,
	initClient,
	makeTypedAPIClient,
	makeTypedNavigate,
	resolveBody,
	revalidate,
	revalidateOnWindowFocus,
	setupGlobalLoadingIndicator,
	submit,
	vormaNavigate,
} from "./src/runtime.ts";
export type {
	APIRequestInitDecorator,
	APIRequestInitDecoratorContext,
	APIRequestInitOverrides,
	ClientLoaderAwaitedServerData,
	ExtractApp,
	ParamsForPattern,
	PermissivePatternBasedProps,
	RouteChangeEvent,
	RouteChangeEventDetail,
	StatusEvent,
	StatusEventDetail,
	SubmitOptions,
	SubmitResult,
	TypedAPIClient,
	UseRouterDataFunction,
	VormaAppBase,
	VormaAppConfig,
	VormaLinkPropsBase,
	VormaLoaderOutput,
	VormaLoaderPattern,
	VormaMutationInput,
	VormaMutationOutput,
	VormaMutationPattern,
	VormaMutationProps,
	VormaQueryInput,
	VormaQueryOutput,
	VormaQueryPattern,
	VormaQueryProps,
	VormaRouteGeneric,
	VormaRoutePropsGeneric,
} from "./src/runtime.ts";
