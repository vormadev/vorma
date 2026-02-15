// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { ConfigEnv, UserConfig } from "vite";
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

type PluginConfigResult = {
	build: {
		rollupOptions: {
			input: unknown;
		};
	};
	server: {
		watch: {
			ignored: unknown;
		};
	};
};

type PluginConfigHook = (
	config: UserConfig,
	env: ConfigEnv,
) =>
	| void
	| Omit<UserConfig, "plugins">
	| null
	| Promise<void | Omit<UserConfig, "plugins"> | null>;

function getPluginConfigHandler(): PluginConfigHook {
	const plugin = vormaVitePlugin(buildPluginConfig());
	const configHook = plugin.config;
	if (!configHook) {
		throw new Error("Expected Vorma Vite plugin to provide a config hook.");
	}
	if (typeof configHook === "function") {
		return configHook as PluginConfigHook;
	}
	return configHook.handler as PluginConfigHook;
}

async function invokePluginConfig(
	config: UserConfig,
	env: ConfigEnv,
): Promise<PluginConfigResult> {
	const configHandler = getPluginConfigHandler();
	const result = await configHandler(config, env);
	if (!result) {
		throw new Error(
			"Expected Vorma Vite config hook to return a config object.",
		);
	}
	return result as PluginConfigResult;
}

describe("vorma vite plugin config merge behavior", () => {
	it("keeps object rollup input entries while adding framework entries", async () => {
		const result = await invokePluginConfig(
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
			{ command: "build", mode: "test" },
		);

		expect(result.build.rollupOptions.input).toEqual({
			__vorma_internal_entry_0: "frontend/src/vorma.entry.tsx",
			__vorma_internal_entry_1: "frontend/src/admin.tsx",
			app: "frontend/src/app.ts",
			admin: "frontend/src/admin-entry.ts",
		});
	});

	it("avoids collisions when user object input already uses internal key names", async () => {
		const result = await invokePluginConfig(
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
			{ command: "build", mode: "test" },
		);

		expect(result.build.rollupOptions.input).toEqual({
			__vorma_internal_entry_0: "frontend/src/user-overlap-0.ts",
			__vorma_internal_entry_1: "frontend/src/user-overlap-1.ts",
			__vorma_internal_entry_2: "frontend/src/vorma.entry.tsx",
			__vorma_internal_entry_3: "frontend/src/admin.tsx",
			app: "frontend/src/app.ts",
		});
	});

	it("keeps string rollup input and appends framework entries", async () => {
		const result = await invokePluginConfig(
			{
				build: {
					rollupOptions: {
						input: "frontend/src/app.ts",
					},
				},
			},
			{ command: "build", mode: "test" },
		);

		expect(result.build.rollupOptions.input).toEqual([
			"frontend/src/vorma.entry.tsx",
			"frontend/src/admin.tsx",
			"frontend/src/app.ts",
		]);
	});

	it("keeps non-array watch ignored values and appends framework ignores", async () => {
		const stringIgnoredResult = await invokePluginConfig(
			{
				server: {
					watch: {
						ignored: "**/*.cache",
					},
				},
			},
			{ command: "serve", mode: "development" },
		);
		expect(stringIgnoredResult.server.watch.ignored).toEqual([
			"**/*.cache",
			"**/.DS_Store",
			"**/*~",
		]);

		const regexpIgnoredResult = await invokePluginConfig(
			{
				server: {
					watch: {
						ignored: /\\.swp$/,
					},
				},
			},
			{ command: "serve", mode: "development" },
		);
		const mergedIgnored = regexpIgnoredResult.server.watch.ignored;
		expect(Array.isArray(mergedIgnored)).toBe(true);
		if (!Array.isArray(mergedIgnored)) {
			throw new Error("Expected merged ignored patterns to be an array.");
		}
		expect(mergedIgnored[0]).toEqual(/\\.swp$/);
		expect(mergedIgnored.slice(1)).toEqual(["**/.DS_Store", "**/*~"]);
	});
});
