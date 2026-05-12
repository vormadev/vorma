import {
	create_core6_api_submit_owner,
	type Core6APISubmitHost,
	type Core6APISubmitOwner,
} from "./api_submit.ts";
import {
	create_core6_route_boot_owner,
	type Core6RouteBootOwner,
} from "./route_boot.ts";
import {
	create_core6_route_navigation_owner,
	type Core6RouteNavigationOwner,
	type Core6RouteNavigationOwnerHost,
} from "./route_navigation_owner.ts";
import {
	create_core6_route_popstate_owner,
	type Core6RoutePopstateHost,
	type Core6RoutePopstateOwner,
	type Core6RoutePopstatePosition,
} from "./route_popstate.ts";
import {
	create_core6_route_prefetch_owner,
	type Core6RoutePrefetchHost,
	type Core6RoutePrefetchOwner,
} from "./route_prefetch.ts";
import {
	create_core6_route_revalidation_owner,
	type Core6RouteRevalidationConfig,
	type Core6RouteRevalidationOwner,
	type Core6RouteRevalidationTimerHost,
} from "./route_revalidation.ts";
import {
	create_core6_route_runtime,
	type Core6RouteRuntime,
	type Core6RouteRuntimeHost,
} from "./route_runtime.ts";

export type Core6ClientCompositionHost = Core6RouteRuntimeHost &
	Core6RouteNavigationOwnerHost &
	Core6RoutePopstateHost &
	Core6RoutePrefetchHost &
	Core6RouteRevalidationTimerHost &
	Core6APISubmitHost;

export type Core6ClientCompositionRevalidationOptions = Pick<
	Core6RouteRevalidationConfig,
	"backoff_base_ms" | "backoff_cap_ms" | "debounce_ms" | "max_retries"
>;

export type Core6ClientCompositionConfig = {
	active_client_build_id: () => string;
	deployment_id?: () => string | null | undefined;
	host: Core6ClientCompositionHost;
	initial_position: Core6RoutePopstatePosition;
	revalidation?: Core6ClientCompositionRevalidationOptions;
};

export type Core6ClientComposition = {
	api_submit: Core6APISubmitOwner;
	boot: Core6RouteBootOwner;
	navigation: Core6RouteNavigationOwner;
	popstate: Core6RoutePopstateOwner;
	prefetch: Core6RoutePrefetchOwner;
	revalidation: Core6RouteRevalidationOwner;
	runtime: Core6RouteRuntime;
};

export function create_core6_client_composition(
	config: Core6ClientCompositionConfig,
): Core6ClientComposition {
	const runtime = create_core6_route_runtime(config.host);
	const boot = create_core6_route_boot_owner({
		runtime,
	});
	const prefetch = create_core6_route_prefetch_owner({
		active_client_build_id: config.active_client_build_id,
		current_route: runtime.current_route,
		deployment_id: config.deployment_id,
		host: config.host,
	});
	const revalidation = create_core6_route_revalidation_owner({
		active_client_build_id: config.active_client_build_id,
		backoff_base_ms: config.revalidation?.backoff_base_ms,
		backoff_cap_ms: config.revalidation?.backoff_cap_ms,
		debounce_ms: config.revalidation?.debounce_ms,
		deployment_id: config.deployment_id,
		host: config.host,
		max_retries: config.revalidation?.max_retries,
		runtime,
	});
	const navigation = create_core6_route_navigation_owner({
		active_client_build_id: config.active_client_build_id,
		deployment_id: config.deployment_id,
		host: config.host,
		prefetch,
		runtime,
	});
	const popstate = create_core6_route_popstate_owner({
		active_client_build_id: config.active_client_build_id,
		deployment_id: config.deployment_id,
		host: config.host,
		initial_position: config.initial_position,
		prefetch,
		runtime,
	});
	const api_submit = create_core6_api_submit_owner({
		active_client_build_id: config.active_client_build_id,
		deployment_id: config.deployment_id,
		host: config.host,
		revalidation,
		runtime,
	});
	return {
		api_submit,
		boot,
		navigation,
		popstate,
		prefetch,
		revalidation,
		runtime,
	};
}
