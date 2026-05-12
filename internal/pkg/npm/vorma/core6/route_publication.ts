import type {
	Core6DiscoveredClientLoader,
	Core6PreparedRoute,
	Core6PreparedRouteError,
	Core6RouteModule,
} from "./route_preparation.ts";

export const core6_route_publish_reason = {
	boot: "boot",
	navigation: "navigation",
	popstate: "popstate",
	revalidation: "revalidation",
} as const;

export type Core6RoutePublishReason =
	(typeof core6_route_publish_reason)[keyof typeof core6_route_publish_reason];

export type Core6RouteStateMatch = {
	clientLoaderData: unknown;
	input: unknown;
	loaderData: unknown;
	pattern: string;
};

export type Core6RouteState = {
	clientBuildID: string;
	error: Core6PreparedRouteError | null;
	historyState: unknown;
	href: string;
	matches: Core6RouteStateMatch[];
	params: Record<string, string>;
	splatValues: string[];
};

export type Core6RouteRenderEntry = {
	client_loader_data: unknown;
	input: unknown;
	loader_data: unknown;
	module: Core6RouteModule;
	module_url: string;
	pattern: string;
};

export type Core6RouteRenderState = {
	client_build_id: string;
	entries: Core6RouteRenderEntry[];
	error: Core6PreparedRouteError | null;
	history_state: unknown;
	params: Record<string, string>;
	splat_values: string[];
};

export type Core6RouteScroll =
	| {
			hash: string;
	  }
	| {
			x: number;
			y: number;
	  };

export type Core6RouteRenderCommit = {
	scroll?: Core6RouteScroll;
	state: Core6RouteRenderState;
};

export type Core6RouteHistoryMutation = {
	href: string;
	replace: boolean;
	state: unknown;
};

export type Core6RoutePublicationSideEffects = {
	css_bundles: string[];
	deps: string[];
	meta_head_els: unknown[];
	rest_head_els: unknown[];
	title_html: string | null;
};

export type Core6RouteUpdateCommit = {
	previous_route: Core6RouteState | null;
	reason: Core6RoutePublishReason;
	route: Core6RouteState;
};

export type Core6RouteCommit = {
	route_render: Core6RouteRenderCommit;
	route_update?: Core6RouteUpdateCommit;
};

export type Core6RoutePublication = {
	client_loaders: Core6DiscoveredClientLoader[];
	commit: Core6RouteCommit;
	history?: Core6RouteHistoryMutation;
	route: Core6RouteState;
	side_effects: Core6RoutePublicationSideEffects | null;
};

export type Core6RoutePublicationHost = {
	publish_route: (publication: Core6RoutePublication) => void;
};

export type Core6RoutePublicationInput = {
	apply_side_effects?: boolean;
	history?: Core6RouteHistoryMutation;
	prepared: Core6PreparedRoute;
	previous_route: Core6RouteState | null;
	reason: Core6RoutePublishReason;
	route_state_equal: (
		previous_route: Core6RouteState,
		next_route: Core6RouteState,
	) => boolean;
	scroll?: Core6RouteScroll;
};

export type Core6RoutePublishInput = Core6RoutePublicationInput & {
	host: Core6RoutePublicationHost;
};

export type Core6SameDocumentRoutePublicationInput = {
	current_render_state: Core6RouteRenderState;
	current_route: Core6RouteState;
	history?: Core6RouteHistoryMutation;
	href: string;
	history_state: unknown;
	reason: Core6RoutePublishReason;
	route_state_equal: (
		previous_route: Core6RouteState,
		next_route: Core6RouteState,
	) => boolean;
	scroll?: Core6RouteScroll;
};

export function build_core6_route_publication(
	input: Core6RoutePublicationInput,
): Core6RoutePublication {
	const route = core6_route_state_from_prepared(input.prepared);
	const route_render = core6_route_render_from_prepared(input.prepared);
	const history = core6_route_history_from_publication_input(
		input.history,
		route,
	);
	const route_render_commit: Core6RouteRenderCommit = {
		state: route_render,
	};
	if (input.scroll !== undefined) {
		route_render_commit.scroll = input.scroll;
	}
	const commit: Core6RouteCommit = {
		route_render: route_render_commit,
	};
	if (
		!input.previous_route ||
		!input.route_state_equal(input.previous_route, route)
	) {
		commit.route_update = {
			previous_route: input.previous_route,
			reason: input.reason,
			route,
		};
	}
	return {
		client_loaders: input.prepared.client_loaders.map((client_loader) => {
			return {
				loader: client_loader.loader,
				pattern: client_loader.pattern,
			};
		}),
		commit,
		...(history ? { history } : {}),
		route,
		side_effects:
			input.apply_side_effects === false
				? null
				: core6_route_side_effects_from_prepared(input.prepared),
	};
}

