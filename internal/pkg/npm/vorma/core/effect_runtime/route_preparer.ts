import { Data, Effect, Fiber, Ref } from "effect";
import { parseSearchParams } from "vorma/kit/json";
import {
	createPatternRegistry,
	findNestedMatches,
	registerPattern,
	type FindNestedMatchesResult,
	type PatternRegistry,
} from "vorma/kit/matcher";
import type { RouteErrorState } from "../types.ts";
import type {
	ClientLoaderFn,
	ClientLoaderKnownMatch,
	ClientLoaderServerState,
	ViewDefinition,
} from "./client_contract.ts";

const MATCHED_PATTERNS_FIELD = "MatchedPatterns";
const SEARCH_SCHEMAS_FIELD = "SearchSchemas";
const LOADERS_DATA_FIELD = "LoadersData";
const IMPORT_URLS_FIELD = "ImportURLs";
const OUTERMOST_SERVER_ERROR_INDEX_FIELD = "OutermostServerErrIdx";
const OUTERMOST_SERVER_ERROR_FIELD = "OutermostServerErr";
const PARAMS_FIELD = "Params";
const SPLAT_VALUES_FIELD = "SplatValues";
const TITLE_FIELD = "Title";
const DANGEROUS_INNER_HTML_FIELD = "dangerousInnerHTML";
const META_HEAD_ELEMENTS_FIELD = "MetaHeadEls";
const REST_HEAD_ELEMENTS_FIELD = "RestHeadEls";
const CSS_BUNDLES_FIELD = "CSSBundles";
const DEPS_FIELD = "Deps";
const abort_error_name = "AbortError";
const abort_error_message = "Aborted";

export const ROUTE_PAYLOAD_FIELDS = {
	matched_patterns: MATCHED_PATTERNS_FIELD,
	search_schemas: SEARCH_SCHEMAS_FIELD,
	loaders_data: LOADERS_DATA_FIELD,
	import_urls: IMPORT_URLS_FIELD,
	outermost_server_error_index: OUTERMOST_SERVER_ERROR_INDEX_FIELD,
	outermost_server_error: OUTERMOST_SERVER_ERROR_FIELD,
	params: PARAMS_FIELD,
	splat_values: SPLAT_VALUES_FIELD,
	title: TITLE_FIELD,
	dangerous_inner_html: DANGEROUS_INNER_HTML_FIELD,
	meta_head_elements: META_HEAD_ELEMENTS_FIELD,
	rest_head_elements: REST_HEAD_ELEMENTS_FIELD,
	css_bundles: CSS_BUNDLES_FIELD,
	deps: DEPS_FIELD,
} as const;

export type RouteModule = Record<string, unknown>;

export type DecodedRoute = {
	pattern: string;
	input: unknown;
	search_schema: unknown;
	module_url: string;
	loader_data: unknown;
	server_error: unknown;
};

export type DecodedPayload = {
	routes: DecodedRoute[];
	params: Record<string, string>;
	splat_values: string[];
	title: string | undefined;
	meta_head_els: unknown[];
	rest_head_els: unknown[];
	css_bundles: string[];
	deps: string[];
};

export type RouteMatchRecord = {
	pattern: string;
	input: unknown;
	module_url: string;
	module: RouteModule;
	loader_data: unknown;
	client_loader_data: unknown;
};

export type RouteRecord = {
	params: Record<string, string>;
	splat_values: string[];
	matches: RouteMatchRecord[];
	error: RouteErrorState | null;
	client_build_id: string;
};

export type PreparedRoute = {
	route: RouteRecord;
	apply_dom_side_effects: Effect.Effect<void, RoutePreparationFailed>;
};

export type ClientLoaderCatalog = {
	set: (
		pattern: string,
		loader: ClientLoaderFn,
		search_schema?: unknown,
	) => Effect.Effect<void>;
	set_search_schema: (
		pattern: string,
		search_schema: unknown,
	) => Effect.Effect<void>;
	get: (pattern: string) => Effect.Effect<ClientLoaderFn | undefined>;
	snapshot: Effect.Effect<ReadonlyMap<string, ClientLoaderFn>>;
	prestart_client_loaders: (
		input: ClientLoaderPrestartInput,
	) => Effect.Effect<ClientLoaderPrestart[]>;
};

