/**
 * Unstable client internals for framework adapters.
 *
 * This subpath is intentionally not part of the stable semver contract.
 */
export {
	buildInitialRouteOutletStoreState,
	buildNavigationLinkAnchorRenderProps,
	buildTypedAdapterRouteComponentMountProps,
	buildTypedLinkDisplayName,
	createRouteOutletAdapterSyncHost,
	createTypedAdapterLinkFactory,
	createTypedAdapterValueHookFactories,
	createVormaRuntimeContext,
	formatOutermostErrorForRendering,
	getDefaultVormaRuntimeContext,
	makeFinalLinkProps,
	navigationInternalLinkPropKeysForAnchors,
	registerClientLoaderForAdapter,
	renderRouteOutletAdapterRenderModel,
	resolvePath,
	resolveRouteOutletAdapterRenderModel,
	resolveRouteOutletComponentMountKey,
	resolveTypedAdapterClientLoaderDataForPatternOrRouteProps,
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterLoaderDataForRoutePropsOrThrow,
	setDefaultVormaRuntimeContext,
	setRuntimeGlobalStateForDefaultContext,
	shouldRemountRouteOutletComponentMount,
} from "./src/runtime.ts";
export type {
	RouteOutletBranchInputState,
	RouteOutletStoreState,
	TypedAdapterLinkDefaultProps,
	TypedAdapterLinkProps,
	VormaAppConfig,
	VormaLinkPropsBase,
	VormaRuntimeContext,
	VormaTypedAdapterAddClientLoaderProps,
} from "./src/runtime.ts";
