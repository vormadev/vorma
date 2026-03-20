import { build } from "esbuild";
import { solidPlugin } from "esbuild-plugin-solid";
await build({
	plugins: [solidPlugin()],
	sourcemap: "linked",
	target: "esnext",
	format: "esm",
	treeShaking: true,
	splitting: true,
	write: true,
	bundle: true,
	entryPoints: ["./vorma2/client/ui-adapters/solid/_index.ts"],
	external: ["vorma", "solid-js"],
	outdir: "./npm_dist/vorma2/client/ui-adapters/solid",
	tsconfig: "./vorma2/client/ui-adapters/solid/tsconfig.json",
});
