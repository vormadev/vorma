/// <reference types="vite/client" />

import { useMemo, type JSX } from "react";
import {
	type UseRouterDataFunction,
	type VormaAppBase,
	type VormaLoaderOutput,
	type VormaLoaderPattern,
	type VormaRouteGeneric,
	type VormaRoutePropsGeneric,
} from "vorma/client";
import {
	registerTypedAdapterClientLoader,
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterIndexedDataForPatternOrRouteProps,
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

export function makeTypedUseRouterData<App extends VormaAppBase>() {
	return useRouterData as UseRouterDataFunction<App, false>;
}

export function makeTypedUseLoaderData<App extends VormaAppBase>() {
	return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRouteProps<App, Pattern>,
	): VormaLoaderOutput<App, Pattern> {
		const loadersData = useLoadersData();
		return loadersData[props.idx];
	};
}

export function makeTypedUsePatternLoaderData<App extends VormaAppBase>() {
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

export function makeTypedAddClientLoader<App extends VormaAppBase>() {
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
			const clientLoadersData = useClientLoadersData();
			const routerData = useRouterData();
			return resolveTypedAdapterIndexedDataForPatternOrRouteProps<Res>({
				pattern,
				matchedPatterns: routerData.matchedPatterns,
				indexedData: clientLoadersData,
				routeProps,
			});
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Res;
			(): Res | undefined;
		};
	};
}
