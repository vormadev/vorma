import { describe, expect, it } from "vitest";
import {
	buildTypedAdapterRoutePropsWithInternalRouteScope,
	createTypedAdapterRouteScope,
	resolveTypedAdapterClientLoaderDataForPatternOrRouteProps,
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterLoaderDataForRoutePropsOrThrow,
} from "../../runtime.ts";

function buildRoutePropsWithScope(props: { idx: number; routeScope: unknown }) {
	return {
		idx: props.idx,
		...buildTypedAdapterRoutePropsWithInternalRouteScope({
			routeScope: props.routeScope,
		}),
	} as any;
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

	it("resolves loader data by route scope against current snapshot", () => {
		const routeScope = createTypedAdapterRouteScope({
			routePropsIndex: 0,
			matchedPattern: "/probe",
		});
		const routeProps = buildRoutePropsWithScope({
			idx: 0,
			routeScope,
		});

		const result =
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				loadersData: ["loader-a"],
			});
		expect(result).toBe("loader-a");
	});

	it("resolves client loader data by route scope against current snapshot", () => {
		const routeScope = createTypedAdapterRouteScope({
			routePropsIndex: 0,
			matchedPattern: "/probe",
		});
		const routeProps = buildRoutePropsWithScope({
			idx: 0,
			routeScope,
		});

		const result =
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				routeProps,
				matchedPatterns: ["/probe"],
				clientLoadersData: ["client-a"],
			});
		expect(result).toBe("client-a");
	});

	it("throws for client-loader route-props path when scope is bound to another pattern", () => {
		const routeScope = createTypedAdapterRouteScope({
			routePropsIndex: 0,
			matchedPattern: "/probe",
		});
		const routeProps = buildRoutePropsWithScope({
			idx: 0,
			routeScope,
		});

		expect(() =>
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/other",
				routeProps,
				matchedPatterns: ["/probe"],
				clientLoadersData: ["client-a"],
			}),
		).toThrow(
			'useClientLoaderData(routeProps) contract violated for pattern "/other": route scope is bound to pattern "/probe".',
		);
	});

	it("throws for route-props path when route scope is missing", () => {
		expect(() =>
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps: { idx: 0 } as any,
				loadersData: ["loader-a"],
			}),
		).toThrow(
			"useLoaderData(routeProps) contract violated: route scope is missing or invalid.",
		);
	});

	it("throws when route props index changes after route scope binding", () => {
		const routeScope = createTypedAdapterRouteScope({
			routePropsIndex: 0,
			matchedPattern: "/probe",
		});
		const routeProps = buildRoutePropsWithScope({
			idx: 1,
			routeScope,
		});

		expect(() =>
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				loadersData: ["loader-a", "loader-b"],
			}),
		).toThrow(
			"useLoaderData(routeProps) contract violated: routeProps.idx changed after route scope binding.",
		);
	});

	it("follows current indexed loader data for a bound route scope across pattern changes", () => {
		const routeScope = createTypedAdapterRouteScope({
			routePropsIndex: 0,
			matchedPattern: "/probe",
		});
		const routeProps = buildRoutePropsWithScope({
			idx: 0,
			routeScope,
		});

		expect(
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				loadersData: ["loader-b"],
			}),
		).toBe("loader-b");
	});

	it("throws when route-scoped loader data is unavailable", () => {
		const routeScope = createTypedAdapterRouteScope({
			routePropsIndex: 1,
			matchedPattern: "/child",
		});
		const routeProps = buildRoutePropsWithScope({
			idx: 1,
			routeScope,
		});

		expect(() =>
			resolveTypedAdapterLoaderDataForRoutePropsOrThrow<string>({
				routeProps,
				loadersData: ["loader-root"],
			}),
		).toThrow(
			"useLoaderData(routeProps) contract violated: no route-scoped loader snapshot is available.",
		);
	});

	it("returns undefined for no-route-props client-loader path when pattern is unmatched", () => {
		const result =
			resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				matchedPatterns: ["/other"],
				clientLoadersData: ["other-client"],
			});

		expect(result).toBeUndefined();
	});

	it("validates route scope initialization contracts", () => {
		expect(() =>
			createTypedAdapterRouteScope({
				routePropsIndex: -1,
				matchedPattern: "/probe",
			}),
		).toThrow(
			"Vorma route scope initialization violated: route index must be a non-negative integer.",
		);
		expect(() =>
			createTypedAdapterRouteScope({
				routePropsIndex: 0,
				matchedPattern: "",
			}),
		).toThrow(
			"Vorma route scope initialization violated: matched pattern is missing at route index.",
		);
	});
});
