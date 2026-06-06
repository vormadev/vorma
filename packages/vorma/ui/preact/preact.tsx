/// <reference types="vite/client" />

import {
	batch,
	signal,
	untracked,
	useComputed,
	useSignal,
	useSignalEffect,
	type ReadonlySignal,
} from "@preact/signals";
import { h, type ComponentType, type HTMLAttributes } from "preact";
import type { JSX } from "preact/jsx-runtime";
import {
	apply_scroll,
	create_adapter_base,
	create_empty_work_state,
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

type CreateVormaClientOptions<A extends AppConfig> =
	AdapterClientOptions<ComponentType> & {
		linkDefaultProps?: Partial<
			Omit<HTMLAttributes<HTMLAnchorElement> & LinkPropsBase, "href">
		>;
		apiDecorator?: ToApiDecorator<A>;
	};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): VormaClient<A, JSX.Element, HTMLAttributes<HTMLAnchorElement>, "signal"> {
	const entries_signal = signal<DecomposedState["entries"]>([]);
	const route_error_signal = signal<DecomposedState["error"]>(null);
	const views_data_signal = signal<unknown[]>([]);
	const client_loaders_data_signal = signal<unknown[]>([]);
	const matched_patterns_signal = signal<string[]>([]);
	const route_state_signal = signal<RouteState | null>(null);
	const link_state_version_signal = signal(0);
	const work_state_signal = signal<WorkState>(create_empty_work_state());

	function get_route_snapshot(): RouteState {
		const route = route_state_signal.value;
		if (!route) {
			throw new Error("Vorma not booted");
		}
		return route;
	}

	function get_work_snapshot(): WorkState {
		return work_state_signal.value;
	}

	function use_computed_signal<T>(compute: () => T): ReadonlySignal<T> {
		const selected_signal = useSignal(
			untracked(() => {
				return compute();
			}),
		);
		const render_selected = untracked(() => {
			return compute();
		});
		if (!jsonDeepEquals(selected_signal.peek(), render_selected)) {
			selected_signal.value = render_selected;
		}

		useSignalEffect(() => {
			const selected = compute();
			if (jsonDeepEquals(selected_signal.peek(), selected)) {
				return;
			}
			selected_signal.value = selected;
		});

		return selected_signal;
	}

	let pending_scroll_intent: ScrollIntent | undefined;

	const adapter_base_res = create_adapter_base(
		app_config,
		(adapter_commit: DecomposedCommit) => {
			if (adapter_commit.scroll_intent) {
				pending_scroll_intent = adapter_commit.scroll_intent;
			}

			batch(() => {
				if (adapter_commit.state) {
					entries_signal.value = adapter_commit.state.entries;
					route_error_signal.value = adapter_commit.state.error;
					views_data_signal.value = adapter_commit.state.views_data;
					client_loaders_data_signal.value =
						adapter_commit.state.client_loaders_data;
					matched_patterns_signal.value = adapter_commit.state.matched_patterns;
				}
				if (adapter_commit.route) {
					route_state_signal.value = adapter_commit.route;
				}
				if (adapter_commit.link_state_version !== undefined) {
					link_state_version_signal.value = adapter_commit.link_state_version;
				}
				if (adapter_commit.work) {
					work_state_signal.value = adapter_commit.work;
				}
			});
		},
		options?.apiDecorator,
	);
	if (!adapter_base_res.ok) {
		throw new Error(`Failed to create Vorma client: ${adapter_base_res.err}`);
	}

	const { core, nav_fns, passthrough } = adapter_base_res.val;

	function useRouteState(): ReadonlySignal<RouteState>;
	function useRouteState<T>(selector: (route: RouteState) => T): ReadonlySignal<T>;
	function useRouteState<T>(
		selector?: (route: RouteState) => T,
	): ReadonlySignal<RouteState | T> {
		return use_computed_signal(() => {
			const route = get_route_snapshot();
			if (selector) {
				return selector(route);
			}
			return route;
		});
	}

	function useWorkState(): ReadonlySignal<WorkState>;
	function useWorkState<T>(selector: (work: WorkState) => T): ReadonlySignal<T>;
	function useWorkState<T>(
		selector?: (work: WorkState) => T,
	): ReadonlySignal<WorkState | T> {
		return use_computed_signal(() => {
			const work = get_work_snapshot();
			if (selector) {
				return selector(work);
			}
			return work;
		});
	}

	function useRouteSync<P extends ToViewPattern<A>>(args: ToRouteSyncArgs<A, P>): void {
		const {
			debounceMs = 0,
			enabled = true,
			replace = true,
			scrollToTop = false,
			...target
		} = args;
		const href_signal = use_computed_signal(() => {
			return passthrough.toHref(target as any);
		});

		useSignalEffect(() => {
			if (!enabled) {
				return;
			}

			const canonical_href = href_signal.value;
			const route_href = route_state_signal.value?.href;
			const pending_href = work_state_signal.value.navigation?.href;
			if (canonical_href === route_href || canonical_href === pending_href) {
				return;
			}

			if (debounceMs > 0) {
				const timeout_id = window.setTimeout(() => {
					void passthrough.navigate({
						href: canonical_href,
						replace,
						scrollToTop,
					});
				}, debounceMs);
				return () => {
					window.clearTimeout(timeout_id);
				};
			}

			void passthrough.navigate({
				href: canonical_href,
				replace,
				scrollToTop,
			});
		});
	}

	function useViewData<P extends ToViewPattern<A>>(
		args: ToViewComponentProps<A, P>,
	): ReadonlySignal<ToViewOutput<A, P>> {
		return use_computed_signal(() => {
			return views_data_signal.value[args.idx] as ToViewOutput<A, P>;
		});
	}

	function usePatternViewData<P extends ToViewPattern<A>>(
		pattern: P,
	): ReadonlySignal<ToViewOutput<A, P> | undefined> {
		return use_computed_signal(() => {
			const patterns = matched_patterns_signal.value;
			const idx = patterns.indexOf(pattern);
			if (idx < 0) {
				return undefined;
			}
			return views_data_signal.value[idx] as ToViewOutput<A, P>;
		});
	}

	function useClientLoaderData<P extends ToViewPattern<A>, T>(
		args: ToViewComponentProps<A, P, T>,
	): ReadonlySignal<T> {
		return use_computed_signal(() => {
			return client_loaders_data_signal.value[args.idx] as T;
		});
	}

	function usePatternClientLoaderData<T>(
		pattern: ToViewPattern<A>,
	): ReadonlySignal<T | undefined> {
		return use_computed_signal(() => {
			const patterns = matched_patterns_signal.value;
			const idx = patterns.indexOf(pattern);
			if (idx < 0) {
				return undefined;
			}
			return client_loaders_data_signal.value[idx] as T;
		});
	}

	function defineView<P extends ToViewPattern<A>, T = any>(
		input: ToDefineViewArgs<A, P, T, JSX.Element>,
	): ViewDefinition {
		return core.defineView(input as any);
	}

	function RootOutlet(
		props: { idx?: number } & Record<string, unknown>,
	): JSX.Element | null {
		const idx = props.idx ?? 0;

		useSignalEffect(() => {
			const _entries = entries_signal.value;
			if (!pending_scroll_intent) {
				return;
			}

			const entry = _entries[idx];
			if (!entry) {
				return;
			}

			if (
				make_entry_id(idx, entry.pattern) !==
				pending_scroll_intent.target_entry_id
			) {
				return;
			}

			const scroll = pending_scroll_intent.scroll;
			pending_scroll_intent = undefined;

			apply_scroll(scroll);
		});

		const current_entries = entries_signal.value;
		const route_error = route_error_signal.value;

		const Outlet = (local?: Record<string, unknown>): JSX.Element | null => {
			return h(RootOutlet, {
				...props,
				...local,
				idx: idx + 1,
			});
		};

		const slot = resolve_outlet_slot(
			current_entries,
			route_error,
			idx,
			core.get_default_error_boundary(),
		);

		switch (slot.kind) {
			case "empty":
				return null;

			case "pass_through": {
				const key =
					idx + 1 < current_entries.length
						? get_entry_key(current_entries[idx + 1]!)
						: `__empty_${idx + 1}`;
				return h(Outlet, { key });
			}

			case "error": {
				const Boundary = slot.boundary;
				return h(Boundary, { error: slot.error });
			}

			case "component": {
				const key = get_entry_key(current_entries[idx]!);
				const Comp = slot.component;
				return h(Comp, { key, idx, Outlet });
			}
		}
	}

	const RootOutletComponent: ComponentType = () => {
		return h(RootOutlet, {});
	};

	function boot(): ReturnType<typeof core.boot> {
		const {
			render: render_root,
			linkDefaultProps: _linkDefaultProps,
			apiDecorator: _apiDecorator,
			...core_options
		} = options ?? {};
		return core.boot({
			...core_options,
			render: render_root
				? () => {
						return render_root({
							RootOutlet: RootOutletComponent,
							rootEl: core.getRootEl(),
						});
					}
				: undefined,
		});
	}

	function BaseLink(
		props: HTMLAttributes<HTMLAnchorElement> &
			LinkPropsBase & { href: string; pattern?: string },
	): JSX.Element {
		const route_state = useRouteState(select_link_route_state);
		const work_state = useWorkState(select_link_work_state);
		const props_signal = useSignal(props);
		if (props_signal.peek() !== props) {
			props_signal.value = props;
		}
		const r = useComputed(() => {
			void link_state_version_signal.value;
			return make_link_props(
				props_signal.value as unknown as Record<string, unknown>,
				nav_fns,
				route_state.value,
				work_state.value,
			);
		});
		const link_props = r.value;
		return h(
			"a",
			{
				"data-external": link_props.is_external || undefined,
				...(link_props.anchor_props as HTMLAttributes<HTMLAnchorElement>),
				onClick: link_props.onClick,
				onPointerDown: link_props.onPointerDown,
				onPointerEnter: link_props.onPointerEnter,
				onFocus: link_props.onFocus,
				onPointerLeave: link_props.onPointerLeave,
				onBlur: link_props.onBlur,
				onTouchCancel: link_props.onTouchCancel,
			},
			props.children,
		);
	}

	const Link = (<P extends ToViewPattern<A>>(
		raw: Omit<HTMLAttributes<HTMLAnchorElement>, "href"> & ToLinkProps<A, P>,
	): JSX.Element => {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const { href, pattern, params, splatValues, search, hash, ...props } = merged;
		return h(BaseLink, {
			...props,
			pattern,
			href:
				href ??
				passthrough.toHref({
					pattern,
					params,
					splatValues,
					search,
					hash,
				} as any),
		});
	}) as <P extends ToViewPattern<A>>(
		props: Omit<HTMLAttributes<HTMLAnchorElement>, "href"> & ToLinkProps<A, P>,
	) => JSX.Element;

	return {
		...passthrough,
		boot,
		defineView,
		RootOutlet,
		Link,
		useRouteSync,
		useRouteState,
		useWorkState,
		useViewData,
		usePatternViewData,
		useClientLoaderData,
		usePatternClientLoaderData,
	};
}
