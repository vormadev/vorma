import { describe, expect, it } from "vitest";
import { resolveVormaPath } from "./path_resolution.ts";

const TEST_CONFIG = {
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

describe("resolveVormaPath", () => {
	it("URL-encodes dynamic parameter values and replaces repeated exact tokens", () => {
		const path = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/users/:id/:id2/:id",
				params: {
					id: "a/b c",
					id2: "second",
				},
			},
		});

		expect(path).toBe("/users/a%2Fb%20c/second/a%2Fb%20c");
	});

	it("URL-encodes splat segments individually", () => {
		const path = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/files/*",
				splatValues: ["docs and notes", "q1/q2"],
			},
		});

		expect(path).toBe("/files/docs%20and%20notes/q1%2Fq2");
	});

	it("strips explicit loader index segments after path resolution", () => {
		const rootPath = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/_index",
			},
		});

		const nestedPath = resolveVormaPath({
			vormaAppConfig: TEST_CONFIG,
			type: "loader",
			props: {
				pattern: "/users/_index",
			},
		});

		expect(rootPath).toBe("/");
		expect(nestedPath).toBe("/users");
	});
});
