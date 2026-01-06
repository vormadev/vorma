import { getClientCookie, setClientCookie } from "vorma/kit/cookies";

/////////////////////////////////////////////////////////////////////
/////// SETUP
/////////////////////////////////////////////////////////////////////

export const THEMES = {
	Dark: "dark",
	Light: "light",
	System: "system",
} as const;

const THEME_VALUES = Object.values(THEMES);
const THEME_KEY = "kit_theme";
const RESOLVED_THEME_KEY = "kit_resolved_theme";
const PREFERS_DARK_QUERY = window.matchMedia("(prefers-color-scheme: dark)");
const CLASSLIST = window.document.documentElement.classList;
const THEME_CHANGE_EVENT_KEY = "theme_change";

export type Theme = (typeof THEME_VALUES)[number];
export type ResolvedTheme = Exclude<Theme, typeof THEMES.System>;
type ThemeChangeEventDetail = {
	theme: Theme;
	resolvedTheme: ResolvedTheme;
};

/////////////////////////////////////////////////////////////////////
///////// BROADCAST CHANNEL
/////////////////////////////////////////////////////////////////////

let bc = __newBC();
__setBCOnMessage(bc);

// bfcache stuff
window.addEventListener("pagehide", () => bc.close());
window.addEventListener("pageshow", () => {
	bc = __newBC();
	__setBCOnMessage(bc);
});

function __newBC(): BroadcastChannel {
	return new BroadcastChannel("kit_theme_channel");
}

function __setBCOnMessage(bc: BroadcastChannel) {
	bc.onmessage = (e) => {
		const detail = e.data as ThemeChangeEventDetail;
		const theme = detail.theme;
		const resolvedTheme = __getResolvedThemeFromTheme(theme);
		__setClassesAndDispatchEvent({ theme, resolvedTheme });
	};
}

/////////////////////////////////////////////////////////////////////
/////// PREFERS COLOR SCHEME EVENT LISTENER
/////////////////////////////////////////////////////////////////////

PREFERS_DARK_QUERY.addEventListener("change", () => {
	if (CLASSLIST.contains(THEMES.System)) {
		setTheme(THEMES.System);
	}
});

/////////////////////////////////////////////////////////////////////
/////// PUBLIC HELPER FUNCTIONS
/////////////////////////////////////////////////////////////////////

export function getTheme(): Theme {
	const theme = getClientCookie(THEME_KEY);
	return __isTheme(theme) ? theme : THEMES.System;
}

export function getThemeLocal(): Theme {
	const theme = localStorage.getItem(THEME_KEY);
	return __isTheme(theme) ? theme : THEMES.System;
}

export function getResolvedTheme(): ResolvedTheme {
	const resolvedTheme = getClientCookie(RESOLVED_THEME_KEY);
	return __isResolvedTheme(resolvedTheme) ? resolvedTheme : THEMES.Light;
}

export function getResolvedThemeLocal(): ResolvedTheme {
	const resolvedTheme = localStorage.getItem(RESOLVED_THEME_KEY);
	return __isResolvedTheme(resolvedTheme) ? resolvedTheme : THEMES.Light;
}

export function setTheme(theme: Theme) {
	const resolvedTheme = __getResolvedThemeFromTheme(theme);
	setClientCookie(THEME_KEY, theme);
	setClientCookie(RESOLVED_THEME_KEY, resolvedTheme);
	const detail: ThemeChangeEventDetail = { theme, resolvedTheme };
	__setClassesAndDispatchEvent(detail);
	bc.postMessage(detail);
}

export function setThemeLocal(theme: Theme) {
	const resolvedTheme = __getResolvedThemeFromTheme(theme);
	localStorage.setItem(THEME_KEY, theme);
	localStorage.setItem(RESOLVED_THEME_KEY, resolvedTheme);
	const detail: ThemeChangeEventDetail = { theme, resolvedTheme };
	__setClassesAndDispatchEvent(detail);
	bc.postMessage(detail);
}

export function addThemeChangeListener(
	listener: (e: CustomEvent<ThemeChangeEventDetail>) => void,
): CleanupFunction {
	window.addEventListener(THEME_CHANGE_EVENT_KEY, listener as EventListener);
	return () => {
		window.removeEventListener(
			THEME_CHANGE_EVENT_KEY,
			listener as EventListener,
		);
	};
}

type CleanupFunction = () => void;

export function initLocal() {
	const theme = localStorage.getItem(THEME_KEY) || THEMES.System;
	const resolvedTheme = __getResolvedThemeFromTheme(
		__isTheme(theme) ? theme : THEMES.System,
	);
	CLASSLIST.add(theme);
	if (theme === THEMES.System) {
		CLASSLIST.add(resolvedTheme);
	}
	localStorage.setItem(THEME_KEY, theme);
	localStorage.setItem(RESOLVED_THEME_KEY, resolvedTheme);
}

export function getNextToggleValue(theme: Theme): Theme {
	switch (theme) {
		case THEMES.System:
			return THEMES.Light;
		case THEMES.Light:
			return THEMES.Dark;
		case THEMES.Dark:
			return THEMES.System;
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL UTILS
/////////////////////////////////////////////////////////////////////

function __getResolvedThemeFromTheme(theme: Theme): ResolvedTheme {
	let resolvedTheme = theme;
	if (resolvedTheme === THEMES.System) {
		resolvedTheme = PREFERS_DARK_QUERY.matches ? THEMES.Dark : THEMES.Light;
	}
	return resolvedTheme;
}

function __setClassesAndDispatchEvent(detail: ThemeChangeEventDetail) {
	CLASSLIST.remove(...THEME_VALUES);
	CLASSLIST.add(detail.theme);
	if (detail.theme === THEMES.System) {
		CLASSLIST.add(detail.resolvedTheme);
	}
	window.dispatchEvent(new CustomEvent(THEME_CHANGE_EVENT_KEY, { detail }));
}

/////////////////////////////////////////////////////////////////////
/////// TYPE GUARDS
/////////////////////////////////////////////////////////////////////

function __isTheme(theme: string | undefined | null): theme is Theme {
	return THEME_VALUES.includes(theme as Theme);
}

function __isResolvedTheme(
	theme: string | undefined | null,
): theme is ResolvedTheme {
	return theme === THEMES.Dark || theme === THEMES.Light;
}
