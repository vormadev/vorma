/// <reference types="vite/client" />

import { computed, signal } from "@preact/signals";
import {
	h,
	type ComponentType,
	type HTMLAttributes,
	type TargetedMouseEvent,
} from "preact";
import { useLayoutEffect, useMemo, useRef } from "preact/hooks";
import type { JSX } from "preact/jsx-runtime";
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

const store_signal = signal<AdapterStoreState | undefined>(undefined);

function get_state(): AdapterStoreState {
	if (store_signal.value) return store_signal.value;
	const initial = build_initial_store();
	store_signal.value = initial;
	return initial;
}

let subscribed = false;
function ensure_subscribed(): void {
	if (subscribed) return;
	subscribed = true;
	subscribe_to_store({
		get_state,
		set_state: (next) => {
			if (next !== store_signal.value) store_signal.value = next;
		},
	});
}

const loaders_data = computed(() => get_state().loaders_data);
const client_loaders_data = computed(() => get_state().client_loaders_data);
const router_data = computed(() => get_state().router_data);
const matched_patterns = computed(() => get_state().matched_patterns);

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
	return (() => router_data.value) as UseRouterDataFunction<App, false>;
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function useLoaderData<P extends VormaLoaderPattern<App>>(
		routeProps: VormaRouteProps<App, P>,
	): VormaLoaderOutput<App, P> {
		verify_route_data_access(
			matched_patterns.value,
			routeProps.idx,
			(routeProps as Record<string, unknown>)[ROUTE_PATTERN_KEY] as
				| string
				| undefined,
		);
		return loaders_data.value[routeProps.idx] as VormaLoaderOutput<App, P>;
	};
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function usePatternLoaderData<P extends VormaLoaderPattern<App>>(
		pattern: P,
	): VormaLoaderOutput<App, P> | undefined {
		const idx = find_pattern_index(matched_patterns.value, pattern);
		return idx < 0
			? undefined
			: (loaders_data.value[idx] as VormaLoaderOutput<App, P>);
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
			const cl = client_loaders_data.value;
			if (routeProps) return cl[routeProps.idx] as R;
			const idx = find_pattern_index(matched_patterns.value, p);
			return idx < 0 ? undefined : (cl[idx] as R);
		} as {
			(props: VormaRouteProps<App, P>): R;
			(): R | undefined;
		};
	};
}

// ─── Link ────────────────────────────────────────────────────────

type PreactLinkEvent = TargetedMouseEvent<HTMLAnchorElement>;

export function VormaLink(
	props: HTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<PreactLinkEvent>,
) {
	const r = make_link_props(props);
	return h(
		"a",
		{
			"data-external": r.dataExternal,
			...(r.anchorProps as HTMLAttributes<HTMLAnchorElement>),
			onPointerEnter: r.onPointerEnter,
			onFocus: r.onFocus,
			onPointerLeave: r.onPointerLeave,
			onBlur: r.onBlur,
			onTouchCancel: r.onTouchCancel,
			onClick: r.onClick,
		},
		props.children,
	);
}

type TypedLinkProps<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = Omit<HTMLAttributes<HTMLAnchorElement>, "href" | "pattern"> &
	VormaLinkPropsBase<PreactLinkEvent> &
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
	return (<P extends VormaLoaderPattern<App>>(
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
		return h(VormaLink, { ...linkProps, href });
	}) as <P extends VormaLoaderPattern<App>>(
		props: TypedLinkProps<App, P>,
	) => JSX.Element;
}

// ─── Root Outlet ─────────────────────────────────────────────────

export function VormaRootOutlet(
	props: { idx?: number } & Record<string, unknown>,
): JSX.Element | null {
	const idx = props.idx ?? 0;
	const ref = useRef(props);
	ref.current = props;

	useLayoutEffect(() => {
		if (idx !== 0) return;
		ensure_subscribed();
	}, [idx]);

	const store = get_state();
	const route_count = store.matched_patterns.length;

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

	const next_key =
		idx + 1 < route_count
			? get_route_key(store, idx + 1)
			: `__empty_${idx + 1}`;
	const Outlet = useMemo(
		() => (local?: Record<string, unknown>) =>
			h(VormaRootOutlet, {
				...ref.current,
				...local,
				idx: idx + 1,
			}),
		[idx, next_key],
	);

	if (idx > route_count) return null;
	if (idx === route_count) return h(Outlet, { key: next_key });

	const err_idx = store.outermost_error_idx;
	if (err_idx != null && idx >= err_idx) {
		const Comp = store.active_error_boundary
			? (store.active_error_boundary as ComponentType<{
					error: unknown;
				}>)
			: undefined;
		if (Comp) return h(Comp, { error: store.outermost_error });
		return h("span", {}, format_error_for_rendering(store.outermost_error));
	}

	const Comp = store.active_components[idx] as ComponentType<any> | undefined;
	if (!Comp) return null;

	const mount_key = `${get_route_key(store, idx)}::${store.matched_patterns[idx]}`;
	return h(Comp, {
		key: mount_key,
		idx,
		Outlet,
		[ROUTE_PATTERN_KEY]: store.matched_patterns[idx],
	});
}
