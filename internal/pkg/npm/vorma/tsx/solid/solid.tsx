/// <reference types="vite/client" />

import {
	batch,
	createEffect,
	createMemo,
	createSignal,
	onCleanup,
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
	select_link_route_state,
	select_link_work_state,
	type AdapterInitOptions,
	type AppConfig,
	type DecomposedState,
	type RouteDefinition,
	type RouteState,
	type ScrollIntent,
	type ToAPIDecorator,
	type ToDefineRouteArgs,
	type ToLinkProps,
	type ToLoaderOutput,
	type ToLoaderPattern,
	type ToRouteComponentProps,
	type ToRouteSyncArgs,
	type VormaClient,
	type WorkState,
} from "vorma/__internal";
import { jsonDeepEquals } from "vorma/kit/json";
import type { LinkPropsBase } from "../../core/types.ts";

export { SubmitError } from "vorma/__internal";
export type {
	ActionKind,
	BeforeRouteCommitFn,
	BeforeRouteTransitionArgs,
	BeforeRouteYieldFn,
	ProgressIndicatorConfig,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
	SubmitResult,
	ToActionInput,
	ToActionKind,
	ToActionMethod,
	ToActionOutput,
	ToActionPattern,
	ToActionSubmitArgs,
	ToActionSubmitArgsByKind,
	ToActionSubmitError,
	ToActionSubmitOutput,
	ToAPIClient,
	ToAPIDecorator,
	ToAPIDecoratorContext,
	ToClientLoaderArgs,
	ToLinkProps,
	ToLoaderInput,
	ToLoaderOutput,
	ToLoaderPattern,
	ToNavigateArgs,
	ToNavigationTarget,
	ToRouteComponentProps,
	ToRouteDestination,
	ToRouteSyncArgs,
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
	apiDecorator?: ToAPIDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): VormaClient<
	A,
	JSX.Element,
	JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
	"accessor",
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

	function useRouteSync<P extends ToLoaderPattern<A>>(
		args: ToRouteSyncArgs<A, P>,
	): void {
		const {
			debounceMs = 0,
			enabled = true,
			replace = true,
			scrollToTop = false,
			...target
		} = args;
		const canonical_href = createMemo(() => {
			return passthrough.toHref(target as any);
		});

		createEffect(() => {
			if (!enabled) {
				return;
			}

			const href = canonical_href();
			const route_href = route_state()?.href;
			const pending_href = work_state().navigation?.href;
			if (href === route_href || href === pending_href) {
				return;
			}

			if (debounceMs > 0) {
				const timeout_id = window.setTimeout(() => {
					void passthrough.navigate({
						href,
						replace,
						scrollToTop,
					});
				}, debounceMs);
				onCleanup(() => {
					window.clearTimeout(timeout_id);
				});
				return;
			}

			void passthrough.navigate({
				href,
				replace,
				scrollToTop,
			});
		});
	}

	function useLoaderData<P extends ToLoaderPattern<A>>(
		args: ToRouteComponentProps<A, P>,
	): Accessor<ToLoaderOutput<A, P>> {
		return createMemo(() => {
			return loaders_data()[args.idx] as ToLoaderOutput<A, P>;
		});
	}

	function usePatternLoaderData<P extends ToLoaderPattern<A>>(
		pattern: P,
	): Accessor<ToLoaderOutput<A, P> | undefined> {
		return createMemo(() => {
			const patterns = matched_patterns();
			const idx = patterns.indexOf(pattern);
			if (idx < 0) {
				return undefined;
			}
			return loaders_data()[idx] as ToLoaderOutput<A, P>;
		});
	}

	function useClientLoaderData<P extends ToLoaderPattern<A>, T>(
		args: ToRouteComponentProps<A, P, T>,
	): Accessor<T> {
		return createMemo(() => {
			return client_loaders_data()[args.idx] as T;
		});
	}

	function usePatternClientLoaderData<T>(
		pattern: ToLoaderPattern<A>,
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

	function defineRoute<P extends ToLoaderPattern<A>, T = any>(
		input: ToDefineRouteArgs<A, P, T, JSX.Element>,
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

		const slot = createMemo(() => {
			return resolve_outlet_slot(
				entries(),
				route_error(),
				idx,
				core.get_default_error_boundary(),
			);
		});

		const entry_key_at_idx = createMemo(() => {
			const e = entries();
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
		props: JSX.AnchorHTMLAttributes<HTMLAnchorElement> &
			LinkPropsBase & { pattern?: string },
	): JSX.Element {
		const link_route_state = useRouteState(select_link_route_state);
		const link_work_state = useWorkState(select_link_work_state);
		const r = createMemo(() => {
			return make_link_props(
				props as Record<string, unknown>,
				nav_fns,
				link_route_state(),
				link_work_state(),
			);
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

	const Link = (<P extends ToLoaderPattern<A>>(
		raw: Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
			ToLinkProps<A, P>,
	): JSX.Element => {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const { href, pattern, params, splatValues, search, hash, ...props } =
			merged;
		const link_href = createMemo(() => {
			return (
				href ??
				passthrough.toHref({
					pattern,
					params,
					splatValues,
					search,
					hash,
				} as any)
			);
		});
		return <BaseLink {...props} pattern={pattern} href={link_href()} />;
	}) as <P extends ToLoaderPattern<A>>(
		props: Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> &
			ToLinkProps<A, P>,
	) => JSX.Element;

	return {
		...passthrough,
		init,
		defineRoute,
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
