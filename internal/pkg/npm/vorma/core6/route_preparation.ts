import type { BeforeRouteCommitFn, BeforeRouteYieldFn } from "../core/types.ts";

export const core6_route_error_source = {
	client_loader: "clientLoader",
	server: "server",
} as const;

export const core6_route_payload_field = {
	client_build_id: "ClientBuildID",
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
} as const;

export const core6_route_preparation_trigger = {
	boot: "boot",
	navigation: "navigation",
	prefetch: "prefetch",
	revalidation: "revalidation",
} as const;

export const core6_abort_error_name = "AbortError";

const core6_client_loader_result_kind = {
	data: "data",
	error: "error",
	skipped: "skipped",
} as const;

export type Core6RoutePreparationTrigger =
	(typeof core6_route_preparation_trigger)[keyof typeof core6_route_preparation_trigger];

export type Core6RouteErrorSource =
	(typeof core6_route_error_source)[keyof typeof core6_route_error_source];

export type Core6RoutePreparationParseInputArgs = {
	pattern: string;
	schema: unknown;
	search_params: URLSearchParams;
};

export type Core6ClientLoaderKnownMatch = {
	pattern: string;
	input: unknown;
};

export type Core6ClientLoaderServerState = {
	clientBuildID: string;
	matches: Array<{
		pattern: string;
		input: unknown;
		loaderData: unknown;
	}>;
	outermostServerError: null | {
		idx: number;
		error: unknown;
	};
	loaderData: unknown;
};

export type Core6ClientLoaderFn = (args: {
	trigger: Core6RoutePreparationTrigger;
	href: string;
	historyState: unknown;
	pattern: string;
	params: Record<string, string>;
	splatValues: string[];
	input: unknown;
	knownMatches: Core6ClientLoaderKnownMatch[];
	serverPromise: Promise<Core6ClientLoaderServerState>;
	signal: AbortSignal;
}) => Promise<unknown>;

export type Core6ViewDefinition = {
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
	client_loader?: Core6ClientLoaderFn;
};

export type Core6RouteModule = Record<string, unknown> & {
	default?: Core6ViewDefinition;
};

export type Core6RoutePreparationHost = {
	import_module: (
		url: string,
		signal: AbortSignal,
	) => Promise<Core6RouteModule>;
	parse_input: (args: Core6RoutePreparationParseInputArgs) => unknown;
	preload_css?: (bundles: string[]) => void;
	set_provisional_route?: (prepared: Core6PreparedRoute) => void;
	wait_for_css?: (bundles: string[], signal: AbortSignal) => Promise<void>;
};

export type Core6RawRoutePayload = {
	ClientBuildID?: string;
	CSSBundles?: string[];
	Deps?: string[];
	ImportURLs?: string[];
	LoadersData?: unknown[];
	MatchedPatterns?: string[];
	MetaHeadEls?: unknown[];
	OutermostServerErr?: unknown;
	OutermostServerErrIdx?: number | null;
	Params?: Record<string, string>;
	RestHeadEls?: unknown[];
	SearchSchemas?: unknown[];
	SplatValues?: string[];
	Title?: {
		dangerousInnerHTML?: string;
	};
};

export type Core6DecodedRoute = {
	input: unknown;
	loader_data: unknown;
	module_url: string;
	pattern: string;
};

export type Core6DecodedRoutePayload = {
	client_build_id: string;
	css_bundles: string[];
	deps: string[];
	meta_head_els: unknown[];
	params: Record<string, string>;
	rest_head_els: unknown[];
	routes: Core6DecodedRoute[];
	server_error: null | {
		idx: number;
		error: unknown;
	};
	splat_values: string[];
	title_html: string | null;
};

export type Core6PreparedRouteMatch = {
	client_loader_data: unknown;
	input: unknown;
	loader_data: unknown;
	module: Core6RouteModule;
	module_url: string;
	pattern: string;
};

export type Core6PreparedRouteError = {
	error: unknown;
	idx: number;
	source: Core6RouteErrorSource;
};

export type Core6DiscoveredClientLoader = {
	loader: Core6ClientLoaderFn;
	pattern: string;
};

