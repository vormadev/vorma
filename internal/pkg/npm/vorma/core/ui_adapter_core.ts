import {
	create_client_core,
	create_typed_api_client,
	create_typed_navigate,
	get_entry_key,
	type AppConfig,
	type LinkNavFns,
	type MakeTypedAPIDecorator,
	type MakeTypedLoaderPattern,
	type MakeTypedRouteProps,
	type MakeTypedRouterData,
	type RouteEntry,
	type RouteState,
	type ScrollIntent,
} from "vorma/__internal";
import { jsonDeepEquals } from "vorma/kit/json";
import { R, type Result } from "vorma/kit/result";
import type { ClientCore, RouteDefinition } from "./create_client_core";
import type {
	MakeTypedAPIClient,
	MakeTypedDefineRouteInput,
	MakeTypedLinkProps,
	MakeTypedLoaderOutput,
	MakeTypedNavigateProps,
} from "./types";

export type DecomposedState = {
	// Always a new reference
	entries: RouteEntry[];

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

type AdapterBase<A extends AppConfig> = {
	core: ClientCore;

	nav_fns: LinkNavFns;

	passthrough: Pick<
		ClientCore,
		| "init"
		| "revalidate"
		| "submit"
		| "getStatus"
		| "getClientBuildID"
		| "getRootEl"
		| "setupGlobalLoadingIndicator"
		| "revalidateOnWindowFocus"
	> & {
		navigate: <P extends MakeTypedLoaderPattern<A>>(
			props: MakeTypedNavigateProps<A, P> & {
				replace?: boolean;
				scrollToTop?: boolean;
				search?: string;
				hash?: string;
				state?: unknown;
			},
		) => Promise<{ didNavigate: boolean }>;

		getRouterData: {
			(): MakeTypedRouterData<A>;
			<P extends MakeTypedLoaderPattern<A>>(
				routeProps: MakeTypedRouteProps<A, P>,
			): MakeTypedRouterData<A, P>;
		};

		apiClient: MakeTypedAPIClient<A>;
	};
};

export function create_adapter_base<A extends AppConfig>(
	app_config: A,
	on_commit: DecomposedCommitFn,
	api_decorator?: MakeTypedAPIDecorator<A>,
): Result<AdapterBase<A>> {
	let prev: DecomposedState = {
		entries: [],
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
		route_state: RouteState,
		scroll_intent?: ScrollIntent,
	): void {
		const next: DecomposedState = {
			entries: route_state.entries,
			loaders_data: stable(
				prev.loaders_data,
				route_state.entries.map((e) => {
					return e.data;
				}),
			),
			client_loaders_data: stable(
				prev.client_loaders_data,
				route_state.entries.map((e) => {
					return e.client_data;
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
		navigate: core.navigate,
		start_prefetch: core.start_prefetch,
		stop_prefetch: core.stop_prefetch,
		save_current_scroll: core.save_current_scroll,
	};

	const navigate = create_typed_navigate<A>(core.navigate);

	const api_client = create_typed_api_client<A>(
		app_config.actionsMountRoot,
		core.submit,
		api_decorator,
	);

	function getRouterData(): MakeTypedRouterData<A>;
	function getRouterData<P extends MakeTypedLoaderPattern<A>>(
		routeProps: MakeTypedRouteProps<A, P>,
	): MakeTypedRouterData<A, P>;
	function getRouterData(_routeProps?: any): any {
		return core.getRouterData();
	}

	return R.ok({
		core,
		nav_fns,
		passthrough: {
			init: core.init,
			navigate,
			revalidate: core.revalidate,
			submit: core.submit,
			getStatus: core.getStatus,
			getClientBuildID: core.getClientBuildID,
			getRootEl: core.getRootEl,
			getRouterData,
			setupGlobalLoadingIndicator: core.setupGlobalLoadingIndicator,
			revalidateOnWindowFocus: core.revalidateOnWindowFocus,
			apiClient: api_client,
		},
	});
}

function stable<T>(prev: T, next: T): T {
	return jsonDeepEquals(prev, next) ? prev : next;
}

type HookReturn<T, Wrapped extends boolean> = Wrapped extends true
	? () => T
	: T;

export type VormaClient<
	A extends AppConfig,
	Element,
	AnchorProps extends object,
	AccessorWrapped extends boolean = false,
> = AdapterBase<A>["passthrough"] & {
	defineRoute: <P extends MakeTypedLoaderPattern<A>, T = any>(
		input: MakeTypedDefineRouteInput<A, P, T, Element>,
	) => RouteDefinition;

	RootOutlet: (
		props: { idx?: number } & Record<string, unknown>,
	) => Element | null;

	Link: <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<AnchorProps, "href" | "pattern"> & MakeTypedLinkProps<A, P>,
	) => Element;

	useLoaderData: <P extends MakeTypedLoaderPattern<A>>(
		props: MakeTypedRouteProps<A, P>,
	) => HookReturn<MakeTypedLoaderOutput<A, P>, AccessorWrapped>;

	usePatternLoaderData: <P extends MakeTypedLoaderPattern<A>>(
		pattern: P,
	) => HookReturn<MakeTypedLoaderOutput<A, P> | undefined, AccessorWrapped>;

	useRouterData: {
		(): HookReturn<MakeTypedRouterData<A>, AccessorWrapped>;
		<P extends MakeTypedLoaderPattern<A>>(
			routeProps: MakeTypedRouteProps<A, P>,
		): HookReturn<MakeTypedRouterData<A, P>, AccessorWrapped>;
	};

	useClientLoaderData: <P extends MakeTypedLoaderPattern<A>, T>(
		props: MakeTypedRouteProps<A, P, T>,
	) => HookReturn<T, AccessorWrapped>;

	usePatternClientLoaderData: <T>(
		pattern: MakeTypedLoaderPattern<A>,
	) => HookReturn<T | undefined, AccessorWrapped>;
};
