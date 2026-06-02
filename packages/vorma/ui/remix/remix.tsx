/// <reference types="vite/client" />

import { createElement, on, type Handle, type Props, type RemixNode } from "remix/ui";
import {
	apply_scroll,
	create_adapter_base,
	get_entry_key,
	make_entry_id,
	make_link_props,
	resolve_outlet_slot,
	select_link_route_state,
	select_link_work_state,
	type AdapterClientOptions,
	type AppConfig,
	type DecomposedCommit,
	type DecomposedState,
	type LinkPropsResult,
	type RouteState,
	type ScrollIntent,
	type ToApiDecorator,
	type ToDefineViewArgs,
	type ToLinkProps,
	type ToRouteSyncArgs,
	type ToViewComponentProps,
	type ToViewOutput,
	type ToViewPattern,
	type ViewDefinition,
	type VormaClient,
	type WorkState,
} from "vorma/__internal";
import { jsonDeepEquals } from "vorma/kit/json";
import type { LinkPropsBase } from "../../core/types.ts";

export { MutationError, QueryError } from "vorma/__internal";
export type {
	BeforeRouteCommitFn,
	BeforeRouteTransitionArgs,
	BeforeRouteYieldFn,
	BuildSkewDetectedEvent,
	MutationResult,
	QueryResult,
	RevalidationReason,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
	ToApiClient,
	ToApiDecorator,
	ToApiDecoratorContext,
	ToClientLoaderArgs,
	ToLinkProps,
	ToMutationArgs,
	ToMutationError,
	ToMutationInput,
	ToMutationMethod,
	ToMutationOutput,
	ToMutationPattern,
	ToNavigateArgs,
	ToNavigationTarget,
	ToQueryArgs,
	ToQueryError,
	ToQueryInput,
	ToQueryMethod,
	ToQueryOutput,
	ToQueryPattern,
	ToRouteDestination,
	ToRouteSyncArgs,
	ToViewComponentProps,
	ToViewInput,
	ToViewOutput,
	ToViewPattern,
	AppConfig as VormaClientSeed,
	WorkIndicator,
	WorkIndicatorOptions,
	WorkState,
} from "vorma/__internal";

const CLICK_EVENT = "click";
const POINTER_DOWN_EVENT = "pointerdown";
const POINTER_ENTER_EVENT = "pointerenter";
const FOCUS_EVENT = "focus";
const POINTER_LEAVE_EVENT = "pointerleave";
const BLUR_EVENT = "blur";
const TOUCH_CANCEL_EVENT = "touchcancel";
const route_state_subscription_key = "route-state";
const work_state_subscription_key = "work-state";
const route_sync_route_subscription_key = "route-sync-route";
const route_sync_work_subscription_key = "route-sync-work";

type RootOutletProps = { idx?: number } & Record<string, unknown>;

export type RemixComponent<P extends object = Record<string, unknown>> = (
	handle: Handle<P>,
) => (props: P) => RemixNode;

type RemixHybridComponent<P extends object> = RemixComponent<P> &
	((props?: P) => RemixNode);

type RemixAnchorProps = Props<"a">;

type RemixLinkProps<A extends AppConfig, P extends ToViewPattern<A>> = Omit<
	RemixAnchorProps,
	"href"
> &
	ToLinkProps<A, P>;

type RemixLink<A extends AppConfig> = {
	<P extends ToViewPattern<A>>(props: RemixLinkProps<A, P>): RemixNode;
	(
		handle: Handle<RemixLinkProps<A, ToViewPattern<A>>>,
	): <P extends ToViewPattern<A>>(props: RemixLinkProps<A, P>) => RemixNode;
};

type RemixDefineViewArgs<A extends AppConfig, P extends ToViewPattern<A>, T> = Omit<
	ToDefineViewArgs<A, P, T, RemixNode>,
	"component" | "errorBoundary"
> & {
	component: RemixViewComponent<A, P, T>;
	errorBoundary?: RemixErrorBoundaryComponent<A>;
};

type RemixStateSelector<TState, TSelected> = (state: TState) => TSelected;

