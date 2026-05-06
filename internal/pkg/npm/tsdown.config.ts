import { type UserConfig, defineConfig } from "tsdown";
import solid from "unplugin-solid/rolldown";

const base = {
	sourcemap: true,
	platform: "browser",
	format: "esm",
	deps: { skipNodeModulesBundle: true },
	dts: { sourcemap: true },
} as const satisfies UserConfig;

export default defineConfig([
	{
		...base,
		entry: ["./vorma/create/main.ts"],
		outDir: "./vorma/create/.dist/",
		platform: "node",
		deps: {}, // We want to bundle "@clack/prompts" and its transitive deps
	},
	{
		...base,
		entry: ["./vorma/core/_index.ts"],
		outDir: "./.dist/vorma/core/",
	},
	{
		...base,
		entry: ["./vorma/tsx/react/react.tsx"],
		outDir: "./.dist/vorma/tsx/react/",
	},
	{
		...base,
		entry: ["./vorma/tsx/preact/preact.tsx"],
		outDir: "./.dist/vorma/tsx/preact/",
	},
	{
		...base,
		entry: ["./vorma/tsx/solid/solid.tsx"],
		outDir: "./.dist/vorma/tsx/solid/",
		plugins: [solid()],
	},
	{
		...base,
		entry: ["./vorma/tsx/remix/remix.tsx"],
		outDir: "./.dist/vorma/tsx/remix/",
	},
	{
		...base,
		entry: ["./vorma/vite/vite.ts"],
		outDir: "./.dist/vorma/vite/",
		platform: "node",
	},
	{
		...base,
		entry: ["./kit/converters/converters.ts"],
		outDir: "./.dist/kit/converters/",
	},
	{
		...base,
		entry: ["./kit/cookies/cookies.ts"],
		outDir: "./.dist/kit/cookies/",
	},
	{
		...base,
		entry: ["./kit/csrf/csrf.ts"],
		outDir: "./.dist/kit/csrf/",
	},
	{
		...base,
		entry: ["./kit/debounce/debounce.ts"],
		outDir: "./.dist/kit/debounce/",
	},
	{
		...base,
		entry: ["./kit/fmt/fmt.ts"],
		outDir: "./.dist/kit/fmt/",
	},
	{
		...base,
		entry: ["./kit/json/json.ts"],
		outDir: "./.dist/kit/json/",
	},
	{
		...base,
		entry: ["./kit/lab/design/core/core.ts"],
		outDir: "./.dist/kit/lab/design/core/",
	},
	{
		...base,
		entry: ["./kit/listeners/listeners.ts"],
		outDir: "./.dist/kit/listeners/",
	},
	{
		...base,
		entry: ["./kit/matcher/matcher.ts"],
		outDir: "./.dist/kit/matcher/",
	},
	{
		...base,
		entry: ["./kit/result/result.ts"],
		outDir: "./.dist/kit/result/",
	},
	{
		...base,
		entry: ["./kit/theme/theme.ts"],
		outDir: "./.dist/kit/theme/",
	},
	{
		...base,
		entry: ["./kit/url/url.ts"],
		outDir: "./.dist/kit/url/",
	},
]);
