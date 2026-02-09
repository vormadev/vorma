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
	entryPoints: ["./vormaclient/solid/index.tsx"],
	external: ["vorma", "solid-js"],
	outdir: "./npm_dist/vormaclient/solid",
	tsconfig: "./vormaclient/solid/tsconfig.json",
});
