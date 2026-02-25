/**
 * Stable public runtime client API.
 *
 * Unstable adapter/runtime internals are exported from `vorma/client/__internal`.
 */
export {
	getRouterData,
	type ClientLoaderAwaitedServerData,
} from "./src/app/context.ts";
export {
	buildMutationURL,
	buildQueryURL,
	makeTypedAPIClient,
	resolveBody,
	type APIRequestInitDecorator,
	type APIRequestInitDecoratorContext,
	type APIRequestInitOverrides,
	type ExtractApp,
	type PermissivePatternBasedProps,
	type TypedAPIClient,
	type VormaAppBase,
	type VormaAppConfig,
	type VormaLoaderOutput,
	type VormaLoaderPattern,
	type VormaMutationInput,
	type VormaMutationOutput,
	type VormaMutationPattern,
	type VormaMutationProps,
	type VormaQueryInput,
	type VormaQueryOutput,
	type VormaQueryPattern,
	type VormaQueryProps,
	type VormaRoutePropsGeneric,
} from "./src/app/helpers.ts";
export { initClient } from "./src/app/init.ts";
export {
	getBuildID,
	getLocation,
	getRootEl,
	getStatus,
	getUnsafeHistoryInstance,
	revalidate,
	submit,
	vormaNavigate,
	type SubmitOptions,
	type SubmitResult,
} from "./src/client.ts";
export {
	revalidateOnWindowFocus,
	setupGlobalLoadingIndicator,
} from "./src/core/extras.ts";
export {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	type RouteChangeEvent,
	type RouteChangeEventDetail,
	type StatusEvent,
	type StatusEventDetail,
} from "./src/platform/events.ts";
export {
	defaultErrorBoundary,
	makeTypedNavigate,
	type ParamsForPattern,
	type UseRouterDataFunction,
	type VormaLinkPropsBase,
	type VormaRouteGeneric,
} from "./src/ui/helpers.ts";
