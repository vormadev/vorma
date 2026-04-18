/// <reference types="vite/client" />

import {
	memo,
	useEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
	type ComponentProps,
	type ComponentType,
	type JSX,
} from "react";
import {
	apply_scroll,
	create_adapter_base,
	get_entry_key,
	make_link_props,
	make_route_id,
	resolve_outlet_slot,
	type AdapterInitOptions,
	type AppConfig,
	type DecomposedState,
	type LinkPropsBase,
	type MakeTypedAPIDecorator,
	type MakeTypedDefineRouteInput,
	type MakeTypedLinkProps,
	type MakeTypedLoaderOutput,
	type MakeTypedLoaderPattern,
	type MakeTypedRouteProps,
	type RouteDefinition,
	type RouteState,
	type ScrollIntent,
	type VormaClient,
	type WorkState,
} from "vorma/__internal";
import { jsonDeepEquals } from "vorma/kit/json";

export type {
	MakeTypedActionInput,
	MakeTypedActionMethod,
	MakeTypedActionOutput,
	MakeTypedActionPattern,
	MakeTypedActionSubmitOutput,
	MakeTypedActionSubmitProps,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	MakeTypedClientLoaderProps,
	MakeTypedLinkProps,
	MakeTypedLoaderInput,
	MakeTypedLoaderOutput,
	MakeTypedLoaderPattern,
	MakeTypedNavProps,
	MakeTypedNavTarget,
	MakeTypedRouteDestination,
	MakeTypedRouteProps,
	ProgressIndicatorConfig,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
	SubmitOptions,
	SubmitResult,
	AppConfig as VormaAppConfig,
	WorkState,
} from "vorma/__internal";

