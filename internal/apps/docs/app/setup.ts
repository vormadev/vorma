const langKeys = ["bash", "go", "json", "typescript", "css"];

const [highlightMod, ...langMods] = await Promise.all([
	import("highlight.js/lib/core"),
	import("highlight.js/lib/languages/bash"),
	import("highlight.js/lib/languages/go"),
	import("highlight.js/lib/languages/json"),
	import("highlight.js/lib/languages/typescript"),
	import("highlight.js/lib/languages/css"),
]);

const highlight = highlightMod.default;

langKeys.forEach((k, i) => {
	const mod = langMods[i];
	if (!mod) {
		throw new Error(`Failed to load highlight.js language module for ${k}`);
	}
	highlight.registerLanguage(k, mod.default);
});

export { htmlToMarkdown } from "mdream";
export { highlight };
