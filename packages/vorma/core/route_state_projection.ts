import type { RouteErrorState, RouteState } from "./types.ts";

export type HistoryPosition = {
	href: string;
	key: string;
	state: unknown;
};

export type RouteSnapshot = {
	position: HistoryPosition;
	route: RouteRecord;
};

export type RouteRecord = {
	params: Record<string, string>;
	splat_values: string[];
	matches: RouteMatchRecord[];
	error: RouteErrorState | null;
	client_build_id: string;
};

export type RouteMatchRecord = {
	pattern: string;
	input: unknown;
	module_url: string;
	hmr_version: number;
	module: Record<string, unknown>;
	view_data: unknown;
	client_loader_data: unknown;
};

export type RouteRenderEntry = {
	pattern: string;
	input: unknown;
	module_url: string;
	hmr_version: number;
	module: Record<string, unknown>;
	view_data: unknown;
	client_loader_data: unknown;
};

export type RouteRenderState = {
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export function route_to_render_state(
	route: RouteRecord,
	history_state: unknown,
): RouteRenderState {
	return {
		entries: route.matches.map((m) => {
			return {
				pattern: m.pattern,
				input: m.input,
				module_url: m.module_url,
				hmr_version: m.hmr_version,
				module: m.module,
				view_data: m.view_data,
				client_loader_data: m.client_loader_data,
			};
		}),
		error: route.error,
		params: route.params,
		splat_values: route.splat_values,
		client_build_id: route.client_build_id,
		history_state,
	};
}

export function route_snapshot_to_state(snapshot: RouteSnapshot): RouteState {
	return route_record_to_state(
		snapshot.route,
		snapshot.position.href,
		snapshot.position.state,
	);
}

export function route_record_to_state(
	route: RouteRecord,
	href: string,
	history_state: unknown,
): RouteState {
	return {
		href,
		historyState: history_state,
		clientBuildId: route.client_build_id,
		params: route.params,
		splatValues: route.splat_values,
		matches: route.matches.map((m) => {
			return {
				pattern: m.pattern,
				input: m.input,
				viewData: m.view_data,
				clientLoaderData: m.client_loader_data,
			};
		}),
		error: route.error,
	};
}