export function publish_core6_route(
	input: Core6RoutePublishInput,
): Core6RoutePublication {
	const publication = build_core6_route_publication(input);
	input.host.publish_route(publication);
	return publication;
}

export function build_core6_same_document_route_publication(
	input: Core6SameDocumentRoutePublicationInput,
): Core6RoutePublication {
	const route = clone_core6_route_state({
		...input.current_route,
		historyState: input.history_state,
		href: input.href,
	});
	const route_render = clone_core6_route_render_state({
		...input.current_render_state,
		history_state: input.history_state,
	});
	const history = core6_route_history_from_publication_input(
		input.history,
		route,
	);
	const route_render_commit: Core6RouteRenderCommit = {
		state: route_render,
	};
	if (input.scroll !== undefined) {
		route_render_commit.scroll = input.scroll;
	}
	const commit: Core6RouteCommit = {
		route_render: route_render_commit,
	};
	if (!input.route_state_equal(input.current_route, route)) {
		commit.route_update = {
			previous_route: input.current_route,
			reason: input.reason,
			route,
		};
	}
	return {
		client_loaders: [],
		commit,
		...(history ? { history } : {}),
		route,
		side_effects: null,
	};
}

export function clone_core6_route_state(
	route: Core6RouteState,
): Core6RouteState {
	return {
		clientBuildID: route.clientBuildID,
		error: route.error,
		historyState: route.historyState,
		href: route.href,
		matches: route.matches.map((match) => {
			return {
				clientLoaderData: match.clientLoaderData,
				input: match.input,
				loaderData: match.loaderData,
				pattern: match.pattern,
			};
		}),
		params: { ...route.params },
		splatValues: [...route.splatValues],
	};
}

export function clone_core6_route_render_state(
	state: Core6RouteRenderState,
): Core6RouteRenderState {
	return {
		client_build_id: state.client_build_id,
		entries: state.entries.map((entry) => {
			return {
				client_loader_data: entry.client_loader_data,
				input: entry.input,
				loader_data: entry.loader_data,
				module: entry.module,
				module_url: entry.module_url,
				pattern: entry.pattern,
			};
		}),
		error: state.error,
		history_state: state.history_state,
		params: { ...state.params },
		splat_values: [...state.splat_values],
	};
}

function core6_route_render_from_prepared(
	prepared: Core6PreparedRoute,
): Core6RouteRenderState {
	return {
		client_build_id: prepared.client_build_id,
		entries: prepared.matches.map((match) => {
			return {
				client_loader_data: match.client_loader_data,
				input: match.input,
				loader_data: match.loader_data,
				module: match.module,
				module_url: match.module_url,
				pattern: match.pattern,
			};
		}),
		error: prepared.error,
		history_state: prepared.history_state,
		params: { ...prepared.params },
		splat_values: [...prepared.splat_values],
	};
}

function core6_route_history_from_publication_input(
	history: Core6RouteHistoryMutation | undefined,
	route: Core6RouteState,
): Core6RouteHistoryMutation | undefined {
	if (!history) {
		return undefined;
	}
	return {
		href: route.href,
		replace: history.replace,
		state: route.historyState,
	};
}

function core6_route_side_effects_from_prepared(
	prepared: Core6PreparedRoute,
): Core6RoutePublicationSideEffects {
	return {
		css_bundles: [...prepared.css_bundles],
		deps: [...prepared.deps],
		meta_head_els: [...prepared.meta_head_els],
		rest_head_els: [...prepared.rest_head_els],
		title_html: prepared.title_html,
	};
}

export function core6_route_state_from_prepared(
	prepared: Core6PreparedRoute,
): Core6RouteState {
	return {
		clientBuildID: prepared.client_build_id,
		error: prepared.error,
		historyState: prepared.history_state,
		href: prepared.href,
		matches: prepared.matches.map((match) => {
			return {
				clientLoaderData: match.client_loader_data,
				input: match.input,
				loaderData: match.loader_data,
				pattern: match.pattern,
			};
		}),
		params: { ...prepared.params },
		splatValues: [...prepared.splat_values],
	};
}
