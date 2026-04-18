/// <reference types="vite/client" />

import {
	batch,
	signal,
	untracked,
	useSignal,
	useSignalEffect,
} from "@preact/signals";
import { h, type ComponentType, type HTMLAttributes } from "preact";
import type { JSX } from "preact/jsx-runtime";
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
		Omit<HTMLAttributes<HTMLAnchorElement> & LinkPropsBase, "href">
	>;
	apiDecorator?: MakeTypedAPIDecorator<A>;
};

export function createVormaClient<A extends AppConfig>(
	app_config: A,
	options?: CreateVormaClientOptions<A>,
): VormaClient<
	A,
	JSX.Element,
	HTMLAttributes<HTMLAnchorElement>,
	false,
	ComponentType
> {
	const entries_signal = signal<DecomposedState["entries"]>([]);
	const route_error_signal = signal<DecomposedState["error"]>(null);
	const loaders_data_signal = signal<unknown[]>([]);
	const client_loaders_data_signal = signal<unknown[]>([]);
	const matched_patterns_signal = signal<string[]>([]);
	const route_state_signal = signal<RouteState | null>(null);
	const work_state_signal = signal<WorkState>({
		navigation: null,
		revalidation: null,
		prefetch: null,
		submissions: [],
	});

	function get_route_snapshot(): RouteState {
		const route = route_state_signal.value;
		if (!route) {
			throw new Error("Vorma not initialized");
		}
		return route;
	}

	function get_work_snapshot(): WorkState {
		return work_state_signal.value;
	}

	function use_signal_selector<State, Selected>(
		get_state: () => State,
		selector: (state: State) => Selected,
	): Selected {
		const selected_signal = useSignal({
			value: untracked(() => {
				return selector(get_state());
			}),
		});
		const render_selected = untracked(() => {
			return selector(get_state());
		});
		if (!jsonDeepEquals(selected_signal.peek().value, render_selected)) {
			selected_signal.value = {
				value: render_selected,
			};
		}

		useSignalEffect(() => {
			const selected = selector(get_state());
			if (jsonDeepEquals(selected_signal.peek().value, selected)) {
				return;
			}
			selected_signal.value = {
				value: selected,
			};
		});

		return selected_signal.value.value;
	}

	let pending_scroll_intent: ScrollIntent | undefined;

	const adapter_base_res = create_adapter_base(
		app_config,
		(decomposed: DecomposedState, scroll_intent?: ScrollIntent) => {
			if (scroll_intent) {
				pending_scroll_intent = scroll_intent;
			}

			batch(() => {
				entries_signal.value = decomposed.entries;
				route_error_signal.value = decomposed.error;
				loaders_data_signal.value = decomposed.loaders_data;
				client_loaders_data_signal.value =
					decomposed.client_loaders_data;
				matched_patterns_signal.value = decomposed.matched_patterns;
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
		return use_signal_selector(get_route_snapshot, select);
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
		return use_signal_selector(get_work_snapshot, select);
	}

	function useLoaderData<P extends MakeTypedLoaderPattern<A>>(
		props: MakeTypedRouteProps<A, P>,
	): MakeTypedLoaderOutput<A, P> {
		return loaders_data_signal.value[props.idx] as MakeTypedLoaderOutput<
			A,
			P
		>;
	}

	function usePatternLoaderData<P extends MakeTypedLoaderPattern<A>>(
		pattern: P,
	): MakeTypedLoaderOutput<A, P> | undefined {
		const patterns = matched_patterns_signal.value;
		const idx = patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return loaders_data_signal.value[idx] as MakeTypedLoaderOutput<A, P>;
	}

	function useClientLoaderData<P extends MakeTypedLoaderPattern<A>, T>(
		props: MakeTypedRouteProps<A, P, T>,
	): T {
		return client_loaders_data_signal.value[props.idx] as T;
	}

	function usePatternClientLoaderData<T>(
		pattern: MakeTypedLoaderPattern<A>,
	): T | undefined {
		const patterns = matched_patterns_signal.value;
		const idx = patterns.indexOf(pattern);
		if (idx < 0) {
			return undefined;
		}
		return client_loaders_data_signal.value[idx] as T;
	}

	function defineRoute<P extends MakeTypedLoaderPattern<A>, T = any>(
		input: MakeTypedDefineRouteInput<A, P, T, JSX.Element>,
	): RouteDefinition {
		return core.defineRoute(input as any);
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
				make_route_id(idx, entry.pattern) !==
				pending_scroll_intent.target_route_id
			) {
				return;
			}

			const scroll = pending_scroll_intent.scroll;
			pending_scroll_intent = undefined;

			apply_scroll(scroll);
		});

		const current_entries = entries_signal.value;
		const route_error = route_error_signal.value;

		const Outlet = (
			local?: Record<string, unknown>,
		): JSX.Element | null => {
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

	const App: ComponentType = () => {
		return h(RootOutlet, {});
	};

	function init(
		options: AdapterInitOptions<ComponentType>,
	): ReturnType<typeof core.init> {
		const { render, onRouteUpdate, onWorkUpdate, ...core_options } =
			options;
		return core.init({
			...core_options,
			onRouteUpdate: (route, previous_route, reason) => {
				route_state_signal.value = route;
				onRouteUpdate?.(route, previous_route, reason);
			},
			onWorkUpdate: (work) => {
				work_state_signal.value = work;
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
		props: HTMLAttributes<HTMLAnchorElement> &
			LinkPropsBase & { href: string },
	): JSX.Element {
		const r = make_link_props(
			props as unknown as Record<string, unknown>,
			nav_fns,
		);
		return h(
			"a",
			{
				"data-external": r.is_external || undefined,
				...(r.anchor_props as HTMLAttributes<HTMLAnchorElement>),
				onClick: r.onClick,
				onPointerDown: r.onPointerDown,
				onPointerEnter: r.onPointerEnter,
				onFocus: r.onFocus,
				onPointerLeave: r.onPointerLeave,
				onBlur: r.onBlur,
				onTouchCancel: r.onTouchCancel,
			},
			props.children,
		);
	}

	const Link = (<P extends MakeTypedLoaderPattern<A>>(
		raw: Omit<HTMLAttributes<HTMLAnchorElement>, "href"> &
			MakeTypedLinkProps<A, P>,
	): JSX.Element => {
		const merged = { ...options?.linkDefaultProps, ...raw } as any;
		const { href, pattern, params, splatValues, search, hash, ...props } =
			merged;
		return h(BaseLink, {
			...props,
			href:
				href ??
				passthrough.buildHref({
					pattern,
					params,
					splatValues,
					search,
					hash,
				} as any),
		});
	}) as <P extends MakeTypedLoaderPattern<A>>(
		props: Omit<HTMLAttributes<HTMLAnchorElement>, "href"> &
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
