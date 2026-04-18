/// <reference types="vite/client" />

import {
	batch,
	createEffect,
	createMemo,
	createSignal,
	Show,
	type Accessor,
	type Component,
	type JSX,
	type ValidComponent,
} from "solid-js";
import { Dynamic } from "solid-js/web";
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
		Omit<
			JSX.AnchorHTMLAttributes<HTMLAnchorElement> & LinkPropsBase,
			"href"
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
	true,
	Component
> {
	const [entries, set_entries] = createSignal<DecomposedState["entries"]>([]);
	const [route_error, set_route_error] =
		createSignal<DecomposedState["error"]>(null);
	const [loaders_data, set_loaders_data] = createSignal<unknown[]>([]);
	const [client_loaders_data, set_client_loaders_data] = createSignal<
		unknown[]
	>([]);
	const [matched_patterns, set_matched_patterns] = createSignal<string[]>([]);
	const [import_urls, set_import_urls] = createSignal<string[]>([]);
	const [entry_keys, set_entry_keys] = createSignal<string[]>([]);
	const [route_state, set_route_state] = createSignal<RouteState | null>(
		null,
		{ equals: jsonDeepEquals },
	);
	const [work_state, set_work_state] = createSignal<WorkState>(
		{
			navigation: null,
			revalidation: null,
			prefetch: null,
			submissions: [],
		},
		{ equals: jsonDeepEquals },
	);

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
				set_route_error(decomposed.error);
				set_loaders_data(decomposed.loaders_data);
				set_client_loaders_data(decomposed.client_loaders_data);
				set_matched_patterns(decomposed.matched_patterns);
				set_import_urls(decomposed.import_urls);
				set_entry_keys(decomposed.entry_keys);

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

	function useRouteState(): Accessor<RouteState>;
	function useRouteState<T>(selector: (route: RouteState) => T): Accessor<T>;
	function useRouteState<T>(
		selector?: (route: RouteState) => T,
	): Accessor<RouteState | T> {
		return createMemo(
			() => {
				const route = route_state();
				if (!route) {
					throw new Error("Vorma not initialized");
				}
				if (selector) {
					return selector(route);
				}
				return route;
			},
			undefined,
			{ equals: jsonDeepEquals },
		);
	}

	function useWorkState(): Accessor<WorkState>;
	function useWorkState<T>(selector: (work: WorkState) => T): Accessor<T>;
	function useWorkState<T>(
		selector?: (work: WorkState) => T,
	): Accessor<WorkState | T> {
		return createMemo(
			() => {
				const work = work_state();
				if (selector) {
					return selector(work);
				}
				return work;
			},
			undefined,
			{ equals: jsonDeepEquals },
		);
	}

	function useLoaderData<P extends MakeTypedLoaderPattern<A>>(
		props: MakeTypedRouteProps<A, P>,
	): Accessor<MakeTypedLoaderOutput<A, P>> {
		return createMemo(() => {
			return loaders_data()[props.idx] as MakeTypedLoaderOutput<A, P>;
		});
	}

	function usePatternLoaderData<P extends MakeTypedLoaderPattern<A>>(
		pattern: P,
	): Accessor<MakeTypedLoaderOutput<A, P> | undefined> {
		return createMemo(() => {
			const patterns = matched_patterns();
			const idx = patterns.indexOf(pattern);
			if (idx < 0) {
				return undefined;
			}
			return loaders_data()[idx] as MakeTypedLoaderOutput<A, P>;
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
				route_error(),
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

	const App: Component = () => {
		return <RootOutlet />;
	};

	function init(
		options: AdapterInitOptions<Component>,
	): ReturnType<typeof core.init> {
		const { render, onRouteUpdate, onWorkUpdate, ...core_options } =
			options;
		return core.init({
			...core_options,
			onRouteUpdate: (route, previous_route, reason) => {
				set_route_state(route);
				onRouteUpdate?.(route, previous_route, reason);
			},
			onWorkUpdate: (work) => {
				set_work_state(work);
				onWorkUpdate?.(work);
			},
			render: render
				? () => {
						return render({ App, el: core.getRootEl() });
					}
				: undefined,
		});
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
		raw: Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
			MakeTypedLinkProps<A, P>,
	): JSX.Element => {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const { href, pattern, params, splatValues, search, hash, ...props } =
			merged;
		const link_href = createMemo(() => {
			return (
				href ??
				passthrough.buildHref({
					pattern,
					params,
					splatValues,
					search,
					hash,
				} as any)
			);
		});
		return <BaseLink {...props} href={link_href()} />;
	}) as <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
			MakeTypedLinkProps<A, P>,
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
