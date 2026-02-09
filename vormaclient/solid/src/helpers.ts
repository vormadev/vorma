/// <reference types="vite/client" />

import { createMemo, type Accessor } from "solid-js";
import type { JSX } from "solid-js/jsx-runtime";
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
		return createMemo(() => {
			return loadersData()[props.idx];
		});
	};
}

export function makeTypedUsePatternLoaderData<App extends VormaAppBase>() {
	return function usePatternLoaderData<
		Pattern extends VormaLoaderPattern<App>,
	>(pattern: Pattern): Accessor<VormaLoaderOutput<App, Pattern> | undefined> {
		const idx = createMemo(() => {
			const matchedPatterns = routerData().matchedPatterns;
			return matchedPatterns.findIndex((p) => p === pattern);
		});
		const loaderData = createMemo(() => {
			const index = idx();
			if (index === -1) {
				return undefined;
			}
			return loadersData()[index];
		});
		return loaderData;
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
		): Accessor<Res | undefined> => {
			return createMemo(() => {
				if (props) {
					return clientLoadersData()[props.idx];
				}
				const matched = routerData().matchedPatterns;
				const idx = matched.findIndex((pattern) => pattern === p);
				if (idx === -1) return undefined;
				return clientLoadersData()[idx];
			});
		};

		return useClientLoaderData as {
			(props: VormaRouteProps<App, Pattern>): Accessor<Res>;
			(): Accessor<Res | undefined>;
		};
	};
}
