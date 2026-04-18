import { jsonDeepEquals } from "vorma/kit/json";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { R, type Result } from "vorma/kit/result";
import { create_typed_api_client } from "./api_client.ts";
import type {
	ClientCore,
	InitOptions as CoreInitOptions,
	RouteDefinition,
} from "./create_client_core";
import {
	create_client_core,
	type RouteErrorState,
	type RouteRenderEntry,
	type RouteRenderState,
	type RouteState,
	type ScrollIntent,
	type WorkState,
} from "./create_client_core.ts";
import { type LinkNavFns } from "./make_link_props.ts";
import { get_entry_key } from "./resolve_outlet_slot.ts";
import type {
	AppConfig,
	LinkPropsBase,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedDefineRouteInput,
	MakeTypedLinkProps,
	MakeTypedLoaderOutput,
	MakeTypedLoaderPattern,
	MakeTypedNavProps,
	MakeTypedNavTarget,
	MakeTypedRouteDestination,
	MakeTypedRouteProps,
} from "./types";
import {
	create_typed_build_href,
	create_typed_navigate,
	create_typed_prefetch,
} from "./url.ts";

export type DecomposedState = {
	// Always a new reference
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;

	// Stable per channel
	loaders_data: unknown[];
	client_loaders_data: unknown[];
	matched_patterns: string[];
	import_urls: string[];
	entry_keys: string[];
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export type DecomposedCommitFn = (
	state: DecomposedState,
	scroll_intent?: ScrollIntent,
) => void;

type LinkAttributeCandidate = {
	url: URL;
	matched_patterns: string[];
};

export type AdapterRenderArgs<App> = {
	App: App;
	el: HTMLElement;
};

export type AdapterInitOptions<App> = Omit<CoreInitOptions, "render"> & {
	render?: (args: AdapterRenderArgs<App>) => void | Promise<void>;
};

type AdapterBase<A extends AppConfig> = {
	core: ClientCore;

	nav_fns: LinkNavFns;

	passthrough: Pick<
		ClientCore,
		"revalidate" | "getRouteState" | "getWorkState"
	> & {
		navigate: <P extends MakeTypedLoaderPattern<A>>(
			props: MakeTypedNavProps<A, P>,
		) => Promise<{ didNavigate: boolean }>;

		prefetch: <P extends MakeTypedLoaderPattern<A>>(
			target: MakeTypedNavTarget<A, P>,
		) => void;

		cancelPrefetch: <P extends MakeTypedLoaderPattern<A>>(
			target: MakeTypedNavTarget<A, P>,
		) => void;

		buildHref: <P extends MakeTypedLoaderPattern<A>>(
			destination: MakeTypedRouteDestination<A, P>,
		) => string;

		apiClient: MakeTypedAPIClient<A>;
	};
};

export function create_adapter_base<A extends AppConfig>(
	app_config: A,
	on_commit: DecomposedCommitFn,
	api_decorator?: MakeTypedAPIDecorator<A>,
): Result<AdapterBase<A>> {
	const attribute_registry_res = createPatternRegistry({
		dynamicParamPrefixRune: ":",
		splatSegmentRune: "*",
		explicitIndexSegment: "_index",
	});
	if (!attribute_registry_res.ok) {
		return R.err(
			`Failed to create link attribute registry: ${attribute_registry_res.err}`,
		);
	}
	const attribute_registry = attribute_registry_res.val;

	let prev: DecomposedState = {
		entries: [],
		error: null,
		loaders_data: [],
		client_loaders_data: [],
		matched_patterns: [],
		import_urls: [],
		entry_keys: [],
		params: {},
		splat_values: [],
		client_build_id: "",
		history_state: undefined,
	};

	function decomposed_commit(
		route_state: RouteRenderState,
		scroll_intent?: ScrollIntent,
	): void {
		for (const entry of route_state.entries) {
			register_link_pattern(entry.pattern);
		}

		const next: DecomposedState = {
			entries: route_state.entries,
			error: route_state.error,
			loaders_data: stable(
				prev.loaders_data,
				route_state.entries.map((e) => {
					return e.loader_data;
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
			client_build_id: stable(
				prev.client_build_id,
				route_state.client_build_id,
			),
			history_state: stable(
				prev.history_state,
				route_state.history_state,
			),
		};

		prev = next;
		on_commit(next, scroll_intent);
	}

	const core_res = create_client_core(app_config, decomposed_commit);
	if (!core_res.ok) {
		return R.err(core_res.err);
	}
	const core = core_res.val;

	const nav_fns: LinkNavFns = {
		navigate: (props) => {
			return core.navigate(props.href, {
				replace: props.replace,
				scrollToTop: props.scrollToTop,
				state: props.state,
			});
		},
		start_prefetch: core.start_prefetch,
		stop_prefetch: core.stop_prefetch,
		save_current_scroll: core.save_current_scroll,
		register_link_pattern,
		get_link_attribute_state,
	};

	const navigate = create_typed_navigate<A>(core.navigate);
	const prefetch = create_typed_prefetch<A>(core.start_prefetch);
	const cancel_prefetch = create_typed_prefetch<A>(core.stop_prefetch);
	const build_href = create_typed_build_href<A>();

	const api_client = create_typed_api_client<A>(
		app_config.actionsMountRoot,
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
			buildHref: build_href,
			revalidate: core.revalidate,
			getRouteState: core.getRouteState,
			getWorkState: core.getWorkState,
			apiClient: api_client,
		},
	});

	function register_link_pattern(pattern: string): void {
		registerPattern(attribute_registry, pattern);
	}

	function href_to_link_candidate(href: string): LinkAttributeCandidate {
		const url = new URL(href, window.location.href);
		const match = findNestedMatches(attribute_registry, url.pathname);
		return {
			url,
			matched_patterns:
				match?.matches.map((m) => {
					return m.registeredPattern.originalPattern;
				}) ?? [],
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
				jsonDeepEquals(
					target.matched_patterns,
					candidate.matched_patterns,
				)) &&
			(match_rules?.includeSearch !== true ||
				target.url.search === candidate.url.search) &&
			(match_rules?.includeHash !== true ||
				target.url.hash === candidate.url.hash)
		);
	}

	function ancestor_link_match(
		target: LinkAttributeCandidate,
		candidate: LinkAttributeCandidate,
	): boolean {
		const target_path =
			target.url.pathname === "/" ? "/" : `${target.url.pathname}/`;
		return (
			target.matched_patterns.length > 0 &&
			candidate.matched_patterns.length >
				target.matched_patterns.length &&
			(target_path === "/" ||
				candidate.url.pathname.startsWith(target_path)) &&
			target.matched_patterns.every((pattern, i) => {
				return candidate.matched_patterns[i] === pattern;
			})
		);
	}

	function get_link_attribute_state(
		href: string,
		match_rules: LinkPropsBase["attributeMatchRules"],
		route_state: RouteState | null,
		work_state: WorkState,
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
			matched_patterns: route_state.matches.map((m) => {
				return m.pattern;
			}),
		};
		const pending = work_state.navigation
			? href_to_link_candidate(work_state.navigation.href)
			: null;

		return {
			active_exact: exact_link_match(target, route, match_rules),
			active_ancestor: ancestor_link_match(target, route),
			pending_exact: pending
				? exact_link_match(target, pending, match_rules)
				: false,
			pending_ancestor: pending
				? ancestor_link_match(target, pending)
				: false,
		};
	}
}

