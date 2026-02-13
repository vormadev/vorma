import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

const sourcePath = (relativePath: string): string =>
	fileURLToPath(new URL(relativePath, import.meta.url));

export default defineConfig({
	resolve: {
		alias: {
			"solid-js/web": sourcePath(
				"../../node_modules/solid-js/web/dist/web.js",
			),
			"solid-js": sourcePath("../../node_modules/solid-js/dist/solid.js"),
			"vorma/client": sourcePath(
				"../../npm_dist/vormaclient/client/index.js",
			),
			"vorma/buildtime": sourcePath(
				"../../npm_dist/vormaclient/client/buildtime.js",
			),
			"vorma/react": sourcePath(
				"../../npm_dist/vormaclient/react/index.js",
			),
			"vorma/solid": sourcePath(
				"../../npm_dist/vormaclient/solid/index.js",
			),
			"vorma/preact": sourcePath(
				"../../npm_dist/vormaclient/preact/index.js",
			),
			"vorma/kit/converters": sourcePath(
				"../../npm_dist/kit/_typescript/converters/converters.js",
			),
			"vorma/kit/cookies": sourcePath(
				"../../npm_dist/kit/_typescript/cookies/cookies.js",
			),
			"vorma/kit/csrf": sourcePath(
				"../../npm_dist/kit/_typescript/csrf/csrf.js",
			),
			"vorma/kit/debounce": sourcePath(
				"../../npm_dist/kit/_typescript/debounce/debounce.js",
			),
			"vorma/kit/fmt": sourcePath(
				"../../npm_dist/kit/_typescript/fmt/fmt.js",
			),
			"vorma/kit/json": sourcePath(
				"../../npm_dist/kit/_typescript/json/json.js",
			),
			"vorma/kit/listeners": sourcePath(
				"../../npm_dist/kit/_typescript/listeners/listeners.js",
			),
			"vorma/kit/matcher/register": sourcePath(
				"../../npm_dist/kit/_typescript/matcher/register.js",
			),
			"vorma/kit/matcher/find-best": sourcePath(
				"../../npm_dist/kit/_typescript/matcher/find_best_match.js",
			),
			"vorma/kit/matcher/find-nested": sourcePath(
				"../../npm_dist/kit/_typescript/matcher/find_nested_matches.js",
			),
			"vorma/kit/theme": sourcePath(
				"../../npm_dist/kit/_typescript/theme/theme.js",
			),
			"vorma/kit/url": sourcePath(
				"../../npm_dist/kit/_typescript/url/url.js",
			),
		},
	},
	test: {
		environment: "jsdom",
		include: ["vormaclient/client/dist_tests/**/*.test.ts"],
	},
});
