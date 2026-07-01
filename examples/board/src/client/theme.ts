import { atom, getDefaultStore } from "jotai";
import { initTheme, type ResolvedTheme, type Theme } from "vorma/kit/theme";

type ThemeAtomState = {
	theme: Theme;
	resolved: ResolvedTheme;
};

const {
	getTheme,
	getResolvedTheme,
	setTheme,
	getNextToggleValue,
	addThemeChangeListener,
} = initTheme();

/*
jotai owns the React-visible state; the kit owns storage, classes, and
cross-tab broadcast. The listener keeps the atom current for ANY source
of change — this tab, another tab, or the OS flipping its color scheme.
*/
export const theme_atom = atom<ThemeAtomState>({
	theme: getTheme(),
	resolved: getResolvedTheme(),
});

addThemeChangeListener((e) => {
	getDefaultStore().set(theme_atom, {
		theme: e.detail.theme,
		resolved: e.detail.resolved_theme,
	});
});

export function toggle_theme() {
	setTheme(getNextToggleValue(getTheme()));
}
