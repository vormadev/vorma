import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

const sourcePath = (relativePath: string): string =>
	fileURLToPath(new URL(relativePath, import.meta.url));

export default defineConfig({
	resolve: {
		alias: {
			"vorma/client/__internal": sourcePath(
				"./vormaclient/client/internal.ts",
			),
			"vorma/client": sourcePath("./vormaclient/client/index.ts"),
			"vorma/buildtime": sourcePath("./vormaclient/client/buildtime.ts"),
			"vorma/react": sourcePath("./vormaclient/react/index.ts"),
			"vorma/solid": sourcePath("./vormaclient/solid/index.ts"),
			"vorma/preact": sourcePath("./vormaclient/preact/index.ts"),
			"vorma/vite": sourcePath("./vormaclient/vite/vite.ts"),
			"vorma/kit/converters": sourcePath(
				"./kit/_typescript/converters/converters.ts",
			),
			"vorma/kit/cookies": sourcePath(
				"./kit/_typescript/cookies/cookies.ts",
			),
			"vorma/kit/csrf": sourcePath("./kit/_typescript/csrf/csrf.ts"),
			"vorma/kit/debounce": sourcePath(
				"./kit/_typescript/debounce/debounce.ts",
			),
			"vorma/kit/fmt": sourcePath("./kit/_typescript/fmt/fmt.ts"),
			"vorma/kit/json": sourcePath("./kit/_typescript/json/json.ts"),
			"vorma/kit/listeners": sourcePath(
				"./kit/_typescript/listeners/listeners.ts",
			),
			"vorma/kit/matcher/register": sourcePath(
				"./kit/_typescript/matcher/register.ts",
			),
			"vorma/kit/matcher/find-best": sourcePath(
				"./kit/_typescript/matcher/find_best_match.ts",
			),
			"vorma/kit/matcher/find-nested": sourcePath(
				"./kit/_typescript/matcher/find_nested_matches.ts",
			),
			"vorma/kit/theme": sourcePath("./kit/_typescript/theme/theme.ts"),
			"vorma/kit/url": sourcePath("./kit/_typescript/url/url.ts"),
		},
	},
	test: { environment: "jsdom" },
});