export type RemixViewScope<A extends AppConfig> = {
	routeState: {
		(): RouteState;
		<T>(selector: (state: RouteState) => T): T;
	};
	workState: {
		(): WorkState;
		<T>(selector: (state: WorkState) => T): T;
	};
	routeSync: <P extends ToViewPattern<A>>(args: ToRouteSyncArgs<A, P>) => void;
	viewData: <P extends ToViewPattern<A>, T = any>(
		props: ToViewComponentProps<A, P, T>,
	) => ToViewOutput<A, P>;
	patternViewData: <P extends ToViewPattern<A>>(
		pattern: P,
	) => ToViewOutput<A, P> | undefined;
	clientLoaderData: <P extends ToViewPattern<A>, T>(
		props: ToViewComponentProps<A, P, T>,
	) => T;
	patternClientLoaderData: <T>(pattern: ToViewPattern<A>) => T | undefined;
};

export type RemixViewComponent<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	T = any,
> = (
	handle: Handle<ToViewComponentProps<A, P, T>>,
	v: RemixViewScope<A>,
) => (props: ToViewComponentProps<A, P, T>) => RemixNode;

type RemixErrorBoundaryComponent<A extends AppConfig> = (
	handle: Handle<{ error: unknown }>,
	v: RemixViewScope<A>,
) => (props: { error: unknown }) => RemixNode;

export type RemixVormaClient<A extends AppConfig> = Omit<
	VormaClient<A, RemixNode, RemixAnchorProps, "value">,
	| "RootOutlet"
	| "Link"
	| "defineView"
	| "useRouteState"
	| "useRouteSync"
	| "useWorkState"
	| "useViewData"
	| "usePatternViewData"
	| "useClientLoaderData"
	| "usePatternClientLoaderData"
> & {
	RootOutlet: RemixHybridComponent<RootOutletProps>;
	Link: RemixLink<A>;
	defineView: <P extends ToViewPattern<A>, T = any>(
		input: RemixDefineViewArgs<A, P, T>,
	) => ViewDefinition;
};

type StateSubscription<TState, TSelected> = {
	selected: TSelected;
	selector: RemixStateSelector<TState, TSelected>;
};

type StateSubscriptions<TState> = Map<string, StateSubscription<TState, unknown>>;

type StateSubscriptionMap<TState> = Map<Handle<any>, StateSubscriptions<TState>>;

type RouteSyncState = {
	pending_href?: string;
	timeout_id?: number;
};

type CreateVormaClientOptions<A extends AppConfig> = AdapterClientOptions<
	RemixHybridComponent<RootOutletProps>
