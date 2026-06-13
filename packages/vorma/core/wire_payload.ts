import { parseSearchParams } from "vorma/kit/json";
import type { DecodedPayload, DecodedRoute } from "./client_core_types.ts";
import type { ViewPayload } from "./wire_contracts.gen.ts";

/*
Decode one raw route payload from the wire into the router's working shape.
Search schemas observed on the way through are reported to the caller, which
owns the schema registry used for client-loader prefetch input parsing.
*/

export function decode_payload(
	raw: unknown,
	url: URL,
	register_search_schema: (pattern: string, schema: unknown) => void,
): DecodedPayload {
	// The cast target is generated from the Rust ViewPayload struct, so
	// every field access below typechecks against the real wire contract.
	const p = raw as ViewPayload;
	const patterns: string[] = p.matched_patterns ?? [];
	const schemas: unknown[] = Array.isArray(p.search_schemas) ? p.search_schemas : [];
	const views_data: unknown[] = p.views_data ?? [];
	const import_urls: string[] = p.import_urls ?? [];
	const err_idx: number | null = p.outermost_server_err_idx ?? null;
	const err_msg: string = p.outermost_server_err ?? "";
	const search_params = url.searchParams;

	const routes: DecodedRoute[] = patterns.map((pattern, i) => {
		const schema = schemas[i];
		register_search_schema(pattern, schema);
		return {
			pattern,
			input: parseSearchParams(schema, search_params),
			module_url: import_urls[i] ?? "",
			view_data: views_data[i],
			server_error: err_idx !== null && i === err_idx ? err_msg : undefined,
		};
	});

	let title: string | undefined;
	if (p.title?.dangerous_inner_html !== undefined) {
		const el = document.createElement("textarea");
		el.innerHTML = p.title.dangerous_inner_html;
		title = el.value;
	}

	return {
		routes,
		params: p.params ?? {},
		splat_values: p.splat_values ?? [],
		title,
		meta_head_els: p.meta_head_els ?? [],
		rest_head_els: p.rest_head_els ?? [],
		css_bundles: p.css_bundles ?? [],
		deps: p.deps ?? [],
	};
}
