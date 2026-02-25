import { describe, expect, it } from "vitest";
import {
	resolveTypedAdapterIndexedDataForPattern,
	resolveTypedAdapterIndexedDataForPatternOrRouteProps,
} from "../../ui/typed_adapter_helpers_runtime.ts";

describe("typed adapter helper runtime selection contracts", () => {
	it("selects indexed data by matched pattern", () => {
		const result = resolveTypedAdapterIndexedDataForPattern<string>({
			pattern: "/probe",
			matchedPatterns: ["/probe", "/other"],
			indexedData: ["probe-a", "other-a"],
		});

		expect(result).toBe("probe-a");
	});

	it("returns pattern-matched data when pattern is matched", () => {
		const result =
			resolveTypedAdapterIndexedDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				matchedPatterns: ["/probe", "/other"],
				indexedData: ["probe-a", "other-a"],
				routeProps: { idx: 0 },
			});

		expect(result).toBe("probe-a");
	});

	it("ignores route-props idx when it points to a different pattern", () => {
		expect(() =>
			resolveTypedAdapterIndexedDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				matchedPatterns: ["/probe", "/other"],
				indexedData: ["probe-a", "other-a"],
				routeProps: { idx: 1 },
			}),
		).toThrow(
			'useClientLoaderData(routeProps) contract violated for pattern "/probe": routeProps.idx resolved to "/other".',
		);
	});

	it("throws when route-props path is used while pattern is not matched", () => {
		expect(() =>
			resolveTypedAdapterIndexedDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				matchedPatterns: ["/other"],
				indexedData: ["other-a"],
				routeProps: { idx: 0 },
			}),
		).toThrow(
			'useClientLoaderData(routeProps) contract violated for pattern "/probe": pattern is not currently matched.',
		);
	});

	it("returns undefined when no route-props are provided and pattern is not matched", () => {
		const result =
			resolveTypedAdapterIndexedDataForPatternOrRouteProps<string>({
				pattern: "/probe",
				matchedPatterns: ["/other"],
				indexedData: ["other-a"],
			});

		expect(result).toBeUndefined();
	});
});
