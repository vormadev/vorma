// Schema grammar constants — internal encoding details of `SearchParamSchema`,
// not meant to be constructed by hand (an app never writes `SEARCH_PARAM_SCHEMA_*`
// directly; a view's typed `input` schema is generated from its Rust input
// struct's shape). Exported so `search_param_serializer.ts` and generated
// code share one definition of the grammar rather than duplicating the
// magic strings.
export const SEARCH_PARAM_SCHEMA_ARRAY_LENGTH = 1;
export const SEARCH_PARAM_SCHEMA_BOOL = "b";
export const SEARCH_PARAM_SCHEMA_MAP = "*";
export const SEARCH_PARAM_SCHEMA_MAP_LENGTH = 2;
export const SEARCH_PARAM_SCHEMA_NUMBER = "n";
export const SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX = "?";
export const SEARCH_PARAM_SCHEMA_STRING = "s";

/**
 * A compact, generated schema describing how to parse a view/resource's
 * search params into typed input. A leaf is one of `"s"`/`"n"`/`"b"`
 * (string/number/bool), optionally prefixed `"?"` for an absent-when-empty
 * optional field; `[schema]` (array length 1) parses a repeated param into
 * an array; `["*", valueSchema]` parses every param under a dotted-key
 * prefix into a map; a plain object schema nests fields with `.`-joined
 * paths in the actual query string. This grammar is a build-time detail of
 * how Vorma projects a Rust input type's shape (derived server-side from
 * the same `Type`/`TsGen` machinery that generates its TypeScript type)
 * onto URL search params — an app consumes it only indirectly, through a
 * view's already-typed `input`, never by hand-authoring a schema value.
 */
export type SearchParamSchema =
	| string
	| SearchParamSchema[]
	| { [key: string]: SearchParamSchema };

const ABSENT = Symbol("ABSENT");

type ParsedSearchParam =
	| string
	| number
	| boolean
	| unknown[]
	| Record<string, unknown>
	| typeof ABSENT;

/**
 * Parse a `URLSearchParams` into typed data using a {@link SearchParamSchema}
 * (or `schema` as `unknown` to match how it actually arrives off the wire).
 * Vorma's router uses this internally to produce a view's typed `input`
 * from the current URL — an app rarely calls this directly, since it
 * already receives typed, parsed input.
 */
export function parseSearchParams(schema: unknown, params: URLSearchParams): unknown {
	const parsed = parse_schema(schema ?? {}, params, "");
	return parsed === ABSENT ? undefined : parsed;
}

function parse_schema(
	schema: unknown,
	params: URLSearchParams,
	path: string,
): ParsedSearchParam {
	if (typeof schema === "string") {
		return parse_scalar(schema, params.getAll(path));
	}

	if (Array.isArray(schema)) {
		if (
			schema.length === SEARCH_PARAM_SCHEMA_MAP_LENGTH &&
			schema[0] === SEARCH_PARAM_SCHEMA_MAP
		) {
			return parse_map(schema[1]!, params, path);
		}
		if (schema.length === SEARCH_PARAM_SCHEMA_ARRAY_LENGTH) {
			return parse_array(schema[0]!, params.getAll(path));
		}
		return [];
	}

	if (typeof schema !== "object" || schema === null) {
		return ABSENT;
	}

	const out: Record<string, unknown> = {};
	for (const [key, child_schema] of Object.entries(schema)) {
		const child_path = path ? `${path}.${key}` : key;
		const value = parse_schema(child_schema, params, child_path);
		if (value !== ABSENT) {
			out[key] = value;
		}
	}
	return out;
}

function parse_scalar(
	schema: string,
	values: string[],
): string | number | boolean | typeof ABSENT {
	const optional = schema.startsWith(SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX);
	const code = optional
		? schema.slice(SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX.length)
		: schema;
	const raw = values[0] ?? "";
	if (optional && raw === "") {
		return ABSENT;
	}
	if (code === SEARCH_PARAM_SCHEMA_NUMBER) {
		return raw === "" ? 0 : Number(raw);
	}
	if (code === SEARCH_PARAM_SCHEMA_BOOL) {
		return parse_bool(raw);
	}
	return raw;
}

function parse_array(schema: unknown, values: string[]): unknown[] {
	if (typeof schema !== "string") {
		return [];
	}
	const out: unknown[] = [];
	for (const value of values) {
		if (value === "") {
			continue;
		}
		const parsed = parse_scalar(schema, [value]);
		if (parsed !== ABSENT) {
			out.push(parsed);
		}
	}
	return out;
}

function parse_map(
	value_schema: unknown,
	params: URLSearchParams,
	path: string,
): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	const prefix = path ? `${path}.` : "";
	for (const key of params.keys()) {
		if (!key.startsWith(prefix)) {
			continue;
		}
		const map_key = key.slice(prefix.length);
		const value = parse_schema(value_schema, params, key);
		if (value !== ABSENT) {
			out[map_key] = value;
		}
	}
	return out;
}

function parse_bool(raw: string): boolean {
	switch (raw) {
		case "1":
		case "t":
		case "T":
		case "TRUE":
		case "true":
		case "True":
			return true;
		default:
			return false;
	}
}
