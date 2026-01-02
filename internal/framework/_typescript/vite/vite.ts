type VormaVitePluginConfig = {
	rollupInput: ReadonlyArray<string>;
	publicPathPrefix: string;
	staticPublicAssetMap: Record<string, string>;
	buildtimePublicURLFuncName: string;
	ignoredPatterns: ReadonlyArray<string>;
	dedupeList: ReadonlyArray<string>;
};

export default function vormaVitePlugin(config: VormaVitePluginConfig): any {
	return {
		name: "vorma-vite-plugin",
		config(c: any, { command }: any) {
			const mp = c.build?.modulePreload;
			const roi = c.build?.rollupOptions?.input;
			const ign = c.server?.watch?.ignored;
			const dedupe = c.resolve?.dedupe;

			const isDev = command === "serve";

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
						input: [
							...config.rollupInput,
							...(Array.isArray(roi) ? roi : []),
						],
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
						ignored: [
							...(Array.isArray(ign) ? ign : []),
							...config.ignoredPatterns,
						],
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
		transform(code: any, id: any) {
			const isNodeModules = /node_modules/.test(id);
			if (isNodeModules) return null;

			const regex = new RegExp(
				`${config.buildtimePublicURLFuncName}\\s*\\(\\s*(["'\`])(.*?)\\1\\s*\\)`,
				"g",
			);

			const needsReplacement = regex.test(code);
			if (!needsReplacement) return null;

			const replacedCode = code.replace(
				regex,
				(_: any, __: any, assetPath: any) => {
					const hashed = config.staticPublicAssetMap[assetPath];
					if (!hashed) return `"${assetPath}"`;
					return `"${config.publicPathPrefix}${hashed}"`;
				},
			);

			if (replacedCode === code) return null;
			return replacedCode;
		},
	};
}
