import { createSignal } from "solid-js";
import { makeTypedAPIClient, makeTypedNavigate } from "vorma/client";
import { addThemeChangeListener, getTheme } from "vorma/kit/theme";
import {
	makeTypedAddClientLoader,
	makeTypedLink,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedUseRouterData,
} from "vorma/solid";
import {
	vormaAppConfig,
	type RouteProps,
} from "../../__wave/vorma.gen/index.ts";

export type { RouteProps };

export const useRouterData = makeTypedUseRouterData(vormaAppConfig);
export const useLoaderData = makeTypedUseLoaderData(vormaAppConfig);
export const usePatternLoaderData =
	makeTypedUsePatternLoaderData(vormaAppConfig);
export const addClientLoader = makeTypedAddClientLoader(vormaAppConfig);
export const navigate = makeTypedNavigate(vormaAppConfig);
export const Link = makeTypedLink(vormaAppConfig, {
	prefetch: "intent",
});

const [theme, setThemeSignal] = createSignal(getTheme());
addThemeChangeListener((event) => setThemeSignal(event.detail.theme));
export { theme };

export const api = makeTypedAPIClient(vormaAppConfig, (ctx) => {
	// Edit this function to decorate request init values globally.
	// Return undefined when you do not want global decoration.
	if (ctx.type === "mutation") {
		return undefined;
	}
	return undefined;
});
