export {
	getBuildID,
	getHistoryInstance,
	getLocation,
	getRootEl,
	getStatus,
	revalidate,
	submit,
	vormaNavigate,
	type SubmitOptions,
} from "./src/client.ts";
// Internal cross-package integration hooks for sibling adapters.
// `__*` exports are intentionally unstable and not end-user APIs.
export { registerClientLoaderPattern as __registerClientLoaderPattern } from "./src/core/render_runtime.ts";
export { defaultErrorBoundary } from "./src/ui/helpers.ts";
export {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	type RouteChangeEvent,
	type StatusEvent,
} from "./src/platform/events.ts";
export {
	revalidateOnWindowFocus,
	setupGlobalLoadingIndicator,
} from "./src/core/extras.ts";
export { runClientLoadersAfterHMRUpdate as __runClientLoadersAfterHMRUpdate } from "./src/core/extras.ts";
export { __registerClientLoaderForAdapter } from "./src/core/extras.ts";
export { initClient } from "./src/app/init.ts";
export { applyScrollState as __applyScrollState } from "./src/platform/scroll.ts";
export {
	makeFinalLinkProps as __makeFinalLinkProps,
	type VormaLinkPropsBase,
} from "./src/ui/helpers.ts";
export {
	type ParamsForPattern,
	type UseRouterDataFunction,
	type VormaRouteGeneric,
} from "./src/ui/helpers.ts";
export { makeTypedNavigate } from "./src/ui/helpers.ts";
export {
	buildMutationURL,
	buildQueryURL,
	resolvePath as __resolvePath,
	resolveBody,
	type ExtractApp,
	type PermissivePatternBasedProps,
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
export {
	getClientRuntimeRenderState as __getClientRuntimeRenderState,
	getRouterData,
	setClientLoaderWaitFn as __setClientLoaderWaitFn,
	type ClientLoaderAwaitedServerData,
} from "./src/app/context.ts";
