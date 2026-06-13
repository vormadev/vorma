/*
Inline the following script into your document <head> before any stylesheets:

```
<script>
{
	const key_prefix = "__vorma_kit"; // update as needed
	const key = `${key_prefix}_theme`;
	const resolved_key = `${key_prefix}_resolved_theme`;
	const themes = { System: "system", Dark: "dark", Light: "light" };
	const raw = localStorage.getItem(key) ?? themes.System;
	const theme = Object.values(themes).includes(raw) ? raw : themes.System;
	let resolved_theme = theme;
	if (resolved_theme === themes.System) {
		resolved_theme = window.matchMedia("(prefers-color-scheme: dark)").matches
			? themes.Dark
			: themes.Light;
	}
	document.documentElement.classList.add(theme);
	if (theme === themes.System) {
		document.documentElement.classList.add(resolved_theme);
	}
	localStorage.setItem(key, theme);
	localStorage.setItem(resolved_key, resolved_theme);
}
</script>
```

Or, if you happen to be using vorma on the backend, you can inject the
same script with `head.script([vorma::kit::theme::script(None)])` —
`None` uses the default key prefix; pass yours if you override it in
`initTheme()` (the two must match).
*/

export const THEMES = {
	Dark: "dark",
	Light: "light",
	System: "system",
} as const;

const DEFAULT_KEY_PREFIX = "__vorma_kit";
const THEME_VALUES = Object.values(THEMES);
const PREFERS_DARK_QUERY = window.matchMedia("(prefers-color-scheme: dark)");
const CLASSLIST = window.document.documentElement.classList;

export type Theme = (typeof THEME_VALUES)[number];
export type ResolvedTheme = Exclude<Theme, typeof THEMES.System>;
type ThemeChangeEventDetail = { theme: Theme; resolved_theme: ResolvedTheme };
type CleanupFunction = () => void;

export function initTheme(keyPrefix = DEFAULT_KEY_PREFIX) {
	const key = `${keyPrefix}_theme`;
	const resolved_key = `${keyPrefix}_resolved_theme`;
	const theme_change_event_key = `${keyPrefix}_theme_change`;

	let bc = new_bc();
	set_bc_on_msg(bc);

	// bfcache stuff
	window.addEventListener("pagehide", () => bc.close());
	window.addEventListener("pageshow", () => {
		bc = new_bc();
		set_bc_on_msg(bc);
	});

	PREFERS_DARK_QUERY.addEventListener("change", () => {
		if (CLASSLIST.contains(THEMES.System)) {
			setTheme(THEMES.System);
		}
	});

	function new_bc(): BroadcastChannel {
		return new BroadcastChannel(`${keyPrefix}_theme_channel`);
	}

	function set_bc_on_msg(bc: BroadcastChannel) {
		bc.onmessage = (e) => {
			const detail = e.data as ThemeChangeEventDetail;
			set_classes_and_dispatch_event({
				theme: detail.theme,
				resolved_theme: get_resolved_theme_from_theme(detail.theme),
			});
		};
	}

	function get_resolved_theme_from_theme(theme: Theme): ResolvedTheme {
		let resolved_theme = theme;
		if (resolved_theme === THEMES.System) {
			resolved_theme = PREFERS_DARK_QUERY.matches ? THEMES.Dark : THEMES.Light;
		}
		return resolved_theme;
	}

	function set_classes_and_dispatch_event(detail: ThemeChangeEventDetail) {
		CLASSLIST.remove(...THEME_VALUES);
		CLASSLIST.add(detail.theme);
		if (detail.theme === THEMES.System) {
			CLASSLIST.add(detail.resolved_theme);
		}
		window.dispatchEvent(new CustomEvent(theme_change_event_key, { detail }));
	}

	function getTheme(): Theme {
		const theme = localStorage.getItem(key) as any;
		return THEME_VALUES.includes(theme) ? theme : THEMES.System;
	}

	function getResolvedTheme(): ResolvedTheme {
		const resolved_theme = localStorage.getItem(resolved_key) as any;
		return resolved_theme === THEMES.Dark || resolved_theme === THEMES.Light
			? resolved_theme
			: THEMES.Light;
	}

	function setTheme(theme: Theme) {
		const resolved_theme = get_resolved_theme_from_theme(theme);
		localStorage.setItem(key, theme);
		localStorage.setItem(resolved_key, resolved_theme);
		const detail: ThemeChangeEventDetail = { theme, resolved_theme: resolved_theme };
		set_classes_and_dispatch_event(detail);
		bc.postMessage(detail);
	}

	function addThemeChangeListener(
		listener: (e: CustomEvent<ThemeChangeEventDetail>) => void,
	): CleanupFunction {
		window.addEventListener(theme_change_event_key, listener as EventListener);
		return () => {
			window.removeEventListener(theme_change_event_key, listener as EventListener);
		};
	}

	function getNextToggleValue(theme: Theme): Theme {
		switch (theme) {
			case THEMES.System:
				return THEMES.Light;
			case THEMES.Light:
				return THEMES.Dark;
			case THEMES.Dark:
				return THEMES.System;
		}
	}

	return {
		getTheme,
		getResolvedTheme,
		setTheme,
		addThemeChangeListener,
		getNextToggleValue,
	};
}