type CreateVormaClientOptions<A extends AppConfig> = {
	linkDefaultProps?: Partial<
		Omit<ComponentProps<"a"> & LinkPropsBase, "href">
	>;
	apiDecorator?: MakeTypedAPIDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): VormaClient<A, JSX.Element, ComponentProps<"a">, false, ComponentType> {
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
		submissions: [],
	};

	const listeners = new Set<() => void>();
	const route_listeners = new Set<() => void>();
	const work_listeners = new Set<() => void>();

	function notify(): void {
		listeners.forEach((fn) => {
			return fn();
		});
	}

	function subscribe(listener: () => void): () => void {
		listeners.add(listener);
		return () => {
			return listeners.delete(listener);
		};
	}

	function subscribe_route(listener: () => void): () => void {
		route_listeners.add(listener);
		return () => {
			return route_listeners.delete(listener);
		};
	}

	function subscribe_work(listener: () => void): () => void {
		work_listeners.add(listener);
		return () => {
			return work_listeners.delete(listener);
		};
	}

	function get_snapshot(): DecomposedState {
		return store;
	}

	function get_route_snapshot(): RouteState {
		if (!route_store) {
			throw new Error("Vorma not initialized");
		}
		return route_store;
	}

	function get_work_snapshot(): WorkState {
		return work_store;
	}

	function notify_route(): void {
		route_listeners.forEach((fn) => {
			return fn();
		});
	}

	function notify_work(): void {
		work_listeners.forEach((fn) => {
			return fn();
		});
	}

	function use_channel<T>(select: (s: DecomposedState) => T): T {
		return useSyncExternalStore(
			subscribe,
			() => {
				return select(get_snapshot());
			},
			() => {
				return select(get_snapshot());
			},
		);
	}

	function use_store_selector<State, Selected>(
		subscribe_fn: (listener: () => void) => () => void,
		get_state: () => State,
		selector: (state: State) => Selected,
	): Selected {
		const selected_ref = useRef<Selected | undefined>(undefined);
		return useSyncExternalStore(
			subscribe_fn,
			() => {
				const next = selector(get_state());
				if (
					selected_ref.current !== undefined &&
					jsonDeepEquals(selected_ref.current, next)
				) {
					return selected_ref.current;
				}
				selected_ref.current = next;
				return next;
			},
			() => {
				return selector(get_state());
			},
		);
	}

	let pending_scroll_intent: ScrollIntent | undefined;

	const adapter_base_res = create_adapter_base(
		app_config,
		(decomposed: DecomposedState, scroll_intent?: ScrollIntent) => {
			if (scroll_intent) {
				pending_scroll_intent = scroll_intent;
			}
			store = decomposed;
			notify();
		},
		options?.apiDecorator,
	);
	if (!adapter_base_res.ok) {
		throw new Error(
			`Failed to create Vorma client: ${adapter_base_res.err}`,
		);
	}

	const { core, nav_fns, passthrough } = adapter_base_res.val;

	function useRouteState(): RouteState;
	function useRouteState<T>(selector: (route: RouteState) => T): T;
	function useRouteState<T>(
		selector?: (route: RouteState) => T,
	): RouteState | T {
		const select: (route: RouteState) => RouteState | T = selector
			? (route: RouteState) => {
					return selector(route);
				}
			: (route: RouteState) => {
					return route;
				};
		return use_store_selector(subscribe_route, get_route_snapshot, select);
	}

	function useWorkState(): WorkState;
	function useWorkState<T>(selector: (work: WorkState) => T): T;
	function useWorkState<T>(selector?: (work: WorkState) => T): WorkState | T {
		const select: (work: WorkState) => WorkState | T = selector
			? (work: WorkState) => {
					return selector(work);
				}
			: (work: WorkState) => {
					return work;
				};
		return use_store_selector(subscribe_work, get_work_snapshot, select);
	}

	function useLoaderData<P extends MakeTypedLoaderPattern<A>>(
		props: MakeTypedRouteProps<A, P>,
	): MakeTypedLoaderOutput<A, P> {
		const loaders = use_channel((s) => {
			return s.loaders_data;
		});
		return loaders[props.idx] as MakeTypedLoaderOutput<A, P>;
	}

	function usePatternLoaderData<P extends MakeTypedLoaderPattern<A>>(
		pattern: P,
	): MakeTypedLoaderOutput<A, P> | undefined {
		const loaders = use_channel((s) => {
			return s.loaders_data;
		});
		const patterns = use_channel((s) => {
			return s.matched_patterns;
		});
		const idx = patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return loaders[idx] as MakeTypedLoaderOutput<A, P>;
	}

	function useClientLoaderData<P extends MakeTypedLoaderPattern<A>, T>(
		props: MakeTypedRouteProps<A, P, T>,
	): T {
		const cl = use_channel((s) => {
			return s.client_loaders_data;
		});
		return cl[props.idx] as T;
	}

	function usePatternClientLoaderData<T>(
		pattern: MakeTypedLoaderPattern<A>,
	): T | undefined {
		const cl = use_channel((s) => {
			return s.client_loaders_data;
		});
		const patterns = use_channel((s) => {
			return s.matched_patterns;
		});
		const idx = patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return cl[idx] as T;
	}

	function defineRoute<P extends MakeTypedLoaderPattern<A>, T = any>(
		input: MakeTypedDefineRouteInput<A, P, T, JSX.Element>,
	): RouteDefinition {
		return core.defineRoute(input as any);
	}

	function RootOutlet(
		props: { idx?: number } & Record<string, unknown>,
	): JSX.Element {
		const idx = props.idx ?? 0;
		const ref = useRef(props);
		ref.current = props;

		const entries = use_channel((s) => {
			return s.entries;
		});
		const route_error = use_channel((s) => {
			return s.error;
		});
		const import_urls = use_channel((s) => {
			return s.import_urls;
		});
		const entry_keys = use_channel((s) => {
			return s.entry_keys;
		});

		useEffect(() => {
			if (!pending_scroll_intent) {
				return;
			}

			const entry = entries[idx];
			if (!entry) {
				return;
			}

			if (
				make_route_id(idx, entry.pattern) !==
				pending_scroll_intent.target_route_id
			) {
				return;
			}

			const scroll = pending_scroll_intent.scroll;
			pending_scroll_intent = undefined;

			apply_scroll(scroll);
		});

		const next_import_url = import_urls[idx + 1];
		const next_entry_key = entry_keys[idx + 1];

		const Outlet = useMemo(() => {
			return (local?: Record<string, unknown>) => {
				return <RootOutlet {...ref.current} {...local} idx={idx + 1} />;
			};
		}, [idx, next_import_url, next_entry_key]);

		const slot = resolve_outlet_slot(
			entries,
			route_error,
			idx,
			core.get_default_error_boundary(),
		);

		switch (slot.kind) {
			case "empty":
				return <></>;
			case "pass_through": {
				const key =
					idx + 1 < entries.length
						? get_entry_key(entries[idx + 1]!)
						: `__empty_${idx + 1}`;
				return <Outlet key={key} />;
			}
			case "error": {
				const Boundary = slot.boundary;
				return <Boundary error={slot.error} />;
			}
			case "component": {
				const key = get_entry_key(entries[idx]!);
				const Comp = slot.component;
				return <Comp key={key} idx={idx} Outlet={Outlet} />;
			}
		}
	}

	const App: ComponentType = () => {
		return <RootOutlet />;
	};

	function init(
		options: AdapterInitOptions<ComponentType>,
	): ReturnType<typeof core.init> {
		const { render, onRouteUpdate, onWorkUpdate, ...core_options } =
			options;
		return core.init({
			...core_options,
			onRouteUpdate: (route, previous_route, reason) => {
				route_store = route;
				notify_route();
				onRouteUpdate?.(route, previous_route, reason);
			},
			onWorkUpdate: (work) => {
				work_store = work;
				notify_work();
				onWorkUpdate?.(work);
			},
			render: render
				? () => {
						return render({ App, el: core.getRootEl() });
					}
				: undefined,
		});
	}

	function BaseLink(props: ComponentProps<"a"> & LinkPropsBase): JSX.Element {
		const r = make_link_props(props as Record<string, unknown>, nav_fns);
		return (
			<a
				data-external={r.is_external || undefined}
				{...(r.anchor_props as ComponentProps<"a">)}
				onClick={r.onClick}
				onPointerDown={r.onPointerDown}
				onPointerEnter={r.onPointerEnter}
				onFocus={r.onFocus}
				onPointerLeave={r.onPointerLeave}
				onBlur={r.onBlur}
				onTouchCancel={r.onTouchCancel}
			>
				{props.children}
			</a>
		);
	}

	const Link = memo(
		<P extends MakeTypedLoaderPattern<A>>(
			raw: Omit<ComponentProps<"a">, "href"> & MakeTypedLinkProps<A, P>,
		): JSX.Element => {
			const merged = { ...options?.linkDefaultProps, ...raw } as any;
			const {
				href,
				pattern,
				params,
				splatValues,
				search,
				hash,
				...props
			} = merged;
			return (
				<BaseLink
					{...props}
					href={
						href ??
						passthrough.buildHref({
							pattern,
							params,
							splatValues,
							search,
							hash,
						} as any)
					}
				/>
			);
		},
	) as unknown as <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<ComponentProps<"a">, "href"> & MakeTypedLinkProps<A, P>,
	) => JSX.Element;

	return {
		...passthrough,
		init,
		defineRoute,
		RootOutlet,
		Link,
		useRouteState,
		useWorkState,
		useLoaderData,
		usePatternLoaderData,
		useClientLoaderData,
		usePatternClientLoaderData,
	};
}
