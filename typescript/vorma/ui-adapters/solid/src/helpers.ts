/// <reference types="vite/client" />

import { createMemo, type Accessor } from "solid-js";
import type { JSX } from "solid-js/jsx-runtime";
import {
	type ExtractApp,
	type UseRouterDataFunction,
	type VormaAppBase,
	type VormaAppConfig,
	type VormaLoaderOutput,
	type VormaLoaderPattern,
	type VormaRouteGeneric,
	type VormaRoutePropsGeneric,
} from "vorma/client";
import {
	registerTypedAdapterClientLoader,
	resolveTypedAdapterClientLoaderDataForPatternOrRouteProps,
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterLoaderDataForRoutePropsOrThrow,
	type VormaTypedAdapterAddClientLoaderProps,
} from "vorma/client/__internal";
import { clientLoadersData, loadersData, routerData } from "./solid.tsx";

export type VormaRouteProps<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = string,
> = VormaRoutePropsGeneric<JSX.Element, App, Pattern>;

export type VormaRoute<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = string,
> = VormaRouteGeneric<JSX.Element, App, Pattern>;

export function makeTypedUseRouterData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return (() => routerData) as UseRouterDataFunction<App, true>;
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRouteProps<App, Pattern>,
	): Accessor<VormaLoaderOutput<App, Pattern>> {
		return createMemo<VormaLoaderOutput<App, Pattern>>(() => {
			return resolveTypedAdapterLoaderDataForRoutePropsOrThrow<
				VormaLoaderOutput<App, Pattern>
			>({
				routeProps: props,
				matchedPatterns: routerData().matchedPatterns,
				loadersData: loadersData(),
				clientLoadersData: clientLoadersData(),
			});
		});
	};
}

export function makeTypedUsePatternLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return function usePatternLoaderData<
		Pattern extends VormaLoaderPattern<App>,
	>(pattern: Pattern): Accessor<VormaLoaderOutput<App, Pattern> | undefined> {
		const loaderData = createMemo<
			VormaLoaderOutput<App, Pattern> | undefined
		>(() => {
			return resolveTypedAdapterIndexedDataForPattern<
				VormaLoaderOutput<App, Pattern>
			>({
				pattern,
				matchedPatterns: routerData().matchedPatterns,
				indexedData: loadersData(),
			});
		});
		return loaderData;
	};
}

export function makeTypedAddClientLoader<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return function addClientLoader<
		Pattern extends VormaLoaderPattern<App>,
		LoaderData extends VormaLoaderOutput<App, Pattern>,
		T = any,
	>(
		props: VormaTypedAdapterAddClientLoaderProps<
			App,
			Pattern,
			LoaderData,
			T
		>,
	) {
		const { pattern, clientLoader } =
			registerTypedAdapterClientLoader(props);
		type Res = Awaited<ReturnType<typeof clientLoader>>;

		const useClientLoaderData = (
			routeProps?: VormaRouteProps<App, Pattern>,
		): Accessor<Res | undefined> => {
			return createMemo<Res | undefined>(() => {
				return resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<Res>(
					{
						pattern,
						matchedPatterns: routerData().matchedPatterns,
						loadersData: loadersData(),
						clientLoadersData: clientLoadersData(),
						routeProps,
					},
				);
			});
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Accessor<Res>;
			(): Accessor<Res | undefined>;
		};
	};
}
