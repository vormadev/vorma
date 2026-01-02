/// <reference types="vite/client" />

import { useMemo } from "preact/hooks";
import type { JSX } from "preact/jsx-runtime";
import {
	__registerClientLoaderPattern,
	__runClientLoadersAfterHMRUpdate,
	__vormaClientGlobal,
	type ClientLoaderAwaitedServerData,
	type ParamsForPattern,
	type UseRouterDataFunction,
	type VormaAppBase,
	type VormaLoaderOutput,
	type VormaLoaderPattern,
	type VormaRouteGeneric,
	type VormaRoutePropsGeneric,
} from "vorma/client";
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
		return loadersData.value[props.idx];
	};
}

export function makeTypedUsePatternLoaderData<App extends VormaAppBase>() {
	return function usePatternLoaderData<
		Pattern extends VormaLoaderPattern<App>,
	>(pattern: Pattern): VormaLoaderOutput<App, Pattern> | undefined {
		const idx = useMemo(() => {
			return routerData.value.matchedPatterns.findIndex(
				(p) => p === pattern,
			);
		}, [pattern]);

		if (idx === -1) {
			return undefined;
		}
		return loadersData.value[idx];
	};
}

export function makeTypedAddClientLoader<App extends VormaAppBase>() {
	const m = __vormaClientGlobal.get("patternToWaitFnMap");
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

		__registerClientLoaderPattern(p as string).catch((error) => {
			console.error("Failed to register client loader pattern:", error);
		});
		(m as any)[p] = fn;

		if (import.meta.env.DEV && props.reRunOnModuleChange) {
			__runClientLoadersAfterHMRUpdate(props.reRunOnModuleChange, p);
		}

		type Res = Awaited<ReturnType<typeof fn>>;

		const useClientLoaderData = (
			props?: VormaRouteProps<App, Pattern>,
		): Res | undefined => {
			const idx = useMemo(() => {
				if (props) {
					return props.idx;
				}
				const matched = routerData.value.matchedPatterns;
				return matched.findIndex((pattern) => pattern === p);
			}, [props]);

			if (idx === -1) return undefined;
			return clientLoadersData.value[idx];
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Res;
			(): Res | undefined;
		};
	};
}