export type RoutePrepareInput = {
	raw_payload: unknown;
	url: URL;
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	history_state: unknown;
	signal?: AbortSignal;
	client_loader_prestarts?: ClientLoaderPrestart[];
};

export type ClientLoaderPrestartInput = {
	url: URL;
	href: string;
	history_state: unknown;
	trigger: "navigation" | "revalidation" | "prefetch";
	signal?: AbortSignal;
};

export type ClientLoaderPrestart = {
	pattern: string;
	resolve_server_state: (
		state: ClientLoaderServerState,
	) => Effect.Effect<void>;
	abort: Effect.Effect<void>;
	result_promise: Promise<unknown>;
};

export type RoutePreparerOptions = {
	client_build_id: string;
	load_module: (
		module_url: string,
	) => Effect.Effect<RouteModule, RouteModuleLoadFailed>;
	catalog?: ClientLoaderCatalog;
	parse_search_params?: (
		schema: unknown,
		search_params: URLSearchParams,
	) => unknown;
	preload_css?: (css_bundles: string[]) => Effect.Effect<void>;
	wait_for_css?: (
		css_bundles: string[],
	) => Effect.Effect<void, RouteCSSFailed>;
	apply_payload_side_effects?: (
		payload: DecodedPayload,
	) => Effect.Effect<void, RoutePreparationFailed>;
	on_provisional_route?: (input: {
		route: RouteRecord;
		prepare_input: RoutePrepareInput;
	}) => Effect.Effect<void>;
	decode_title?: (html: string) => string;
};

export type RoutePreparer = {
	prepare_route: (
		input: RoutePrepareInput,
	) => Effect.Effect<PreparedRoute, RoutePreparationFailed>;
	catalog: ClientLoaderCatalog;
};

export class RoutePayloadDecodeFailed extends Data.TaggedError(
	"RoutePayloadDecodeFailed",
)<{
	readonly error: unknown;
}> {}

export class RouteModuleLoadFailed extends Data.TaggedError(
	"RouteModuleLoadFailed",
)<{
	readonly module_url: string;
	readonly error: unknown;
}> {}

export class RouteClientLoaderFailed extends Data.TaggedError(
	"RouteClientLoaderFailed",
)<{
	readonly idx: number;
	readonly pattern: string;
	readonly error: unknown;
}> {}

export class RouteCSSFailed extends Data.TaggedError("RouteCSSFailed")<{
	readonly error: unknown;
}> {}

export class RouteDOMSideEffectFailed extends Data.TaggedError(
	"RouteDOMSideEffectFailed",
)<{
	readonly error: unknown;
}> {}

export type RoutePreparationFailed =
	| RoutePayloadDecodeFailed
	| RouteModuleLoadFailed
	| RouteClientLoaderFailed
	| RouteCSSFailed
	| RouteDOMSideEffectFailed;

type ClientLoaderSlot =
	| {
			readonly _tag: "Empty";
	  }
	| {
			readonly _tag: "Active";
			readonly fiber: Fiber.RuntimeFiber<
				unknown,
				RouteClientLoaderFailed
			>;
			readonly abort: Effect.Effect<void>;
	  };

type ClientLoaderResult = { data: unknown } | { error: unknown } | undefined;

export function make_client_loader_catalog(): Effect.Effect<
	ClientLoaderCatalog,
	never
