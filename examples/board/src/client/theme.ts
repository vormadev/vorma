import { atom, getDefaultStore } from "jotai";
import { initTheme } from "vorma/kit/theme";

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
export const themeAtom = atom({
	theme: getTheme(),
	resolved: getResolvedTheme(),
});

addThemeChangeListener((e) => {
	getDefaultStore().set(themeAtom, {
		theme: e.detail.theme,
		resolved: e.detail.resolved_theme,
	});
});

export function toggleTheme() {
	setTheme(getNextToggleValue(getTheme()));
}