> & {
	linkDefaultProps?: Partial<Omit<RemixAnchorProps & LinkPropsBase, "href">>;
	apiDecorator?: ToApiDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): RemixVormaClient<A> {
	let store: DecomposedState = {
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
	let route_store: RouteState | null = null;
	let work_store: WorkState = {
		navigation: null,
		revalidation: null,
		prefetch: null,
		apiRequests: [],
	};
	const route_render_handles = new Set<Handle<any>>();
	const link_handles = new Set<Handle<any>>();
	const route_state_subscriptions: StateSubscriptionMap<RouteState> = new Map();
	const work_state_subscriptions: StateSubscriptionMap<WorkState> = new Map();
	const route_sync_states = new Map<Handle<any>, RouteSyncState>();

	let pending_scroll_intent: ScrollIntent | undefined;

	const adapter_base_res = create_adapter_base(
		app_config,
		(adapter_commit: DecomposedCommit) => {
			const previous_link_work_state = select_link_work_state(work_store);
			let should_notify_route_render = false;
			let should_notify_route = false;
			let should_notify_work = false;
			let should_notify_links = false;
			if (adapter_commit.scroll_intent) {
				pending_scroll_intent = adapter_commit.scroll_intent;
			}
			if (adapter_commit.state) {
				store = adapter_commit.state;
				should_notify_route_render = true;
			}
			if (adapter_commit.route) {
				route_store = adapter_commit.route;
				should_notify_route = true;
				should_notify_links = true;
			}
			if (adapter_commit.link_state_version !== undefined) {
				should_notify_links = true;
			}
			if (adapter_commit.work) {
				work_store = adapter_commit.work;
				should_notify_work = true;
			}
			const next_link_work_state = select_link_work_state(work_store);
			if (!jsonDeepEquals(previous_link_work_state, next_link_work_state)) {
				should_notify_links = true;
			}
			if (should_notify_route_render) {
				notify_handle_set(route_render_handles);
			}
			if (should_notify_route && route_store) {
				notify_state_subscriptions(route_state_subscriptions, route_store);
			}
			if (should_notify_work) {
				notify_state_subscriptions(work_state_subscriptions, work_store);
			}
			if (should_notify_links) {
				notify_handle_set(link_handles);
			}
		},
		options?.apiDecorator,
	);
	if (!adapter_base_res.ok) {
		throw new Error(`Failed to create Vorma client: ${adapter_base_res.err}`);
	}

	const { core, nav_fns, passthrough } = adapter_base_res.val;

	function default_selector<TState>(state: TState): TState {
		return state;
	}

	function track_handle_set(handles: Set<Handle<any>>, handle: Handle<any>): void {
		if (handles.has(handle)) {
			return;
		}
		handles.add(handle);
		handle.signal.addEventListener(
			"abort",
			() => {
				handles.delete(handle);
			},
			{ once: true },
		);
	}

	function track_handle_map<TValue>(
		handles: Map<Handle<any>, TValue>,
		handle: Handle<any>,
	): void {
		if (handles.has(handle)) {
			return;
		}
		handle.signal.addEventListener(
			"abort",
			() => {
				handles.delete(handle);
			},
			{ once: true },
		);
	}

	function notify_handle_set(handles: Set<Handle<any>>): void {
		handles.forEach((handle) => {
			void handle.update();
		});
	}

	function track_state_subscription<TState, TSelected>(
		subscriptions: StateSubscriptionMap<TState>,
		subscription_key: string,
		handle: Handle<any>,
		state: TState,
		selector?: RemixStateSelector<TState, TSelected>,
	): TSelected {
		track_handle_map(subscriptions, handle);
		const state_selector =
			selector ?? (default_selector as RemixStateSelector<TState, TSelected>);
		const selected = state_selector(state);
		let handle_subscriptions = subscriptions.get(handle);
		if (!handle_subscriptions) {
			handle_subscriptions = new Map();
			subscriptions.set(handle, handle_subscriptions);
		}
		handle_subscriptions.set(subscription_key, {
			selected,
			selector: state_selector as RemixStateSelector<TState, unknown>,
		});
		return selected;
	}

	function notify_state_subscriptions<TState>(
		subscriptions: StateSubscriptionMap<TState>,
		state: TState,
	): void {
		subscriptions.forEach((handle_subscriptions, handle) => {
			let should_update = false;
			handle_subscriptions.forEach((subscription) => {
				const selected = subscription.selector(state);
				if (jsonDeepEquals(subscription.selected, selected)) {
					return;
				}
				subscription.selected = selected;
				should_update = true;
			});
			if (should_update) {
				void handle.update();
			}
		});
	}

	function clear_route_sync_state(state: RouteSyncState): void {
		if (state.timeout_id !== undefined) {
			window.clearTimeout(state.timeout_id);
		}
		state.timeout_id = undefined;
		state.pending_href = undefined;
	}

	function get_route_sync_state(handle: Handle<any>): RouteSyncState {
		const existing = route_sync_states.get(handle);
		if (existing) {
			return existing;
		}

		const state: RouteSyncState = {};
		route_sync_states.set(handle, state);
		handle.signal.addEventListener(
			"abort",
			() => {
				clear_route_sync_state(state);
				route_sync_states.delete(handle);
			},
			{ once: true },
		);
		return state;
	}

	function is_remix_handle<P extends object>(
		value: Handle<P> | P | undefined,
	): value is Handle<P> {
		return (
			!!value &&
			typeof value === "object" &&
			"props" in value &&
			"signal" in value &&
			"update" in value
		);
	}

	function make_hybrid_component<P extends object>(
		component: RemixComponent<P>,
	): RemixHybridComponent<P> {
		return ((input?: Handle<P> | P) => {
			if (is_remix_handle(input)) {
				return component(input);
			}
			return create_remix_element(component, (input ?? {}) as P);
		}) as RemixHybridComponent<P>;
	}

	function create_remix_element<P extends object>(
		type: RemixComponent<P>,
		props: P,
	): RemixNode {
		const next_props = { ...props } as P & { children?: RemixNode };
		const children = next_props.children;
		delete next_props.children;
		if (children === undefined) {
			return createElement(type, next_props);
		}
		const child_nodes = Array.isArray(children) ? children : [children];
		return createElement(type, next_props, ...child_nodes);
	}

	function get_route_snapshot(): RouteState {
		if (!route_store) {
			throw new Error("Vorma not booted");
		}
		return route_store;
	}

	function get_work_snapshot(): WorkState {
		return work_store;
	}

	function route_state(handle: Handle<any>): RouteState;
	function route_state<T>(handle: Handle<any>, selector: (route: RouteState) => T): T;
	function route_state<T>(
		handle: Handle<any>,
		selector?: (route: RouteState) => T,
	): RouteState | T {
		const route = get_route_snapshot();
		return track_state_subscription(
			route_state_subscriptions,
			route_state_subscription_key,
			handle,
			route,
			selector,
		);
	}

	function work_state(handle: Handle<any>): WorkState;
	function work_state<T>(handle: Handle<any>, selector: (work: WorkState) => T): T;
	function work_state<T>(
		handle: Handle<any>,
		selector?: (work: WorkState) => T,
	): WorkState | T {
		const work = get_work_snapshot();
		return track_state_subscription(
			work_state_subscriptions,
			work_state_subscription_key,
			handle,
			work,
			selector,
		);
	}

	function route_sync<P extends ToViewPattern<A>>(
		handle: Handle<any>,
		args: ToRouteSyncArgs<A, P>,
	): void;
	function route_sync<P extends ToViewPattern<A>>(
		handle: Handle<any>,
		args: ToRouteSyncArgs<A, P>,
	): void {
		const route_sync_state = get_route_sync_state(handle);

		track_state_subscription(
			route_state_subscriptions,
			route_sync_route_subscription_key,
			handle,
			get_route_snapshot(),
			(route) => {
				return route.href;
			},
		);
		track_state_subscription(
			work_state_subscriptions,
			route_sync_work_subscription_key,
			handle,
			work_store,
			(work) => {
				return work.navigation?.href ?? null;
			},
		);

		const {
			debounceMs = 0,
			enabled = true,
			replace = true,
			scrollToTop = false,
			...target
		} = args;
		if (!enabled) {
			clear_route_sync_state(route_sync_state);
			return;
		}

		const canonical_href = passthrough.toHref(target as any);
		const route_href = route_store?.href;
		const pending_href = work_store.navigation?.href;
		if (canonical_href === route_href || canonical_href === pending_href) {
			clear_route_sync_state(route_sync_state);
			return;
		}
		if (canonical_href === route_sync_state.pending_href) {
			return;
		}

		clear_route_sync_state(route_sync_state);
		route_sync_state.pending_href = canonical_href;
		route_sync_state.timeout_id = window.setTimeout(() => {
			route_sync_state.timeout_id = undefined;
			route_sync_state.pending_href = undefined;
			if (
				canonical_href === route_store?.href ||
				canonical_href === work_store.navigation?.href
			) {
				return;
			}
			void passthrough.navigate({
				href: canonical_href,
				replace,
				scrollToTop,
			});
		}, debounceMs);
	}

	function view_data<P extends ToViewPattern<A>, T = any>(
		args: ToViewComponentProps<A, P, T>,
	): ToViewOutput<A, P> {
		return store.views_data[args.idx] as ToViewOutput<A, P>;
	}

	function pattern_view_data<P extends ToViewPattern<A>>(
		pattern: P,
	): ToViewOutput<A, P> | undefined {
		const idx = store.matched_patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return store.views_data[idx] as ToViewOutput<A, P>;
	}

	function client_loader_data<P extends ToViewPattern<A>, T>(
		args: ToViewComponentProps<A, P, T>,
	): T {
		return store.client_loaders_data[args.idx] as T;
	}

	function pattern_client_loader_data<T>(pattern: ToViewPattern<A>): T | undefined {
		const idx = store.matched_patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return store.client_loaders_data[idx] as T;
	}

	function create_view_scope(handle: Handle<any>): RemixViewScope<A> {
		const route_state_fn = (<T,>(
			selector?: RemixStateSelector<RouteState, T>,
		): RouteState | T => {
			if (selector) {
				return route_state(handle, selector);
			}
			return route_state(handle);
		}) as RemixViewScope<A>["routeState"];
		const work_state_fn = (<T,>(
			selector?: RemixStateSelector<WorkState, T>,
		): WorkState | T => {
			if (selector) {
				return work_state(handle, selector);
			}
			return work_state(handle);
		}) as RemixViewScope<A>["workState"];

		return {
			routeState: route_state_fn,
			workState: work_state_fn,
			routeSync: <P extends ToViewPattern<A>>(
				args: ToRouteSyncArgs<A, P>,
			): void => {
				return route_sync(handle, args);
			},
			viewData: <P extends ToViewPattern<A>, T = any>(
				props: ToViewComponentProps<A, P, T>,
			): ToViewOutput<A, P> => {
				return view_data(props);
			},
			patternViewData: <P extends ToViewPattern<A>>(
				pattern: P,
			): ToViewOutput<A, P> | undefined => {
				return pattern_view_data(pattern);
			},
			clientLoaderData: <P extends ToViewPattern<A>, T>(
				props: ToViewComponentProps<A, P, T>,
			): T => {
				return client_loader_data(props);
			},
			patternClientLoaderData: <T,>(pattern: ToViewPattern<A>): T | undefined => {
				return pattern_client_loader_data(pattern);
			},
		};
	}

	function defineView<P extends ToViewPattern<A>, T = any>(
		input: RemixDefineViewArgs<A, P, T>,
	): ViewDefinition {
		const { component, errorBoundary, ...core_input } = input;
		const remix_component: RemixComponent<ToViewComponentProps<A, P, T>> = (
			handle,
		) => {
			return component(handle, create_view_scope(handle));
		};
		const remix_error_boundary:
			| RemixComponent<{
					error: unknown;
			  }>
			| undefined = errorBoundary
			? (handle) => {
					return errorBoundary(handle, create_view_scope(handle));
				}
			: undefined;
		return core.defineView({
			...core_input,
			component: (props: ToViewComponentProps<A, P, T>) => {
				return create_remix_element(remix_component, props);
			},
			errorBoundary: remix_error_boundary
				? (props: { error: unknown }) => {
						return create_remix_element(remix_error_boundary, props);
					}
				: undefined,
		});
	}

	function root_outlet_component(
		handle: Handle<RootOutletProps>,
	): (props: RootOutletProps) => RemixNode {
		track_handle_set(route_render_handles, handle);
		return (props: RootOutletProps) => {
			return render_root_outlet(handle, props);
		};
	}

	function render_root_outlet(
		handle: Handle<RootOutletProps>,
		props: RootOutletProps,
	): RemixNode {
		const idx = props.idx ?? 0;

		if (pending_scroll_intent) {
			const entry = store.entries[idx];
			if (
				entry &&
				make_entry_id(idx, entry.pattern) ===
					pending_scroll_intent.target_entry_id
			) {
				const scroll = pending_scroll_intent.scroll;
				pending_scroll_intent = undefined;
				handle.queueTask((signal: AbortSignal) => {
					if (signal.aborted) {
						return;
					}
					apply_scroll(scroll);
				});
			}
		}

		const Outlet = make_outlet(props, idx);
		const slot = resolve_outlet_slot(
			store.entries,
			store.error,
			idx,
			core.get_default_error_boundary(),
		);

		switch (slot.kind) {
			case "empty":
				return null;
			case "pass_through": {
				const key =
					idx + 1 < store.entries.length
						? get_entry_key(store.entries[idx + 1]!)
						: `__empty_${idx + 1}`;
				return create_remix_element(Outlet, { key });
			}
			case "error": {
				const key = `__error_${idx}`;
				const boundary = slot.boundary;
				return boundary({ key, error: slot.error } as any);
			}
			case "component": {
				const key = get_entry_key(store.entries[idx]!);
				const component = slot.component;
				return component({ key, idx, Outlet });
			}
		}
	}

	const RootOutlet = make_hybrid_component(root_outlet_component);
	const RootOutletApp = RootOutlet;

	function make_outlet(
		parent_props: RootOutletProps,
		idx: number,
	): RemixHybridComponent<Record<string, unknown>> {
		return make_hybrid_component((handle) => {
			track_handle_set(route_render_handles, handle);
			return (local?: Record<string, unknown>) => {
				const local_props = local ?? {};
				return create_remix_element(RootOutlet, {
					...parent_props,
					...local_props,
					idx: idx + 1,
				});
			};
		});
	}

	function boot(): ReturnType<typeof core.boot> {
		const {
			render: render_root,
			linkDefaultProps: _link_default_props,
			apiDecorator: _api_decorator,
			...core_options
		} = options ?? {};
		return core.boot({
			...core_options,
			render: render_root
				? () => {
						return render_root({
							RootOutlet: RootOutletApp,
							rootEl: core.getRootEl(),
						});
					}
				: undefined,
		});
	}

	function with_link_mixins(
		anchor_props: RemixAnchorProps,
		result: LinkPropsResult,
	): RemixAnchorProps {
		const mixins: unknown[] = [];
		if (anchor_props.mix !== undefined) {
			mixins.push(anchor_props.mix);
		}
		if (result.onClick) {
			mixins.push(
				on<HTMLAnchorElement, typeof CLICK_EVENT>(CLICK_EVENT, (event) => {
					return result.onClick?.(event);
				}),
			);
		}
		if (result.onPointerDown) {
			mixins.push(
				on<HTMLAnchorElement, typeof POINTER_DOWN_EVENT>(
					POINTER_DOWN_EVENT,
					(event) => {
						return result.onPointerDown?.(event);
					},
				),
			);
		}
		if (result.onPointerEnter) {
			mixins.push(
				on<HTMLAnchorElement, typeof POINTER_ENTER_EVENT>(
					POINTER_ENTER_EVENT,
					(event) => {
						return result.onPointerEnter?.(event);
					},
				),
			);
		}
		if (result.onFocus) {
			mixins.push(
				on<HTMLAnchorElement, typeof FOCUS_EVENT>(FOCUS_EVENT, (event) => {
					return result.onFocus?.(event);
				}),
			);
		}
		if (result.onPointerLeave) {
			mixins.push(
				on<HTMLAnchorElement, typeof POINTER_LEAVE_EVENT>(
					POINTER_LEAVE_EVENT,
					(event) => {
						return result.onPointerLeave?.(event);
					},
				),
			);
		}
		if (result.onBlur) {
			mixins.push(
				on<HTMLAnchorElement, typeof BLUR_EVENT>(BLUR_EVENT, (event) => {
					return result.onBlur?.(event);
				}),
			);
		}
		if (result.onTouchCancel) {
			mixins.push(
				on<HTMLAnchorElement, typeof TOUCH_CANCEL_EVENT>(
					TOUCH_CANCEL_EVENT,
					(event) => {
						return result.onTouchCancel?.(event);
					},
				),
			);
		}

		if (mixins.length === 0) {
			return anchor_props;
		}
		return {
			...anchor_props,
			mix: mixins as RemixAnchorProps["mix"],
		};
	}

	type BaseLinkProps = RemixAnchorProps & LinkPropsBase & { pattern?: string };

	const BaseLink = make_hybrid_component<BaseLinkProps>((handle) => {
		track_handle_set(link_handles, handle);
		return (props: BaseLinkProps) => {
			const route_state = route_store ? select_link_route_state(route_store) : null;
			const work_state = select_link_work_state(work_store);
			const result = make_link_props(
				props as Record<string, unknown>,
				nav_fns,
				route_state,
				work_state,
			);
			const anchor_props = with_link_mixins(
				result.anchor_props as RemixAnchorProps,
				result,
			);
			return (
				<a data-external={result.is_external || undefined} {...anchor_props}>
					{props.children}
				</a>
			);
		};
	});

	function render_link<P extends ToViewPattern<A>>(
		raw: RemixLinkProps<A, P>,
	): RemixNode {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const { href, pattern, params, splatValues, search, hash, ...props } = merged;
		return (
			<BaseLink
				{...props}
				pattern={pattern}
				href={
					href ??
					passthrough.toHref({
						pattern,
						params,
						splatValues,
						search,
						hash,
					} as any)
				}
			/>
		);
	}

	const Link = ((input: unknown) => {
		if (is_remix_handle(input as any)) {
			return <P extends ToViewPattern<A>>(props: RemixLinkProps<A, P>) => {
				return render_link(props);
			};
		}
		return render_link(input as RemixLinkProps<A, ToViewPattern<A>>);
	}) as RemixLink<A>;

	return {
		...passthrough,
		boot,
		defineView,
		RootOutlet,
		Link,
	};
}
