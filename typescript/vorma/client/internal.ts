export { __registerClientLoaderForAdapter as registerClientLoaderForAdapter } from "./src/core/extras.ts";
export { applyScrollState } from "./src/platform/scroll.ts";
export {
	makeFinalLinkProps,
	resolveTypedLinkHref,
	type VormaLinkPropsBase,
	type UseRouterDataFunction,
} from "./src/ui/helpers.ts";
export { resolvePath, type VormaAppConfig } from "./src/app/helpers.ts";
export {
	getClientRuntimeRenderState,
	setClientLoaderWaitFn,
} from "./src/app/context.ts";
export {
	areRouteOutletBranchInputsEqualByIdentity,
	areRouteOutletLocationsEqual,
	buildRouteOutletBranchInputState,
	buildCurrentRouteOutletLocationState,
	buildInitialRouteOutletNavigationState,
	buildNextRouteOutletNavigationState,
	buildRouteOutletBranchState,
	type RouteOutletBranchInputState,
	type RouteOutletBranchState,
	type RouteOutletLocationState,
	type RouteOutletNavigationState,
} from "./src/ui/route_outlet_runtime.ts";
