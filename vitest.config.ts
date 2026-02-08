import { transformAsync } from "@babel/core";
import babelTsPreset from "@babel/preset-typescript";
import solidPreset from "babel-preset-solid";
import type { PluginOption } from "vite";
import { defineConfig } from "vitest/config";

const SOLID_SOURCE_TSX_RE =
	/\/internal\/framework\/_typescript\/solid\/src\/.*\.tsx$/;

function solidTsxTransformForVitest(): PluginOption {
	return {
		name: "solid-tsx-transform-for-vitest",
		enforce: "pre",
		async transform(code, id) {
			const sourcePath = id.split("?")[0]?.replace(/\\/g, "/");
			if (!sourcePath || !SOLID_SOURCE_TSX_RE.test(sourcePath)) {
				return null;
			}

			const result = await transformAsync(code, {
				filename: sourcePath,
				babelrc: false,
				configFile: false,
				sourceMaps: true,
				presets: [
					[solidPreset, { generate: "dom" }],
					[babelTsPreset, { isTSX: true, allExtensions: true }],
				],
			});

			if (!result?.code) {
				return null;
			}

			return {
				code: result.code,
				map: result.map ?? null,
			};
		},
	};
}

export default defineConfig({
	plugins: [solidTsxTransformForVitest()],
	resolve: {
		alias: [
			{
				find: /^solid-js$/,
				replacement: "solid-js/dist/solid.js",
			},
			{
				find: /^solid-js\/web$/,
				replacement: "solid-js/web/dist/web.js",
			},
		],
	},
	test: { environment: "jsdom" },
});
