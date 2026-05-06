/// <reference types="vite/client" />

import {
	createElement,
	on,
	type Handle,
	type Props,
	type RemixNode,
} from "remix/ui";
import {
	apply_scroll,
	create_adapter_base,
	get_entry_key,
	make_link_props,
	make_route_id,
	resolve_outlet_slot,
	select_link_route_state,
	select_link_work_state,
	type AdapterClientOptions,
	type AppConfig,
	type DecomposedState,
	type LinkPropsResult,
	type RouteState,
	type ScrollIntent,
	type ToAPIDecorator,
	type ToDefineViewArgs,
	type ToLinkProps,
	type ToLoaderOutput,
	type ToRouteComponentProps,
	type ToRouteSyncArgs,
	type ToViewPattern,
	type ViewDefinition,
	type VormaClient,
	type WorkState,
} from "vorma/__internal";
import type { LinkPropsBase } from "../../core/types.ts";

export { MutationError, QueryError } from "vorma/__internal";
export type {
	BeforeRouteCommitFn,
	BeforeRouteTransitionArgs,
	BeforeRouteYieldFn,
	BuildSkewDetectedEvent,
	MutationResult,
	ProgressIndicatorConfig,
	QueryResult,
	RevalidationReason,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
	ToAPIClient,
	ToAPIDecorator,
	ToAPIDecoratorContext,
	ToClientLoaderArgs,
	ToLinkProps,
	ToLoaderInput,
	ToLoaderOutput,
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
	ToRouteComponentProps,
	ToRouteDestination,
	ToRouteSyncArgs,
	ToViewPattern,
	AppConfig as VormaClientSeed,
	WorkState,
} from "vorma/__internal";

const CLICK_EVENT = "click";
const POINTER_DOWN_EVENT = "pointerdown";
const POINTER_ENTER_EVENT = "pointerenter";
const FOCUS_EVENT = "focus";
const POINTER_LEAVE_EVENT = "pointerleave";
const BLUR_EVENT = "blur";
const TOUCH_CANCEL_EVENT = "touchcancel";

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

type RemixDefineViewArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	T,
> = Omit<
	ToDefineViewArgs<A, P, T, RemixNode>,
	"component" | "errorBoundary"
> & {
	component: RemixComponent<ToRouteComponentProps<A, P, T>>;
	errorBoundary?: RemixComponent<{ error: unknown }>;
};

export type RemixVormaClient<A extends AppConfig> = Omit<
	VormaClient<A, RemixNode, RemixAnchorProps, "value">,
	"RootOutlet" | "Link" | "defineView"
> & {
	RootOutlet: RemixHybridComponent<RootOutletProps>;
	Link: RemixLink<A>;
	defineView: <P extends ToViewPattern<A>, T = any>(
		input: RemixDefineViewArgs<A, P, T>,
	) => ViewDefinition;
};

type CreateVormaClientOptions<A extends AppConfig> = AdapterClientOptions<
	RemixHybridComponent<RootOutletProps>
