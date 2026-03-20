/// <reference types="vite/client" />

import {
	memo,
	useLayoutEffect,
	useMemo,
	useRef,
	useSyncExternalStore,
	type ComponentProps,
	type ComponentType,
	type JSX,
} from "react";
import {
	apply_pending_scroll,
	build_initial_store,
	build_typed_link_href,
	consume_pending_scroll,
	find_pattern_index,
	format_error_for_rendering,
	get_route_key,
	make_link_props,
	register_client_loader,
	ROUTE_PATTERN_KEY,
	subscribe_to_store,
	verify_route_data_access,
} from "vorma/client/__internal";
import type {
	AdapterStoreState,
	ExtractApp,
	PermissivePatternBasedProps,
	UseRouterDataFunction,
	VormaAppBase,
	VormaAppConfig,
	VormaLinkPropsBase,
	VormaLoaderOutput,
	VormaLoaderPattern,
	VormaRouteGeneric,
	VormaRoutePropsGeneric,
	VormaTypedAdapterAddClientLoaderProps,
} from "../../types.ts";

// ─── Store ───────────────────────────────────────────────────────

type Listener = () => void;
let store_state: AdapterStoreState | undefined;
const listeners = new Set<Listener>();

function get_state(): AdapterStoreState {
	if (store_state) return store_state;
	store_state = build_initial_store();
	return store_state;
}

function set_state(next: AdapterStoreState): void {
	if (next === store_state) return;
	store_state = next;
	listeners.forEach((fn) => fn());
}

function subscribe(listener: Listener): () => void {
	listeners.add(listener);
	return () => listeners.delete(listener);
}

function use_selector<T>(select: (s: AdapterStoreState) => T): T {
	return useSyncExternalStore(
		subscribe,
		() => select(get_state()),
		() => select(get_state()),
	);
}

let subscribed = false;
function ensure_subscribed(): void {
	if (subscribed) return;
	subscribed = true;
	subscribe_to_store({ get_state, set_state });
}

// ─── Typed Value Hooks ───────────────────────────────────────────

export type VormaRouteProps<
	App extends VormaAppBase = any,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = VormaRoutePropsGeneric<JSX.Element, App, P>;

export type VormaRoute<
	App extends VormaAppBase = any,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = VormaRouteGeneric<JSX.Element, App, P>;

export function makeTypedUseRouterData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return (() => use_selector((s) => s.router_data)) as UseRouterDataFunction<
		App,
		false
	>;
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function useLoaderData<P extends VormaLoaderPattern<App>>(
		routeProps: VormaRouteProps<App, P>,
	): VormaLoaderOutput<App, P> {
		const loaders = use_selector((s) => s.loaders_data);
		const matched = use_selector((s) => s.matched_patterns);
		verify_route_data_access(
			matched,
			routeProps.idx,
			(routeProps as Record<string, unknown>)[ROUTE_PATTERN_KEY] as
				| string
				| undefined,
		);
		return loaders[routeProps.idx] as VormaLoaderOutput<App, P>;
	};
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function usePatternLoaderData<P extends VormaLoaderPattern<App>>(
		pattern: P,
	): VormaLoaderOutput<App, P> | undefined {
		const loaders = use_selector((s) => s.loaders_data);
		const matched = use_selector((s) => s.matched_patterns);
		return useMemo(() => {
			const idx = find_pattern_index(matched, pattern);
			return idx < 0
				? undefined
				: (loaders[idx] as VormaLoaderOutput<App, P>);
		}, [pattern, loaders, matched]);
	};
}

export function makeTypedAddClientLoader<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function addClientLoader<
		P extends VormaLoaderPattern<App>,
		LD extends VormaLoaderOutput<App, P>,
		R,
	>(props: VormaTypedAdapterAddClientLoaderProps<App, P, LD, R>) {
		register_client_loader(props);
		const p = props.pattern;
		return function useClientLoaderData(
			routeProps?: VormaRouteProps<App, P>,
		): R | undefined {
			const cl = use_selector((s) => s.client_loaders_data);
			const matched = use_selector((s) => s.matched_patterns);
			if (routeProps) return cl[routeProps.idx] as R;
			const idx = find_pattern_index(matched, p);
			return idx < 0 ? undefined : (cl[idx] as R);
		} as {
			(props: VormaRouteProps<App, P>): R;
			(): R | undefined;
		};
	};
}

// ─── Link ────────────────────────────────────────────────────────

