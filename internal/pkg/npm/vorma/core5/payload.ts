import { parseSearchParams } from "vorma/kit/json";
import type { RoutePayload, RoutePayloadMatch } from "./model.ts";

export type WireRoutePayload = {
	MatchedPatterns?: string[];
	SearchSchemas?: unknown[];
	LoadersData?: unknown[];
	ImportURLs?: string[];
	OutermostServerErrIdx?: number | null;
	OutermostServerErr?: unknown;
	Params?: Record<string, string>;
	SplatValues?: string[];
	Title?: { dangerousInnerHTML?: string };
	MetaHeadEls?: unknown[];
	RestHeadEls?: unknown[];
	CSSBundles?: string[];
	Deps?: string[];
	ClientBuildID?: string;
	DeploymentID?: string;
};

export type SearchSchemaRegistry = {
	get: (pattern: string) => unknown;
	set: (pattern: string, schema: unknown) => void;
};

export type DecodeHTMLEntities = (raw: string) => string;

export type DecoderDeps = {
	search_schemas: SearchSchemaRegistry;
	decode_html_entities: DecodeHTMLEntities;
};

export function decode_route_payload(
	raw: unknown,
	url: URL,
	deps: DecoderDeps,
): RoutePayload {
	const data = (
		raw && typeof raw === "object" ? raw : {}
	) as WireRoutePayload;

	const patterns: string[] = data.MatchedPatterns ?? [];
	const schemas: unknown[] = Array.isArray(data.SearchSchemas)
		? data.SearchSchemas
		: [];
	const loaders: unknown[] = data.LoadersData ?? [];
	const imports: string[] = data.ImportURLs ?? [];
	const error_idx: number | null =
		typeof data.OutermostServerErrIdx === "number"
			? data.OutermostServerErrIdx
			: null;
	const error_value = data.OutermostServerErr;

	const routes: RoutePayloadMatch[] = patterns.map((pattern, idx) => {
		const schema = schemas[idx];
		deps.search_schemas.set(pattern, schema);
		return {
			pattern,
			input: parseSearchParams(schema, url.searchParams),
			module_url: imports[idx] ?? "",
			loader_data: loaders[idx],
			server_error:
				error_idx !== null && error_idx === idx
					? error_value
					: undefined,
		};
	});

	let title: string | undefined;
	if (data.Title?.dangerousInnerHTML !== undefined) {
		title = deps.decode_html_entities(data.Title.dangerousInnerHTML);
	}

	return {
		routes,
		params: data.Params ?? {},
		splat_values: data.SplatValues ?? [],
		title,
		meta_head_els: data.MetaHeadEls ?? [],
		rest_head_els: data.RestHeadEls ?? [],
		css_bundles: data.CSSBundles ?? [],
		deps: data.Deps ?? [],
		server_build_id: data.ClientBuildID ?? "",
		deployment_id: data.DeploymentID ?? "",
	};
}

export function browser_decode_html_entities(raw: string): string {
	const el = document.createElement("textarea");
	el.innerHTML = raw;
	return el.value;
}

export function create_search_schema_registry(): SearchSchemaRegistry {
	const map = new Map<string, unknown>();
	return {
		get: (pattern) => map.get(pattern),
		set: (pattern, schema) => {
			map.set(pattern, schema);
		},
	};
}
