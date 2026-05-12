import {
	core6_route_preparation_trigger,
	type Core6ClientLoaderServerState,
	type Core6PreparedRoute,
	type Core6RouteModule,
	type Core6ViewDefinition,
} from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RoutePublication,
	type Core6RouteRenderEntry,
	type Core6RouteRenderState,
	type Core6RouteState,
} from "./route_publication.ts";
import type { Core6RouteRuntime } from "./route_runtime.ts";
import {
	core6_scope_cancelled_reason,
	core6_scope_stale_reason,
	create_core6_scope_manager,
	run_core6_scope_stage,
	type Core6Scope,
	type Core6ScopeCommitResult,
} from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

const core6_route_hmr_scope_kind = {
	update: "update",
} as const;

export const core6_route_hmr_result_kind = {
	busy: "busy",
	ignored: "ignored",
	interrupted: "interrupted",
	published: "published",
} as const;

export type Core6RouteHMRResultKind =
	(typeof core6_route_hmr_result_kind)[keyof typeof core6_route_hmr_result_kind];

export type Core6RouteHMRRuntime = Pick<
	Core6RouteRuntime,
	| "current_render_state"
	| "current_route"
	| "current_transaction_kind"
	| "run_route_prepared"
>;

export type Core6RouteHMRHost = {
	cache_module: (module_url: string, module: Core6RouteModule) => void;
	normalize_module_url: (url: string) => string;
	report_client_loader_error: (error: unknown) => void;
	route_state_equal: (
		previous_route: Core6RouteState,
		next_route: Core6RouteState,
	) => boolean;
};

export type Core6RouteHMRConfig = {
	host: Core6RouteHMRHost;
	runtime: Core6RouteHMRRuntime;
};

export type Core6RouteHMRUpdateInput = {
	module: Core6RouteModule;
	raw_module_url: string;
};

export type Core6RouteHMRViewConfig = {
	pattern: string;
	run_client_loader: boolean;
};

export type Core6RouteHMRStatus = {
	module_url: string;
	pattern: string;
	run_client_loader: boolean;
};

type Core6RouteHMRStatusRecord = Core6RouteHMRStatus & {
	scope_id: Core6RouteHMRScope["id"];
};

export type Core6RouteHMRPublishedResult = {
	kind: typeof core6_route_hmr_result_kind.published;
	publication: Core6RoutePublication;
};

export type Core6RouteHMRResult =
	| Core6RouteHMRPublishedResult
	| {
			kind: typeof core6_route_hmr_result_kind.busy;
	  }
	| {
			kind: typeof core6_route_hmr_result_kind.ignored;
	  }
	| {
			kind: typeof core6_route_hmr_result_kind.interrupted;
			reason:
				| typeof core6_scope_cancelled_reason
				| typeof core6_scope_stale_reason;
	  };

export type Core6RouteHMROwner = {
	cancel_current: () => boolean;
	configure_view: (input: Core6RouteHMRViewConfig) => void;
	current_status: () => Core6RouteHMRStatus | null;
	update: (input: Core6RouteHMRUpdateInput) => Promise<Core6RouteHMRResult>;
};

type Core6RouteHMRScope = Core6Scope<typeof core6_route_hmr_scope_kind.update>;

type Core6RouteHMRMatch = {
	idx: number;
	entry: Core6RouteRenderEntry;
	module_url: string;
	render_state: Core6RouteRenderState;
	route: Core6RouteState;
};

