import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { ConfigEnv, Plugin, UserConfig } from "vite";

export type VormaVitePluginConfig = {
	rollupInput: ReadonlyArray<string>;
	publicPathPrefix: string;
	buildtimePublicURLFuncName: string;
	ignoredPatterns: ReadonlyArray<string>;
	dedupeList: ReadonlyArray<string>;
	importMetaURL: string;
};

function merge_rollup_input(
	vorma_input: ReadonlyArray<string>,
	existing_input: unknown,
): Array<string> | Record<string, string> {
	if (typeof existing_input === "string") {
		return [...vorma_input, existing_input];
	}
	if (Array.isArray(existing_input)) {
		return [...vorma_input, ...existing_input];
	}
	if (typeof existing_input === "object" && existing_input !== null) {
		const existing_obj = existing_input as Record<string, string>;
		const merged: Record<string, string> = { ...existing_obj };
		const used_keys = new Set(Object.keys(existing_obj));
		let next_idx = 0;
		for (let i = 0; i < vorma_input.length; i++) {
			let key = `__vorma_internal_entry_${next_idx}`;
			while (used_keys.has(key)) {
				next_idx++;
				key = `__vorma_internal_entry_${next_idx}`;
			}
			merged[key] = vorma_input[i] || "";
			used_keys.add(key);
			next_idx++;
		}
		return merged;
	}
	return [...vorma_input];
}

type WatchIgnored = NonNullable<
	NonNullable<NonNullable<UserConfig["server"]>["watch"]>["ignored"]
>;

function merge_watch_ignored(
	existing: WatchIgnored | undefined,
	vorma_patterns: ReadonlyArray<string>,
): WatchIgnored {
	if (Array.isArray(existing)) {
		return [...existing, ...vorma_patterns];
	}
	if (existing !== undefined) {
		return [existing, ...vorma_patterns] as WatchIgnored;
	}
	return [...vorma_patterns] as WatchIgnored;
}

export default function vormaVitePlugin(config: VormaVitePluginConfig): Plugin {
	const gen_dir = dirname(fileURLToPath(config.importMetaURL));
	const filemap_path = join(gen_dir, "filemap.json");

	function read_filemap(): Record<string, string> {
		return JSON.parse(readFileSync(filemap_path, "utf-8"));
	}

	return {
		name: "vorma-vite-plugin",

		config(c: UserConfig, { command }: ConfigEnv) {
			const mp = c.build?.modulePreload;
			const roi = c.build?.rollupOptions?.input;
			const ign = c.server?.watch?.ignored;
			const dedupe = c.resolve?.dedupe;

			return {
				base: command === "serve" ? "/" : config.publicPathPrefix,
				build: {
					target: "es2022",
					emptyOutDir: false,
					modulePreload: {
						polyfill: false,
						...(typeof mp === "object" ? mp : {}),
					},
					rollupOptions: {
						...c.build?.rollupOptions,
						input: merge_rollup_input(config.rollupInput, roi),
						preserveEntrySignatures: "exports-only",
						output: {
							assetFileNames:
								"wave_out_vite_[name]-[hash][extname]",
							chunkFileNames: "wave_out_vite_[name]-[hash].js",
							entryFileNames: "wave_out_vite_[name]-[hash].js",
						},
					},
				},
				server: {
					headers: {
						...c.server?.headers,
						"cache-control": "no-store",
					},
					watch: {
						...c.server?.watch,
						ignored: merge_watch_ignored(
							ign,
							config.ignoredPatterns,
						),
					},
				},
				resolve: {
					dedupe: [
						...(Array.isArray(dedupe) ? dedupe : []),
						...config.dedupeList,
					],
				},
			};
		},

		transform(code: string, id: string) {
			if (/node_modules/.test(id)) return null;

			const regex = new RegExp(
				`${config.buildtimePublicURLFuncName}\\s*\\(\\s*(["'\`])(.*?)\\1\\s*\\)`,
				"g",
			);
			if (!regex.test(code)) return null;

			const filemap = read_filemap();
			const missing: string[] = [];

			const replaced = code.replace(
				regex,
				(full_match: string, _quote: string, asset_path: string) => {
					const hashed = filemap[asset_path];
					if (!hashed) {
						missing.push(asset_path);
						return full_match;
					}
					return `"${config.publicPathPrefix}${hashed}"`;
				},
			);

			if (missing.length > 0) {
				const calls = missing
					.sort()
					.map((p) => `${config.buildtimePublicURLFuncName}("${p}")`)
					.join(", ");
				throw new Error(
					`[vorma-vite-plugin] unresolved static public asset(s): ${calls}`,
				);
			}

			if (replaced === code) return null;
			return replaced;
		},
	};
}
