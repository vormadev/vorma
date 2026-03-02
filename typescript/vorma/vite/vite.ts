import { readFileSync, statSync } from "node:fs";
import { resolve, sep } from "node:path";
import type { ConfigEnv, Plugin, UserConfig, ViteDevServer } from "vite";

export type VormaVitePluginConfig = {
	rollupInput: ReadonlyArray<string>;
	publicPathPrefix: string;
	buildtimePublicURLFuncName: string;
	distDir: string;
	ignoredPatterns: ReadonlyArray<string>;
	dedupeList: ReadonlyArray<string>;
};

const FILEMAP_CHANGED_NOTIFY_ENDPOINT_PATH = "/__wave_notify_filemap_changed";

function mergeRollupInput(
	vormaRollupInput: ReadonlyArray<string>,
	existingRollupInput: unknown,
): Array<string> | Record<string, string> {
	if (typeof existingRollupInput === "string") {
		return [...vormaRollupInput, existingRollupInput];
	}

	if (Array.isArray(existingRollupInput)) {
		return [...vormaRollupInput, ...existingRollupInput];
	}

	if (
		typeof existingRollupInput === "object" &&
		existingRollupInput !== null
	) {
		const existingObjectInput = existingRollupInput as Record<
			string,
			string
		>;
		const mergedObjectInput: Record<string, string> = {
			...existingObjectInput,
		};
		const usedInputKeys = new Set(Object.keys(existingObjectInput));
		let nextInternalKeyIndex = 0;

		for (
			let inputIndex = 0;
			inputIndex < vormaRollupInput.length;
			inputIndex++
		) {
			let internalKey = `__vorma_internal_entry_${nextInternalKeyIndex}`;
			while (usedInputKeys.has(internalKey)) {
				nextInternalKeyIndex++;
				internalKey = `__vorma_internal_entry_${nextInternalKeyIndex}`;
			}
			mergedObjectInput[internalKey] = vormaRollupInput[inputIndex] || "";
			usedInputKeys.add(internalKey);
			nextInternalKeyIndex++;
		}

		return mergedObjectInput;
	}

	return [...vormaRollupInput];
}

type UserConfigServerWatchIgnored = NonNullable<
	NonNullable<UserConfig["server"]>["watch"]
>["ignored"];

function mergeServerWatchIgnoredPatterns(
	existingIgnoredPatterns: UserConfigServerWatchIgnored,
	vormaIgnoredPatterns: ReadonlyArray<string>,
): UserConfigServerWatchIgnored {
	if (Array.isArray(existingIgnoredPatterns)) {
		return [...existingIgnoredPatterns, ...vormaIgnoredPatterns];
	}

	if (existingIgnoredPatterns !== undefined) {
		return [
			existingIgnoredPatterns,
			...vormaIgnoredPatterns,
		] as UserConfigServerWatchIgnored;
	}

	return [...vormaIgnoredPatterns] as UserConfigServerWatchIgnored;
}

