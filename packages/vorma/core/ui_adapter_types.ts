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

// Internal: how a candidate `Link` href/route resolves for active/pending
// attribute comparisons — see `make_link_props.ts`'s exact/ancestor match
// logic. Never part of the public surface.
export type ClientMatcherFactory = () => Promise<ClientMatcher>;

export type LinkAttributeCandidate = {
	url: URL;
	matched_patterns: string[];
};

// Internal: `create_adapter_base`'s per-channel decomposition of route
// render state into individually-stable arrays, so a framework's reactive
// primitive (React's `useSyncExternalStore`, Solid signals) only
// re-renders the channels that actually changed on a given commit — e.g.
// `views_data` stays reference-equal across a commit that only changed
// scroll intent. `entries` is deliberately the one channel that is always a
// fresh reference (per-entry identity is the adapter's own concern via
// `entry_keys`).
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

/** Internal: one commit `create_adapter_base` publishes to a UI adapter — every field independently optional, same shape as {@link ClientCommit} one layer up but decomposed for adapter consumption. */
export type DecomposedCommit = {
	link_state_version?: number;
	route?: RouteState;
	route_reason?: RouteUpdateReason;
	scroll_intent?: ScrollIntent;
	state?: DecomposedState;
	work?: WorkState;
};

/** Internal: the callback signature `create_adapter_base` drives with every {@link DecomposedCommit}. */
export type DecomposedCommitFn = (commit: DecomposedCommit) => void;

/** Args an adapter's `render` callback receives — see {@link AdapterClientOptions}. */
export type AdapterRenderArgs<RootOutletComponent> = {
	RootOutlet: RootOutletComponent;
	rootEl: HTMLElement;
};

/**
 * `ClientOptions` with `render` replaced by a framework-typed variant that
 * receives {@link AdapterRenderArgs} (the mounted `RootOutlet` component and
 * the DOM element Vorma reserves for it) instead of the bare no-args form —
 * every official adapter's `createVormaClient(config, options)` accepts
 * this shape. Every other `ClientOptions` field (`workIndicator`,
 * `revalidateOnWindowFocus`, `useViewTransitions`, etc.) is unchanged; see
 * `ClientOptions` for those.
 */
export type AdapterClientOptions<RootOutletComponent> = Omit<
	CoreClientOptions,
	"render"
> & {
	render?: (args: AdapterRenderArgs<RootOutletComponent>) => void | Promise<void>;
};

// Internal: shared machinery `create_adapter_base` returns, consumed by
// each framework adapter's `createVormaClient` to build the public
// `VormaClient` surface. `core` is the raw client-core handle; `nav_fns`
// backs `Link`'s active/pending/prefetch wiring; `resolve_outlet_slot`
// decides what `RootOutlet` renders at a given depth (a component, an
// error boundary, a pass-through to the next outlet, or nothing).
// `passthrough` is the typed subset every adapter re-exports verbatim on
// its own `VormaClient` — see that type for the public docs on each member.
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

/**
 * How each adapter's stateful hooks (`useRouteState`, `useWorkState`,
 * `useViewData`, etc.) return their value:
 *
 * - `"value"` (React, Preact): the current value directly, re-rendering the
 *   calling component on change — the ordinary hook shape.
 * - `"accessor"` (Solid): a zero-arg function to CALL for the current
 *   value, matching Solid's own signal-read convention (`useRouteState()()`
 *   reads the route; the hook itself only subscribes).
 *
 * A hand-written adapter on `vorma/__internal` could in principle target a
 * `"signal"` mode (a `ReadonlySignal`, e.g. for `@preact/signals`), though
 * none of the three official adapters use it today.
 */
type HookReturn<T, Mode extends "value" | "accessor" | "signal"> = Mode extends "accessor"
	? () => T
	: Mode extends "signal"
		? ReadonlySignal<T>
		: T;

type StateSelector<State, Selected> = (state: State) => Selected;

/**
 * The object `createVormaClient(appConfig, options)` returns — the entire
 * framework-facing surface for building a Vorma app's client. Each official
 * adapter (`vorma/react`, `vorma/preact`, `vorma/solid`) instantiates this
 * with its own `Element`/`AnchorProps`/`HookReturnMode`; the docs below are
 * shared across all three (see {@link HookReturn} for exactly how the hook
 * return shape differs by adapter, and each adapter's own module docs for
 * any other genuine divergence).
 *
 * **Lifecycle**
 * - `boot()`: starts the client — reads the server-embedded initial route
 *   payload, prepares the first route, and (if `options.render` was
 *   supplied) mounts `RootOutlet`. Call once, after registering every view
 *   module the app needs (each adapter's entry point wires this at import
 *   time in the ordinary case).
 * - `RootOutlet`: renders the matched view chain at the current nesting
 *   depth — the framework-native root component `options.render` receives
 *   as `RootOutlet` (see {@link AdapterRenderArgs}). A view's own
 *   `props.Outlet` (see {@link ToViewComponentProps}) is how a parent view
 *   renders its matched child; an app never needs to reference `RootOutlet`
 *   directly beyond the initial mount.
 * - `defineView(args)`: declares one view module's default export — see
 *   {@link ToDefineViewArgs} for `component`/`errorBoundary`/`clientLoader`/
 *   the `beforeRouteCommit`/`beforeRouteYield` transition hooks.
 *
 * **Navigation**
 * - `navigate(args)`: typed programmatic navigation — see
 *   {@link ToNavigateArgs}.
 * - `Link`: the anchor component — Vorma's own props
 *   ({@link ToLinkProps}) layer onto the framework's native anchor props
 *   (verbatim `AnchorProps`, minus `href`, which the typed target derives).
 * - `prefetch`/`cancelPrefetch`: imperative prefetch outside of `Link`'s
 *   own `prefetch="intent"` hover/focus behavior (e.g. prefetching on a
 *   custom gesture); `toHref` resolves a typed destination to a plain
 *   string URL, useful for building `href`s outside of `Link` itself
 *   (sharing/canonical URLs, `<a>` tags Vorma does not own).
 * - `useRouteSync(args)`: see {@link ToRouteSyncArgs} — keeps a typed route
 *   destination in sync with local component state (search boxes, filters)
 *   without hand-wiring the navigation.
 *
 * **Reading state**
 * - `useRouteState`/`useWorkState`: subscribe to the current
 *   {@link RouteState}/{@link WorkState} (or a selected slice via the
 *   overload taking a selector — prefer the selector form when only part
 *   of the state matters, so the component only re-renders on changes to
 *   that slice); `getRouteState`/`getWorkState` are their imperative,
 *   non-reactive twins (a one-off read outside render — an event handler,
 *   an analytics call — not a subscription).
 * - `useViewData(props)`/`usePatternViewData(pattern)`: a view's own typed
 *   output data — the `idx`-keyed form (via
 *   {@link ToViewComponentProps}) is what a view component itself uses;
 *   the pattern-keyed form is for reading a DIFFERENT matched view's data
 *   (e.g. a layout reading a child tab's data for a shared title bar) and
 *   returns `undefined` when that pattern is not currently matched.
 * - `useClientLoaderData(props)`/`usePatternClientLoaderData(pattern)`: the
 *   client-loader twins of the above — see {@link ToClientLoaderArgs}.
 *
 * **Everything else** (`revalidate`, `getRouteState`, `getWorkState`,
 * `workIndicator`, `apiClient`) is documented on its own type — see
 * {@link RevalidationResult}, {@link RouteState}, {@link WorkState},
 * {@link WorkIndicator}, {@link ToApiClient}.
 */
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
