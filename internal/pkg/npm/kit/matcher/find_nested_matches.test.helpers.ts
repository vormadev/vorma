import type { Result } from "vorma/kit/result";
import {
	createPatternRegistry,
	type Params,
	type RegisteredPattern,
	registerPattern,
	type RegistrationOptions,
} from "./register.ts";

export const NestedPatterns = [
	"/_index", // Index
	"/articles/_index", // Index
	"/articles/test/articles/_index", // Index
	"/bear/_index", // Index
	"/dashboard/_index", // Index
	"/dashboard/customers/_index", // Index
	"/dashboard/customers/:customer_id/_index", // Index
	"/dashboard/customers/:customer_id/orders/_index", // Index
	"/dynamic-index/:pagename/_index", // Index
	"/lion/_index", // Index
	"/tiger/_index", // Index
	"/tiger/:tiger_id/_index", // Index

	// NOTE: This will evaluate to an empty string -- should match to everything
	"/",

	"/*",
	"/bear",
	"/bear/:bear_id",
	"/bear/:bear_id/*",
	"/dashboard",
	"/dashboard/*",
	"/dashboard/customers",
	"/dashboard/customers/:customer_id",
	"/dashboard/customers/:customer_id/orders",
	"/dashboard/customers/:customer_id/orders/:order_id",
	"/dynamic-index/index",
	"/lion",
	"/lion/*",
	"/tiger",
	"/tiger/:tiger_id",
	"/tiger/:tiger_id/:tiger_cub_id",
	"/tiger/:tiger_id/*",

	// for when you don't care about dynamic params but still want to match exactly one segment
	"/a/b/:",
	"/c/d/e/:_",
	"/f/g/h/i/:/:",
	"/j/k/l/m/n/:_/:_",
];

interface TestNestedScenario {
	Path: string;
	ExpectedMatches: string[];
	SplatValues: string[] | null;
	Params: Params | null;
}

