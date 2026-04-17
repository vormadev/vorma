/// <reference types="vite/client" />

import { batch, effect, signal } from "@preact/signals";
import { h, type ComponentType, type HTMLAttributes } from "preact";
import { useEffect, useMemo, useRef } from "preact/hooks";
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
	type MakeTypedRouterData,
	type RouteDefinition,
	type RouteEntry,
	type ScrollIntent,
	type VormaClient,
} from "vorma/__internal";

export type {
	Href,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	MakeTypedClientLoaderProps,
	MakeTypedLinkProps,
	MakeTypedLoaderInput,
	MakeTypedLoaderOutput,
	MakeTypedLoaderPattern,
	MakeTypedMutationInput,
	MakeTypedMutationOutput,
	MakeTypedMutationPattern,
	MakeTypedMutationProps,
	MakeTypedNavigateOptions,
	MakeTypedQueryInput,
	MakeTypedQueryOutput,
	MakeTypedQueryPattern,
	MakeTypedQueryProps,
	MakeTypedRouteDestination,
	MakeTypedRouteProps,
	MakeTypedRouterData,
	MakeTypedRouteTarget,
	ProgressIndicatorConfig,
	RevalidationResult,
	SubmitOptions,
	SubmitResult,
	AppConfig as VormaAppConfig,
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
	const entries_signal = signal<RouteEntry[]>([]);
	const loaders_data_signal = signal<unknown[]>([]);
	const client_loaders_data_signal = signal<unknown[]>([]);
	const matched_patterns_signal = signal<string[]>([]);
	const import_urls_signal = signal<string[]>([]);
	const entry_keys_signal = signal<string[]>([]);
	const params_signal = signal<Record<string, string>>({});
	const splat_values_signal = signal<string[]>([]);
	const build_id_signal = signal("");
	const history_state_signal = signal<unknown>(undefined);

	let pending_scroll_intent: ScrollIntent | undefined;

	const adapter_base_res = create_adapter_base(
		app_config,
		(decomposed: DecomposedState, scroll_intent?: ScrollIntent) => {
			if (scroll_intent) {
				pending_scroll_intent = scroll_intent;
			}

			batch(() => {
				entries_signal.value = decomposed.entries;
				loaders_data_signal.value = decomposed.loaders_data;
				client_loaders_data_signal.value =
					decomposed.client_loaders_data;
				matched_patterns_signal.value = decomposed.matched_patterns;
				import_urls_signal.value = decomposed.import_urls;
				entry_keys_signal.value = decomposed.entry_keys;
				params_signal.value = decomposed.params;
				splat_values_signal.value = decomposed.splat_values;
				build_id_signal.value = decomposed.client_build_id;
				history_state_signal.value = decomposed.history_state;
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

	function useRouterData(): MakeTypedRouterData<A>;
	function useRouterData<P extends MakeTypedLoaderPattern<A>>(
		routeProps: MakeTypedRouteProps<A, P>,
	): MakeTypedRouterData<A, P>;
	function useRouterData(_routeProps?: any): any {
		return {
			clientBuildID: build_id_signal.value,
			matchedPatterns: matched_patterns_signal.value,
			splatValues: splat_values_signal.value,
			params: params_signal.value,
			historyState: history_state_signal.value,
			rootData:
				matched_patterns_signal.value[0] === "/"
					? loaders_data_signal.value[0]
					: undefined,
		};
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
		const ref = useRef(props);
		ref.current = props;

		const current_import_url = signal(import_urls_signal.value[idx]);
		const current_entry_key = signal(entry_keys_signal.value[idx]);
		const next_import_url = signal(import_urls_signal.value[idx + 1]);
		const next_entry_key = signal(entry_keys_signal.value[idx + 1]);

		useEffect(() => {
			const dispose = effect(() => {
				const urls = import_urls_signal.value;
				const keys = entry_keys_signal.value;

				batch(() => {
					const new_url = urls[idx];
					const new_key = keys[idx];
					if (current_import_url.value !== new_url) {
						current_import_url.value = new_url;
					}
					if (current_entry_key.value !== new_key) {
						current_entry_key.value = new_key;
					}

					const new_next_url = urls[idx + 1];
					const new_next_key = keys[idx + 1];
					if (next_import_url.value !== new_next_url) {
						next_import_url.value = new_next_url;
					}
					if (next_entry_key.value !== new_next_key) {
						next_entry_key.value = new_next_key;
					}
				});
			});
			return dispose;
		}, [idx]);

		useEffect(() => {
			if (!pending_scroll_intent) {
				return;
			}

			const current_entries = entries_signal.value;
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

			apply_scroll(scroll);
		});

		const current_entries = entries_signal.value;

		const Outlet = useMemo(() => {
			return (local?: Record<string, unknown>) => {
				return h(RootOutlet, {
					...ref.current,
					...local,
					idx: idx + 1,
				});
			};
		}, [idx, next_import_url.value, next_entry_key.value]);

		const slot = resolve_outlet_slot(
			current_entries,
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
				// Subscribe to per-instance tracking signals.
				const _url = current_import_url.value;
				const _key = current_entry_key.value;

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
		const { render, ...core_options } = options;
		return core.init({
			...core_options,
			render: render
				? () => {
						return render({ App, el: core.getRootEl() });
					}
				: undefined,
		});
	}

	function BaseLink(
		props: HTMLAttributes<HTMLAnchorElement> & LinkPropsBase,
	): JSX.Element {
		const r = make_link_props(props as Record<string, unknown>, nav_fns);
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
		const { href: target, ...link_props } = merged;
		const href =
			typeof target === "string" ? target : passthrough.toHref(target);
		return h(BaseLink, { ...link_props, href });
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
		useLoaderData,
		usePatternLoaderData,
		useRouterData,
		useClientLoaderData,
		usePatternClientLoaderData,
	};
}