function stable<T>(prev: T, next: T): T {
	return jsonDeepEquals(prev, next) ? prev : next;
}

type HookReturn<T, Wrapped extends boolean> = Wrapped extends true
	? () => T
	: T;

type StateSelector<State, Selected> = (state: State) => Selected;

export type VormaClient<
	A extends AppConfig,
	Element,
	AnchorProps extends object,
	AccessorWrapped extends boolean = false,
	App = unknown,
> = AdapterBase<A>["passthrough"] & {
	init: (options: AdapterInitOptions<App>) => Promise<Result<void>>;

	defineRoute: <P extends MakeTypedLoaderPattern<A>, T = any>(
		input: MakeTypedDefineRouteInput<A, P, T, Element>,
	) => RouteDefinition;

	RootOutlet: (
		props: { idx?: number } & Record<string, unknown>,
	) => Element | null;

	Link: <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<AnchorProps, "href"> & MakeTypedLinkProps<A, P>,
	) => Element;

	useRouteState: {
		(): HookReturn<RouteState, AccessorWrapped>;
		<T>(
			selector: StateSelector<RouteState, T>,
		): HookReturn<T, AccessorWrapped>;
	};

	useWorkState: {
		(): HookReturn<WorkState, AccessorWrapped>;
		<T>(
			selector: StateSelector<WorkState, T>,
		): HookReturn<T, AccessorWrapped>;
	};

	useLoaderData: <P extends MakeTypedLoaderPattern<A>>(
		props: MakeTypedRouteProps<A, P>,
	) => HookReturn<MakeTypedLoaderOutput<A, P>, AccessorWrapped>;

	usePatternLoaderData: <P extends MakeTypedLoaderPattern<A>>(
		pattern: P,
	) => HookReturn<MakeTypedLoaderOutput<A, P> | undefined, AccessorWrapped>;

	useClientLoaderData: <P extends MakeTypedLoaderPattern<A>, T>(
		props: MakeTypedRouteProps<A, P, T>,
	) => HookReturn<T, AccessorWrapped>;

	usePatternClientLoaderData: <T>(
		pattern: MakeTypedLoaderPattern<A>,
	) => HookReturn<T | undefined, AccessorWrapped>;
};
