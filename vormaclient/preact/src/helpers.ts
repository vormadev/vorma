/// <reference types="vite/client" />

import type { JSX } from "preact/jsx-runtime";
import {
	type ClientLoaderAwaitedServerData,
	type ParamsForPattern,
	type UseRouterDataFunction,
	type VormaAppBase,
	type VormaLoaderOutput,
	type VormaLoaderPattern,
	type VormaRouteGeneric,
	type VormaRoutePropsGeneric,
} from "vorma/client";
import { registerClientLoaderForAdapter } from "vorma/client/__internal";
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
		const idx = routerData.value.matchedPatterns.findIndex(
			(matchedPattern) => matchedPattern === pattern,
		);

		if (idx === -1) {
			return undefined;
		}
		return loadersData.value[idx] as VormaLoaderOutput<App, Pattern>;
	};
}

export function makeTypedAddClientLoader<App extends VormaAppBase>() {
	return function addClientLoader<
		Pattern extends VormaLoaderPattern<App>,
		LoaderData extends VormaLoaderOutput<App, Pattern>,
		T = any,
	>(props: {
		pattern: Pattern;
		clientLoader: (props: {
			params: Record<ParamsForPattern<App, Pattern>, string>;
			splatValues: string[];
			serverDataPromise: Promise<
				ClientLoaderAwaitedServerData<App["rootData"], LoaderData>
			>;
			signal: AbortSignal;
		}) => Promise<T>;
		reRunOnModuleChange?: ImportMeta;
	}) {
		const p = props.pattern;
		const fn = props.clientLoader;

		registerClientLoaderForAdapter({
			pattern: p as string,
			waitFn: fn as any,
			reRunOnModuleChange: props.reRunOnModuleChange,
		});

		type Res = Awaited<ReturnType<typeof fn>>;

		const useClientLoaderData = (
			props?: VormaRouteProps<App, Pattern>,
		): Res | undefined => {
			const idx = props
				? props.idx
				: routerData.value.matchedPatterns.findIndex(
						(matchedPattern) => matchedPattern === p,
					);

			if (idx === -1) return undefined;
			return clientLoadersData.value[idx] as Res | undefined;
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Res;
			(): Res | undefined;
		};
	};
}
