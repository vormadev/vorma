import { describe, expect, it } from "vitest";
import { serializeToSearchParams } from "./search_param_serializer.ts";

describe("URLSearchParams Serializer", () => {
	// Basic Types
	it("Basic types", () => {
		const input = { name: "John", age: 30, active: true, height: 1.75 };
		expect(serializeToSearchParams(input).toString()).toBe(
			"active=true&age=30&height=1.75&name=John",
		);
	});

	it("Slice of strings", () => {
		const input = { tags: ["go", "programming", "test"] };
		expect(serializeToSearchParams(input).toString()).toBe(
			"tags=go&tags=programming&tags=test",
		);
	});

	it("Mixed types", () => {
		const input = { name: "Alice", age: 25, scores: [90, 85, 95] };
		expect(serializeToSearchParams(input).toString()).toBe(
			"age=25&name=Alice&scores=90&scores=85&scores=95",
		);
	});

	// Pointer Fields (simulated with null in TypeScript)
	it("Pointer fields", () => {
		const input = { name: "Jane", age: 28, salary: 50000.5, isEmployee: true };
		expect(serializeToSearchParams(input).toString()).toBe(
			"age=28&isEmployee=true&name=Jane&salary=50000.5",
		);
	});

	it("Nil pointer fields", () => {
		const input = { name: "John", age: 30, salary: null };
		expect(serializeToSearchParams(input).toString()).toBe("age=30&name=John&salary=");
	});

	it("Slice of pointers", () => {
		const input = { scores: [90, 85, 95] };
		expect(serializeToSearchParams(input).toString()).toBe(
			"scores=90&scores=85&scores=95",
		);
	});

	it("Empty values -- pointers", () => {
		const input = { name: null, age: null, active: null };
		expect(serializeToSearchParams(input).toString()).toBe("active=&age=&name=");
	});

	it("Empty values -- non-pointers", () => {
		const input = { name: "", age: 0, active: false };
		expect(serializeToSearchParams(input).toString()).toBe("active=false&age=0&name=");
	});

	// Nested Structs
	it("Nested structs", () => {
		const input = { name: "John", address: { city: "NewYork", zip: 10001 } };
		expect(serializeToSearchParams(input).toString()).toBe(
			"address.city=NewYork&address.zip=10001&name=John",
		);
	});

	it("Double nested structs", () => {
		const input = {
			name: "John",
			address: {
				city: "NewYork",
				zip: 10001,
				location: { lat: 40.7128, lng: -74.006 },
			},
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"address.city=NewYork&address.location.lat=40.7128&address.location.lng=-74.006&address.zip=10001&name=John",
		);
	});

	it("Nested struct with slice", () => {
		const input = {
			name: "John",
			address: {
				city: "NewYork",
				zip: 10001,
				phones: ["1234567890", "0987654321"],
			},
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"address.city=NewYork&address.phones=1234567890&address.phones=0987654321&address.zip=10001&name=John",
		);
	});

	// Embedded Structs
	it("Embedded structs", () => {
		const input = { embeddedField: "embeddedValue" };
		expect(serializeToSearchParams(input).toString()).toBe(
			"embeddedField=embeddedValue",
		);
	});

	it("Double embedded structs", () => {
		const input = {
			embeddedField: "embeddedValue",
			embeddedField2: "embeddedValue2",
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"embeddedField=embeddedValue&embeddedField2=embeddedValue2",
		);
	});

	// Maps
	it("Basic map", () => {
		const input = { data: { key1: "value1", key2: "value2" } };
		expect(serializeToSearchParams(input).toString()).toBe(
			"data.key1=value1&data.key2=value2",
		);
	});

	it("Map with slice values", () => {
		const input = {
			data: { tags: ["go", "programming"], scores: ["85", "90"] },
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"data.scores=85&data.scores=90&data.tags=go&data.tags=programming",
		);
	});

	it("Empty map", () => {
		const input = { data: {} };
		expect(serializeToSearchParams(input).toString()).toBe("data=");
	});

	it("Map with empty values", () => {
		const input = { data: { key1: "", key2: "" } };
		expect(serializeToSearchParams(input).toString()).toBe("data.key1=&data.key2=");
	});

	it("Map with pointer values", () => {
		const input = { data: { name: "John", age: "30", active: "true" } };
		expect(serializeToSearchParams(input).toString()).toBe(
			"data.active=true&data.age=30&data.name=John",
		);
	});

	it("Struct with multiple maps of different value types", () => {
		const input = {
			stringMap: { key1: "value1" },
			intMap: { key2: 42 },
			boolMap: { key3: true },
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"boolMap.key3=true&intMap.key2=42&stringMap.key1=value1",
		);
	});

	it("Map key with dot", () => {
		const input = { data: { "key.with.dot": "value" } };
		expect(serializeToSearchParams(input).toString()).toBe("data.key.with.dot=value");
	});

	// Pointers to Complex Types
	it("Basic map pointer", () => {
		const input = { data: { key1: "value1", key2: "value2" } };
		expect(serializeToSearchParams(input).toString()).toBe(
			"data.key1=value1&data.key2=value2",
		);
	});

	it("Basic struct pointer", () => {
		const input = { data: { key1: "value1", key2: "value2" } };
		expect(serializeToSearchParams(input).toString()).toBe(
			"data.key1=value1&data.key2=value2",
		);
	});

	it("Basic slice pointer", () => {
		const input = { data: ["value1", "value2"] };
		expect(serializeToSearchParams(input).toString()).toBe("data=value1&data=value2");
	});

	// Misc
	it("Triple nested structs", () => {
		const input = { level1: { level2: { level3: { field: "value" } } } };
		expect(serializeToSearchParams(input).toString()).toBe(
			"level1.level2.level3.field=value",
		);
	});

	it("Empty query parameters", () => {
		const input = {
			name_ptr: null,
			name: "",
			age_ptr: null,
			age: 0,
			tags_ptr: [],
			tags: [],
			someStruct_ptr: { field: "" },
			someStruct: { field: "" },
			someMap_ptr: {},
			someMap: {},
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"age=0&age_ptr=&name=&name_ptr=&someMap=&someMap_ptr=&someStruct.field=&someStruct_ptr.field=&tags=&tags_ptr=",
		);
	});

	// Ordering
	it("Sorted top-level keys", () => {
		const input = { b: 2, a: 1, c: 3 };
		expect(serializeToSearchParams(input).toString()).toBe("a=1&b=2&c=3");
	});

	it("Sorted nested keys", () => {
		const input = { c: { y: "yes", x: "no" }, a: 1 };
		expect(serializeToSearchParams(input).toString()).toBe("a=1&c.x=no&c.y=yes");
	});

	it("Empty query parameters with sorting", () => {
		const input = {
			name_ptr: null,
			name: "",
			age_ptr: null,
			age: 0,
			tags_ptr: [],
			tags: [],
			someStruct_ptr: { field: "" },
			someStruct: { field: "" },
			someMap_ptr: {},
			someMap: {},
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"age=0&age_ptr=&name=&name_ptr=&someMap=&someMap_ptr=&someStruct.field=&someStruct_ptr.field=&tags=&tags_ptr=",
		);
	});

	it("Array order preserved", () => {
		const input = { tags: ["go", "test"], a: 1 };
		expect(serializeToSearchParams(input).toString()).toBe("a=1&tags=go&tags=test");
	});

	it("Encodes URI properly", () => {
		const input = {
			"a&b": "c d#e",
			nested: { "x/y": ["f g", null] },
			empty: [],
			unicode: "🚀",
		};
		expect(serializeToSearchParams(input).toString()).toBe(
			"a%26b=c+d%23e&empty=&nested.x%2Fy=f+g&nested.x%2Fy=&unicode=%F0%9F%9A%80",
		);
	});
});
