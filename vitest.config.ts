import { resolve } from "node:path";
import { configDefaults, defineConfig } from "vitest/config";

const sourcePath = (relativePath: string): string =>
	resolve(process.cwd(), relativePath);

export default defineConfig({
	resolve: {
		alias: {
			"solid-js/web": sourcePath(
				"./node_modules/solid-js/web/dist/web.js",
			),
			"solid-js": sourcePath("./node_modules/solid-js/dist/solid.js"),
			"vorma/client/__internal": sourcePath(
				"./typescript/vorma/client/internal.ts",
			),
			"vorma/client": sourcePath("./typescript/vorma/client/index.ts"),
			"vorma/buildtime": sourcePath(
				"./typescript/vorma/client/buildtime.ts",
			),
			"vorma/react": sourcePath(
				"./typescript/vorma/ui-adapters/react/index.tsx",
			),
			"vorma/solid": sourcePath(
				"./typescript/vorma/ui-adapters/solid/index.tsx",
			),
			"vorma/preact": sourcePath(
				"./typescript/vorma/ui-adapters/preact/index.tsx",
			),
			"vorma/vite": sourcePath("./typescript/vorma/vite/vite.ts"),
			"vorma/kit/converters": sourcePath(
				"./typescript/kit/converters/converters.ts",
			),
			"vorma/kit/cookies": sourcePath(
				"./typescript/kit/cookies/cookies.ts",
			),
			"vorma/kit/csrf": sourcePath("./typescript/kit/csrf/csrf.ts"),
			"vorma/kit/debounce": sourcePath(
				"./typescript/kit/debounce/debounce.ts",
			),
			"vorma/kit/fmt": sourcePath("./typescript/kit/fmt/fmt.ts"),
			"vorma/kit/json": sourcePath("./typescript/kit/json/json.ts"),
			"vorma/kit/listeners": sourcePath(
				"./typescript/kit/listeners/listeners.ts",
			),
			"vorma/kit/matcher/register": sourcePath(
				"./typescript/kit/matcher/register.ts",
			),
			"vorma/kit/matcher/find-best": sourcePath(
				"./typescript/kit/matcher/find_best_match.ts",
			),
			"vorma/kit/matcher/find-nested": sourcePath(
				"./typescript/kit/matcher/find_nested_matches.ts",
			),
			"vorma/kit/theme": sourcePath("./typescript/kit/theme/theme.ts"),
			"vorma/kit/url": sourcePath("./typescript/kit/url/url.ts"),
		},
	},
	test: {
		environment: "jsdom",
		exclude: [...configDefaults.exclude, "e2e/**"],
	},
});
