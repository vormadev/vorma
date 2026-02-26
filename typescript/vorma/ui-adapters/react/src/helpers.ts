/// <reference types="vite/client" />

import { useMemo, type JSX } from "react";
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
import {
	useClientLoadersData,
	useLoadersData,
	useRouterData,
} from "./react.tsx";

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
	return useRouterData as UseRouterDataFunction<App, false>;
}

export function makeTypedUseLoaderData<C extends VormaAppConfig>(
	vormaAppConfig: C,
) {
	void vormaAppConfig;
	type App = ExtractApp<C>;
	return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRouteProps<App, Pattern>,
	): VormaLoaderOutput<App, Pattern> {
		const loadersData = useLoadersData();
		const clientLoadersData = useClientLoadersData();
		const routerData = useRouterData();
		return resolveTypedAdapterLoaderDataForRoutePropsOrThrow<
			VormaLoaderOutput<App, Pattern>
		>({
			routeProps: props,
			matchedPatterns: routerData.matchedPatterns,
			loadersData,
			clientLoadersData,
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
	>(pattern: Pattern): VormaLoaderOutput<App, Pattern> | undefined {
		const loadersData = useLoadersData();
		const routerData = useRouterData();

		return useMemo(() => {
			return resolveTypedAdapterIndexedDataForPattern<
				VormaLoaderOutput<App, Pattern>
			>({
				pattern,
				matchedPatterns: routerData.matchedPatterns,
				indexedData: loadersData,
			});
		}, [loadersData, pattern, routerData.matchedPatterns]);
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
		): Res | undefined => {
			const loadersData = useLoadersData();
			const clientLoadersData = useClientLoadersData();
			const routerData = useRouterData();
			return resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<Res>(
				{
					pattern,
					matchedPatterns: routerData.matchedPatterns,
					loadersData,
					clientLoadersData,
					routeProps,
				},
			);
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Res;
			(): Res | undefined;
		};
	};
}
