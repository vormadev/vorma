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

type ThemeStorageBackend = {
	get_item: (key: string) => string | null | undefined;
	set_item: (key: string, value: string) => void;
};

const cookie_theme_storage_backend: ThemeStorageBackend = {
	get_item: getClientCookie,
	set_item: setClientCookie,
};

const local_theme_storage_backend: ThemeStorageBackend = {
	get_item: (key: string) => localStorage.getItem(key),
	set_item: (key: string, value: string) => localStorage.setItem(key, value),
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
		__setClassesAndDispatchEvent({
			theme: detail.theme,
			resolvedTheme: __getResolvedThemeFromTheme(detail.theme),
		});
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
	return read_theme_or_default(cookie_theme_storage_backend);
}

export function getThemeLocal(): Theme {
	return read_theme_or_default(local_theme_storage_backend);
}

export function getResolvedTheme(): ResolvedTheme {
	return read_resolved_theme_or_default(cookie_theme_storage_backend);
}

export function getResolvedThemeLocal(): ResolvedTheme {
	return read_resolved_theme_or_default(local_theme_storage_backend);
}

export function setTheme(theme: Theme) {
	write_theme_and_broadcast(cookie_theme_storage_backend, theme);
}

export function setThemeLocal(theme: Theme) {
	write_theme_and_broadcast(local_theme_storage_backend, theme);
}

function write_theme_and_broadcast(storage_backend: ThemeStorageBackend, theme: Theme) {
	const resolved_theme = __getResolvedThemeFromTheme(theme);
	storage_backend.set_item(THEME_KEY, theme);
	storage_backend.set_item(RESOLVED_THEME_KEY, resolved_theme);
	const detail: ThemeChangeEventDetail = { theme, resolvedTheme: resolved_theme };
	__setClassesAndDispatchEvent(detail);
	bc.postMessage(detail);
}

export function addThemeChangeListener(
	listener: (e: CustomEvent<ThemeChangeEventDetail>) => void,
): CleanupFunction {
	window.addEventListener(THEME_CHANGE_EVENT_KEY, listener as EventListener);
	return () => {
		window.removeEventListener(THEME_CHANGE_EVENT_KEY, listener as EventListener);
	};
}

type CleanupFunction = () => void;

export function initLocal() {
	const theme = localStorage.getItem(THEME_KEY) || THEMES.System;
	const resolved_theme = __getResolvedThemeFromTheme(
		__isTheme(theme) ? theme : THEMES.System,
	);
	CLASSLIST.add(theme);
	if (theme === THEMES.System) {
		CLASSLIST.add(resolved_theme);
	}
	localStorage.setItem(THEME_KEY, theme);
	localStorage.setItem(RESOLVED_THEME_KEY, resolved_theme);
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
	let resolved_theme = theme;
	if (resolved_theme === THEMES.System) {
		resolved_theme = PREFERS_DARK_QUERY.matches ? THEMES.Dark : THEMES.Light;
	}
	return resolved_theme;
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

function read_theme_or_default(storage_backend: ThemeStorageBackend): Theme {
	const theme = storage_backend.get_item(THEME_KEY);
	return __isTheme(theme) ? theme : THEMES.System;
}

function read_resolved_theme_or_default(
	storage_backend: ThemeStorageBackend,
): ResolvedTheme {
	const resolved_theme = storage_backend.get_item(RESOLVED_THEME_KEY);
	return __isResolvedTheme(resolved_theme) ? resolved_theme : THEMES.Light;
}

function __isResolvedTheme(theme: string | undefined | null): theme is ResolvedTheme {
	return theme === THEMES.Dark || theme === THEMES.Light;
}
