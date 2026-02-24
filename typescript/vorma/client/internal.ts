/**
 * Unstable client internals for framework adapters.
 *
 * This subpath is intentionally not part of the stable semver contract.
 */
export {
	getClientRuntimeRenderState,
	setClientLoaderWaitFn,
} from "./src/app/context.ts";
export { resolvePath, type VormaAppConfig } from "./src/app/helpers.ts";
export { __registerClientLoaderForAdapter as registerClientLoaderForAdapter } from "./src/core/extras.ts";
export { applyScrollState } from "./src/platform/scroll.ts";
export {
	makeFinalLinkProps,
	resolveTypedLinkHref,
	type UseRouterDataFunction,
	type VormaLinkPropsBase,
} from "./src/ui/helpers.ts";
export { createRouteOutletRuntimeListenerInitializer } from "./src/ui/route_outlet_listener_runtime.ts";
export {
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildCurrentRouteOutletLocationState,
	buildInitialRouteOutletNavigationState,
	buildInitialRouteOutletStoreState,
	buildNextRouteOutletNavigationState,
	buildNextRouteOutletStoreStateFromRuntime,
	buildRouteOutletBranchInputState,
	buildRouteOutletBranchState,
	resolveRouteOutletBranchRenderState,
	shouldRemountRouteOutletComponentMount,
	syncRouteOutletStoreStateFromRuntime,
	type RouteOutletBranchInputState,
	type RouteOutletBranchRenderState,
	type RouteOutletBranchState,
	type RouteOutletLocationState,
	type RouteOutletNavigationState,
	type RouteOutletStoreState,
} from "./src/ui/route_outlet_runtime.ts";
export {
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterIndexedDataForPatternOrRouteProps,
	type VormaTypedAdapterAddClientLoaderProps,
} from "./src/ui/typed_adapter_helpers_runtime.ts";
export {
	buildTypedLinkDisplayName,
	buildTypedLinkHrefForRouteResolution,
	buildTypedLinkResolvedProps,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
} from "./src/ui/typed_adapter_link_props.ts";
export { mergeTypedAdapterLinkPropsWithDefaults } from "./src/ui/typed_adapter_link_runtime.ts";
