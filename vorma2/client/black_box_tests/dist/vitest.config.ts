import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

function source_path(relative_path: string): string {
	return fileURLToPath(new URL(relative_path, import.meta.url));
}

export default defineConfig({
	resolve: {
		alias: {
			"solid-js/web": source_path(
				"../../../../node_modules/solid-js/web/dist/web.js",
			),
			"solid-js": source_path(
				"../../../../node_modules/solid-js/dist/solid.js",
			),
			"vorma/client": source_path(
				"../../../../npm_dist/vorma2/client/_index.js",
			),
			"vorma/client/__internal": source_path(
				"../../../../npm_dist/vorma2/client/_internal.js",
			),
			"vorma/client/__internal/hmr_dev": source_path(
				"../../../../npm_dist/vorma2/client/_hmr_dev.js",
			),
			"vorma/buildtime": source_path(
				"../../../../npm_dist/vorma2/client/_buildtime.js",
			),
			"vorma/testing": source_path(
				"../../../../npm_dist/vorma2/client/_testing.js",
			),
			"vorma/react": source_path(
				"../../../../npm_dist/vorma2/client/ui-adapters/react/_index.js",
			),
			"vorma/solid": source_path(
				"../../../../npm_dist/vorma2/client/ui-adapters/solid/_index.js",
			),
			"vorma/preact": source_path(
				"../../../../npm_dist/vorma2/client/ui-adapters/preact/_index.js",
			),
			"vorma/kit/converters": source_path(
				"../../../../npm_dist/typescript/kit/converters/converters.js",
			),
			"vorma/kit/cookies": source_path(
				"../../../../npm_dist/typescript/kit/cookies/cookies.js",
			),
			"vorma/kit/csrf": source_path(
				"../../../../npm_dist/typescript/kit/csrf/csrf.js",
			),
			"vorma/kit/debounce": source_path(
				"../../../../npm_dist/typescript/kit/debounce/debounce.js",
			),
			"vorma/kit/fmt": source_path(
				"../../../../npm_dist/typescript/kit/fmt/fmt.js",
			),
			"vorma/kit/json": source_path(
				"../../../../npm_dist/typescript/kit/json/json.js",
			),
			"vorma/kit/listeners": source_path(
				"../../../../npm_dist/typescript/kit/listeners/listeners.js",
			),
			"vorma/kit/matcher/register": source_path(
				"../../../../npm_dist/typescript/kit/matcher/register.js",
			),
			"vorma/kit/matcher/find-best": source_path(
				"../../../../npm_dist/typescript/kit/matcher/find_best_match.js",
			),
			"vorma/kit/matcher/find-nested": source_path(
				"../../../../npm_dist/typescript/kit/matcher/find_nested_matches.js",
			),
			"vorma/kit/matcher/utils": source_path(
				"../../../../npm_dist/typescript/kit/matcher/utils.js",
			),
			"vorma/kit/theme": source_path(
				"../../../../npm_dist/typescript/kit/theme/theme.js",
			),
			"vorma/kit/url": source_path(
				"../../../../npm_dist/typescript/kit/url/url.js",
			),
		},
	},
	test: {
		environment: "jsdom",
		include: ["vorma2/client/black_box_tests/dist/**/*.test.ts"],
		setupFiles: ["vorma2/client/black_box_tests/dist/setup.ts"],
	},
});
