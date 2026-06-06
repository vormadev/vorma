import type { ReadonlySignal } from "@preact/signals";
import { jsonDeepEquals } from "vorma/kit/json";
import { R, type Result } from "vorma/kit/result";
import { create_typed_api_client } from "./api_client.ts";
import { create_client_matcher, type ClientMatcher } from "./client_wasm/matcher.ts";
import type {
	ClientCore,
	ClientOptions as CoreClientOptions,
	ViewDefinition,
} from "./create_client_core";
import {
	create_client_core,
	type ClientCommit,
	type RouteRenderEntry,
	type RouteRenderState,
	type ScrollIntent,
} from "./create_client_core.ts";
import {
	type LinkNavFns,
	type LinkRouteState,
	type LinkWorkState,
} from "./make_link_props.ts";
import { get_entry_key } from "./resolve_outlet_slot.ts";
import type {
	AppConfig,
	LinkPropsBase,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
	ToApiClient,
	ToApiDecorator,
	ToDefineViewArgs,
	ToLinkProps,
	ToNavigateArgs,
	ToNavigationTarget,
	ToRouteDestination,
	ToRouteSyncArgs,
	ToViewComponentProps,
	ToViewOutput,
	ToViewPattern,
} from "./types";
import {
	create_typed_navigate,
	create_typed_prefetch,
	create_typed_to_href,
} from "./url.ts";
import type { WorkState } from "./work_state.ts";

type ClientMatcherFactory = () => Promise<ClientMatcher>;

