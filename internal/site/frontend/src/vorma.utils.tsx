import { createSignal } from "solid-js";
import { makeTypedNavigate } from "vorma/client";
import { addThemeChangeListener, getTheme } from "vorma/kit/theme";
import {
	makeTypedAddClientLoader,
	makeTypedLink,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedUseRouterData,
} from "vorma/solid";
import { vormaAppConfig, type RouteProps, type VormaApp } from "./vorma.gen.ts";

export type { RouteProps };
export const useRouterData = makeTypedUseRouterData<VormaApp>();
export const useLoaderData = makeTypedUseLoaderData<VormaApp>();
export const usePatternLoaderData = makeTypedUsePatternLoaderData<VormaApp>();
export const addClientLoader = makeTypedAddClientLoader<VormaApp>();
export const navigate = makeTypedNavigate(vormaAppConfig);
export const Link = makeTypedLink(vormaAppConfig, {
	prefetch: "intent",
});

/////////////////////////////////////////////////////////////////////
/////// THEME
/////////////////////////////////////////////////////////////////////

const [theme, set_theme_signal] = createSignal(getTheme());
addThemeChangeListener((e) => set_theme_signal(e.detail.theme));
export { theme };
