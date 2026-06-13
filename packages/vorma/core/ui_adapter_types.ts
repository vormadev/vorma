import type { ReadonlySignal } from "@preact/signals";
import type { Result } from "vorma/kit/result";
import type { ClientMatcher } from "./client_wasm/matcher.ts";
import type {
	ClientCore,
	ClientOptions as CoreClientOptions,
} from "./create_client_core";
import type {
	RouteRenderEntry,
	ScrollIntent,
	ViewDefinition,
} from "./create_client_core.ts";
import type { LinkNavFns } from "./link_types.ts";
import type { OutletSlotResolver } from "./resolve_outlet_slot.ts";
import type {
	AppConfig,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
	ToApiClient,
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
import type { WorkState } from "./work_state.ts";

export type ClientMatcherFactory = () => Promise<ClientMatcher>;

export type LinkAttributeCandidate = {
	url: URL;
	matched_patterns: string[];
};

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

export type AdapterBase<A extends AppConfig> = {
	core: ClientCore;

	nav_fns: LinkNavFns;

	resolve_outlet_slot: OutletSlotResolver;

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