export type DecomposedState = {
	// Always a new reference
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;

	// Stable per channel
	views_data: unknown[];
	client_loaders_data: unknown[];
	matched_patterns: string[];
	import_urls: string[];
	entry_keys: string[];
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export type DecomposedCommit = {
	link_state_version?: number;
	route?: RouteState;
	route_reason?: RouteUpdateReason;
	scroll_intent?: ScrollIntent;
	state?: DecomposedState;
	work?: WorkState;
};

export type DecomposedCommitFn = (commit: DecomposedCommit) => void;

type LinkAttributeCandidate = {
	url: URL;
	matched_patterns: string[];
};

let client_matcher_factory_for_test: ClientMatcherFactory | null = null;

export function set_client_matcher_factory_for_test(
	factory: ClientMatcherFactory | null,
): () => void {
	const prev_factory = client_matcher_factory_for_test;
	client_matcher_factory_for_test = factory;
	return () => {
		client_matcher_factory_for_test = prev_factory;
	};
}

export type AdapterRenderArgs<RootOutletComponent> = {
	RootOutlet: RootOutletComponent;
	rootEl: HTMLElement;
};

export type AdapterClientOptions<RootOutletComponent> = Omit<
	CoreClientOptions,
	"render"
> & {
	render?: (args: AdapterRenderArgs<RootOutletComponent>) => void | Promise<void>;
};

type AdapterBase<A extends AppConfig> = {
	core: ClientCore;

	nav_fns: LinkNavFns;

	passthrough: Pick<
		ClientCore,
		"revalidate" | "getRouteState" | "getWorkState" | "workIndicator"
	> & {
		navigate: <P extends ToViewPattern<A>>(
			args: ToNavigateArgs<A, P>,
		) => Promise<{ didNavigate: boolean }>;

		prefetch: <P extends ToViewPattern<A>>(target: ToNavigationTarget<A, P>) => void;

		cancelPrefetch: <P extends ToViewPattern<A>>(
			target: ToNavigationTarget<A, P>,
		) => void;

		toHref: <P extends ToViewPattern<A>>(
			destination: ToRouteDestination<A, P>,
		) => string;

		apiClient: ToApiClient<A>;
	};
};

export function create_adapter_base<A extends AppConfig>(
	app_config: A,
	on_commit: DecomposedCommitFn,
	api_decorator?: ToApiDecorator<A>,
): Result<AdapterBase<A>> {
	let prev: DecomposedState = {
		entries: [],
		error: null,
		views_data: [],
		client_loaders_data: [],
		matched_patterns: [],
		import_urls: [],
		entry_keys: [],
		params: {},
		splat_values: [],
		client_build_id: "",
		history_state: undefined,
	};
	let link_matcher: ClientMatcher | null = null;
	let link_matcher_loading = false;
	let link_state_version = 0;
	const link_state_listeners = new Set<() => void>();
	const queued_link_patterns = new Set<string>();

	function decomposed_commit(client_commit: ClientCommit): void {
		const adapter_commit: DecomposedCommit = {};
		const route_render = client_commit.route_render;
		if (route_render) {
			adapter_commit.state = decompose_route_render_state(route_render.state);
			adapter_commit.scroll_intent = route_render.scroll_intent;
		}
		if (client_commit.route_update) {
			adapter_commit.route = client_commit.route_update.route;
			adapter_commit.route_reason = client_commit.route_update.reason;
		}
		if (client_commit.work) {
			adapter_commit.work = client_commit.work;
		}
		on_commit(adapter_commit);
	}

	function emit_link_state_update(): void {
		link_state_version++;
		on_commit({ link_state_version });
		for (const listener of link_state_listeners) {
			listener();
		}
	}

	function start_link_matcher_load(): void {
		if (link_matcher || link_matcher_loading) {
			return;
		}
		link_matcher_loading = true;
		void (client_matcher_factory_for_test ?? create_client_matcher)()
			.then((matcher) => {
				link_matcher = matcher;
				for (const pattern of queued_link_patterns) {
					matcher.register_pattern(pattern);
				}
				emit_link_state_update();
			})
			.catch(() => {});
	}

	function decompose_route_render_state(
		route_state: RouteRenderState,
	): DecomposedState {
		for (const entry of route_state.entries) {
			register_link_pattern(entry.pattern);
		}

		const next: DecomposedState = {
			entries: route_state.entries,
			error: route_state.error,
			views_data: stable(
				prev.views_data,
				route_state.entries.map((e) => {
					return e.view_data;
				}),
			),
			client_loaders_data: stable(
				prev.client_loaders_data,
				route_state.entries.map((e) => {
					return e.client_loader_data;
				}),
			),
			matched_patterns: stable(
				prev.matched_patterns,
				route_state.entries.map((e) => {
					return e.pattern;
				}),
			),
			import_urls: stable(
				prev.import_urls,
				route_state.entries.map((e) => {
					return e.module_url;
				}),
			),
			entry_keys: stable(
				prev.entry_keys,
				route_state.entries.map((e) => {
					return get_entry_key(e);
				}),
			),
			params: stable(prev.params, route_state.params),
			splat_values: stable(prev.splat_values, route_state.splat_values),
			client_build_id: stable(prev.client_build_id, route_state.client_build_id),
			history_state: stable(prev.history_state, route_state.history_state),
		};

		prev = next;
		return next;
	}

	const core_res = create_client_core(app_config, decomposed_commit);
	if (!core_res.ok) {
		return R.err(core_res.err);
	}
	const core = core_res.val;

	const nav_fns: LinkNavFns = {
		navigate: (args) => {
			return core.navigate(args.href, {
				replace: args.replace,
				scrollToTop: args.scrollToTop,
				skipWorkIndicator: args.skipWorkIndicator,
				state: args.state,
			});
		},
		start_prefetch: core.start_prefetch,
		stop_prefetch: core.stop_prefetch,
		save_current_scroll: core.save_current_scroll,
		register_link_pattern,
		get_link_attribute_state,
		get_link_state_version: () => {
			return link_state_version;
		},
		subscribe_link_state: (listener) => {
			link_state_listeners.add(listener);
			return () => {
				link_state_listeners.delete(listener);
			};
		},
	};
	start_link_matcher_load();

	const navigate = create_typed_navigate<A>(core.navigate);
	const prefetch = create_typed_prefetch<A>(core.start_prefetch);
	const cancel_prefetch = create_typed_prefetch<A>(core.stop_prefetch);
	const to_href = create_typed_to_href<A>();

	const api_client = create_typed_api_client<A>(
		app_config.apiMountRoot,
		core.submit_inner,
		api_decorator,
	);

	return R.ok({
		core,
		nav_fns,
		passthrough: {
			navigate,
			prefetch,
			cancelPrefetch: cancel_prefetch,
			toHref: to_href,
			revalidate: core.revalidate,
			getRouteState: core.getRouteState,
			getWorkState: core.getWorkState,
			workIndicator: core.workIndicator,
			apiClient: api_client,
		},
	});

	function register_link_pattern(pattern: string): void {
		if (queued_link_patterns.has(pattern)) {
			return;
		}
		queued_link_patterns.add(pattern);
		link_matcher?.register_pattern(pattern);
	}

	function href_to_link_candidate(href: string): LinkAttributeCandidate {
		const url = new URL(href, window.location.href);
		const match = link_matcher?.find_nested_matches(url.pathname);
		return {
			url,
			matched_patterns: match?.patterns ?? [],
		};
	}

	function exact_link_match(
		target: LinkAttributeCandidate,
		candidate: LinkAttributeCandidate,
		match_rules: LinkPropsBase["attributeMatchRules"],
	): boolean {
		return (
			target.url.pathname === candidate.url.pathname &&
			(target.matched_patterns.length === 0 ||
				candidate.matched_patterns.length === 0 ||
				jsonDeepEquals(target.matched_patterns, candidate.matched_patterns)) &&
			(match_rules?.includeSearch !== true ||
				target.url.search === candidate.url.search) &&
			(match_rules?.includeHash !== true || target.url.hash === candidate.url.hash)
		);
	}

	function ancestor_link_match(
		target: LinkAttributeCandidate,
		candidate: LinkAttributeCandidate,
	): boolean {
		const target_path = target.url.pathname === "/" ? "/" : `${target.url.pathname}/`;
		return (
			target.matched_patterns.length > 0 &&
			candidate.matched_patterns.length > target.matched_patterns.length &&
			(target_path === "/" || candidate.url.pathname.startsWith(target_path)) &&
			target.matched_patterns.every((pattern, i) => {
				return candidate.matched_patterns[i] === pattern;
			})
		);
	}

	function get_link_attribute_state(
		href: string,
		match_rules: LinkPropsBase["attributeMatchRules"],
		route_state: LinkRouteState | null,
		work_state: LinkWorkState,
	) {
		if (match_rules?.skip === true || !route_state) {
			return {
				active_exact: false,
				active_ancestor: false,
				pending_exact: false,
				pending_ancestor: false,
			};
		}

		const target = href_to_link_candidate(href);
		const route_url = new URL(route_state.href, window.location.href);
		const route = {
			url: route_url,
			matched_patterns: route_state.matched_patterns,
		};
		const pending = work_state.navigation_href
			? href_to_link_candidate(work_state.navigation_href)
			: null;

		return {
			active_exact: exact_link_match(target, route, match_rules),
			active_ancestor: ancestor_link_match(target, route),
			pending_exact: pending
				? exact_link_match(target, pending, match_rules)
				: false,
			pending_ancestor: pending ? ancestor_link_match(target, pending) : false,
		};
	}
}

function stable<T>(prev: T, next: T): T {
	return jsonDeepEquals(prev, next) ? prev : next;
}

type HookReturn<T, Mode extends "value" | "accessor" | "signal"> = Mode extends "accessor"
	? () => T
	: Mode extends "signal"
		? ReadonlySignal<T>
		: T;

type StateSelector<State, Selected> = (state: State) => Selected;

export type VormaClient<
	A extends AppConfig,
	Element,
	AnchorProps extends object,
	HookReturnMode extends "value" | "accessor" | "signal" = "value",
> = AdapterBase<A>["passthrough"] & {
	boot: () => Promise<Result<void>>;

	defineView: <P extends ToViewPattern<A>, T = any>(
		input: ToDefineViewArgs<A, P, T, Element>,
	) => ViewDefinition;

	RootOutlet: (props: { idx?: number } & Record<string, unknown>) => Element | null;

	Link: <P extends ToViewPattern<A>>(
		props: Omit<AnchorProps, "href"> & ToLinkProps<A, P>,
	) => Element;

	useRouteSync: <P extends ToViewPattern<A>>(args: ToRouteSyncArgs<A, P>) => void;

	useRouteState: {
		(): HookReturn<RouteState, HookReturnMode>;
		<T>(selector: StateSelector<RouteState, T>): HookReturn<T, HookReturnMode>;
	};

	useWorkState: {
		(): HookReturn<WorkState, HookReturnMode>;
		<T>(selector: StateSelector<WorkState, T>): HookReturn<T, HookReturnMode>;
	};

	useViewData: <P extends ToViewPattern<A>>(
		args: ToViewComponentProps<A, P>,
	) => HookReturn<ToViewOutput<A, P>, HookReturnMode>;

	usePatternViewData: <P extends ToViewPattern<A>>(
		pattern: P,
	) => HookReturn<ToViewOutput<A, P> | undefined, HookReturnMode>;

	useClientLoaderData: <P extends ToViewPattern<A>, T>(
		args: ToViewComponentProps<A, P, T>,
	) => HookReturn<T, HookReturnMode>;

	usePatternClientLoaderData: <T>(
		pattern: ToViewPattern<A>,
	) => HookReturn<T | undefined, HookReturnMode>;
};
