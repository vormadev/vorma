import { makeTypedAPIClient, makeTypedNavigate } from "vorma/client";
import {
	makeTypedAddClientLoader,
	makeTypedLink,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedUseRouterData,
} from "vorma/preact";
import { vormaAppConfig } from "../gen/vorma/index.ts";

export const Link = makeTypedLink(vormaAppConfig, { prefetch: "intent" });
export const navigate = makeTypedNavigate(vormaAppConfig);

export const useRouterData = makeTypedUseRouterData(vormaAppConfig);
export const useLoaderData = makeTypedUseLoaderData(vormaAppConfig);
export const usePatternLoaderData =
	makeTypedUsePatternLoaderData(vormaAppConfig);
export const addClientLoader = makeTypedAddClientLoader(vormaAppConfig);

export const api = makeTypedAPIClient(vormaAppConfig, (ctx) => {
	// Edit this function to decorate request init values globally.
	// Return undefined when you do not want global decoration.
	if (ctx.type === "mutation") {
		return undefined;
	}
	return undefined;
});
