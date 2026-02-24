/// <reference types="vite/client" />

import { createMemo, type Accessor } from "solid-js";
import type { JSX } from "solid-js/jsx-runtime";
import {
	type UseRouterDataFunction,
	type VormaAppBase,
	type VormaLoaderOutput,
	type VormaLoaderPattern,
	type VormaRouteGeneric,
	type VormaRoutePropsGeneric,
} from "vorma/client";
import {
	registerClientLoaderForAdapter,
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterIndexedDataForPatternOrRouteProps,
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

export function makeTypedUseRouterData<App extends VormaAppBase>() {
	return (() => routerData) as UseRouterDataFunction<App, true>;
}

export function makeTypedUseLoaderData<App extends VormaAppBase>() {
	return function useLoaderData<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRouteProps<App, Pattern>,
	): Accessor<VormaLoaderOutput<App, Pattern>> {
		return createMemo<VormaLoaderOutput<App, Pattern>>(() => {
			return loadersData()[props.idx] as VormaLoaderOutput<App, Pattern>;
		});
	};
}

export function makeTypedUsePatternLoaderData<App extends VormaAppBase>() {
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
		const pattern = props.pattern;
		const clientLoader = props.clientLoader;
		registerClientLoaderForAdapter({
			pattern: pattern as string,
			waitFn: clientLoader as any,
			reRunOnModuleChange: props.reRunOnModuleChange,
		});
		type Res = Awaited<ReturnType<typeof clientLoader>>;

		const useClientLoaderData = (
			routeProps?: VormaRouteProps<App, Pattern>,
		): Accessor<Res | undefined> => {
			return createMemo<Res | undefined>(() => {
				return resolveTypedAdapterIndexedDataForPatternOrRouteProps<Res>(
					{
						pattern,
						matchedPatterns: routerData().matchedPatterns,
						indexedData: clientLoadersData(),
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
