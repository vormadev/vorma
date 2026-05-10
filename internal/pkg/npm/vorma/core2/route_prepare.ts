import { parseSearchParams } from "vorma/kit/json";
import type { RouteErrorState, RouteState } from "../core/types.ts";
import type {
	HeadEl,
	RouteRenderEntry,
	RouteRenderState,
} from "./client_contract.ts";
import type { PreparedRoute } from "./model.ts";

const payload_field = {
	css_bundles: "CSSBundles",
	deps: "Deps",
	import_urls: "ImportURLs",
	loaders_data: "LoadersData",
	matched_patterns: "MatchedPatterns",
	meta_head_els: "MetaHeadEls",
	outermost_server_error: "OutermostServerErr",
	outermost_server_error_idx: "OutermostServerErrIdx",
	params: "Params",
	rest_head_els: "RestHeadEls",
	search_schemas: "SearchSchemas",
	splat_values: "SplatValues",
	title: "Title",
	title_html: "dangerousInnerHTML",
} as const;

export const client_loader_outcome_kind = {
	failure: "failure",
	skipped: "skipped",
	success: "success",
} as const;

export type DecodedRoutePayload = {
	css_bundles: string[];
	deps: string[];
	meta_head_els: HeadEl[];
	params: Record<string, string>;
	rest_head_els: HeadEl[];
	routes: DecodedRoute[];
	splat_values: string[];
	title: string | undefined;
};

export type DecodedRoute = {
	input: unknown;
	loader_data: unknown;
	module_url: string;
	pattern: string;
	search_schema: unknown;
	server_error: unknown;
};

export type ClientLoaderOutcome =
	| {
			data: unknown;
			kind: typeof client_loader_outcome_kind.success;
	  }
	| {
			error: unknown;
			kind: typeof client_loader_outcome_kind.failure;
	  }
	| {
			kind: typeof client_loader_outcome_kind.skipped;
	  };

export type RouteModuleMap = ReadonlyMap<string, Record<string, unknown>>;

export type DecodeRoutePayloadOptions = {
	decode_html_text: (html: string) => string;
};

export type BuildPreparedRouteInput = {
	client_build_id: string;
	client_loader_outcomes: ClientLoaderOutcome[];
	history_state: unknown;
	href: string;
	modules: RouteModuleMap;
	payload: DecodedRoutePayload;
};

export function decode_route_payload(
	raw: unknown,
	url: URL,
	options: DecodeRoutePayloadOptions,
): DecodedRoutePayload {
	const payload = as_record(raw);
	const patterns = as_string_array(payload[payload_field.matched_patterns]);
	const schemas = as_unknown_array(payload[payload_field.search_schemas]);
	const loaders_data = as_unknown_array(payload[payload_field.loaders_data]);
	const import_urls = as_string_array(payload[payload_field.import_urls]);
	const server_error_idx = as_number_or_null(
		payload[payload_field.outermost_server_error_idx],
	);
	const server_error = payload[payload_field.outermost_server_error];
	const routes = patterns.map((pattern, idx) => {
		return {
			pattern,
			input: parseSearchParams(schemas[idx], url.searchParams),
			module_url: import_urls[idx] ?? "",
			search_schema: schemas[idx],
			loader_data: loaders_data[idx],
			server_error:
				server_error_idx !== null && idx === server_error_idx
					? server_error
					: undefined,
		};
	});
	const title_record = as_record_or_null(payload[payload_field.title]);
	const raw_title = title_record
		? title_record[payload_field.title_html]
		: undefined;
	return {
		routes,
		params: as_string_record(payload[payload_field.params]),
		splat_values: as_string_array(payload[payload_field.splat_values]),
		title:
			typeof raw_title === "string"
				? options.decode_html_text(raw_title)
				: undefined,
		meta_head_els: as_head_els(payload[payload_field.meta_head_els]),
		rest_head_els: as_head_els(payload[payload_field.rest_head_els]),
		css_bundles: as_string_array(payload[payload_field.css_bundles]),
		deps: as_string_array(payload[payload_field.deps]),
	};
}

