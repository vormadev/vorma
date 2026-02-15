import { readFileSync, statSync } from "node:fs";
import { resolve } from "node:path";
import type { Plugin } from "vite";

type VormaVitePluginConfig = {
	rollupInput: ReadonlyArray<string>;
	publicPathPrefix: string;
	staticPublicAssetMap: Record<string, string>;
	buildtimePublicURLFuncName: string;
	filemapJSONPath: string;
	ignoredPatterns: ReadonlyArray<string>;
	dedupeList: ReadonlyArray<string>;
};

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

function mergeServerWatchIgnoredPatterns(
	existingIgnoredPatterns: unknown,
	vormaIgnoredPatterns: ReadonlyArray<string>,
): Array<unknown> {
	if (Array.isArray(existingIgnoredPatterns)) {
		return [...existingIgnoredPatterns, ...vormaIgnoredPatterns];
	}

	if (existingIgnoredPatterns !== undefined) {
		return [existingIgnoredPatterns, ...vormaIgnoredPatterns];
	}

	return [...vormaIgnoredPatterns];
}

export default function vormaVitePlugin(config: VormaVitePluginConfig): Plugin {
	// Cache for dev mode filemap reading.
	// In dev mode, we read from the JSON file so we can pick up changes
	// without restarting Vite. The mtime check allows us to avoid re-reading
	// the file on every transform if it hasn't changed.
	let cachedMap: Record<string, string> | null = null;
	let cachedMtime: number = 0;
	let isDev = false;
	let resolvedFilemapPath: string | null = null;

	/**
	 * Gets the current filemap, reading from disk in dev mode.
	 * In production builds, uses the static map passed at plugin creation.
	 */
	function getFilemap(): Record<string, string> {
		if (!isDev) {
			return config.staticPublicAssetMap;
		}

		if (!resolvedFilemapPath) {
			resolvedFilemapPath = resolve(
				process.cwd(),
				config.filemapJSONPath,
			);
		}

		try {
			const stat = statSync(resolvedFilemapPath);
			const mtime = stat.mtimeMs;

			// Return cached version if file hasn't changed
			if (cachedMap && mtime === cachedMtime) {
				return cachedMap;
			}

			const content = readFileSync(resolvedFilemapPath, "utf-8");
			cachedMap = JSON.parse(content);
			cachedMtime = mtime;
			return cachedMap!;
		} catch {
			// Fallback to initial config if file can't be read.
			// This handles the case where the JSON file doesn't exist yet
			// (e.g., on first build before Wave has written it).
			return config.staticPublicAssetMap;
		}
	}

	return {
		name: "vorma-vite-plugin",

		config(c: any, { command }: any) {
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
								"vorma_out_vite_[name]-[hash][extname]",
							chunkFileNames: "vorma_out_vite_[name]-[hash].js",
							entryFileNames: "vorma_out_vite_[name]-[hash].js",
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
		 * Configures the dev server with an endpoint for filemap cache invalidation.
		 * Wave calls this endpoint after updating public static files, which:
		 * 1. Clears the cached filemap so the next transform reads fresh data
		 * 2. Invalidates all modules in Vite's module graph
		 * 3. Triggers a browser reload via Vite's HMR websocket
		 *
		 * This is much faster than cycling Vite (stopping and restarting the process).
		 */
		configureServer(server: any) {
			server.middlewares.use((req: any, res: any, next: any) => {
				if (req.url !== "/__vorma_invalidate_filemap") {
					return next();
				}

				console.log(
					"[vorma-vite-plugin] Filemap invalidation triggered",
				);

				// Clear the filemap cache so the next transform reads fresh data
				cachedMap = null;
				cachedMtime = 0;

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

		transform(code: any, id: any) {
			const isNodeModules = /node_modules/.test(id);
			if (isNodeModules) return null;

			const regex = new RegExp(
				`${config.buildtimePublicURLFuncName}\\s*\\(\\s*(["'\`])(.*?)\\1\\s*\\)`,
				"g",
			);

			const needsReplacement = regex.test(code);
			if (!needsReplacement) return null;

			// Get the current filemap (reads from disk in dev mode)
			const filemap = getFilemap();

			const replacedCode = code.replace(
				regex,
				(_: any, __: any, assetPath: any) => {
					const hashed = filemap[assetPath];
					if (!hashed) return `"${assetPath}"`;
					return `"${config.publicPathPrefix}${hashed}"`;
				},
			);

			if (replacedCode === code) return null;
			return replacedCode;
		},
	};
}
