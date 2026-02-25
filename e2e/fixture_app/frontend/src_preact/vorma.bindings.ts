import { getStatus, makeTypedAPIClient, makeTypedNavigate } from "vorma/client";
import {
	makeTypedAddClientLoader,
	makeTypedLink,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedUseRouterData,
} from "vorma/preact";
import { vormaAppConfig } from "./vorma.gen/index.ts";

export const Link = makeTypedLink(vormaAppConfig, { prefetch: "intent" });
export const navigate = makeTypedNavigate(vormaAppConfig);
export const useRouterData = makeTypedUseRouterData(vormaAppConfig);
export const useLoaderData = makeTypedUseLoaderData(vormaAppConfig);
export const usePatternLoaderData =
	makeTypedUsePatternLoaderData(vormaAppConfig);
export const addClientLoader = makeTypedAddClientLoader(vormaAppConfig);

export const api = makeTypedAPIClient(vormaAppConfig, (ctx) => {
	if (ctx.type === "mutation") {
		return undefined;
	}
	return undefined;
});

export const uiAdapterName = "preact";

export type E2EBridge = {
	api: {
		mutate: (props: any) => Promise<any>;
	};
	navigate: (props: any) => Promise<any>;
	getStatus: typeof getStatus;
};

declare global {
	interface Window {
		__vormaE2EBridge?: E2EBridge;
	}
}

export function installE2EBridge(): void {
	if (typeof window === "undefined") {
		return;
	}
	window.__vormaE2EBridge = {
		api,
		navigate,
		getStatus,
	};
}