type ReactLinkEvent = React.MouseEvent<HTMLAnchorElement>;

export function VormaLink(
	props: ComponentProps<"a"> & VormaLinkPropsBase<ReactLinkEvent>,
) {
	const r = make_link_props(props);
	return (
		<a
			data-external={r.dataExternal}
			{...(r.anchorProps as ComponentProps<"a">)}
			onPointerEnter={r.onPointerEnter}
			onFocus={r.onFocus}
			onPointerLeave={r.onPointerLeave}
			onBlur={r.onBlur}
			onTouchCancel={r.onTouchCancel}
			onClick={r.onClick}
		>
			{props.children}
		</a>
	);
}

type TypedLinkProps<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = Omit<ComponentProps<"a">, "href" | "pattern"> &
	VormaLinkPropsBase<ReactLinkEvent> &
	PermissivePatternBasedProps<App, P> & {
		search?: string;
		hash?: string;
	};

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: Partial<
		Omit<
			TypedLinkProps<ExtractApp<C>>,
			"pattern" | "params" | "splatValues"
		>
	>,
) {
	type App = ExtractApp<C>;
	const TypedLink = <P extends VormaLoaderPattern<App>>(
		raw: TypedLinkProps<App, P>,
	) => {
		const merged = { ...defaultProps, ...raw } as any;
		const { pattern, params, splatValues, search, hash, ...linkProps } =
			merged;
		const href = build_typed_link_href({
			app_config: vormaAppConfig,
			pattern,
			params,
			splat_values: splatValues,
			search,
			hash,
		});
		return <VormaLink {...linkProps} href={href} />;
	};
	return memo(TypedLink) as <P extends VormaLoaderPattern<App>>(
		props: TypedLinkProps<App, P>,
	) => JSX.Element;
}

// ─── Root Outlet ─────────────────────────────────────────────────

export function VormaRootOutlet(
	props: { idx?: number } & Record<string, unknown>,
): JSX.Element {
	const idx = props.idx ?? 0;
	const ref = useRef(props);
	ref.current = props;

	useLayoutEffect(() => {
		if (idx !== 0) return;
		ensure_subscribed();
	}, [idx]);

	const matched_patterns = use_selector((s) => s.matched_patterns);
	const active_components = use_selector((s) => s.active_components);
	const active_error_boundary = use_selector((s) => s.active_error_boundary);
	const outermost_error = use_selector((s) => s.outermost_error);
	const outermost_error_idx = use_selector((s) => s.outermost_error_idx);
	const import_urls = use_selector((s) => s.import_urls);
	const export_keys = use_selector((s) => s.export_keys);

	const route_count = matched_patterns.length;

	// Scroll restoration: apply pending scroll after DOM commit so
	// that hash targets created by newly mounted route components
	// exist before we scroll.
	const scroll_id_ref = useRef(0);
	useLayoutEffect(() => {
		if (idx !== 0) return;
		const pending = consume_pending_scroll();
		if (pending.id > scroll_id_ref.current) {
			scroll_id_ref.current = pending.id;
			apply_pending_scroll(pending.scroll);
		}
	});

	// Build a stable store-like object for get_route_key.
	const store_view = useMemo(
		() => ({ matched_patterns, import_urls, export_keys }) as any,
		[matched_patterns, import_urls, export_keys],
	);

	const next_key =
		idx + 1 < route_count
			? get_route_key(store_view, idx + 1)
			: `__empty_${idx + 1}`;
	const Outlet = useMemo(
		() => (local?: Record<string, unknown>) => (
			<VormaRootOutlet {...ref.current} {...local} idx={idx + 1} />
		),
		[idx, next_key],
	);

	if (idx > route_count) return <></>;
	if (idx === route_count) return <Outlet key={next_key} />;

	if (outermost_error_idx != null && idx >= outermost_error_idx) {
		const Comp = active_error_boundary as
			| ComponentType<{ error: unknown }>
			| undefined;
		if (Comp) return <Comp error={outermost_error} />;
		return <>{format_error_for_rendering(outermost_error)}</>;
	}

	const Comp = active_components[idx] as ComponentType<any> | undefined;
	if (!Comp) return <></>;

	const mount_key = `${get_route_key(store_view, idx)}::${matched_patterns[idx]}`;
	return (
		<Comp
			key={mount_key}
			idx={idx}
			Outlet={Outlet}
			{...{ [ROUTE_PATTERN_KEY]: matched_patterns[idx] }}
		/>
	);
}
