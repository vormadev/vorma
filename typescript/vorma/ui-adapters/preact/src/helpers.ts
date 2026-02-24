/// <reference types="vite/client" />

import type { JSX } from "preact/jsx-runtime";
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
import { clientLoadersData, loadersData, routerData } from "./preact.tsx";

export type VormaRouteProps<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = string,
> = VormaRoutePropsGeneric<JSX.Element, App, Pattern>;

export type VormaRoute<
	App extends VormaAppBase = any,
	Pattern extends VormaLoaderPattern<App> = string,
> = VormaRouteGeneric<JSX.Element, App, Pattern>;

export function makeTypedUseRouterData<App extends VormaAppBase>() {
	return (() => {
		return routerData.value;
	}) as UseRouterDataFunction<App, false>;
}

export function makeTypedUseLoaderData<App extends VormaAppBase>() {
	return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRouteProps<App, Pattern>,
	): VormaLoaderOutput<App, Pattern> {
		return loadersData.value[props.idx] as VormaLoaderOutput<App, Pattern>;
	};
}

export function makeTypedUsePatternLoaderData<App extends VormaAppBase>() {
	return function usePatternLoaderData<
		Pattern extends VormaLoaderPattern<App>,
	>(pattern: Pattern): VormaLoaderOutput<App, Pattern> | undefined {
		return resolveTypedAdapterIndexedDataForPattern<
			VormaLoaderOutput<App, Pattern>
		>({
			pattern,
			matchedPatterns: routerData.value.matchedPatterns,
			indexedData: loadersData.value,
		});
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
			return resolveTypedAdapterIndexedDataForPatternOrRouteProps<Res>({
				pattern,
				matchedPatterns: routerData.value.matchedPatterns,
				indexedData: clientLoadersData.value,
				routeProps,
			});
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Res;
			(): Res | undefined;
		};
	};
}