> {
	return Effect.gen(function* () {
		const loaders = yield* Ref.make<ReadonlyMap<string, ClientLoaderFn>>(
			new Map(),
		);
		const search_schemas = yield* Ref.make<ReadonlyMap<string, unknown>>(
			new Map(),
		);
		const registry = create_pattern_registry();

		const set_search_schema = (
			pattern: string,
			search_schema: unknown,
		): Effect.Effect<void> => {
			return Ref.update(search_schemas, (current) => {
				const next = new Map(current);
				next.set(pattern, search_schema);
				return next;
			});
		};

		const register_loader_pattern = (
			pattern: string,
		): Effect.Effect<void> => {
			return Effect.sync(() => {
				const result = registerPattern(registry, pattern);
				if (!result.ok) {
					throw new Error(result.err);
				}
			});
		};

		return {
			set: (pattern, loader, search_schema) => {
				return Effect.gen(function* () {
					if (search_schema !== undefined) {
						yield* set_search_schema(pattern, search_schema);
					}
					yield* register_loader_pattern(pattern);
					yield* Ref.update(loaders, (current) => {
						const next = new Map(current);
						next.set(pattern, loader);
						return next;
					});
				});
			},
			set_search_schema,
			prestart_client_loaders: (input) => {
				return Effect.gen(function* () {
					const current_loaders = yield* Ref.get(loaders);
					const current_schemas = yield* Ref.get(search_schemas);
					const match = find_known_client_loader_match(
						registry,
						input.url,
					);
					if (!match) {
						return [];
					}
					const known_matches = match.matches.map((route_match) => {
						const pattern =
							route_match.registeredPattern.originalPattern;
						return {
							pattern,
							input: parseSearchParams(
								current_schemas.get(pattern),
								input.url.searchParams,
							),
						};
					});
					const input_by_pattern = new Map(
						known_matches.map((known_match) => {
							return [
								known_match.pattern,
								known_match.input,
							] as const;
						}),
					);
					return match.matches.flatMap((route_match) => {
						const pattern =
							route_match.registeredPattern.originalPattern;
						const loader = current_loaders.get(pattern);
						if (!loader) {
							return [];
						}
						return [
							make_client_loader_prestart({
								href: input.href,
								history_state: input.history_state,
								known_matches,
								loader,
								params: match.params,
								pattern,
								route_input: input_by_pattern.get(pattern),
								signal: input.signal,
								splat_values: match.splatValues,
								trigger: input.trigger,
							}),
						];
					});
				});
			},
			get: (pattern) => {
				return Ref.get(loaders).pipe(
					Effect.map((current) => {
						return current.get(pattern);
					}),
				);
			},
			snapshot: Ref.get(loaders),
		};
	});
}

export function make_route_preparer(
	options: RoutePreparerOptions,
): Effect.Effect<RoutePreparer, never> {
	return Effect.gen(function* () {
		const catalog =
			options.catalog ?? (yield* make_client_loader_catalog());
		const parse_search_params =
			options.parse_search_params ?? parseSearchParams;
		const preload_css =
			options.preload_css ??
			((_css_bundles: string[]): Effect.Effect<void> => {
				return Effect.void;
			});
		const wait_for_css =
			options.wait_for_css ??
			((_css_bundles: string[]): Effect.Effect<void, RouteCSSFailed> => {
				return Effect.void;
			});
		const apply_payload_side_effects =
			options.apply_payload_side_effects ??
			((
				_payload: DecodedPayload,
			): Effect.Effect<void, RoutePreparationFailed> => {
				return Effect.void;
			});
		const decode_title =
			options.decode_title ??
			((html: string): string => {
				return html;
			});

		const prepare_route = (
			input: RoutePrepareInput,
		): Effect.Effect<PreparedRoute, RoutePreparationFailed> => {
			return Effect.gen(function* () {
				const payload = yield* decode_payload(
					input.raw_payload,
					input.url,
					parse_search_params,
					decode_title,
				);
				yield* register_search_schemas(catalog, payload);
				yield* preload_css(payload.css_bundles);
				const modules = yield* prepare_modules(payload);
				const provisional_route = build_route_record(
					payload,
					modules,
					[],
					options.client_build_id,
				);
				if (options.on_provisional_route) {
					yield* options.on_provisional_route({
						route: provisional_route,
						prepare_input: input,
					});
				}
				const client_loader_results = yield* run_client_loaders(
					catalog,
					payload,
					input,
					options.client_build_id,
				);
				yield* wait_for_css(payload.css_bundles);
				return {
					route: build_route_record(
						payload,
						modules,
						client_loader_results,
						options.client_build_id,
					),
					apply_dom_side_effects: apply_payload_side_effects(payload),
				};
			});
		};

		const prepare_modules = (
			payload: DecodedPayload,
		): Effect.Effect<
			ReadonlyMap<string, RouteModule>,
			RouteModuleLoadFailed
		> => {
			return Effect.gen(function* () {
				const module_urls = [
					...new Set(
						payload.routes
							.map((route) => {
								return route.module_url;
							})
							.filter((module_url) => {
								return module_url.length > 0;
							}),
					),
				];
				const pairs = yield* Effect.forEach(
					module_urls,
					(module_url) => {
						return options.load_module(module_url).pipe(
							Effect.map((module) => {
								return [module_url, module] as const;
							}),
						);
					},
					{ concurrency: "unbounded" },
				);
				const modules = new Map<string, RouteModule>(pairs);
				yield* Effect.forEach(
					payload.routes,
					(route) => {
						const module = modules.get(route.module_url);
						const view_definition = module?.default as
							| Partial<ViewDefinition>
							| undefined;
						if (
							typeof view_definition?.client_loader !== "function"
						) {
							return Effect.void;
						}
						return catalog.set(
							route.pattern,
							view_definition.client_loader,
							route.search_schema,
						);
					},
					{ discard: true },
				);
				return modules;
			});
		};

		return { prepare_route, catalog };
	});
}