export function create_core6_route_hmr_owner(
	config: Core6RouteHMRConfig,
): Core6RouteHMROwner {
	const scope_manager =
		create_core6_scope_manager<typeof core6_route_hmr_scope_kind.update>();
	const rerun_client_loader_patterns = new Set<string>();
	let current_status: Core6RouteHMRStatusRecord | null = null;

	function current_match(module_url: string): Core6RouteHMRMatch | null {
		const route = config.runtime.current_route();
		const render_state = config.runtime.current_render_state();
		if (!route || !render_state) {
			return null;
		}
		const idx = render_state.entries.findIndex((entry) => {
			return (
				config.host.normalize_module_url(entry.module_url) ===
				module_url
			);
		});
		if (idx === -1) {
			return null;
		}
		const entry = render_state.entries[idx];
		if (!entry) {
			return null;
		}
		return {
			entry,
			idx,
			module_url,
			render_state,
			route,
		};
	}

	return {
		cancel_current: () => {
			return scope_manager.cancel_current();
		},
		configure_view: (input) => {
			if (input.run_client_loader) {
				rerun_client_loader_patterns.add(input.pattern);
				return;
			}
			rerun_client_loader_patterns.delete(input.pattern);
		},
		current_status: () => {
			if (!current_status) {
				return null;
			}
			return {
				module_url: current_status.module_url,
				pattern: current_status.pattern,
				run_client_loader: current_status.run_client_loader,
			};
		},
		update: async (input) => {
			const module_url = config.host.normalize_module_url(
				input.raw_module_url,
			);
			config.host.cache_module(module_url, input.module);
			const match = current_match(module_url);
			if (!match) {
				return { kind: core6_route_hmr_result_kind.ignored };
			}
			if (config.runtime.current_transaction_kind() !== null) {
				return { kind: core6_route_hmr_result_kind.busy };
			}

			const scope = scope_manager.start(
				core6_route_hmr_scope_kind.update,
			);
			const run_client_loader = rerun_client_loader_patterns.has(
				match.entry.pattern,
			);
			current_status = {
				module_url,
				pattern: match.entry.pattern,
				run_client_loader,
				scope_id: scope.id,
			};
			try {
				return await run_core6_route_hmr_update({
					config,
					input,
					match,
					run_client_loader,
					scope,
				});
			} finally {
				if (scope_manager.current()?.id === scope.id) {
					scope.complete(() => {
						return null;
					});
				}
				if (current_status?.scope_id === scope.id) {
					current_status = null;
				}
			}
		},
	};
}

async function run_core6_route_hmr_update(input: {
	config: Core6RouteHMRConfig;
	input: Core6RouteHMRUpdateInput;
	match: Core6RouteHMRMatch;
	run_client_loader: boolean;
	scope: Core6RouteHMRScope;
}): Promise<Core6RouteHMRResult> {
	const client_loader_result = await run_core6_route_hmr_client_loader(input);
	if (!client_loader_result.ok) {
		return {
			kind: core6_route_hmr_result_kind.interrupted,
			reason: client_loader_result.reason,
		};
	}
	if (input.config.runtime.current_transaction_kind() !== null) {
		return { kind: core6_route_hmr_result_kind.busy };
	}
	if (!route_matches_hmr_expectation(input.config, input.match)) {
		return {
			kind: core6_route_hmr_result_kind.interrupted,
			reason: core6_scope_stale_reason,
		};
	}

	const publish_result = input.config.runtime.run_route_prepared({
		intent: {
			apply_side_effects: false,
			history_state: input.match.route.historyState,
			href: input.match.route.href,
			preparation_trigger: core6_route_preparation_trigger.revalidation,
			publish_reason: core6_route_publish_reason.revalidation,
			search_params: new URLSearchParams(
				new URL(input.match.route.href).search,
			),
		},
		kind: core6_route_transaction_kind.revalidation,
		prepared: build_core6_route_hmr_prepared_route({
			client_loader_data: client_loader_result.value,
			match: input.match,
			module: input.input.module,
		}),
	});
	if (!publish_result.ok) {
		return {
			kind: core6_route_hmr_result_kind.interrupted,
			reason: publish_result.reason,
		};
	}
	return {
		kind: core6_route_hmr_result_kind.published,
		publication: publish_result.value,
	};
}

