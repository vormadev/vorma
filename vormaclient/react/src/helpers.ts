/// <reference types="vite/client" />

import { useMemo, type JSX } from "react";
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

function useMatchedPatternIndex(pattern: string): number {
	const routerData = useRouterData();
	return useMemo(() => {
		return routerData.matchedPatterns.findIndex(
			(matchedPattern) => matchedPattern === pattern,
		);
	}, [routerData.matchedPatterns, pattern]);
}

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
		const idx = useMatchedPatternIndex(pattern);

		if (idx === -1) {
			return undefined;
		}
		return loadersData[idx];
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
			const clientLoadersData = useClientLoadersData();
			const matchedPatternIndex = useMatchedPatternIndex(p);

			const idx = useMemo(() => {
				if (props) {
					return props.idx;
				}
				return matchedPatternIndex;
			}, [props, matchedPatternIndex]);

			if (idx === -1) return undefined;
			return clientLoadersData[idx];
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Res;
			(): Res | undefined;
		};
	};
}