function create_pattern_registry(): PatternRegistry {
	const result = createPatternRegistry({
		dynamicParamPrefixRune: ":",
		splatSegmentRune: "*",
		explicitIndexSegment: "_index",
	});
	if (!result.ok) {
		throw new Error(result.err);
	}
	return result.val;
}

function find_known_client_loader_match(
	registry: PatternRegistry,
	url: URL,
): FindNestedMatchesResult | null {
	const exact_match = findNestedMatches(registry, url.pathname);
	if (exact_match) {
		return exact_match;
	}
	const segments = url.pathname.split("/").filter(Boolean);
	for (let idx = segments.length; idx >= 0; idx--) {
		const partial =
			idx === 0 ? "/" : "/" + segments.slice(0, idx).join("/");
		const partial_match = findNestedMatches(registry, partial);
		if (partial_match) {
			return partial_match;
		}
	}
	return null;
}

function register_search_schemas(
	catalog: ClientLoaderCatalog,
	payload: DecodedPayload,
): Effect.Effect<void> {
	return Effect.forEach(
		payload.routes,
		(route) => {
			return catalog.set_search_schema(
				route.pattern,
				route.search_schema,
			);
		},
		{ discard: true },
	);
}

function make_client_loader_prestart(input: {
	href: string;
	history_state: unknown;
	known_matches: ClientLoaderKnownMatch[];
	loader: ClientLoaderFn;
	params: Record<string, string>;
	pattern: string;
	route_input: unknown;
	signal: AbortSignal | undefined;
	splat_values: string[];
	trigger: "navigation" | "revalidation" | "prefetch";
}): ClientLoaderPrestart {
	let resolve_server_state!: (state: ClientLoaderServerState) => void;
	let reject_server_state!: (error: unknown) => void;
	const server_promise = new Promise<ClientLoaderServerState>(
		(resolve, reject) => {
			resolve_server_state = resolve;
			reject_server_state = reject;
		},
	);
	server_promise.catch(() => {});

	const controller = new AbortController();
	const abort = (): void => {
		if (!controller.signal.aborted) {
			controller.abort();
		}
		reject_server_state(new_abort_error());
	};
	if (input.signal?.aborted) {
		abort();
	} else {
		input.signal?.addEventListener("abort", abort, { once: true });
	}

	let result_promise: Promise<unknown>;
	try {
		result_promise = input.loader({
			trigger: input.trigger,
			href: input.href,
			historyState: input.history_state,
			pattern: input.pattern,
			params: input.params,
			splatValues: input.splat_values,
			input: input.route_input,
			knownMatches: input.known_matches,
			serverPromise: server_promise,
			signal: controller.signal,
		});
	} catch (error) {
		result_promise = Promise.reject(error);
	}
	result_promise.catch(() => {});

	return {
		pattern: input.pattern,
		resolve_server_state: (state) => {
			return Effect.sync(() => {
				resolve_server_state(state);
			});
		},
		abort: Effect.sync(abort),
		result_promise,
	};
}