async function run_core6_route_hmr_client_loader(input: {
	config: Core6RouteHMRConfig;
	input: Core6RouteHMRUpdateInput;
	match: Core6RouteHMRMatch;
	run_client_loader: boolean;
	scope: Core6RouteHMRScope;
}): Promise<Core6ScopeCommitResult<unknown>> {
	const view = input.input.module.default as Core6ViewDefinition | undefined;
	const client_loader = view?.client_loader;
	if (!input.run_client_loader || !client_loader) {
		return input.scope.commit(() => {
			return input.match.entry.client_loader_data;
		});
	}
	return await run_core6_scope_stage(input.scope, async (signal) => {
		try {
			return await client_loader({
				historyState: input.match.route.historyState,
				href: input.match.route.href,
				input: input.match.entry.input,
				knownMatches: input.match.render_state.entries.map((entry) => {
					return {
						input: entry.input,
						pattern: entry.pattern,
					};
				}),
				params: { ...input.match.route.params },
				pattern: input.match.entry.pattern,
				serverPromise: Promise.resolve(
					core6_route_hmr_server_state(input.match),
				),
				signal,
				splatValues: [...input.match.route.splatValues],
				trigger: core6_route_preparation_trigger.revalidation,
			});
		} catch (error) {
			if (!signal.aborted) {
				input.config.host.report_client_loader_error(error);
			}
			return input.match.entry.client_loader_data;
		}
	});
}

function route_matches_hmr_expectation(
	config: Core6RouteHMRConfig,
	match: Core6RouteHMRMatch,
): boolean {
	const current_route = config.runtime.current_route();
	const current_render_state = config.runtime.current_render_state();
	if (!current_route || !current_render_state) {
		return false;
	}
	if (!config.host.route_state_equal(current_route, match.route)) {
		return false;
	}
	const current_entry = current_render_state.entries[match.idx];
	return (
		current_entry !== undefined &&
		config.host.normalize_module_url(current_entry.module_url) ===
			match.module_url
	);
}

function build_core6_route_hmr_prepared_route(input: {
	client_loader_data: unknown;
	match: Core6RouteHMRMatch;
	module: Core6RouteModule;
}): Core6PreparedRoute {
	const entries = input.match.render_state.entries.map((entry, idx) => {
		if (idx !== input.match.idx) {
			return entry;
		}
		return {
			...entry,
			client_loader_data: input.client_loader_data,
			module: input.module,
		};
	});
	const client_loaders: Core6PreparedRoute["client_loaders"] = [];
	for (const entry of entries) {
		const loader = (entry.module.default as Core6ViewDefinition | undefined)
			?.client_loader;
		if (loader) {
			client_loaders.push({
				loader,
				pattern: entry.pattern,
			});
		}
	}
	return {
		client_build_id: input.match.render_state.client_build_id,
		client_loaders,
		css_bundles: [],
		deps: [],
		error: input.match.render_state.error,
		history_state: input.match.render_state.history_state,
		href: input.match.route.href,
		matches: entries.map((entry) => {
			return {
				client_loader_data: entry.client_loader_data,
				input: entry.input,
				loader_data: entry.loader_data,
				module: entry.module,
				module_url: entry.module_url,
				pattern: entry.pattern,
			};
		}),
		meta_head_els: [],
		params: { ...input.match.render_state.params },
		rest_head_els: [],
		splat_values: [...input.match.render_state.splat_values],
		title_html: null,
	};
}

function core6_route_hmr_server_state(
	match: Core6RouteHMRMatch,
): Core6ClientLoaderServerState {
	const server_error =
		match.render_state.error?.source === "server"
			? {
					error: match.render_state.error.error,
					idx: match.render_state.error.idx,
				}
			: null;
	return {
		clientBuildID: match.render_state.client_build_id,
		loaderData: match.entry.loader_data,
		matches: match.render_state.entries.map((entry) => {
			return {
				input: entry.input,
				loaderData: entry.loader_data,
				pattern: entry.pattern,
			};
		}),
		outermostServerError: server_error,
	};
}