export const NestedScenarios: TestNestedScenario[] = [
	{
		Path: "/does-not-exist",
		SplatValues: ["does-not-exist"],
		ExpectedMatches: ["", "/*"],
		Params: null,
	},
	{
		Path: "/this-should-be-ignored",
		SplatValues: ["this-should-be-ignored"],
		ExpectedMatches: ["", "/*"],
		Params: null,
	},
	{
		Path: "/",
		ExpectedMatches: ["", "/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/lion",
		ExpectedMatches: ["", "/lion", "/lion/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/lion/123",
		SplatValues: ["123"],
		ExpectedMatches: ["", "/lion", "/lion/*"],
		Params: null,
	},
	{
		Path: "/lion/123/456",
		SplatValues: ["123", "456"],
		ExpectedMatches: ["", "/lion", "/lion/*"],
		Params: null,
	},
	{
		Path: "/lion/123/456/789",
		SplatValues: ["123", "456", "789"],
		ExpectedMatches: ["", "/lion", "/lion/*"],
		Params: null,
	},
	{
		Path: "/tiger",
		ExpectedMatches: ["", "/tiger", "/tiger/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/tiger/123",
		Params: { tiger_id: "123" },
		ExpectedMatches: [
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/",
		],
		SplatValues: null,
	},
	{
		Path: "/tiger/123/456",
		Params: { tiger_id: "123", tiger_cub_id: "456" },
		ExpectedMatches: [
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/:tiger_cub_id",
		],
		SplatValues: null,
	},
	{
		Path: "/tiger/123/456/789",
		Params: { tiger_id: "123" },
		SplatValues: ["456", "789"],
		ExpectedMatches: [
			"",
			"/tiger",
			"/tiger/:tiger_id",
			"/tiger/:tiger_id/*",
		],
	},
	{
		Path: "/bear",
		ExpectedMatches: ["", "/bear", "/bear/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/bear/123",
		Params: { bear_id: "123" },
		ExpectedMatches: ["", "/bear", "/bear/:bear_id"],
		SplatValues: null,
	},
	{
		Path: "/bear/123/456",
		Params: { bear_id: "123" },
		SplatValues: ["456"],
		ExpectedMatches: ["", "/bear", "/bear/:bear_id", "/bear/:bear_id/*"],
	},
	{
		Path: "/bear/123/456/789",
		Params: { bear_id: "123" },
		SplatValues: ["456", "789"],
		ExpectedMatches: ["", "/bear", "/bear/:bear_id", "/bear/:bear_id/*"],
	},
	{
		Path: "/dashboard",
		ExpectedMatches: ["", "/dashboard", "/dashboard/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/dashboard/asdf",
		SplatValues: ["asdf"],
		ExpectedMatches: ["", "/dashboard", "/dashboard/*"],
		Params: null,
	},
	{
		Path: "/dashboard/customers",
		ExpectedMatches: [
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/",
		],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/dashboard/customers/123",
		Params: { customer_id: "123" },
		ExpectedMatches: [
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/",
		],
		SplatValues: null,
	},
	{
		Path: "/dashboard/customers/123/orders",
		Params: { customer_id: "123" },
		ExpectedMatches: [
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/orders",
			"/dashboard/customers/:customer_id/orders/",
		],
		SplatValues: null,
	},
	{
		Path: "/dashboard/customers/123/orders/456",
		Params: { customer_id: "123", order_id: "456" },
		ExpectedMatches: [
			"",
			"/dashboard",
			"/dashboard/customers",
			"/dashboard/customers/:customer_id",
			"/dashboard/customers/:customer_id/orders",
			"/dashboard/customers/:customer_id/orders/:order_id",
		],
		SplatValues: null,
	},
	{
		Path: "/articles",
		ExpectedMatches: ["", "/articles/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/articles/bob",
		SplatValues: ["articles", "bob"],
		ExpectedMatches: ["", "/*"],
		Params: null,
	},
	{
		Path: "/articles/test",
		SplatValues: ["articles", "test"],
		ExpectedMatches: ["", "/*"],
		Params: null,
	},
	{
		Path: "/articles/test/articles",
		ExpectedMatches: ["", "/articles/test/articles/"],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/dynamic-index/index",
		ExpectedMatches: [
			"",
			// no underscore prefix, so not really an index!
			"/dynamic-index/index",
		],
		SplatValues: null,
		Params: null,
	},
	{
		Path: "/a/b/hi",
		ExpectedMatches: ["", "/a/b/:"],
		Params: { "": "hi" },
		SplatValues: null,
	},
	{
		Path: "/c/d/e/hi",
		ExpectedMatches: ["", "/c/d/e/:_"],
		Params: { _: "hi" },
		SplatValues: null,
	},
	{
		Path: "/f/g/h/i/hi/hi2",
		ExpectedMatches: ["", "/f/g/h/i/:/:"],
		Params: { "": "hi2" },
		SplatValues: null,
	},
	{
		Path: "/j/k/l/m/n/hi/hi2",
		ExpectedMatches: ["", "/j/k/l/m/n/:_/:_"],
		Params: { _: "hi2" },
		SplatValues: null,
	},
];

export const differentOptsToTest: (RegistrationOptions | undefined)[] = [
	undefined,
	{ explicitIndexSegment: "_index" },
	{ dynamicParamPrefixRune: "$" },
	{ splatSegmentRune: "#" },
	{
		explicitIndexSegment: "_______",
		dynamicParamPrefixRune: "<",
		splatSegmentRune: ">",
	},
	{
		explicitIndexSegment: "",
		dynamicParamPrefixRune: "<",
		splatSegmentRune: ">",
	},
];

// Helper functions
export function equalParams(a: Params | null, b: Params): boolean {
	// Consider nil and empty as the same
	if (
		(a === null || Object.keys(a).length === 0) &&
		Object.keys(b).length === 0
	) {
		return true;
	}
	if (a === null) {
		return false;
	}
	return JSON.stringify(a) === JSON.stringify(b);
}

export function equalSplat(a: string[] | null, b: string[]): boolean {
	// Consider nil and empty slice as the same
	if ((a === null || a.length === 0) && b.length === 0) {
		return true;
	}
	if (a === null) {
		return false;
	}
	return JSON.stringify(a) === JSON.stringify(b);
}

// Helper function to normalize a pattern for testing
function normalizePatternForTesting(props: {
	pattern: string;
	incomingIndexSegment: string;
}): Result<RegisteredPattern> {
	const { pattern, incomingIndexSegment } = props;
	// Create a temporary registry just for normalization
	const tempRegistryRes = createPatternRegistry({
		explicitIndexSegment: incomingIndexSegment,
	});

	if (!tempRegistryRes.ok) {
		throw new Error(
			`createPatternRegistry() error: ${tempRegistryRes.err}`,
		);
	}

	const tempRegistry = tempRegistryRes.val;

	// We need to access the normalized pattern, so we'll register it and return
	return registerPattern(tempRegistry, pattern);
}

export function modifyPatternsToOpts(
	incomingPatterns: string[],
	incomingIndexSegment: string,
	opts?: RegistrationOptions,
): string[] {
	const rps = incomingPatterns.map((p) => {
		const res = normalizePatternForTesting({
			pattern: p,
			incomingIndexSegment,
		});
		if (!res.ok) {
			throw new Error(`Error normalizing pattern "${p}": ${res.err}`);
		}
		return res.val;
	});
	const newPatterns: string[] = [];

	for (const rp of rps) {
		let pattern = "";
		for (const seg of rp.normalizedSegments) {
			pattern += "/";
			switch (seg.segType) {
				case "static":
					pattern += seg.normalizedVal;
					break;
				case "dynamic":
					pattern +=
						(opts?.dynamicParamPrefixRune || ":") +
						seg.normalizedVal.substring(1);
					break;
				case "splat":
					pattern += opts?.splatSegmentRune || "*";
					break;
				case "index":
					pattern += opts?.explicitIndexSegment || "";
					break;
			}
		}
		newPatterns.push(pattern);
	}

	return newPatterns;
}