> & {
	linkDefaultProps?: Partial<Omit<RemixAnchorProps & LinkPropsBase, "href">>;
	apiDecorator?: ToAPIDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): RemixVormaClient<A> {
	let store: DecomposedState = {
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
	let route_store: RouteState | null = null;
	let work_store: WorkState = {
		navigation: null,
		revalidation: null,
		prefetch: null,
		apiRequests: [],
	};
	const handles = new Set<Handle<any>>();

	let pending_scroll_intent: ScrollIntent | undefined;
	let route_sync_timeout_id: number | undefined;
	let route_sync_pending_href: string | undefined;

	const adapter_base_res = create_adapter_base(
		app_config,
		(decomposed: DecomposedState, scroll_intent?: ScrollIntent) => {
			if (scroll_intent) {
				pending_scroll_intent = scroll_intent;
			}
			store = decomposed;
			notify_handles();
		},
		options?.apiDecorator,
	);
	if (!adapter_base_res.ok) {
		throw new Error(
			`Failed to create Vorma client: ${adapter_base_res.err}`,
		);
	}

	const { core, nav_fns, passthrough } = adapter_base_res.val;

	function track_handle(handle: Handle<any>): void {
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

	function notify_handles(): void {
		handles.forEach((handle) => {
			void handle.update();
		});
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

	function useRouteState(): RouteState;
	function useRouteState<T>(selector: (route: RouteState) => T): T;
	function useRouteState<T>(
		selector?: (route: RouteState) => T,
	): RouteState | T {
		const route = get_route_snapshot();
		if (selector) {
			return selector(route);
		}
		return route;
	}

	function useWorkState(): WorkState;
	function useWorkState<T>(selector: (work: WorkState) => T): T;
	function useWorkState<T>(selector?: (work: WorkState) => T): WorkState | T {
		const work = get_work_snapshot();
		if (selector) {
			return selector(work);
		}
		return work;
	}

	function clear_route_sync_timeout(): void {
		if (route_sync_timeout_id !== undefined) {
			window.clearTimeout(route_sync_timeout_id);
		}
		route_sync_timeout_id = undefined;
		route_sync_pending_href = undefined;
	}

	function useRouteSync<P extends ToViewPattern<A>>(
		args: ToRouteSyncArgs<A, P>,
	): void {
		const {
			debounceMs = 0,
			enabled = true,
			replace = true,
			scrollToTop = false,
			...target
		} = args;
		if (!enabled) {
			clear_route_sync_timeout();
			return;
		}

		const canonical_href = passthrough.toHref(target as any);
		const route_href = route_store?.href;
		const pending_href = work_store.navigation?.href;
		if (
			canonical_href === route_href ||
			canonical_href === pending_href ||
			canonical_href === route_sync_pending_href
		) {
			return;
		}

		clear_route_sync_timeout();
		route_sync_pending_href = canonical_href;
		route_sync_timeout_id = window.setTimeout(() => {
			route_sync_timeout_id = undefined;
			route_sync_pending_href = undefined;
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

	function useLoaderData<P extends ToViewPattern<A>>(
		args: ToRouteComponentProps<A, P>,
	): ToLoaderOutput<A, P> {
		return store.loaders_data[args.idx] as ToLoaderOutput<A, P>;
	}

	function usePatternLoaderData<P extends ToViewPattern<A>>(
		pattern: P,
	): ToLoaderOutput<A, P> | undefined {
		const idx = store.matched_patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return store.loaders_data[idx] as ToLoaderOutput<A, P>;
	}

	function useClientLoaderData<P extends ToViewPattern<A>, T>(
		args: ToRouteComponentProps<A, P, T>,
	): T {
		return store.client_loaders_data[args.idx] as T;
	}

	function usePatternClientLoaderData<T>(
		pattern: ToViewPattern<A>,
	): T | undefined {
		const idx = store.matched_patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return store.client_loaders_data[idx] as T;
	}

	function defineView<P extends ToViewPattern<A>, T = any>(
		input: RemixDefineViewArgs<A, P, T>,
	): ViewDefinition {
		const {
			component,
			errorBoundary: error_boundary,
			...core_input
		} = input;
		return core.defineView({
			...core_input,
			component: (props: ToRouteComponentProps<A, P, T>) => {
				return create_remix_element(component, props);
			},
			errorBoundary: error_boundary
				? (props: { error: unknown }) => {
						return create_remix_element(error_boundary, props);
					}
				: undefined,
		});
	}

	function root_outlet_component(
		handle: Handle<RootOutletProps>,
	): (props: RootOutletProps) => RemixNode {
		track_handle(handle);
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
				make_route_id(idx, entry.pattern) ===
					pending_scroll_intent.target_route_id
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
			track_handle(handle);
			return (local: Record<string, unknown> = {}) => {
				return create_remix_element(RootOutlet, {
					...parent_props,
					...local,
					idx: idx + 1,
				});
			};
		});
	}

	function boot(): ReturnType<typeof core.boot> {
		const {
			render: render_root,
			onRouteUpdate: on_route_update,
			onWorkUpdate: on_work_update,
			linkDefaultProps: _link_default_props,
			apiDecorator: _api_decorator,
			...core_options
		} = options ?? {};
		return core.boot({
			...core_options,
			onRouteUpdate: (route, previous_route, reason) => {
				route_store = route;
				notify_handles();
				on_route_update?.(route, previous_route, reason);
			},
			onWorkUpdate: (work) => {
				work_store = work;
				notify_handles();
				on_work_update?.(work);
			},
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
				on<HTMLAnchorElement, typeof CLICK_EVENT>(
					CLICK_EVENT,
					(event) => {
						return result.onClick?.(event);
					},
				),
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
				on<HTMLAnchorElement, typeof FOCUS_EVENT>(
					FOCUS_EVENT,
					(event) => {
						return result.onFocus?.(event);
					},
				),
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
				on<HTMLAnchorElement, typeof BLUR_EVENT>(
					BLUR_EVENT,
					(event) => {
						return result.onBlur?.(event);
					},
				),
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

	type BaseLinkProps = RemixAnchorProps &
		LinkPropsBase & { pattern?: string };

	const BaseLink = make_hybrid_component<BaseLinkProps>((handle) => {
		track_handle(handle);
		return (props: BaseLinkProps) => {
			const route_state = route_store
				? select_link_route_state(route_store)
				: null;
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
				<a
					data-external={result.is_external || undefined}
					{...anchor_props}
				>
					{props.children}
				</a>
			);
		};
	});

	function render_link<P extends ToViewPattern<A>>(
		raw: RemixLinkProps<A, P>,
	): RemixNode {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const {
			href,
			pattern,
			params,
			splatValues: splat_values,
			search,
			hash,
			...props
		} = merged;
		return (
			<BaseLink
				{...props}
				pattern={pattern}
				href={
					href ??
					passthrough.toHref({
						pattern,
						params,
						splatValues: splat_values,
						search,
						hash,
					} as any)
				}
			/>
		);
	}

	const Link = ((input: unknown) => {
		if (is_remix_handle(input as any)) {
			const handle = input as Handle<RemixLinkProps<A, ToViewPattern<A>>>;
			track_handle(handle);
			return <P extends ToViewPattern<A>>(
				props: RemixLinkProps<A, P>,
			) => {
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
		useRouteSync,
		useRouteState,
		useWorkState,
		useLoaderData,
		usePatternLoaderData,
		useClientLoaderData,
		usePatternClientLoaderData,
	};
}
