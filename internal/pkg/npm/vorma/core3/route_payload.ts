import { parseSearchParams } from "vorma/kit/json";
import type { RouteFacts, RouteRenderFacts } from "./model.ts";

export const boot_payload_field = {
	client_build_id: "ClientBuildID",
	deployment_id: "DeploymentID",
} as const;

export const route_payload_field = {
	import_urls: "ImportURLs",
	loaders_data: "LoadersData",
	matched_patterns: "MatchedPatterns",
	outermost_server_error: "OutermostServerErr",
	outermost_server_error_idx: "OutermostServerErrIdx",
	params: "Params",
	search_schemas: "SearchSchemas",
	splat_values: "SplatValues",
} as const;

export type DecodedRoutePayload = {
	render: RouteRenderFacts;
	route: RouteFacts;
};

export type RoutePayloadDecodeInput = {
	client_build_id: string;
	history_state: unknown;
	href: string;
	payload: unknown;
};

export function decode_route_payload(
	input: RoutePayloadDecodeInput,
): DecodedRoutePayload {
	const url = new URL(input.href);
	const raw =
		input.payload && typeof input.payload === "object"
			? (input.payload as Record<string, unknown>)
			: {};
	const patterns = string_array(raw[route_payload_field.matched_patterns]);
	const schemas = unknown_array(raw[route_payload_field.search_schemas]);
	const loader_data = unknown_array(raw[route_payload_field.loaders_data]);
	const import_urls = string_array(raw[route_payload_field.import_urls]);
	const matches = patterns.map((pattern, idx) => {
		return {
			client_loader_data: undefined,
			input: parseSearchParams(schemas[idx], url.searchParams),
			loader_data: loader_data[idx],
			module: undefined,
			module_url: import_urls[idx] ?? "",
			pattern,
		};
	});
	const error_idx = raw[route_payload_field.outermost_server_error_idx];
	const error =
		typeof error_idx === "number"
			? {
					error: raw[route_payload_field.outermost_server_error],
					idx: error_idx,
					source: "server" as const,
				}
			: null;
	const params = raw[route_payload_field.params];
	const splat_values = string_array(raw[route_payload_field.splat_values]);
	const route = {
		client_build_id: input.client_build_id,
		error,
		history_state: input.history_state,
		href: url.href,
		matches,
		params:
			params && typeof params === "object"
				? (params as Record<string, string>)
				: {},
		splat_values,
	} satisfies RouteFacts;
	return {
		render: {
			client_build_id: route.client_build_id,
			entries: route.matches,
			error: route.error,
			history_state: route.history_state,
			params: route.params,
			splat_values: route.splat_values,
		},
		route,
	};
}

function string_array(value: unknown): string[] {
	if (!Array.isArray(value)) {
		return [];
	}
	return value.filter((entry): entry is string => {
		return typeof entry === "string";
	});
}

function unknown_array(value: unknown): unknown[] {
	if (!Array.isArray(value)) {
		return [];
	}
	return value;
}