function decode_payload(
	raw_payload: unknown,
	url: URL,
	parse_search_params: (
		schema: unknown,
		search_params: URLSearchParams,
	) => unknown,
	decode_title: (html: string) => string,
): Effect.Effect<DecodedPayload, RoutePayloadDecodeFailed> {
	return Effect.try({
		try: () => {
			const payload = record_payload(raw_payload);
			const patterns = string_array_field(
				payload,
				MATCHED_PATTERNS_FIELD,
			);
			const schemas = array_field(payload, SEARCH_SCHEMAS_FIELD);
			const loaders_data = array_field(payload, LOADERS_DATA_FIELD);
			const import_urls = string_array_field(payload, IMPORT_URLS_FIELD);
			const server_error_idx = nullable_number_field(
				payload,
				OUTERMOST_SERVER_ERROR_INDEX_FIELD,
			);
			const server_error = payload[OUTERMOST_SERVER_ERROR_FIELD];
			const routes = patterns.map((pattern, idx) => {
				const search_schema = schemas[idx];
				return {
					pattern,
					input: parse_search_params(search_schema, url.searchParams),
					search_schema,
					module_url: import_urls[idx] ?? "",
					loader_data: loaders_data[idx],
					server_error:
						server_error_idx === idx ? server_error : undefined,
				};
			});
			return {
				routes,
				params: string_record_field(payload, PARAMS_FIELD),
				splat_values: string_array_field(payload, SPLAT_VALUES_FIELD),
				title: title_field(payload, decode_title),
				meta_head_els: array_field(payload, META_HEAD_ELEMENTS_FIELD),
				rest_head_els: array_field(payload, REST_HEAD_ELEMENTS_FIELD),
				css_bundles: string_array_field(payload, CSS_BUNDLES_FIELD),
				deps: string_array_field(payload, DEPS_FIELD),
			};
		},
		catch: (error) => {
			return new RoutePayloadDecodeFailed({ error });
		},
	});
}

function run_client_loaders(
	catalog: ClientLoaderCatalog,
	payload: DecodedPayload,
	input: RoutePrepareInput,
	client_build_id: string,
): Effect.Effect<ClientLoaderResult[], RouteClientLoaderFailed> {
	return Effect.gen(function* () {
		const prestarts_by_pattern = new Map<string, ClientLoaderPrestart>();
		for (const prestart of input.client_loader_prestarts ?? []) {
			prestarts_by_pattern.set(prestart.pattern, prestart);
		}
		const server_error_idx = payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		const known_matches = payload.routes.map((route) => {
			return {
				pattern: route.pattern,
				input: route.input,
			};
		});
		const slots: ClientLoaderSlot[] = [];
		const retained_prestarts = new Set<ClientLoaderPrestart>();
		for (let idx = 0; idx < payload.routes.length; idx++) {
			const route = payload.routes[idx]!;
			const prestart = prestarts_by_pattern.get(route.pattern);
			if (server_error_idx !== -1 && idx >= server_error_idx) {
				if (prestart) {
					yield* prestart.abort;
				}
				slots.push({ _tag: "Empty" });
				continue;
			}
			if (prestart) {
				yield* prestart.resolve_server_state(
					build_server_state(payload.routes, idx, client_build_id),
				);
				retained_prestarts.add(prestart);
				const fiber = yield* Effect.fork(
					adopt_client_loader_prestart(prestart, idx, route.pattern),
				);
				slots.push({
					_tag: "Active",
					fiber,
					abort: prestart.abort,
				});
				continue;
			}
			const loader = yield* catalog.get(route.pattern);
			if (!loader) {
				slots.push({ _tag: "Empty" });
				continue;
			}
			const server_state = build_server_state(
				payload.routes,
				idx,
				client_build_id,
			);
			const fiber = yield* Effect.fork(
				run_client_loader(loader, route, {
					idx,
					input,
					known_matches,
					params: payload.params,
					server_state,
					splat_values: payload.splat_values,
				}),
			);
			slots.push({
				_tag: "Active",
				fiber,
				abort: Fiber.interrupt(fiber).pipe(Effect.asVoid),
			});
		}
		for (const prestart of input.client_loader_prestarts ?? []) {
			if (!retained_prestarts.has(prestart)) {
				yield* prestart.abort;
			}
		}

		const results: ClientLoaderResult[] = [];
		for (let idx = 0; idx < slots.length; idx++) {
			const slot = slots[idx]!;
			if (slot._tag === "Empty") {
				results.push(undefined);
				continue;
			}
			const result = yield* Fiber.join(slot.fiber).pipe(Effect.either);
			if (result._tag === "Right") {
				results.push({ data: result.right });
				continue;
			}
			if (is_abort_error(result.left.error)) {
				results.push(undefined);
				break;
			}
			for (
				let interrupt_idx = idx + 1;
				interrupt_idx < slots.length;
				interrupt_idx++
			) {
				const later_slot = slots[interrupt_idx]!;
				if (later_slot._tag === "Active") {
					yield* later_slot.abort;
					yield* Fiber.interrupt(later_slot.fiber).pipe(
						Effect.asVoid,
					);
				}
			}
			results.push({ error: result.left.error });
			break;
		}
		return results;
	});
}

