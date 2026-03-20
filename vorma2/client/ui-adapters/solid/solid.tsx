/// <reference types="vite/client" />

import {
	createMemo,
	createRenderEffect,
	createSignal,
	onCleanup,
	onMount,
	Show,
	type Accessor,
	type JSX,
	type ValidComponent,
} from "solid-js";
import { Dynamic } from "solid-js/web";
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

const [store_signal, set_store_signal] = createSignal<
	AdapterStoreState | undefined
>(undefined);

function get_state(): AdapterStoreState {
	const current = store_signal();
	if (current) return current;
	const initial = build_initial_store();
	set_store_signal(initial);
	return initial;
}

let subscribed = false;
function ensure_subscribed(): () => void {
	if (subscribed) return () => {};
	subscribed = true;
	return subscribe_to_store({
		get_state,
		set_state: (next) => {
			if (next !== store_signal()) set_store_signal(next);
		},
	});
}

// ─── Accessors ──────────────────────────────────────────────────

function read_loaders_data() {
	return get_state().loaders_data;
}
function read_client_loaders_data() {
	return get_state().client_loaders_data;
}
function read_matched_patterns() {
	return get_state().matched_patterns;
}
function read_router_data() {
	return get_state().router_data;
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
	return (() =>
		createMemo(() => read_router_data())) as UseRouterDataFunction<
		App,
		true
	>;
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function useLoaderData<P extends VormaLoaderPattern<App>>(
		routeProps: VormaRouteProps<App, P>,
	): Accessor<VormaLoaderOutput<App, P>> {
		return createMemo(() => {
			verify_route_data_access(
				read_matched_patterns(),
				routeProps.idx,
				(routeProps as Record<string, unknown>)[ROUTE_PATTERN_KEY] as
					| string
					| undefined,
			);
			return read_loaders_data()[routeProps.idx] as VormaLoaderOutput<
				App,
				P
			>;
		});
	};
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(c: C) {
	void c;
	type App = ExtractApp<C>;
	return function usePatternLoaderData<P extends VormaLoaderPattern<App>>(
		pattern: P,
	): Accessor<VormaLoaderOutput<App, P> | undefined> {
		return createMemo(() => {
			const idx = find_pattern_index(read_matched_patterns(), pattern);
			return idx < 0
				? undefined
				: (read_loaders_data()[idx] as VormaLoaderOutput<App, P>);
		});
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
		): Accessor<R | undefined> {
			return createMemo(() => {
				const cl = read_client_loaders_data();
				if (routeProps) return cl[routeProps.idx] as R;
				const idx = find_pattern_index(read_matched_patterns(), p);
				return idx < 0 ? undefined : (cl[idx] as R);
			});
		} as {
			(props: VormaRouteProps<App, P>): Accessor<R>;
			(): Accessor<R | undefined>;
		};
	};
}

// ─── Link ────────────────────────────────────────────────────────

type SolidLinkEvent = Event;

export function VormaLink(
	props: JSX.AnchorHTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<SolidLinkEvent>,
) {
	const r = createMemo(() => make_link_props(props));
	const safe = createMemo(
		() => r().anchorProps as JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
	);
	return (
		<a
			data-external={r().dataExternal}
			{...safe()}
			onPointerEnter={r().onPointerEnter}
			onFocus={r().onFocus}
			onPointerLeave={r().onPointerLeave}
			onBlur={r().onBlur}
			onTouchCancel={r().onTouchCancel}
			onClick={r().onClick}
		>
			{props.children}
		</a>
	);
}

type TypedLinkProps<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href" | "pattern"> &
	VormaLinkPropsBase<SolidLinkEvent> &
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
		const href = createMemo(() =>
			build_typed_link_href({
				app_config: vormaAppConfig,
				pattern,
				params,
				splat_values: splatValues,
				search,
				hash,
			}),
		);
		return (
			<VormaLink
				{...(linkProps as JSX.AnchorHTMLAttributes<HTMLAnchorElement>)}
				href={href()}
			/>
		);
	}) as <P extends VormaLoaderPattern<App>>(
		props: TypedLinkProps<App, P>,
	) => JSX.Element;
}

// ─── Root Outlet ─────────────────────────────────────────────────

export function VormaRootOutlet(
	props: { idx?: number } & Record<string, unknown>,
): JSX.Element {
	const idx = props.idx ?? 0;
	get_state();

	onMount(() => {
		if (idx !== 0) return;
		const unsub = ensure_subscribed();
		onCleanup(unsub);
	});

	const store = createMemo(() => get_state());
	const route_count = createMemo(() => store().matched_patterns.length);
	const err_idx = createMemo(() => store().outermost_error_idx);
	const is_error = createMemo(() => err_idx() != null && idx >= err_idx()!);
	const comp = createMemo(() =>
		is_error() ? null : store().active_components[idx],
	);
	const next_key = createMemo(() =>
		idx + 1 < route_count()
			? get_route_key(store(), idx + 1)
			: `__empty_${idx + 1}`,
	);
	const mount_key = createMemo(
		() =>
			`${get_route_key(store(), idx)}::${store().matched_patterns[idx]}`,
	);
	const matched_pattern = createMemo(() => store().matched_patterns[idx]);

	const Outlet = createMemo(() => {
		void next_key();
		return (local?: Record<string, unknown>): JSX.Element => (
			<VormaRootOutlet {...props} {...local} idx={idx + 1} />
		);
	});

	// Scroll restoration: apply pending scroll after the render
	// phase so that hash targets created by newly mounted route
	// components exist before we scroll.
	let last_scroll_id = 0;
	createRenderEffect(() => {
		store(); // reactive dependency — re-runs when store changes
		if (idx !== 0) return;
		const pending = consume_pending_scroll();
		if (pending.id > last_scroll_id) {
			last_scroll_id = pending.id;
			apply_pending_scroll(pending.scroll);
		}
	});

	return (
		<>
			<Show when={comp()}>
				<Show when={mount_key()} keyed>
					<Dynamic
						component={comp() as ValidComponent}
						idx={idx}
						Outlet={Outlet()}
						{...{ [ROUTE_PATTERN_KEY]: matched_pattern() }}
					/>
				</Show>
			</Show>

			<Show when={!comp() && !is_error() && idx < route_count()}>
				<Show when={next_key()} keyed>
					{Outlet()()}
				</Show>
			</Show>

			<Show when={is_error()}>
				<Show
					when={store().active_error_boundary}
					fallback={
						<>
							{format_error_for_rendering(
								store().outermost_error,
							)}
						</>
					}
				>
					<Dynamic
						component={
							store().active_error_boundary as ValidComponent
						}
						error={store().outermost_error}
					/>
				</Show>
			</Show>
		</>
	);
}
