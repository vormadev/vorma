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
	entryPoints: ["./typescript/vorma/ui-adapters/solid/index.ts"],
	external: ["vorma", "solid-js"],
	outdir: "./npm_dist/typescript/vorma/ui-adapters/solid",
	tsconfig: "./typescript/vorma/ui-adapters/solid/tsconfig.json",
});
