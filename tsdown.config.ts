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
		entry: ["./packages/create-vorma/main.ts"],
		outDir: "./packages/create-vorma/.dist/",
		platform: "node",
		deps: {}, // We want to bundle "@clack/prompts" and its transitive deps
	},
	{
		...base,
		entry: ["./packages/vorma/core/_index.ts"],
		outDir: "./packages/vorma/.dist/core/",
	},
	{
		...base,
		entry: ["./packages/vorma/ui/react/react.tsx"],
		outDir: "./packages/vorma/.dist/ui/react/",
	},
	{
		...base,
		entry: ["./packages/vorma/ui/preact/preact.tsx"],
		outDir: "./packages/vorma/.dist/ui/preact/",
	},
	{
		...base,
		entry: ["./packages/vorma/ui/solid/solid.tsx"],
		outDir: "./packages/vorma/.dist/ui/solid/",
		plugins: [solid()],
	},
	{
		...base,
		entry: ["./packages/vorma/ui/remix/remix.tsx"],
		outDir: "./packages/vorma/.dist/ui/remix/",
	},
	{
		...base,
		entry: ["./packages/vorma/vite/vite.ts"],
		outDir: "./packages/vorma/.dist/vite/",
		platform: "node",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/converters/converters.ts"],
		outDir: "./packages/vorma/.dist/kit/converters/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/cookies/cookies.ts"],
		outDir: "./packages/vorma/.dist/kit/cookies/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/csrf/csrf.ts"],
		outDir: "./packages/vorma/.dist/kit/csrf/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/debounce/debounce.ts"],
		outDir: "./packages/vorma/.dist/kit/debounce/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/fmt/fmt.ts"],
		outDir: "./packages/vorma/.dist/kit/fmt/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/json/json.ts"],
		outDir: "./packages/vorma/.dist/kit/json/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/lab/design/core/core.ts"],
		outDir: "./packages/vorma/.dist/kit/lab/design/core/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/lab/design/components/remix/remix.ts"],
		outDir: "./packages/vorma/.dist/kit/lab/design/components/remix/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/listeners/listeners.ts"],
		outDir: "./packages/vorma/.dist/kit/listeners/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/result/result.ts"],
		outDir: "./packages/vorma/.dist/kit/result/",
	},
	{
		...base,
		entry: ["./packages/vorma/kit/theme/theme.ts"],
		outDir: "./packages/vorma/.dist/kit/theme/",
	},
]);
