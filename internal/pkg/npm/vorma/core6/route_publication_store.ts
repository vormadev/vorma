import type { Core6ClientLoaderFn } from "./route_preparation.ts";
import {
	build_core6_same_document_route_publication,
	clone_core6_route_render_state,
	clone_core6_route_state,
	type Core6RouteHistoryMutation,
	type Core6RoutePublication,
	type Core6RoutePublicationHost,
	type Core6RoutePublishReason,
	type Core6RouteRenderState,
	type Core6RouteScroll,
	type Core6RouteState,
} from "./route_publication.ts";

export type Core6RoutePublicationStoreHost = {
	commit_publication: (publication: Core6RoutePublication) => void;
};

export type Core6RoutePublicationStore = Core6RoutePublicationHost & {
	client_loader: (pattern: string) => Core6ClientLoaderFn | undefined;
	client_loader_patterns: () => string[];
	current_render_state: () => Core6RouteRenderState | null;
	current_route: () => Core6RouteState | null;
	publish_same_document: (
		input: Core6RoutePublicationStoreSameDocumentInput,
	) => Core6RoutePublication | null;
};

export type Core6RoutePublicationStoreSameDocumentInput = {
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

export function create_core6_route_publication_store(
	host: Core6RoutePublicationStoreHost,
): Core6RoutePublicationStore {
	let current_route: Core6RouteState | null = null;
	let current_render_state: Core6RouteRenderState | null = null;
	const client_loaders = new Map<string, Core6ClientLoaderFn>();

	return {
		client_loader: (pattern) => {
			return client_loaders.get(pattern);
		},
		client_loader_patterns: () => {
			return [...client_loaders.keys()];
		},
		current_render_state: () => {
			if (!current_render_state) {
				return null;
			}
			return clone_core6_route_render_state(current_render_state);
		},
		current_route: () => {
			if (!current_route) {
				return null;
			}
			return clone_core6_route_state(current_route);
		},
		publish_route: (publication) => {
			current_route = clone_core6_route_state(publication.route);
			current_render_state = clone_core6_route_render_state(
				publication.commit.route_render.state,
			);
			const published_client_loader_patterns = new Set(
				publication.client_loaders.map((client_loader) => {
					return client_loader.pattern;
				}),
			);
			for (const match of publication.route.matches) {
				if (!published_client_loader_patterns.has(match.pattern)) {
					client_loaders.delete(match.pattern);
				}
			}
			for (const client_loader of publication.client_loaders) {
				client_loaders.set(client_loader.pattern, client_loader.loader);
			}
			host.commit_publication(publication);
		},
		publish_same_document: (input) => {
			if (!current_route || !current_render_state) {
				return null;
			}
			const publication = build_core6_same_document_route_publication({
				current_render_state,
				current_route,
				href: input.href,
				history_state: input.history_state,
				reason: input.reason,
				route_state_equal: input.route_state_equal,
				scroll: input.scroll,
				...(input.history ? { history: input.history } : {}),
			});
			current_route = clone_core6_route_state(publication.route);
			current_render_state = clone_core6_route_render_state(
				publication.commit.route_render.state,
			);
			host.commit_publication(publication);
			return publication;
		},
	};
}
