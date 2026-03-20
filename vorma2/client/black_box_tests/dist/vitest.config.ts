import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

const sourcePath = (relativePath: string): string =>
	fileURLToPath(new URL(relativePath, import.meta.url));

export default defineConfig({
	resolve: {
		alias: {
			"solid-js/web": sourcePath(
				"../../../../node_modules/solid-js/web/dist/web.js",
			),
			"solid-js": sourcePath(
				"../../../../node_modules/solid-js/dist/solid.js",
			),
			"vorma/client": sourcePath(
				"../../../../npm_dist/vorma2/client/_index.js",
			),
			"vorma/client/__internal": sourcePath(
				"../../../../npm_dist/vorma2/client/_internal.js",
			),
			"vorma/client/__internal/hmr_dev": sourcePath(
				"../../../../npm_dist/vorma2/client/_hmr_dev.js",
			),
			"vorma/buildtime": sourcePath(
				"../../../../npm_dist/vorma2/client/_buildtime.js",
			),
			"vorma/testing": sourcePath(
				"../../../../npm_dist/vorma2/client/_testing.js",
			),
			"vorma/react": sourcePath(
				"../../../../npm_dist/vorma2/client/ui-adapters/react/_index.js",
			),
			"vorma/solid": sourcePath(
				"../../../../npm_dist/vorma2/client/ui-adapters/solid/_index.js",
			),
			"vorma/preact": sourcePath(
				"../../../../npm_dist/vorma2/client/ui-adapters/preact/_index.js",
			),
			"vorma/kit/converters": sourcePath(
				"../../../../npm_dist/typescript/kit/converters/converters.js",
			),
			"vorma/kit/cookies": sourcePath(
				"../../../../npm_dist/typescript/kit/cookies/cookies.js",
			),
			"vorma/kit/csrf": sourcePath(
				"../../../../npm_dist/typescript/kit/csrf/csrf.js",
			),
			"vorma/kit/debounce": sourcePath(
				"../../../../npm_dist/typescript/kit/debounce/debounce.js",
			),
			"vorma/kit/fmt": sourcePath(
				"../../../../npm_dist/typescript/kit/fmt/fmt.js",
			),
			"vorma/kit/json": sourcePath(
				"../../../../npm_dist/typescript/kit/json/json.js",
			),
			"vorma/kit/listeners": sourcePath(
				"../../../../npm_dist/typescript/kit/listeners/listeners.js",
			),
			"vorma/kit/matcher/register": sourcePath(
				"../../../../npm_dist/typescript/kit/matcher/register.js",
			),
			"vorma/kit/matcher/find-best": sourcePath(
				"../../../../npm_dist/typescript/kit/matcher/find_best_match.js",
			),
			"vorma/kit/matcher/find-nested": sourcePath(
				"../../../../npm_dist/typescript/kit/matcher/find_nested_matches.js",
			),
			"vorma/kit/matcher/utils": sourcePath(
				"../../../../npm_dist/typescript/kit/matcher/utils.js",
			),
			"vorma/kit/theme": sourcePath(
				"../../../../npm_dist/typescript/kit/theme/theme.js",
			),
			"vorma/kit/url": sourcePath(
				"../../../../npm_dist/typescript/kit/url/url.js",
			),
		},
	},
	test: {
		environment: "jsdom",
		include: ["typescript/vorma/black_box_tests/dist/**/*.test.ts"],
	},
});