export type Core6PreparedRoute = {
	client_build_id: string;
	client_loaders: Core6DiscoveredClientLoader[];
	css_bundles: string[];
	deps: string[];
	error: Core6PreparedRouteError | null;
	history_state: unknown;
	href: string;
	matches: Core6PreparedRouteMatch[];
	meta_head_els: unknown[];
	params: Record<string, string>;
	rest_head_els: unknown[];
	splat_values: string[];
	title_html: string | null;
};

export type Core6RoutePayloadDecodeInput = {
	client_build_id?: string;
	host: Pick<Core6RoutePreparationHost, "parse_input">;
	raw_payload: unknown;
	search_params: URLSearchParams;
};

export type Core6RoutePreparationInput = {
	host: Core6RoutePreparationHost;
	href: string;
	history_state: unknown;
	payload: Core6DecodedRoutePayload;
	signal: AbortSignal;
	trigger: Core6RoutePreparationTrigger;
};

type Core6ClientLoaderResult =
	| {
			kind: typeof core6_client_loader_result_kind.data;
			data: unknown;
	  }
	| {
			kind: typeof core6_client_loader_result_kind.error;
			error: unknown;
	  }
	| {
			kind: typeof core6_client_loader_result_kind.skipped;
	  };

export function decode_core6_route_payload(
	input: Core6RoutePayloadDecodeInput,
): Core6DecodedRoutePayload {
	const raw_payload = (input.raw_payload ?? {}) as Core6RawRoutePayload;
	const raw_patterns =
		raw_payload[core6_route_payload_field.matched_patterns];
	const patterns: string[] = Array.isArray(raw_patterns) ? raw_patterns : [];
	const raw_schemas = raw_payload[core6_route_payload_field.search_schemas];
	const schemas: unknown[] = Array.isArray(raw_schemas) ? raw_schemas : [];
	const raw_loaders_data =
		raw_payload[core6_route_payload_field.loaders_data];
	const loaders_data: unknown[] = Array.isArray(raw_loaders_data)
		? raw_loaders_data
		: [];
	const raw_import_urls = raw_payload[core6_route_payload_field.import_urls];
	const import_urls: string[] = Array.isArray(raw_import_urls)
		? raw_import_urls
		: [];
	const raw_server_error_idx =
		raw_payload[core6_route_payload_field.outermost_server_error_idx];

	return {
		client_build_id:
			raw_payload[core6_route_payload_field.client_build_id] ??
			input.client_build_id ??
			"",
		css_bundles: raw_payload[core6_route_payload_field.css_bundles] ?? [],
		deps: raw_payload[core6_route_payload_field.deps] ?? [],
		meta_head_els:
			raw_payload[core6_route_payload_field.meta_head_els] ?? [],
		params: raw_payload[core6_route_payload_field.params] ?? {},
		rest_head_els:
			raw_payload[core6_route_payload_field.rest_head_els] ?? [],
		routes: patterns.map((pattern, idx) => {
			return {
				input: input.host.parse_input({
					pattern,
					schema: schemas[idx],
					search_params: input.search_params,
				}),
				loader_data: loaders_data[idx],
				module_url: import_urls[idx] ?? "",
				pattern,
			};
		}),
		server_error:
			typeof raw_server_error_idx === "number"
				? {
						idx: raw_server_error_idx,
						error: raw_payload[
							core6_route_payload_field.outermost_server_error
						],
					}
				: null,
		splat_values: raw_payload[core6_route_payload_field.splat_values] ?? [],
		title_html:
			raw_payload[core6_route_payload_field.title]?.dangerousInnerHTML ??
			null,
	};
}

export async function prepare_core6_route(
	input: Core6RoutePreparationInput,
): Promise<Core6PreparedRoute> {
	input.host.preload_css?.(input.payload.css_bundles);
	const modules = await import_core6_route_modules(input);
	throw_if_core6_route_preparation_aborted(input.signal);

	if (input.trigger === core6_route_preparation_trigger.boot) {
		input.host.set_provisional_route?.(
			build_core6_prepared_route(input, modules, []),
		);
	}

	const client_loader_results = await run_core6_client_loaders(
		input,
		modules,
	);
	throw_if_core6_route_preparation_aborted(input.signal);
	await input.host.wait_for_css?.(input.payload.css_bundles, input.signal);
	throw_if_core6_route_preparation_aborted(input.signal);

	return build_core6_prepared_route(input, modules, client_loader_results);
}

