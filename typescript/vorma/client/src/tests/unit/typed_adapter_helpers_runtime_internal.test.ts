import { describe, expect, it } from "vitest";
import {
	buildTypedAdapterRoutePropsWithInternalRouteInstanceToken,
	createTypedAdapterRouteInstanceToken,
	markTypedAdapterRouteInstanceTokenActive,
	markTypedAdapterRouteInstanceTokenDisposed,
	resolveTypedAdapterClientLoaderDataForPatternOrRouteProps,
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterLoaderDataForRoutePropsOrThrow,
	syncTypedAdapterRouteInstanceStoreFromNavigationState,
} from "../../ui/typed_adapter_helpers_runtime.ts";

function buildRoutePropsWithToken(props: {
	idx: number;
	routeInstanceToken: unknown;
}) {
	return {
		idx: props.idx,
		...buildTypedAdapterRoutePropsWithInternalRouteInstanceToken({
			routeInstanceToken: props.routeInstanceToken,
		}),
	} as any;
}

function buildRouteKey(props: {
	idx: number;
	importURL: string;
	exportKey: string;
	matchedPattern?: string;
}): string {
	return JSON.stringify([
		props.idx,
		props.importURL,
		props.exportKey,
		props.matchedPattern ?? "/probe",
	]);
}

