import type {
	BrowserPosition,
	PreparedRoute,
	RouteFacts,
	RouteRenderFacts,
	RouteState,
} from "./model.ts";

export function route_facts_at_position(
	route: RouteFacts,
	position: Pick<BrowserPosition, "href" | "state">,
): RouteFacts {
	return {
		...route,
		history_state: position.state,
		href: position.href,
	};
}

export function route_render_facts_at_position(
	render: RouteRenderFacts,
	position: Pick<BrowserPosition, "state">,
): RouteRenderFacts {
	return {
		...render,
		history_state: position.state,
	};
}

export function prepared_route_at_position(
	prepared: PreparedRoute,
	position: Pick<BrowserPosition, "href" | "state">,
): PreparedRoute {
	return {
		...prepared,
		render: route_render_facts_at_position(prepared.render, position),
		route: route_facts_at_position(prepared.route, position),
	};
}

export function route_facts_to_state(route: RouteFacts): RouteState {
	return {
		clientBuildID: route.client_build_id,
		error: route.error,
		historyState: route.history_state,
		href: route.href,
		matches: route.matches.map((match) => {
			return {
				clientLoaderData: match.client_loader_data,
				input: match.input,
				loaderData: match.loader_data,
				pattern: match.pattern,
			};
		}),
		params: route.params as Record<string, string>,
		splatValues: route.splat_values as string[],
	};
}
