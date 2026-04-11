const languages = ["bash", "go", "json", "typescript", "css"];

const [highlight, ...promises] = await Promise.all([
	import("highlight.js/lib/core").then((x) => {
		return x.default;
	}),
	import("highlight.js/lib/languages/bash").then((x) => {
		return x.default;
	}),
	import("highlight.js/lib/languages/go").then((x) => {
		return x.default;
	}),
	import("highlight.js/lib/languages/json").then((x) => {
		return x.default;
	}),
	import("highlight.js/lib/languages/typescript").then((x) => {
		return x.default;
	}),
	import("highlight.js/lib/languages/css").then((x) => {
		return x.default;
	}),
]);

for (const lang of languages) {
	const first = promises.shift();
	if (!first) {
		break;
	}
	highlight.registerLanguage(lang, first);
}

export { htmlToMarkdown } from "mdream";
export { highlight };
