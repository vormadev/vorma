import type { ReadonlySignal } from "@preact/signals";
import { Effect, Result as EffectResult } from "effect";
import { R, type Result } from "vorma/kit/result";
import { create_typed_api_client } from "./api_client.ts";
import type {
	ClientCore,
	ClientOptions as CoreClientOptions,
	ViewDefinition,
} from "./create_client_core";
import {
	create_client_core,
	type ClientCommit,
	type ScrollIntent,
	type WorkState,
} from "./create_client_core.ts";
import {
	make_adapter_runtime,
	type DecomposedState as AdapterDecomposedState,
	type AdapterRuntime,
} from "./effect_runtime/adapter_runtime.ts";
import {
	make_link_intent_runtime,
	type LinkIntentRuntime,
} from "./effect_runtime/link_intent_runtime.ts";
import {
	make_outlet_slot_runtime,
	type OutletSlotRuntime,
} from "./effect_runtime/outlet_slot_runtime.ts";
import { type LinkNavFns } from "./make_link_props.ts";
import type {
	AppConfig,
	RouteState,
	RouteUpdateReason,
	ToAPIClient,
	ToAPIDecorator,
	ToDefineViewArgs,
	ToLinkProps,
	ToLoaderOutput,
	ToNavigateArgs,
	ToNavigationTarget,
	ToRouteComponentProps,
	ToRouteDestination,
	ToRouteSyncArgs,
	ToViewPattern,
} from "./types";
import {
	create_typed_navigate,
	create_typed_prefetch,
	create_typed_to_href,
} from "./url.ts";

export type DecomposedState = AdapterDecomposedState;

export type DecomposedCommit = {
	route?: RouteState;
	route_reason?: RouteUpdateReason;
	scroll_intent?: ScrollIntent;
	state?: DecomposedState;
	work?: WorkState;
};

export type DecomposedCommitFn = (commit: DecomposedCommit) => void;

// __TODO why is this generic called "App"? Shouldn't it be "Component" or something?
export type AdapterRenderArgs<App> = {
	RootOutlet: App;
	rootEl: HTMLElement;
};

export type AdapterClientOptions<App> = Omit<CoreClientOptions, "render"> & {
	render?: (args: AdapterRenderArgs<App>) => void | Promise<void>;
};

type AdapterBase<A extends AppConfig> = {
	adapter_runtime: AdapterRuntime;

	core: ClientCore;

	link_intent_runtime: LinkIntentRuntime;

	nav_fns: LinkNavFns;

	outlet_slot_runtime: OutletSlotRuntime;

	passthrough: Pick<
		ClientCore,
		"revalidate" | "getRouteState" | "getWorkState" | "workIndicator"
	> & {
		navigate: <P extends ToViewPattern<A>>(
			args: ToNavigateArgs<A, P>,
		) => Promise<{ didNavigate: boolean }>;

		prefetch: <P extends ToViewPattern<A>>(
			target: ToNavigationTarget<A, P>,
		) => void;

		cancelPrefetch: <P extends ToViewPattern<A>>(
			target: ToNavigationTarget<A, P>,
		) => void;

		toHref: <P extends ToViewPattern<A>>(
			destination: ToRouteDestination<A, P>,
		) => string;

		apiClient: ToAPIClient<A>;
	};
};

export function create_adapter_base<A extends AppConfig>(
	app_config: A,
	on_commit: DecomposedCommitFn,
	api_decorator?: ToAPIDecorator<A>,
): Result<AdapterBase<A>> {
	const adapter_runtime_result = Effect.runSync(
		Effect.result(make_adapter_runtime()),
	);
	if (EffectResult.isFailure(adapter_runtime_result)) {
		return R.err(
			`Failed to create adapter runtime: ${adapter_runtime_result.failure.reason}`,
		);
	}
	const adapter_runtime = adapter_runtime_result.success;

	function decomposed_commit(client_commit: ClientCommit): void {
		const adapter_commit: DecomposedCommit = {};
		const route_render = client_commit.route_render;
		if (route_render) {
			adapter_commit.state = adapter_runtime.decompose_route_render_state(
				route_render.state,
			);
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

	const core_res = create_client_core(app_config, decomposed_commit);
	if (!core_res.ok) {
		return R.err(core_res.err);
	}
	const core = core_res.val;
	const link_intent_runtime = Effect.runSync(make_link_intent_runtime());
	const outlet_slot_runtime = Effect.runSync(make_outlet_slot_runtime());

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
		register_link_pattern: adapter_runtime.register_link_pattern,
		get_link_attribute_state: adapter_runtime.get_link_attribute_state,
	};

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
		adapter_runtime,
		core,
		link_intent_runtime,
		nav_fns,
		outlet_slot_runtime,
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
}

type HookReturn<
	T,
	Mode extends "value" | "accessor" | "signal",
> = Mode extends "accessor"
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

	RootOutlet: (
		props: { idx?: number } & Record<string, unknown>,
	) => Element | null;

	Link: <P extends ToViewPattern<A>>(
		props: Omit<AnchorProps, "href"> & ToLinkProps<A, P>,
	) => Element;

	useRouteSync: <P extends ToViewPattern<A>>(
		args: ToRouteSyncArgs<A, P>,
	) => void;

	useRouteState: {
		(): HookReturn<RouteState, HookReturnMode>;
		<T>(
			selector: StateSelector<RouteState, T>,
		): HookReturn<T, HookReturnMode>;
	};

	useWorkState: {
		(): HookReturn<WorkState, HookReturnMode>;
		<T>(
			selector: StateSelector<WorkState, T>,
		): HookReturn<T, HookReturnMode>;
	};

	useLoaderData: <P extends ToViewPattern<A>>(
		args: ToRouteComponentProps<A, P>,
	) => HookReturn<ToLoaderOutput<A, P>, HookReturnMode>;

	usePatternLoaderData: <P extends ToViewPattern<A>>(
		pattern: P,
	) => HookReturn<ToLoaderOutput<A, P> | undefined, HookReturnMode>;

	useClientLoaderData: <P extends ToViewPattern<A>, T>(
		args: ToRouteComponentProps<A, P, T>,
	) => HookReturn<T, HookReturnMode>;

	usePatternClientLoaderData: <T>(
		pattern: ToViewPattern<A>,
	) => HookReturn<T | undefined, HookReturnMode>;
};