function abort_core6_later_client_loaders(
	controllers: AbortController[],
	idx: number,
): void {
	for (let later_idx = idx + 1; later_idx < controllers.length; later_idx++) {
		controllers[later_idx]?.abort();
	}
}

function build_core6_client_loader_server_state(
	payload: Core6DecodedRoutePayload,
	idx: number,
): Core6ClientLoaderServerState {
	const route = payload.routes[idx]!;
	return {
		clientBuildID: payload.client_build_id,
		loaderData: route.loader_data,
		matches: payload.routes.map((match) => {
			return {
				input: match.input,
				loaderData: match.loader_data,
				pattern: match.pattern,
			};
		}),
		outermostServerError: payload.server_error,
	};
}

function build_core6_known_matches(
	payload: Core6DecodedRoutePayload,
): Core6ClientLoaderKnownMatch[] {
	return payload.routes.map((route) => {
		return {
			input: route.input,
			pattern: route.pattern,
		};
	});
}

function build_core6_prepared_route(
	input: Core6RoutePreparationInput,
	modules: Map<string, Core6RouteModule>,
	client_loader_results: Core6ClientLoaderResult[],
): Core6PreparedRoute {
	const client_loader_error_idx = client_loader_results.findIndex(
		(result) => {
			return result.kind === core6_client_loader_result_kind.error;
		},
	);
	const error = build_core6_prepared_route_error(
		input.payload,
		client_loader_results,
		client_loader_error_idx,
	);

	return {
		client_build_id: input.payload.client_build_id,
		client_loaders: discover_core6_client_loaders(input.payload, modules),
		css_bundles: [...input.payload.css_bundles],
		deps: [...input.payload.deps],
		error,
		history_state: input.history_state,
		href: input.href,
		matches: input.payload.routes.map((route, idx) => {
			const result = client_loader_results[idx];
			const can_use_client_loader_data =
				client_loader_error_idx === -1 ||
				idx <= client_loader_error_idx;
			return {
				client_loader_data:
					can_use_client_loader_data &&
					result?.kind === core6_client_loader_result_kind.data
						? result.data
						: undefined,
				input: route.input,
				loader_data: route.loader_data,
				module: modules.get(route.module_url) ?? {},
				module_url: route.module_url,
				pattern: route.pattern,
			};
		}),
		meta_head_els: [...input.payload.meta_head_els],
		params: { ...input.payload.params },
		rest_head_els: [...input.payload.rest_head_els],
		splat_values: [...input.payload.splat_values],
		title_html: input.payload.title_html,
	};
}

function build_core6_prepared_route_error(
	payload: Core6DecodedRoutePayload,
	client_loader_results: Core6ClientLoaderResult[],
	client_loader_error_idx: number,
): Core6PreparedRouteError | null {
	if (payload.server_error) {
		return {
			error: payload.server_error.error,
			idx: payload.server_error.idx,
			source: core6_route_error_source.server,
		};
	}
	if (client_loader_error_idx === -1) {
		return null;
	}
	const result = client_loader_results[client_loader_error_idx]!;
	if (result.kind !== core6_client_loader_result_kind.error) {
		return null;
	}
	return {
		error: core6_error_string(result.error),
		idx: client_loader_error_idx,
		source: core6_route_error_source.client_loader,
	};
}

function core6_error_string(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}
	return String(error);
}

function create_core6_route_abort_error(): DOMException {
	return new DOMException("Aborted", core6_abort_error_name);
}

function discover_core6_client_loaders(
	payload: Core6DecodedRoutePayload,
	modules: Map<string, Core6RouteModule>,
): Core6DiscoveredClientLoader[] {
	const client_loaders: Core6DiscoveredClientLoader[] = [];
	for (const route of payload.routes) {
		const loader = modules.get(route.module_url)?.default?.client_loader;
		if (loader) {
			client_loaders.push({
				loader,
				pattern: route.pattern,
			});
		}
	}
	return client_loaders;
}