function adopt_client_loader_prestart(
	prestart: ClientLoaderPrestart,
	idx: number,
	pattern: string,
): Effect.Effect<unknown, RouteClientLoaderFailed> {
	return Effect.tryPromise({
		try: () => {
			return prestart.result_promise;
		},
		catch: (error) => {
			return new RouteClientLoaderFailed({
				idx,
				pattern,
				error,
			});
		},
	});
}

function run_client_loader(
	loader: ClientLoaderFn,
	route: DecodedRoute,
	context: {
		idx: number;
		input: RoutePrepareInput;
		known_matches: ClientLoaderKnownMatch[];
		params: Record<string, string>;
		server_state: ClientLoaderServerState;
		splat_values: string[];
	},
): Effect.Effect<unknown, RouteClientLoaderFailed> {
	return Effect.tryPromise({
		try: (signal) => {
			return loader({
				trigger: context.input.trigger,
				href: context.input.href,
				historyState: context.input.history_state,
				pattern: route.pattern,
				params: context.params,
				splatValues: context.splat_values,
				input: route.input,
				knownMatches: context.known_matches,
				serverPromise: Promise.resolve(context.server_state),
				signal: merge_abort_signals([signal, context.input.signal]),
			});
		},
		catch: (error) => {
			return new RouteClientLoaderFailed({
				idx: context.idx,
				pattern: route.pattern,
				error,
			});
		},
	});
}

function merge_abort_signals(
	signals: Array<AbortSignal | undefined>,
): AbortSignal {
	const live_signals = signals.filter((signal): signal is AbortSignal => {
		return signal !== undefined;
	});
	if (live_signals.length === 1) {
		return live_signals[0]!;
	}
	const controller = new AbortController();
	for (const signal of live_signals) {
		if (signal.aborted) {
			controller.abort();
			break;
		}
		signal.addEventListener(
			"abort",
			() => {
				controller.abort();
			},
			{ once: true },
		);
	}
	return controller.signal;
}

function build_server_state(
	routes: DecodedRoute[],
	idx: number,
	client_build_id: string,
): ClientLoaderServerState {
	const server_error_idx = routes.findIndex((route) => {
		return route.server_error !== undefined;
	});
	return {
		clientBuildID: client_build_id,
		matches: routes.map((route) => {
			return {
				pattern: route.pattern,
				input: route.input,
				loaderData: route.loader_data,
			};
		}),
		outermostServerError:
			server_error_idx === -1
				? null
				: {
						idx: server_error_idx,
						error: routes[server_error_idx]!.server_error,
					},
		loaderData: routes[idx]?.loader_data,
	};
}

