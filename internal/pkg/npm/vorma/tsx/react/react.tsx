/// <reference types="vite/client" />

import {
	memo,
	useEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
	type ComponentProps,
	type JSX,
} from "react";
import {
	apply_scroll,
	build_typed_link_href,
	create_adapter_base,
	get_entry_key,
	make_link_props,
	make_route_id,
	resolve_outlet_slot,
	type AppConfig,
	type DecomposedState,
	type LinkPropsBase,
	type MakeTypedAPIDecorator,
	type MakeTypedDefineRouteInput,
	type MakeTypedLinkProps,
	type MakeTypedLoaderOutput,
	type MakeTypedLoaderPattern,
	type MakeTypedRouteProps,
	type MakeTypedRouterData,
	type RouteDefinition,
	type ScrollIntent,
	type VormaClient,
} from "vorma/__internal";

export type {
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	MakeTypedClientLoaderProps,
	MakeTypedLinkProps,
	MakeTypedLoaderOutput,
	MakeTypedLoaderPattern,
	MakeTypedMutationInput,
	MakeTypedMutationOutput,
	MakeTypedMutationPattern,
	MakeTypedMutationProps,
	MakeTypedNavigateProps,
	MakeTypedQueryInput,
	MakeTypedQueryOutput,
	MakeTypedQueryPattern,
	MakeTypedQueryProps,
	MakeTypedRouteProps,
	MakeTypedRouterData,
	RevalidationResult,
	SubmitOptions,
	SubmitResult,
	AppConfig as VormaAppConfig,
} from "vorma/__internal";

type CreateVormaClientOptions<A extends AppConfig> = {
	linkDefaultProps?: Partial<
		Omit<
			ComponentProps<"a"> & LinkPropsBase,
			"pattern" | "params" | "splatValues"
		>
	>;
	apiDecorator?: MakeTypedAPIDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): VormaClient<A, JSX.Element, ComponentProps<"a">> {
	let store: DecomposedState = {
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

	const listeners = new Set<() => void>();

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

	function get_snapshot(): DecomposedState {
		return store;
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

	function useRouterData(): MakeTypedRouterData<A>;
	function useRouterData<P extends MakeTypedLoaderPattern<A>>(
		routeProps: MakeTypedRouteProps<A, P>,
	): MakeTypedRouterData<A, P>;
	function useRouterData(_routeProps?: any): any {
		const loaders = use_channel((s) => {
			return s.loaders_data;
		});
		const patterns = use_channel((s) => {
			return s.matched_patterns;
		});
		const splat = use_channel((s) => {
			return s.splat_values;
		});
		const p = use_channel((s) => {
			return s.params;
		});
		const bid = use_channel((s) => {
			return s.client_build_id;
		});
		const history_state = use_channel((s) => {
			return s.history_state;
		});
		return {
			clientBuildID: bid,
			matchedPatterns: patterns,
			splatValues: splat,
			params: p,
			historyState: history_state,
			rootData: patterns[0] === "/" ? loaders[0] : undefined,
		};
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
			raw: Omit<ComponentProps<"a">, "href" | "pattern"> &
				MakeTypedLinkProps<A, P>,
		): JSX.Element => {
			const merged = { ...options?.linkDefaultProps, ...raw } as any;
			const {
				pattern,
				params,
				splatValues,
				search,
				hash,
				...link_props
			} = merged;
			const href = build_typed_link_href(
				pattern,
				params,
				splatValues,
				search,
				hash,
			);
			return <BaseLink {...link_props} href={href} />;
		},
	) as unknown as <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<ComponentProps<"a">, "href" | "pattern"> &
			MakeTypedLinkProps<A, P>,
	) => JSX.Element;

	return {
		...passthrough,
		defineRoute,
		RootOutlet,
		Link,
		useLoaderData,
		usePatternLoaderData,
		useRouterData,
		useClientLoaderData,
		usePatternClientLoaderData,
	};
}