async function import_core6_route_modules(
	input: Core6RoutePreparationInput,
): Promise<Map<string, Core6RouteModule>> {
	const urls = input.payload.routes
		.map((route) => {
			return route.module_url;
		})
		.filter((url): url is string => {
			return url !== "";
		});
	const unique_urls = [...new Set(urls)];
	const pairs = await Promise.all(
		unique_urls.map(async (url) => {
			const module = await input.host.import_module(url, input.signal);
			return [url, module] as const;
		}),
	);
	const modules = new Map(pairs);
	throw_if_core6_route_preparation_aborted(input.signal);

	return modules;
}

function is_core6_abort_error(error: unknown): boolean {
	return (
		error instanceof DOMException && error.name === core6_abort_error_name
	);
}

async function run_core6_client_loaders(
	input: Core6RoutePreparationInput,
	modules: Map<string, Core6RouteModule>,
): Promise<Core6ClientLoaderResult[]> {
	const known_matches = build_core6_known_matches(input.payload);
	const controllers: AbortController[] = [];
	const promises: Array<Promise<Core6ClientLoaderResult>> = [];

	for (let idx = 0; idx < input.payload.routes.length; idx++) {
		const route = input.payload.routes[idx]!;
		if (
			input.payload.server_error &&
			idx >= input.payload.server_error.idx
		) {
			controllers.push(new AbortController());
			promises.push(
				Promise.resolve({
					kind: core6_client_loader_result_kind.skipped,
				}),
			);
			continue;
		}

		const loader = modules.get(route.module_url)?.default?.client_loader;
		if (!loader) {
			controllers.push(new AbortController());
			promises.push(
				Promise.resolve({
					kind: core6_client_loader_result_kind.skipped,
				}),
			);
			continue;
		}

		const controller = new AbortController();
		controllers.push(controller);
		propagate_core6_route_preparation_abort(input.signal, controller);
		promises.push(
			run_core6_client_loader({
				controller,
				idx,
				input,
				known_matches,
				loader,
				route,
				controllers,
			}),
		);
	}

	return await Promise.all(promises);
}

function propagate_core6_route_preparation_abort(
	parent_signal: AbortSignal,
	controller: AbortController,
): void {
	if (parent_signal.aborted) {
		controller.abort();
		return;
	}
	parent_signal.addEventListener(
		"abort",
		() => {
			controller.abort();
		},
		{ once: true },
	);
}

async function run_core6_client_loader(input: {
	controller: AbortController;
	controllers: AbortController[];
	idx: number;
	input: Core6RoutePreparationInput;
	known_matches: Core6ClientLoaderKnownMatch[];
	loader: Core6ClientLoaderFn;
	route: Core6DecodedRoute;
}): Promise<Core6ClientLoaderResult> {
	try {
		const data = await input.loader({
			trigger: input.input.trigger,
			href: input.input.href,
			historyState: input.input.history_state,
			pattern: input.route.pattern,
			params: { ...input.input.payload.params },
			splatValues: [...input.input.payload.splat_values],
			input: input.route.input,
			knownMatches: input.known_matches.map((match) => {
				return { ...match };
			}),
			serverPromise: Promise.resolve(
				build_core6_client_loader_server_state(
					input.input.payload,
					input.idx,
				),
			),
			signal: input.controller.signal,
		});
		if (data === undefined) {
			return { kind: core6_client_loader_result_kind.skipped };
		}
		return {
			data,
			kind: core6_client_loader_result_kind.data,
		};
	} catch (error) {
		if (is_core6_abort_error(error)) {
			return { kind: core6_client_loader_result_kind.skipped };
		}
		abort_core6_later_client_loaders(input.controllers, input.idx);
		return {
			error,
			kind: core6_client_loader_result_kind.error,
		};
	}
}

function throw_if_core6_route_preparation_aborted(signal: AbortSignal): void {
	if (signal.aborted) {
		throw create_core6_route_abort_error();
	}
}