describe("typed adapter helper runtime selection contracts", () => {
	it("does not expose legacy route-props indexed resolver through internal exports", async () => {
		const internalExports = await import("../../../internal.ts");
		expect(
			"resolveTypedAdapterIndexedDataForPatternOrRouteProps" in
				internalExports,
		).toBe(false);
	});

	it("selects indexed data by matched pattern", () => {
		const result = resolveTypedAdapterIndexedDataForPattern<string>({
			pattern: "/probe",
			matchedPatterns: ["/probe", "/other"],
			indexedData: ["probe-a", "other-a"],
		});

		expect(result).toBe("probe-a");
	});

	it("resolves loader data by explicit route-instance store transitions", () => {
		const routeInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: 0,
			routeKey: buildRouteKey({
				idx: 0,
				importURL: "/root.js",
				exportKey: "default",
			}),
			matchedPatterns: ["/probe"],
			loadersData: ["loader-a"],
			clientLoadersData: ["client-a"],
		});
		const routeProps = buildRoutePropsWithToken({
			idx: 0,
			routeInstanceToken,
		});
		markTypedAdapterRouteInstanceTokenActive({
			routeInstanceToken,
		});

		const activeResult =
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-a"],
				clientLoadersData: ["client-a"],
			});
		expect(activeResult).toBe("loader-a");

		syncTypedAdapterRouteInstanceStoreFromNavigationState({
			matchedPatterns: ["/root"],
			loadersData: ["root-b"],
			clientLoadersData: ["root-client-b"],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});
		const exitingResult =
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				matchedPatterns: ["/root"],
				loadersData: ["root-b"],
				clientLoadersData: ["root-client-b"],
			});
		expect(exitingResult).toBe("loader-a");

		syncTypedAdapterRouteInstanceStoreFromNavigationState({
			matchedPatterns: ["/probe"],
			loadersData: ["loader-c"],
			clientLoadersData: ["client-c"],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});
		const reboundResult =
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-c"],
				clientLoadersData: ["client-c"],
			});
		expect(reboundResult).toBe("loader-c");
	});

	it("marks route instance exiting on route-key replacement and keeps previous snapshot", () => {
		const routeInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: 0,
			routeKey: buildRouteKey({
				idx: 0,
				importURL: "/root-a.js",
				exportKey: "default",
			}),
			matchedPatterns: ["/probe"],
			loadersData: ["loader-a"],
			clientLoadersData: ["client-a"],
		});
		const routeProps = buildRoutePropsWithToken({
			idx: 0,
			routeInstanceToken,
		});
		markTypedAdapterRouteInstanceTokenActive({
			routeInstanceToken,
		});

		syncTypedAdapterRouteInstanceStoreFromNavigationState({
			matchedPatterns: ["/probe"],
			loadersData: ["loader-b"],
			clientLoadersData: ["client-b"],
			importURLs: ["/root-b.js"],
			exportKeys: ["default"],
		});
		const exitingResult =
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-b"],
				clientLoadersData: ["client-b"],
			});
		expect(exitingResult).toBe("loader-a");
	});

	it("resolves client loader data by explicit route-instance store transitions", () => {
		const routeInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: 0,
			routeKey: buildRouteKey({
				idx: 0,
				importURL: "/root.js",
				exportKey: "default",
			}),
			matchedPatterns: ["/probe"],
			loadersData: ["loader-a"],
			clientLoadersData: ["client-a"],
		});
		const routeProps = buildRoutePropsWithToken({
			idx: 0,
			routeInstanceToken,
		});
		markTypedAdapterRouteInstanceTokenActive({
			routeInstanceToken,
		});

		const activeResult =
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-a"],
				clientLoadersData: ["client-a"],
			});
		expect(activeResult).toBe("client-a");

		syncTypedAdapterRouteInstanceStoreFromNavigationState({
			matchedPatterns: ["/root"],
			loadersData: ["root-b"],
			clientLoadersData: ["root-client-b"],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});
		const exitingResult =
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				routeProps,
				matchedPatterns: ["/root"],
				loadersData: ["root-b"],
				clientLoadersData: ["root-client-b"],
			});
		expect(exitingResult).toBe("client-a");

		syncTypedAdapterRouteInstanceStoreFromNavigationState({
			matchedPatterns: ["/probe"],
			loadersData: ["loader-c"],
			clientLoadersData: ["client-c"],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});
		const reboundResult =
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-c"],
				clientLoadersData: ["client-c"],
			});
		expect(reboundResult).toBe("client-c");
	});

	it("throws for client loader route-props path when token is bound to a different pattern", () => {
		const routeInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: 0,
			routeKey: buildRouteKey({
				idx: 0,
				importURL: "/root.js",
				exportKey: "default",
			}),
			matchedPatterns: ["/probe"],
			loadersData: ["loader-a"],
			clientLoadersData: ["client-a"],
		});
		const routeProps = buildRoutePropsWithToken({
			idx: 0,
			routeInstanceToken,
		});

		expect(() =>
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/other",
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-a"],
				clientLoadersData: ["client-a"],
			}),
		).toThrow(
			'useClientLoaderData(routeProps) contract violated for pattern "/other": route instance is bound to pattern "/probe".',
		);
	});

	it("throws for route-props path when route instance token is missing", () => {
		expect(() =>
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps: { idx: 0 } as any,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-a"],
				clientLoadersData: ["client-a"],
			}),
		).toThrow(
			"useLoaderData(routeProps) contract violated: route instance token is missing or invalid.",
		);
	});

	it("throws for route-props path when route instance token is disposed", () => {
		const routeInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: 0,
			routeKey: buildRouteKey({
				idx: 0,
				importURL: "/root.js",
				exportKey: "default",
			}),
			matchedPatterns: ["/probe"],
			loadersData: ["loader-a"],
			clientLoadersData: ["client-a"],
		});
		markTypedAdapterRouteInstanceTokenDisposed({
			routeInstanceToken,
		});
		const routeProps = buildRoutePropsWithToken({
			idx: 0,
			routeInstanceToken,
		});

		expect(() =>
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-a"],
				clientLoadersData: ["client-a"],
			}),
		).toThrow(
			"useLoaderData(routeProps) contract violated: route instance has been disposed.",
		);
	});

	it("throws when route props index changes after route instance binding", () => {
		const routeInstanceToken = createTypedAdapterRouteInstanceToken({
			routePropsIndex: 0,
			routeKey: buildRouteKey({
				idx: 0,
				importURL: "/root.js",
				exportKey: "default",
			}),
			matchedPatterns: ["/probe"],
			loadersData: ["loader-a"],
			clientLoadersData: ["client-a"],
		});
		const routeProps = buildRoutePropsWithToken({
			idx: 1,
			routeInstanceToken,
		});

		expect(() =>
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				matchedPatterns: ["/probe", "/child"],
				loadersData: ["loader-a", "loader-b"],
				clientLoadersData: ["client-a", "client-b"],
			}),
		).toThrow(
			"useLoaderData(routeProps) contract violated: routeProps.idx changed after route instance binding.",
		);
	});

	it("returns undefined for no-route-props client-loader path when pattern is unmatched", () => {
		const result =
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				matchedPatterns: ["/other"],
				loadersData: ["other-loader"],
				clientLoadersData: ["other-client"],
			});

		expect(result).toBeUndefined();
	});

	it("does not sync snapshots for tokens that were created but never mounted", () => {
		const unmountedRouteInstanceToken =
			createTypedAdapterRouteInstanceToken({
				routePropsIndex: 0,
				routeKey: buildRouteKey({
					idx: 0,
					importURL: "/root.js",
					exportKey: "default",
				}),
				matchedPatterns: ["/probe"],
				loadersData: ["loader-a"],
				clientLoadersData: ["client-a"],
			});
		const unmountedRouteProps = buildRoutePropsWithToken({
			idx: 0,
			routeInstanceToken: unmountedRouteInstanceToken,
		});

		syncTypedAdapterRouteInstanceStoreFromNavigationState({
			matchedPatterns: ["/probe"],
			loadersData: ["loader-b"],
			clientLoadersData: ["client-b"],
			importURLs: ["/root.js"],
			exportKeys: ["default"],
		});

		expect(
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps: unmountedRouteProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-b"],
				clientLoadersData: ["client-b"],
			}),
		).toBe("loader-a");
		expect(
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				routeProps: unmountedRouteProps,
				matchedPatterns: ["/probe"],
				loadersData: ["loader-b"],
				clientLoadersData: ["client-b"],
			}),
		).toBe("client-a");
	});
});
