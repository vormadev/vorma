import { describe, expect, it } from "vitest";
import {
	SEARCH_PARAM_SCHEMA_BOOL,
	SEARCH_PARAM_SCHEMA_MAP,
	SEARCH_PARAM_SCHEMA_NUMBER,
	SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX,
	SEARCH_PARAM_SCHEMA_STRING,
	type SearchParamSchema,
	parseSearchParams,
} from "./search_param_parser.ts";
import { serializeToSearchParams } from "./search_param_serializer.ts";

describe("URLSearchParams Parser", () => {
	it("parses compact scalar schemas", () => {
		const params = serializeToSearchParams({
			active: true,
			page: 2,
			q: "ada",
		});
		const schema = {
			active: SEARCH_PARAM_SCHEMA_BOOL,
			page: SEARCH_PARAM_SCHEMA_NUMBER,
			q: SEARCH_PARAM_SCHEMA_STRING,
		};

		expect(parseSearchParams(schema, params)).toEqual({
			active: true,
			page: 2,
			q: "ada",
		});
	});

	it("restores missing non-pointer zero values", () => {
		const schema = {
			active: SEARCH_PARAM_SCHEMA_BOOL,
			page: SEARCH_PARAM_SCHEMA_NUMBER,
			q: SEARCH_PARAM_SCHEMA_STRING,
			tags: [SEARCH_PARAM_SCHEMA_STRING],
		};

		expect(parseSearchParams(schema, new URLSearchParams())).toEqual({
			active: false,
			page: 0,
			q: "",
			tags: [],
		});
	});

	it("omits missing optional scalar values", () => {
		const params = serializeToSearchParams({
			optionalPage: null,
			q: "ada",
		});
		const schema = {
			optionalPage:
				SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_NUMBER,
			q: SEARCH_PARAM_SCHEMA_STRING,
		};

		expect(parseSearchParams(schema, params)).toEqual({
			q: "ada",
		});
	});

	it("omits explicit empty optional scalar values", () => {
		const schema = {
			active: SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_BOOL,
			page: SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_NUMBER,
			q: SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_STRING,
		};
		const params = new URLSearchParams("active=&page=&q=");

		expect(parseSearchParams(schema, params)).toEqual({});
	});

	it("uses only the first repeated scalar value", () => {
		const schema = {
			active: SEARCH_PARAM_SCHEMA_BOOL,
			page: SEARCH_PARAM_SCHEMA_NUMBER,
			q: SEARCH_PARAM_SCHEMA_STRING,
		};
		const params = new URLSearchParams(
			"active=false&active=true&page=2&page=3&q=first&q=second",
		);

		expect(parseSearchParams(schema, params)).toEqual({
			active: false,
			page: 2,
			q: "first",
		});
	});

	it("parses arrays and nested objects", () => {
		const params = serializeToSearchParams({
			address: { city: "Austin", zip: 78701 },
			scores: [1, 2, 3],
		});
		const schema = {
			address: {
				city: SEARCH_PARAM_SCHEMA_STRING,
				zip: SEARCH_PARAM_SCHEMA_NUMBER,
			},
			scores: [SEARCH_PARAM_SCHEMA_NUMBER],
		};

		expect(parseSearchParams(schema, params)).toEqual({
			address: { city: "Austin", zip: 78701 },
			scores: [1, 2, 3],
		});
	});

	it("parses deeply nested objects", () => {
		const params = serializeToSearchParams({
			level1: {
				level2: {
					level3: {
						field: "value",
					},
				},
			},
		});
		const schema = {
			level1: {
				level2: {
					level3: {
						field: SEARCH_PARAM_SCHEMA_STRING,
					},
				},
			},
		};

		expect(parseSearchParams(schema, params)).toEqual({
			level1: {
				level2: {
					level3: {
						field: "value",
					},
				},
			},
		});
	});

	it("drops empty array entries", () => {
		const schema = {
			bools: [SEARCH_PARAM_SCHEMA_BOOL],
			numbers: [SEARCH_PARAM_SCHEMA_NUMBER],
			strings: [SEARCH_PARAM_SCHEMA_STRING],
		};
		const params = new URLSearchParams(
			"bools=&bools=true&numbers=&numbers=42&strings=&strings=value",
		);

		expect(parseSearchParams(schema, params)).toEqual({
			bools: [true],
			numbers: [42],
			strings: ["value"],
		});
	});

	it("parses maps with scalar and array values", () => {
		const params = serializeToSearchParams({
			flags: { beta: true, dev: false },
			groups: { a: [1, 2], b: [3] },
		});
		const schema = {
			flags: [SEARCH_PARAM_SCHEMA_MAP, SEARCH_PARAM_SCHEMA_BOOL],
			groups: [SEARCH_PARAM_SCHEMA_MAP, [SEARCH_PARAM_SCHEMA_NUMBER]],
		};

		expect(parseSearchParams(schema, params)).toEqual({
			flags: { beta: true, dev: false },
			groups: { a: [1, 2], b: [3] },
		});
	});

	it("does not let map prefixes catch sibling fields", () => {
		const schema = {
			data: [SEARCH_PARAM_SCHEMA_MAP, SEARCH_PARAM_SCHEMA_STRING],
			database: SEARCH_PARAM_SCHEMA_STRING,
		};
		const params = new URLSearchParams(
			"data.key=value&database=postgres&database.extra=ignored",
		);

		expect(parseSearchParams(schema, params)).toEqual({
			data: { key: "value" },
			database: "postgres",
		});
	});

	it("parses repeated map array values once per map key", () => {
		const schema = {
			data: [SEARCH_PARAM_SCHEMA_MAP, [SEARCH_PARAM_SCHEMA_STRING]],
		};
		const params = new URLSearchParams(
			"data.tags=go&data.tags=test&data.scores=85&data.scores=90",
		);

		expect(parseSearchParams(schema, params)).toEqual({
			data: {
				scores: ["85", "90"],
				tags: ["go", "test"],
			},
		});
	});

	it("omits empty optional map values", () => {
		const schema = {
			data: [
				SEARCH_PARAM_SCHEMA_MAP,
				SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_STRING,
			],
		};
		const params = new URLSearchParams("data.empty=&data.name=Jane");

		expect(parseSearchParams(schema, params)).toEqual({
			data: {
				name: "Jane",
			},
		});
	});

	it("parses pointer complex values as present zero-ish structures", () => {
		const schema = {
			address: {
				city: SEARCH_PARAM_SCHEMA_STRING,
				zip: SEARCH_PARAM_SCHEMA_NUMBER,
			},
			labels: [SEARCH_PARAM_SCHEMA_MAP, SEARCH_PARAM_SCHEMA_STRING],
			tags: [SEARCH_PARAM_SCHEMA_STRING],
		};

		expect(parseSearchParams(schema, new URLSearchParams())).toEqual({
			address: { city: "", zip: 0 },
			labels: {},
			tags: [],
		});
	});

	it("restores mixed empty query semantics", () => {
		const schema = {
			age: SEARCH_PARAM_SCHEMA_NUMBER,
			age_ptr: SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_NUMBER,
			is_fun: SEARCH_PARAM_SCHEMA_BOOL,
			is_fun_ptr: SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_BOOL,
			name: SEARCH_PARAM_SCHEMA_STRING,
			name_ptr: SEARCH_PARAM_SCHEMA_OPTIONAL_PREFIX + SEARCH_PARAM_SCHEMA_STRING,
			someMap: [SEARCH_PARAM_SCHEMA_MAP, SEARCH_PARAM_SCHEMA_STRING],
			someMap_ptr: [SEARCH_PARAM_SCHEMA_MAP, SEARCH_PARAM_SCHEMA_STRING],
			someStruct: {
				field: SEARCH_PARAM_SCHEMA_STRING,
			},
			someStruct_ptr: {
				field: SEARCH_PARAM_SCHEMA_STRING,
			},
			tags: [SEARCH_PARAM_SCHEMA_STRING],
			tags_ptr: [SEARCH_PARAM_SCHEMA_STRING],
		};
		const params = new URLSearchParams(
			"age=0&age_ptr=&is_fun=false&is_fun_ptr=&name=&name_ptr=&someMap=&someMap_ptr=&someStruct.field=&someStruct_ptr.field=&tags=&tags_ptr=",
		);

		expect(parseSearchParams(schema, params)).toEqual({
			age: 0,
			is_fun: false,
			name: "",
			someMap: {},
			someMap_ptr: {},
			someStruct: { field: "" },
			someStruct_ptr: { field: "" },
			tags: [],
			tags_ptr: [],
		});
	});

	it("parses common bool accepted values", () => {
		const schema = {
			a: SEARCH_PARAM_SCHEMA_BOOL,
			b: SEARCH_PARAM_SCHEMA_BOOL,
			c: SEARCH_PARAM_SCHEMA_BOOL,
			d: SEARCH_PARAM_SCHEMA_BOOL,
			e: SEARCH_PARAM_SCHEMA_BOOL,
			f: SEARCH_PARAM_SCHEMA_BOOL,
		};
		const params = new URLSearchParams("a=1&b=t&c=T&d=TRUE&e=true&f=True");

		expect(parseSearchParams(schema, params)).toEqual({
			a: true,
			b: true,
			c: true,
			d: true,
			e: true,
			f: true,
		});
	});

	it("parses non-true bool values as false", () => {
		const schema = {
			a: SEARCH_PARAM_SCHEMA_BOOL,
			b: SEARCH_PARAM_SCHEMA_BOOL,
			c: SEARCH_PARAM_SCHEMA_BOOL,
			d: SEARCH_PARAM_SCHEMA_BOOL,
			e: SEARCH_PARAM_SCHEMA_BOOL,
		};
		const params = new URLSearchParams("a=0&b=f&c=FALSE&d=false&e=nope");

		expect(parseSearchParams(schema, params)).toEqual({
			a: false,
			b: false,
			c: false,
			d: false,
			e: false,
		});
	});

	it("documents number parsing edge behavior", () => {
		const schema = {
			blank: SEARCH_PARAM_SCHEMA_NUMBER,
			decimal: SEARCH_PARAM_SCHEMA_NUMBER,
			invalid: SEARCH_PARAM_SCHEMA_NUMBER,
			negative: SEARCH_PARAM_SCHEMA_NUMBER,
		};
		const params = new URLSearchParams(
			"blank=&decimal=1.75&invalid=notanumber&negative=-42",
		);

		expect(parseSearchParams(schema, params)).toEqual({
			blank: 0,
			decimal: 1.75,
			invalid: Number.NaN,
			negative: -42,
		});
	});

	it("keeps dots inside scalar map keys", () => {
		const params = serializeToSearchParams({
			data: { "key.with.dot": "value" },
		});
		const schema = {
			data: [SEARCH_PARAM_SCHEMA_MAP, SEARCH_PARAM_SCHEMA_STRING],
		};

		expect(parseSearchParams(schema, params)).toEqual({
			data: { "key.with.dot": "value" },
		});
	});

	it("returns an empty object for nullish root schema", () => {
		const params = serializeToSearchParams({
			q: "ignored",
		});

		expect(parseSearchParams(null, params)).toEqual({});
		expect(parseSearchParams(undefined, params)).toEqual({});
	});

	it("keeps malformed array schemas stable", () => {
		const params = new URLSearchParams("data=value");

		expect(parseSearchParams([], params)).toEqual([]);
		expect(
			parseSearchParams(
				[SEARCH_PARAM_SCHEMA_STRING, SEARCH_PARAM_SCHEMA_NUMBER],
				params,
			),
		).toEqual([]);
	});

	it("keeps non-scalar array element schemas stable", () => {
		const schema: SearchParamSchema = [
			{
				q: SEARCH_PARAM_SCHEMA_STRING,
			},
		];
		const params = new URLSearchParams("data=value");

		expect(parseSearchParams(schema, params)).toEqual([]);
	});

	it("treats unknown scalar codes as strings", () => {
		const schema = {
			q: "unknown",
		};
		const params = new URLSearchParams("q=value");

		expect(parseSearchParams(schema, params)).toEqual({
			q: "value",
		});
	});
});
