// @vitest-environment node

import { describe, expect, it } from "vitest";
import vormaVitePlugin from "./vite.ts";

function buildPluginConfig() {
	return {
		rollupInput: ["frontend/src/vorma.entry.tsx", "frontend/src/admin.tsx"],
		publicPathPrefix: "/static/",
		staticPublicAssetMap: {},
		buildtimePublicURLFuncName: "waveBuildtimeURL",
		filemapJSONPath: "backend/dist/static/vorma.gen/public_filemap.json",
		ignoredPatterns: ["**/.DS_Store", "**/*~"],
		dedupeList: ["react", "react-dom"],
	};
}

describe("vorma vite plugin config merge behavior", () => {
	it("keeps object rollup input entries while adding framework entries", () => {
		const plugin = vormaVitePlugin(buildPluginConfig());

		const result = plugin.config(
			{
				build: {
					rollupOptions: {
						input: {
							app: "frontend/src/app.ts",
							admin: "frontend/src/admin-entry.ts",
						},
					},
				},
			},
			{ command: "build" },
		);

		expect(result.build.rollupOptions.input).toEqual({
			__vorma_internal_entry_0: "frontend/src/vorma.entry.tsx",
			__vorma_internal_entry_1: "frontend/src/admin.tsx",
			app: "frontend/src/app.ts",
			admin: "frontend/src/admin-entry.ts",
		});
	});

	it("avoids collisions when user object input already uses internal key names", () => {
		const plugin = vormaVitePlugin(buildPluginConfig());

		const result = plugin.config(
			{
				build: {
					rollupOptions: {
						input: {
							__vorma_internal_entry_0:
								"frontend/src/user-overlap-0.ts",
							__vorma_internal_entry_1:
								"frontend/src/user-overlap-1.ts",
							app: "frontend/src/app.ts",
						},
					},
				},
			},
			{ command: "build" },
		);

		expect(result.build.rollupOptions.input).toEqual({
			__vorma_internal_entry_0: "frontend/src/user-overlap-0.ts",
			__vorma_internal_entry_1: "frontend/src/user-overlap-1.ts",
			__vorma_internal_entry_2: "frontend/src/vorma.entry.tsx",
			__vorma_internal_entry_3: "frontend/src/admin.tsx",
			app: "frontend/src/app.ts",
		});
	});

	it("keeps string rollup input and appends framework entries", () => {
		const plugin = vormaVitePlugin(buildPluginConfig());

		const result = plugin.config(
			{
				build: {
					rollupOptions: {
						input: "frontend/src/app.ts",
					},
				},
			},
			{ command: "build" },
		);

		expect(result.build.rollupOptions.input).toEqual([
			"frontend/src/vorma.entry.tsx",
			"frontend/src/admin.tsx",
			"frontend/src/app.ts",
		]);
	});

	it("keeps non-array watch ignored values and appends framework ignores", () => {
		const plugin = vormaVitePlugin(buildPluginConfig());

		const stringIgnoredResult = plugin.config(
			{
				server: {
					watch: {
						ignored: "**/*.cache",
					},
				},
			},
			{ command: "serve" },
		);
		expect(stringIgnoredResult.server.watch.ignored).toEqual([
			"**/*.cache",
			"**/.DS_Store",
			"**/*~",
		]);

		const regexpIgnoredResult = plugin.config(
			{
				server: {
					watch: {
						ignored: /\\.swp$/,
					},
				},
			},
			{ command: "serve" },
		);
		const mergedIgnored = regexpIgnoredResult.server.watch.ignored;
		expect(Array.isArray(mergedIgnored)).toBe(true);
		expect(mergedIgnored[0]).toEqual(/\\.swp$/);
		expect(mergedIgnored.slice(1)).toEqual(["**/.DS_Store", "**/*~"]);
	});
});
