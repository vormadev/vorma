// @vitest-environment node

import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { ConfigEnv, Plugin, UserConfig } from "vite";
import { describe, expect, it } from "vitest";
import vormaVitePlugin from "./vite.ts";

function createCanonicalPublicFileMapFixture(map: Record<string, string>): {
	distDir: string;
} {
	const fixtureRoot = mkdtempSync(join(tmpdir(), "vorma-vite-plugin-test-"));
	const distDir = join(fixtureRoot, "backend", "dist");
	const staticPublicOutDir = join(distDir, "static", "assets", "public");
	const staticInternalOutDir = join(distDir, "static", "internal");
	mkdirSync(staticPublicOutDir, { recursive: true });
	mkdirSync(staticInternalOutDir, { recursive: true });

	const canonicalPublicFileMapPath =
		"wave_out_wave_owned_public_filemap_test.json";
	const canonicalPublicFileMapPayload: Record<
		string,
		{ dist: string; hash: string; prehashed: boolean }
	> = {};
	for (const [sourcePath, distPath] of Object.entries(map)) {
		canonicalPublicFileMapPayload[sourcePath] = {
			dist: distPath,
			hash: `hash_${sourcePath}`,
			prehashed: false,
		};
	}
	writeFileSync(
		join(staticPublicOutDir, canonicalPublicFileMapPath),
		JSON.stringify(canonicalPublicFileMapPayload),
	);

	const publicFileMapRefPath = join(
		staticInternalOutDir,
		"public_file_map_file_ref.txt",
	);
	writeFileSync(publicFileMapRefPath, canonicalPublicFileMapPath);

	return { distDir };
}

function buildPluginConfig(map: Record<string, string> = {}) {
	const canonicalPublicFileMapFixture =
		createCanonicalPublicFileMapFixture(map);
	return {
		rollupInput: ["frontend/src/vorma.entry.tsx", "frontend/src/admin.tsx"],
		publicPathPrefix: "/static/",
		buildtimePublicURLFuncName: "waveBuildtimeURL",
		distDir: canonicalPublicFileMapFixture.distDir,
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

type PluginTransformResult = string | { code: string } | null;

type PluginTransformHook = (
	code: string,
	id: string,
) => PluginTransformResult | Promise<PluginTransformResult>;

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

function getPluginTransformHandler(plugin: Plugin): PluginTransformHook {
	const transformHook = plugin.transform;
	if (!transformHook) {
		throw new Error(
			"Expected Vorma Vite plugin to provide a transform hook.",
		);
	}
	if (typeof transformHook === "function") {
		return transformHook as PluginTransformHook;
	}
	return transformHook.handler as PluginTransformHook;
}

async function invokePluginTransform(
	plugin: Plugin,
	code: string,
): Promise<PluginTransformResult> {
	const transformHandler = getPluginTransformHandler(plugin);
	return transformHandler(code, "/tmp/source.ts");
}

function getTransformedCode(result: PluginTransformResult): string {
	if (typeof result === "string") {
		return result;
	}
	if (
		result &&
		typeof result === "object" &&
		typeof result.code === "string"
	) {
		return result.code;
	}
	throw new Error("Expected transform result to include code.");
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

describe("vorma vite plugin static public URL transform behavior", () => {
	it("replaces mapped buildtime public URL calls", async () => {
		const plugin = vormaVitePlugin(
			buildPluginConfig({
				"images/logo.svg": "wave_out_images_logo_deadbeef.svg",
			}),
		);

		const transformed = await invokePluginTransform(
			plugin,
			`const logoURL = waveBuildtimeURL("images/logo.svg");`,
		);
		const transformedCode = getTransformedCode(transformed);
		expect(transformedCode).toContain(
			`const logoURL = "/static/wave_out_images_logo_deadbeef.svg";`,
		);
	});

	it("throws when a buildtime public URL lookup is missing from the file map", async () => {
		const plugin = vormaVitePlugin(buildPluginConfig({}));

		await expect(
			invokePluginTransform(
				plugin,
				`const logoURL = waveBuildtimeURL("images/missing.svg");`,
			),
		).rejects.toThrow("unresolved static public asset lookup(s)");
	});
});
