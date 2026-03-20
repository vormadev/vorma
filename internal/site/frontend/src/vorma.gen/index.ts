const routes = [] as const;
export const vormaAppConfig = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
	importMetaURL: import.meta.url,
} as const;
export const vormaViteConfig = {
	rollupInput: [],
	publicPathPrefix: "/",
	buildtimePublicURLFuncName: "hashedURL",
	ignoredPatterns: ["**/*.go", "**/backend/.waveout/**/*", "**/frontend/src/vorma.gen/**/*"],
	dedupeList: [],
	importMetaURL: import.meta.url,
} as const;
import { staticPublicAssetMap } from "./filemap";
export { staticPublicAssetMap };