export default function vormaVitePlugin(config: VormaVitePluginConfig): Plugin {
	let cachedMap: Record<string, string> | null = null;
	let cachedRefMtime: number = 0;
	let cachedFileMapMtime: number = 0;
	let cachedFileMapPath = "";
	let isDev = false;
	let resolvedStaticDistDirPath: string | null = null;

	type CanonicalPublicFileMap = Record<
		string,
		{ dist: string; hash: string; prehashed: boolean }
	>;

	function getResolvedStaticDistDirPath(): string {
		if (!resolvedStaticDistDirPath) {
			resolvedStaticDistDirPath = resolve(
				process.cwd(),
				config.distDir,
				"static",
			);
		}
		return resolvedStaticDistDirPath;
	}

	function getResolvedPublicFileMapRefPath(): string {
		return resolve(
			getResolvedStaticDistDirPath(),
			"internal",
			"public_file_map_file_ref.txt",
		);
	}

	function getResolvedStaticPublicOutDir(): string {
		return resolve(getResolvedStaticDistDirPath(), "assets", "public");
	}

	function resolveCanonicalPublicFileMapPathFromRef(
		refTargetPath: string,
	): string {
		const trimmedRefTargetPath = refTargetPath.trim();
		if (trimmedRefTargetPath === "") {
			throw new Error("[vorma-vite-plugin] public filemap ref is empty.");
		}

		const staticPublicOutDir = getResolvedStaticPublicOutDir();
		const resolvedCanonicalPath = resolve(
			staticPublicOutDir,
			trimmedRefTargetPath,
		);
		const normalizedStaticPublicOutDir = staticPublicOutDir.endsWith(sep)
			? staticPublicOutDir
			: `${staticPublicOutDir}${sep}`;
		if (
			resolvedCanonicalPath !== staticPublicOutDir &&
			!resolvedCanonicalPath.startsWith(normalizedStaticPublicOutDir)
		) {
			throw new Error(
				`[vorma-vite-plugin] public filemap ref escapes static public out dir: ${trimmedRefTargetPath}`,
			);
		}

		return resolvedCanonicalPath;
	}

	function flattenCanonicalPublicFileMap(
		canonicalPublicFileMap: CanonicalPublicFileMap,
	): Record<string, string> {
		const flattenedMap: Record<string, string> = {};
		for (const [sourcePath, value] of Object.entries(
			canonicalPublicFileMap,
		)) {
			if (!value || typeof value.dist !== "string" || value.dist === "") {
				throw new Error(
					`[vorma-vite-plugin] canonical public filemap entry is missing dist for ${sourcePath}.`,
				);
			}
			flattenedMap[sourcePath] = value.dist;
		}
		return flattenedMap;
	}

	function getFilemap(): Record<string, string> {
		const resolvedRefPath = getResolvedPublicFileMapRefPath();
		const refStat = statSync(resolvedRefPath);
		const refContent = readFileSync(resolvedRefPath, "utf-8");
		const resolvedCanonicalPath =
			resolveCanonicalPublicFileMapPathFromRef(refContent);
		const canonicalStat = statSync(resolvedCanonicalPath);

		const refMtime = refStat.mtimeMs;
		const canonicalMtime = canonicalStat.mtimeMs;
		if (
			cachedMap &&
			cachedFileMapPath === resolvedCanonicalPath &&
			cachedRefMtime === refMtime &&
			cachedFileMapMtime === canonicalMtime
		) {
			return cachedMap;
		}

		const canonicalContent = readFileSync(resolvedCanonicalPath, "utf-8");
		const canonicalPublicFileMap = JSON.parse(
			canonicalContent,
		) as CanonicalPublicFileMap;
		cachedMap = flattenCanonicalPublicFileMap(canonicalPublicFileMap);
		cachedRefMtime = refMtime;
		cachedFileMapMtime = canonicalMtime;
		cachedFileMapPath = resolvedCanonicalPath;
		return cachedMap;
	}

	return {
		name: "vorma-vite-plugin",

		config(c: UserConfig, { command }: ConfigEnv) {
			isDev = command === "serve";

			const mp = c.build?.modulePreload;
			const roi = c.build?.rollupOptions?.input;
			const ign = c.server?.watch?.ignored;
			const dedupe = c.resolve?.dedupe;

			return {
				base: isDev ? "/" : config.publicPathPrefix,
				build: {
					target: "es2022",
					emptyOutDir: false,
					modulePreload: {
						polyfill: false,
						...(typeof mp === "object" ? mp : {}),
					},
					rollupOptions: {
						...c.build?.rollupOptions,
						input: mergeRollupInput(config.rollupInput, roi),
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
						// ensure versions of dynamic imports without the latest
						// hmr updates are not cached by the browser during dev
						"cache-control": "no-store",
					},
					watch: {
						...c.server?.watch,
						ignored: mergeServerWatchIgnoredPatterns(
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

		/**
		 * Configures the dev server with a generic filemap-changed notify endpoint.
		 * Wave calls this endpoint after updating public static files, which:
		 * 1. Clears the cached filemap so the next transform reads fresh data
		 * 2. Invalidates all modules in Vite's module graph
		 * 3. Triggers a browser reload via Vite's HMR websocket
		 *
		 * This is much faster than cycling Vite (stopping and restarting the process).
		 */
		configureServer(server: ViteDevServer) {
			server.middlewares.use((req, res, next) => {
				if (req.url !== FILEMAP_CHANGED_NOTIFY_ENDPOINT_PATH) {
					return next();
				}

				console.log(
					"[vorma-vite-plugin] Filemap-changed notification received",
				);

				// Clear the filemap cache so the next transform reads fresh data
				cachedMap = null;
				cachedRefMtime = 0;
				cachedFileMapMtime = 0;
				cachedFileMapPath = "";

				// Invalidate all modules in Vite's module graph.
				// This is simpler than tracking which specific modules use
				// waveBuildtimeURL() and fast enough for typical project sizes
				// (a few ms for hundreds of modules).
				for (const mod of server.moduleGraph.idToModuleMap.values()) {
					server.moduleGraph.invalidateModule(mod);
				}

				// Trigger a full browser reload via Vite's HMR websocket.
				// The browser will re-request modules, Vite will re-transform them
				// (cache miss due to invalidation), and they'll get the new URLs.
				server.ws.send({ type: "full-reload" });

				res.statusCode = 200;
				res.end("ok");
			});
		},

		transform(code: string, id: string) {
			const isNodeModules = /node_modules/.test(id);
			if (isNodeModules) return null;

			const regex = new RegExp(
				`${config.buildtimePublicURLFuncName}\\s*\\(\\s*(["'\`])(.*?)\\1\\s*\\)`,
				"g",
			);

			const needsReplacement = regex.test(code);
			if (!needsReplacement) return null;

			// Get the current filemap from canonical Wave output.
			const filemap = getFilemap();
			const missingStaticPublicAssets = new Set<string>();

			const replacedCode = code.replace(
				regex,
				(_fullMatch: string, _quoteChar: string, assetPath: string) => {
					const hashed = filemap[assetPath];
					if (!hashed) {
						missingStaticPublicAssets.add(assetPath);
						return _fullMatch;
					}
					return `"${config.publicPathPrefix}${hashed}"`;
				},
			);

			if (missingStaticPublicAssets.size > 0) {
				const unresolvedCalls = Array.from(missingStaticPublicAssets)
					.sort()
					.map(
						(assetPath) =>
							`${config.buildtimePublicURLFuncName}("${assetPath}")`,
					)
					.join(", ");

				throw new Error(
					`[vorma-vite-plugin] unresolved static public asset lookup(s): ${unresolvedCalls}`,
				);
			}

			if (replacedCode === code) return null;
			return replacedCode;
		},
	};
}
