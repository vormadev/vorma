/// <reference types="vite/client" />

import {
	batch,
	createEffect,
	createMemo,
	createSignal,
	Show,
	type Accessor,
	type JSX,
	type ValidComponent,
} from "solid-js";
import { Dynamic } from "solid-js/web";
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
	type MakeTypedLoaderO,
	type MakeTypedLoaderPattern,
	type MakeTypedRouteProps,
	type MakeTypedRouterData,
	type RouteDefinition,
	type RouteEntry,
	type ScrollIntent,
	type VormaClient,
} from "vorma/__internal";

export type { AppConfig as VormaAppConfig };

type CreateVormaClientOptions<A extends AppConfig> = {
	linkDefaultProps?: Partial<
		Omit<
			JSX.AnchorHTMLAttributes<HTMLAnchorElement> & LinkPropsBase,
			"pattern" | "params" | "splatValues"
		>
	>;
	apiDecorator?: MakeTypedAPIDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): VormaClient<
	A,
	JSX.Element,
	JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
	true
> {
	const [entries, set_entries] = createSignal<RouteEntry[]>([]);
	const [loaders_data, set_loaders_data] = createSignal<unknown[]>([]);
	const [client_loaders_data, set_client_loaders_data] = createSignal<
		unknown[]
	>([]);
	const [matched_patterns, set_matched_patterns] = createSignal<string[]>([]);
	const [import_urls, set_import_urls] = createSignal<string[]>([]);
	const [entry_keys, set_entry_keys] = createSignal<string[]>([]);
	const [params, set_params] = createSignal<Record<string, string>>({});
	const [splat_values, set_splat_values] = createSignal<string[]>([]);
	const [build_id, set_build_id] = createSignal("");
	const [history_state, set_history_state] = createSignal<unknown>(undefined);

	let pending_scroll_intent: ScrollIntent | undefined;

	// Used to trigger the scroll effect.
	const [nav_counter, set_nav_counter] = createSignal(0);

	const adapter_base_res = create_adapter_base(
		app_config,
		(decomposed: DecomposedState, scroll_intent?: ScrollIntent) => {
			if (scroll_intent) {
				pending_scroll_intent = scroll_intent;
			}

			batch(() => {
				set_entries(decomposed.entries);
				set_loaders_data(decomposed.loaders_data);
				set_client_loaders_data(decomposed.client_loaders_data);
				set_matched_patterns(decomposed.matched_patterns);
				set_import_urls(decomposed.import_urls);
				set_entry_keys(decomposed.entry_keys);
				set_params(decomposed.params);
				set_splat_values(decomposed.splat_values);
				set_build_id(decomposed.client_build_id);
				set_history_state(decomposed.history_state);

				if (scroll_intent) {
					set_nav_counter((c) => {
						return c + 1;
					});
				}
			});
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
	): Accessor<MakeTypedLoaderO<A, P>> {
		return createMemo(() => {
			return loaders_data()[props.idx] as MakeTypedLoaderO<A, P>;
		});
	}

	function usePatternLoaderData<P extends MakeTypedLoaderPattern<A>>(
		pattern: P,
	): Accessor<MakeTypedLoaderO<A, P> | undefined> {
		return createMemo(() => {
			const patterns = matched_patterns();
			const idx = patterns.indexOf(pattern);
			if (idx < 0) {
				return undefined;
			}
			return loaders_data()[idx] as MakeTypedLoaderO<A, P>;
		});
	}

	function useRouterData(): Accessor<MakeTypedRouterData<A>>;
	function useRouterData<P extends MakeTypedLoaderPattern<A>>(
		routeProps: MakeTypedRouteProps<A, P>,
	): Accessor<MakeTypedRouterData<A, P>>;
	function useRouterData(_routeProps?: any): any {
		return createMemo(() => {
			const patterns = matched_patterns();
			return {
				clientBuildID: build_id(),
				matchedPatterns: patterns,
				splatValues: splat_values(),
				params: params(),
				historyState: history_state(),
				rootData: patterns[0] === "/" ? loaders_data()[0] : undefined,
			};
		});
	}

	function useClientLoaderData<P extends MakeTypedLoaderPattern<A>, T>(
		props: MakeTypedRouteProps<A, P, T>,
	): Accessor<T> {
		return createMemo(() => {
			return client_loaders_data()[props.idx] as T;
		});
	}

	function usePatternClientLoaderData<T>(
		pattern: MakeTypedLoaderPattern<A>,
	): Accessor<T | undefined> {
		return createMemo(() => {
			const patterns = matched_patterns();
			const idx = patterns.indexOf(pattern);
			if (idx < 0) {
				return undefined;
			}
			return client_loaders_data()[idx] as T;
		});
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

		createEffect(() => {
			const _counter = nav_counter();

			if (!pending_scroll_intent) {
				return;
			}

			const current_entries = entries();
			const entry = current_entries[idx];
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

			requestAnimationFrame(() => {
				apply_scroll(scroll);
			});
		});

		const current_entries = createMemo(() => {
			return entries();
		});

		const slot = createMemo(() => {
			return resolve_outlet_slot(
				current_entries(),
				idx,
				core.get_default_error_boundary(),
			);
		});

		const entry_key_at_idx = createMemo(() => {
			const e = current_entries();
			return idx < e.length ? get_entry_key(e[idx]!) : "";
		});

		const remount_key_next = createMemo(() => {
			const urls = import_urls();
			const keys = entry_keys();
			return `${urls[idx + 1]}|${keys[idx + 1]}`;
		});

		const component = createMemo(() => {
			const s = slot();
			return s.kind === "component"
				? (s.component as ValidComponent)
				: undefined;
		});

		const error_boundary = createMemo(() => {
			const s = slot();
			return s.kind === "error"
				? (s.boundary as ValidComponent)
				: undefined;
		});

		const error_value = createMemo(() => {
			const s = slot();
			return s.kind === "error" ? s.error : undefined;
		});

		const is_pass_through = createMemo(() => {
			return slot().kind === "pass_through";
		});

		const Outlet = (local?: Record<string, unknown>): JSX.Element => {
			return (
				<Show when={remount_key_next()} keyed>
					<RootOutlet {...props} {...local} idx={idx + 1} />
				</Show>
			);
		};

		return (
			<>
				<Show when={component()}>
					<Show when={entry_key_at_idx()} keyed>
						<Dynamic
							component={component()!}
							idx={idx}
							Outlet={Outlet}
						/>
					</Show>
				</Show>

				<Show when={is_pass_through()}>{Outlet()}</Show>

				<Show when={error_boundary()}>
					<Dynamic
						component={error_boundary()!}
						error={error_value()}
					/>
				</Show>
			</>
		);
	}

	function BaseLink(
		props: JSX.AnchorHTMLAttributes<HTMLAnchorElement> & LinkPropsBase,
	): JSX.Element {
		const r = createMemo(() => {
			return make_link_props(props as Record<string, unknown>, nav_fns);
		});
		const anchor_props = createMemo(() => {
			return r()
				.anchor_props as JSX.AnchorHTMLAttributes<HTMLAnchorElement>;
		});
		return (
			<a
				data-external={r().is_external || undefined}
				{...anchor_props()}
				onClick={r().onClick}
				onPointerDown={r().onPointerDown}
				onPointerEnter={r().onPointerEnter}
				onFocus={r().onFocus}
				onPointerLeave={r().onPointerLeave}
				onBlur={r().onBlur}
				onTouchCancel={r().onTouchCancel}
			>
				{props.children}
			</a>
		);
	}

	const Link = (<P extends MakeTypedLoaderPattern<A>>(
		raw: Omit<
			JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
			"href" | "pattern"
		> &
			MakeTypedLinkProps<A, P>,
	): JSX.Element => {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const { pattern, params, splatValues, search, hash, ...link_props } =
			merged;
		const href = createMemo(() => {
			return build_typed_link_href(
				pattern,
				params,
				splatValues,
				search,
				hash,
			);
		});
		return <BaseLink {...link_props} href={href()} />;
	}) as <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<
			JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
			"href" | "pattern"
		> &
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
