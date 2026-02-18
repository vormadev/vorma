import { describe, expect, it } from "vitest";
import {
	buildClientModuleMapFromRouteModuleMetadata,
	mergeClientModuleMapWithRouteModuleMetadata,
} from "../../core/navigation/runtime_navigation_successful_runtime.ts";

describe("route metadata module-map helpers", () => {
	it("builds a fresh module map from route metadata arrays with default export fallbacks", () => {
		const clientModuleMap = buildClientModuleMapFromRouteModuleMetadata({
			routeModuleMetadata: {
				matchedPatterns: ["/route-a", "/route-b"],
				importURLs: ["/route-a.js", "/route-b.js"],
				exportKeys: ["", "NamedExport"],
				errorExportKeys: ["", "RouteError"],
			},
		});

		expect(clientModuleMap).toEqual({
			"/route-a": {
				importURL: "/route-a.js",
				exportKey: "default",
				errorExportKey: "",
			},
			"/route-b": {
				importURL: "/route-b.js",
				exportKey: "NamedExport",
				errorExportKey: "RouteError",
			},
		});
	});

	it("merges route metadata into existing module map entries without mutating the input map", () => {
		const existingClientModuleMap = {
			"/existing": {
				importURL: "/existing.js",
				exportKey: "default",
				errorExportKey: "",
			},
		};

		const mergedClientModuleMap =
			mergeClientModuleMapWithRouteModuleMetadata({
				currentClientModuleMap: existingClientModuleMap,
				routeModuleMetadata: {
					matchedPatterns: ["/new"],
					importURLs: ["/new.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
				},
			});

		expect(mergedClientModuleMap).toEqual({
			"/existing": {
				importURL: "/existing.js",
				exportKey: "default",
				errorExportKey: "",
			},
			"/new": {
				importURL: "/new.js",
				exportKey: "default",
				errorExportKey: "",
			},
		});
		expect(mergedClientModuleMap).not.toBe(existingClientModuleMap);
		expect(existingClientModuleMap).toEqual({
			"/existing": {
				importURL: "/existing.js",
				exportKey: "default",
				errorExportKey: "",
			},
		});
	});

	it("ignores incomplete route metadata rows and tolerates undefined arrays", () => {
		expect(
			mergeClientModuleMapWithRouteModuleMetadata({
				currentClientModuleMap: {
					"/existing": {
						importURL: "/existing.js",
						exportKey: "default",
						errorExportKey: "",
					},
				},
				routeModuleMetadata: {
					matchedPatterns: ["/missing-import", ""],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
				},
			}),
		).toEqual({
			"/existing": {
				importURL: "/existing.js",
				exportKey: "default",
				errorExportKey: "",
			},
		});

		expect(
			mergeClientModuleMapWithRouteModuleMetadata({
				currentClientModuleMap: undefined,
				routeModuleMetadata: {},
			}),
		).toEqual({});
	});
});