function build_route_record(
	payload: DecodedPayload,
	modules: ReadonlyMap<string, RouteModule>,
	client_loader_results: ClientLoaderResult[],
	client_build_id: string,
): RouteRecord {
	const matches = payload.routes.map((route, idx) => {
		const client_loader_result = client_loader_results[idx];
		return {
			pattern: route.pattern,
			input: route.input,
			module_url: route.module_url,
			module: modules.get(route.module_url) ?? {},
			loader_data: route.loader_data,
			client_loader_data:
				client_loader_result && "data" in client_loader_result
					? client_loader_result.data
					: undefined,
		};
	});
	const server_error_idx = payload.routes.findIndex((route) => {
		return route.server_error !== undefined;
	});
	if (server_error_idx !== -1) {
		return {
			params: payload.params,
			splat_values: payload.splat_values,
			matches,
			error: {
				idx: server_error_idx,
				error: payload.routes[server_error_idx]!.server_error,
				source: "server",
			},
			client_build_id,
		};
	}
	const client_loader_error_idx = client_loader_results.findIndex(
		(result) => {
			return result !== undefined && "error" in result;
		},
	);
	return {
		params: payload.params,
		splat_values: payload.splat_values,
		matches,
		error:
			client_loader_error_idx === -1
				? null
				: {
						idx: client_loader_error_idx,
						error: route_error_value(
							(
								client_loader_results[
									client_loader_error_idx
								] as {
									error: unknown;
								}
							).error,
						),
						source: "clientLoader",
					},
		client_build_id,
	};
}

function route_error_value(error: unknown): unknown {
	if (error instanceof Error) {
		return error.message;
	}
	return error;
}

function record_payload(raw_payload: unknown): Record<string, unknown> {
	if (
		raw_payload &&
		typeof raw_payload === "object" &&
		!Array.isArray(raw_payload)
	) {
		return raw_payload as Record<string, unknown>;
	}
	throw new Error("Route payload must be an object.");
}

function array_field(
	payload: Record<string, unknown>,
	field: string,
): unknown[] {
	const value = payload[field];
	if (value === undefined) {
		return [];
	}
	if (Array.isArray(value)) {
		return value;
	}
	throw new Error(`Route payload field ${field} must be an array.`);
}

function string_array_field(
	payload: Record<string, unknown>,
	field: string,
): string[] {
	return array_field(payload, field).map((value) => {
		if (typeof value !== "string") {
			throw new Error(
				`Route payload field ${field} must contain only strings.`,
			);
		}
		return value;
	});
}

function string_record_field(
	payload: Record<string, unknown>,
	field: string,
): Record<string, string> {
	const value = payload[field];
	if (value === undefined) {
		return {};
	}
	if (!value || typeof value !== "object" || Array.isArray(value)) {
		throw new Error(`Route payload field ${field} must be an object.`);
	}
	const out: Record<string, string> = {};
	for (const [key, raw_value] of Object.entries(value)) {
		if (typeof raw_value === "string") {
			out[key] = raw_value;
		}
	}
	return out;
}

function nullable_number_field(
	payload: Record<string, unknown>,
	field: string,
): number | null {
	const value = payload[field];
	if (value === undefined || value === null) {
		return null;
	}
	if (typeof value === "number") {
		return value;
	}
	throw new Error(`Route payload field ${field} must be a number or null.`);
}

function title_field(
	payload: Record<string, unknown>,
	decode_title: (html: string) => string,
): string | undefined {
	const value = payload[TITLE_FIELD];
	if (!value || typeof value !== "object" || Array.isArray(value)) {
		return undefined;
	}
	const raw_title = (value as Record<string, unknown>)[
		DANGEROUS_INNER_HTML_FIELD
	];
	if (typeof raw_title !== "string") {
		return undefined;
	}
	return decode_title(raw_title);
}

function new_abort_error(): DOMException {
	return new DOMException(abort_error_message, abort_error_name);
}

function is_abort_error(error: unknown): boolean {
	return error instanceof DOMException && error.name === abort_error_name;
}