export function build_prepared_route(
	input: BuildPreparedRouteInput,
): PreparedRoute {
	const entries: RouteRenderEntry[] = input.payload.routes.map(
		(route, idx) => {
			const outcome = input.client_loader_outcomes[idx];
			return {
				pattern: route.pattern,
				input: route.input,
				module_url: route.module_url,
				module: input.modules.get(route.module_url) ?? {},
				loader_data: route.loader_data,
				client_loader_data:
					outcome?.kind === client_loader_outcome_kind.success
						? outcome.data
						: undefined,
			};
		},
	);
	const error = route_error_from_payload(
		input.payload.routes,
		input.client_loader_outcomes,
	);
	const render: RouteRenderState = {
		entries,
		error,
		params: input.payload.params,
		splat_values: input.payload.splat_values,
		client_build_id: input.client_build_id,
		history_state: input.history_state,
	};
	const route: RouteState = {
		href: input.href,
		historyState: input.history_state,
		clientBuildID: input.client_build_id,
		params: input.payload.params,
		splatValues: input.payload.splat_values,
		matches: entries.map((entry) => {
			return {
				pattern: entry.pattern,
				input: entry.input,
				loaderData: entry.loader_data,
				clientLoaderData: entry.client_loader_data,
			};
		}),
		error,
	};
	return {
		dom: {
			title: input.payload.title,
			meta_head_els: input.payload.meta_head_els,
			rest_head_els: input.payload.rest_head_els,
			css_bundles: input.payload.css_bundles,
			deps: input.payload.deps,
		},
		render,
		route,
	};
}

function route_error_from_payload(
	routes: DecodedRoute[],
	client_loader_outcomes: ClientLoaderOutcome[],
): RouteErrorState | null {
	const server_error_idx = routes.findIndex((route) => {
		return route.server_error !== undefined;
	});
	if (server_error_idx !== -1) {
		return {
			idx: server_error_idx,
			error: routes[server_error_idx]!.server_error,
			source: "server",
		};
	}
	const client_loader_error_idx = client_loader_outcomes.findIndex(
		(outcome) => {
			return outcome?.kind === client_loader_outcome_kind.failure;
		},
	);
	if (client_loader_error_idx === -1) {
		return null;
	}
	const outcome = client_loader_outcomes[client_loader_error_idx];
	if (!outcome || outcome.kind !== client_loader_outcome_kind.failure) {
		return null;
	}
	return {
		idx: client_loader_error_idx,
		error:
			outcome.error instanceof Error
				? outcome.error.message
				: outcome.error,
		source: "clientLoader",
	};
}

function as_record(value: unknown): Record<string, unknown> {
	if (!value || typeof value !== "object") {
		return {};
	}
	return value as Record<string, unknown>;
}

function as_record_or_null(value: unknown): Record<string, unknown> | null {
	if (!value || typeof value !== "object") {
		return null;
	}
	return value as Record<string, unknown>;
}

function as_unknown_array(value: unknown): unknown[] {
	return Array.isArray(value) ? value : [];
}

function as_string_array(value: unknown): string[] {
	if (!Array.isArray(value)) {
		return [];
	}
	return value.filter((entry): entry is string => {
		return typeof entry === "string";
	});
}

function as_number_or_null(value: unknown): number | null {
	return typeof value === "number" ? value : null;
}

function as_string_record(value: unknown): Record<string, string> {
	const record = as_record(value);
	const out: Record<string, string> = {};
	for (const [key, entry] of Object.entries(record)) {
		if (typeof entry === "string") {
			out[key] = entry;
		}
	}
	return out;
}

function as_head_els(value: unknown): HeadEl[] {
	if (!Array.isArray(value)) {
		return [];
	}
	return value.filter((entry): entry is HeadEl => {
		if (!entry || typeof entry !== "object") {
			return false;
		}
		const record = entry as Partial<HeadEl>;
		return (
			typeof record.tag === "string" &&
			!!record.attributesKnownSafe &&
			typeof record.attributesKnownSafe === "object"
		);
	});
}
