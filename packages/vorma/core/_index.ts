/*
The `vorma/__internal` entry point: the framework-agnostic core every
official UI adapter (`vorma/react`, `vorma/preact`, `vorma/solid`) is built
from, re-exported here as a single barrel. The `__` prefix is deliberate —
this surface is internal, unstable, and undocumented by convention (see
`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`); it
exists so the three adapters share one implementation, not as a
general-purpose public API. Each re-exported item's real documentation
lives at its own definition (imported below) rather than being duplicated
here — this file has no logic and no docs of its own to add. A custom
adapter for a UI framework Vorma does not ship officially is the intended,
if unsupported, consumer of this module.
*/
export { MutationError, QueryError, create_typed_api_client } from "./api_client.ts";
export type { ClientMatcher, ClientMatcherNestedMatch } from "./client_wasm/matcher.ts";
export {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	LINK_ACTIVE_ANCESTOR_ATTR,
	LINK_ACTIVE_EXACT_ATTR,
	LINK_PENDING_ANCESTOR_ATTR,
	LINK_PENDING_EXACT_ATTR,
} from "./constants.ts";
export {
	apply_scroll,
	create_client_core,
	create_empty_work_state,
	make_entry_id,
	type BuildSkewDetectedEvent,
	type ClientCommit,
	type ClientOptions,
	type CommitFn,
	type RevalidationReason,
	type RouteRenderEntry,
	type RouteRenderState,
	type ScrollIntent,
	type ScrollState,
	type ViewDefinition,
	type WorkIndicator,
	type WorkIndicatorOptions,
	type WorkState,
} from "./create_client_core.ts";
export {
	make_link_props,
	select_link_route_state,
	select_link_work_state,
	type LinkNavFns,
	type LinkPropsResult,
	type LinkRouteState,
	type LinkWorkState,
} from "./make_link_props.ts";
export { get_entry_key, type OutletSlot } from "./resolve_outlet_slot.ts";
export type {
	ApiClientOutput,
	AppConfig,
	BeforeRouteCommitFn,
	BeforeRouteTransitionArgs,
	BeforeRouteYieldFn,
	ClientLoaderKnownMatch,
	ClientLoaderServerState,
	MutationResult,
	QueryResult,
	RevalidationResult,
	RouteErrorState,
	RouteMatchState,
	RouteState,
	RouteUpdateReason,
	ToApiClient,
	ToApiDecorator,
	ToApiDecoratorContext,
	ToClientLoaderArgs,
	ToDefineViewArgs,
	ToLinkProps,
	ToMutationArgs,
	ToMutationError,
	ToMutationInput,
	ToMutationMethod,
	ToMutationOutput,
	ToMutationPattern,
	ToNavigateArgs,
	ToNavigationTarget,
	ToQueryArgs,
	ToQueryError,
	ToQueryInput,
	ToQueryMethod,
	ToQueryOutput,
	ToQueryPattern,
	ToRouteDestination,
	ToRouteSyncArgs,
	ToViewComponentProps,
	ToViewInput,
	ToViewOutput,
	ToViewPattern,
} from "./types.ts";
export {
	create_adapter_base,
	set_client_matcher_factory_for_test,
	type AdapterClientOptions,
	type AdapterRenderArgs,
	type DecomposedCommit,
	type DecomposedCommitFn,
	type DecomposedState,
	type VormaClient,
} from "./ui_adapter_core.ts";
export {
	build_resource_url,
	create_typed_navigate,
	create_typed_prefetch,
	create_typed_to_href,
	resolve_body,
	resolve_path,
	to_typed_href,
} from "./url.ts";
