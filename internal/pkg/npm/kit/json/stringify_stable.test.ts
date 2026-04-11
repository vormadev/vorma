import { describe, expect, it } from "vitest";
import { jsonDeepEquals } from "./deep_equals.ts";
import { jsonStringifyStable } from "./stringify_stable.ts";

describe("jsonStringifyStable", () => {
	describe("basic functionality", () => {
		it("should stringify primitives correctly", () => {
			expect(must_jsonStringifyStable(42)).toBe("42");
			expect(must_jsonStringifyStable("hello")).toBe('"hello"');
			expect(must_jsonStringifyStable(true)).toBe("true");
			expect(must_jsonStringifyStable(null)).toBe("null");
		});

		it("should stringify arrays correctly", () => {
			expect(must_jsonStringifyStable([])).toBe("[]");
			expect(must_jsonStringifyStable([1, 2, 3])).toBe("[1,2,3]");
			expect(must_jsonStringifyStable(["a", "b", "c"])).toBe(
				'["a","b","c"]',
			);
		});

		it("should stringify objects correctly", () => {
			expect(must_jsonStringifyStable({})).toBe("{}");
			expect(must_jsonStringifyStable({ a: 1 })).toBe('{"a":1}');
			expect(must_jsonStringifyStable({ a: 1, b: 2 })).toBe(
				'{"a":1,"b":2}',
			);
		});
	});

	describe("deterministic object key ordering", () => {
		it("should produce consistent output regardless of key insertion order", () => {
			// Create objects with different insertion orders
			const obj1 = { a: 1, b: 2, c: 3 };

			// Create the same object but with different insertion order
			const obj2 = {} as Record<string, any>;
			obj2.c = 3;
			obj2.a = 1;
			obj2.b = 2;

			// Both should stringify to the same result
			expect(must_jsonStringifyStable(obj1)).toBe(
				must_jsonStringifyStable(obj2),
			);
			// And the keys should be alphabetically sorted
			expect(must_jsonStringifyStable(obj1)).toBe('{"a":1,"b":2,"c":3}');
		});

		it("should handle objects with numeric and non-alphanumeric keys", () => {
			const obj = {
				"2": "numeric",
				"1": "also numeric",
				_a: "underscore",
				a: "alpha",
			};
			expect(must_jsonStringifyStable(obj)).toBe(
				'{"1":"also numeric","2":"numeric","_a":"underscore","a":"alpha"}',
			);
		});
	});

	describe("nested structures", () => {
		it("should handle nested objects consistently", () => {
			const nested1 = { a: 1, b: { d: 4, c: 3 } };
			const nested2 = { a: 1, b: { c: 3, d: 4 } };

			expect(must_jsonStringifyStable(nested1)).toBe(
				must_jsonStringifyStable(nested2),
			);
			expect(must_jsonStringifyStable(nested1)).toBe(
				'{"a":1,"b":{"c":3,"d":4}}',
			);
		});

		it("should handle nested arrays consistently", () => {
			const arrObj1 = { a: [1, { c: 3, b: 2 }] };
			const arrObj2 = { a: [1, { b: 2, c: 3 }] };

			expect(must_jsonStringifyStable(arrObj1)).toBe(
				must_jsonStringifyStable(arrObj2),
			);
		});

		it("should handle deeply nested mixed structures", () => {
			const complex = {
				z: 26,
				a: 1,
				nested: {
					y: 25,
					x: [10, { c: 3, b: 2, a: 1 }],
					a: "first",
				},
				arr: [5, 4, 3, 2, 1],
			};

			// Expected output with keys sorted alphabetically at each level
			const expected =
				'{"a":1,"arr":[5,4,3,2,1],"nested":{"a":"first","x":[10,{"a":1,"b":2,"c":3}],"y":25},"z":26}';
			expect(must_jsonStringifyStable(complex)).toBe(expected);
		});
	});

	describe("edge cases", () => {
		it("should handle undefined values", () => {
			// In standard JSON.stringify, undefined becomes null in arrays and is omitted in objects
			expect(must_jsonStringifyStable([undefined])).toBe("[null]");
			expect(must_jsonStringifyStable({ a: undefined })).toBe("{}");
		});

		it("should handle special number values", () => {
			// NaN and Infinity become null in standard JSON
			expect(must_jsonStringifyStable(Number.NaN)).toBe("null");
			expect(must_jsonStringifyStable(Number.POSITIVE_INFINITY)).toBe(
				"null",
			);
			expect(must_jsonStringifyStable(-Number.POSITIVE_INFINITY)).toBe(
				"null",
			);
		});

		it("should handle empty slots in arrays", () => {
			// Create an array with empty slots
			const sparseArray = Array(3);
			sparseArray[0] = 1;
			sparseArray[2] = 3;

			// Empty slots should become null in JSON
			expect(must_jsonStringifyStable(sparseArray)).toBe("[1,null,3]");
		});

		it("should handle special characters in strings", () => {
			expect(must_jsonStringifyStable("Line1\nLine2")).toBe(
				'"Line1\\nLine2"',
			);
			expect(must_jsonStringifyStable("Tab\t")).toBe('"Tab\\t"');
			expect(must_jsonStringifyStable('Quote"')).toBe('"Quote\\""');
		});

		it("should handle circular references", () => {
			const circular: any = { name: "circular" };
			circular.self = circular;

			// Expect an error for circular references
			expect(jsonStringifyStable(circular).ok).toBe(false);
		});
	});

	describe("stability verification", () => {
		it("should produce identical output for equivalent objects with different property orders", () => {
			// Generate a bunch of equivalent objects with randomized property order
			const testCases = [
				{ obj1: { a: 1, b: 2, c: 3 }, obj2: { c: 3, a: 1, b: 2 } },
				{
					obj1: { foo: "bar", baz: [1, 2, 3] },
					obj2: { baz: [1, 2, 3], foo: "bar" },
				},
				{
					obj1: { a: { x: 1, y: 2 }, b: [3, 4] },
					obj2: { b: [3, 4], a: { y: 2, x: 1 } },
				},
			];

			for (const { obj1, obj2 } of testCases) {
				const str1 = must_jsonStringifyStable(obj1);
				const str2 = must_jsonStringifyStable(obj2);
				expect(str1).toBe(str2);
			}
		});

		it("should maintain array order", () => {
			// Arrays should maintain their order
			const arr1 = [3, 1, 2];
			const arr2 = [3, 1, 2]; // Same order
			const arr3 = [1, 2, 3]; // Different order

			expect(must_jsonStringifyStable(arr1)).toBe(
				must_jsonStringifyStable(arr2),
			);
			expect(must_jsonStringifyStable(arr1)).not.toBe(
				must_jsonStringifyStable(arr3),
			);
		});

		// The critical test that's missing: objects within arrays should have stable key order
		it("should stabilize key order in objects within arrays", () => {
			// First array with objects that have keys in different orders
			const arr1 = [
				{ id: 1, name: "Alice" },
				{ id: 2, name: "Bob" },
			];

			// Second array with the same objects but keys in different order
			const arr2 = [
				{ name: "Alice", id: 1 },
				{ name: "Bob", id: 2 },
			];

			const str1 = must_jsonStringifyStable(arr1);
			const str2 = must_jsonStringifyStable(arr2);

			// This should pass if the function properly stabilizes object keys at all levels
			expect(str1).toBe(str2);
		});

		// Additional test for objects in nested arrays
		it("should stabilize key order in deeply nested array structures", () => {
			const nested1 = {
				data: [
					[
						{ x: 1, y: 2 },
						{ a: 3, b: 4 },
					],
					[{ c: 5, d: 6 }],
				],
			};

			const nested2 = {
				data: [
					[
						{ y: 2, x: 1 },
						{ b: 4, a: 3 },
					],
					[{ d: 6, c: 5 }],
				],
			};

			const str1 = must_jsonStringifyStable(nested1);
			const str2 = must_jsonStringifyStable(nested2);

			expect(str1).toBe(str2);
		});

		// Test for mixed array types
		it("should handle mixed arrays with various types", () => {
			const mixed1 = [
				1,
				"string",
				{ obj1: "value1", obj2: "value2" },
				[{ nested1: true, nested2: false }],
			];

			const mixed2 = [
				1,
				"string",
				{ obj2: "value2", obj1: "value1" },
				[{ nested2: false, nested1: true }],
			];

			const str1 = must_jsonStringifyStable(mixed1);
			const str2 = must_jsonStringifyStable(mixed2);

			expect(str1).toBe(str2);
		});

		// Test with large number of nested properties
		it("should handle objects with many nested properties", () => {
			const large1: Record<string, any> = {};
			const large2: Record<string, any> = {};

			// Create objects with 100 properties in different orders
			for (let i = 0; i < 100; i++) {
				large1[`prop${i}`] = { value: i };
			}

			// Add properties in reverse order
			for (let i = 99; i >= 0; i--) {
				large2[`prop${i}`] = { value: i };
			}

			const str1 = must_jsonStringifyStable(large1);
			const str2 = must_jsonStringifyStable(large2);

			expect(str1).toBe(str2);
		});
	});

	describe("functional verification", () => {
		it("should round-trip data correctly", () => {
			const testObjects = [
				42,
				"hello",
				true,
				[1, 2, 3],
				{ a: 1, b: "two", c: true },
				{ nested: { objects: [1, 2, { x: "y" }] } },
			];

			for (const obj of testObjects) {
				const jsonString = must_jsonStringifyStable(obj);
				const parsedBack = JSON.parse(jsonString);
				expect(jsonDeepEquals(obj, parsedBack)).toBe(true);
			}
		});
	});

	describe("performance considerations", () => {
		it("should handle large objects", () => {
			// Create a moderately large object
			const largeObj = {};
			for (let i = 0; i < 1000; i++) {
				// @ts-ignore
				largeObj[`key${i}`] = i;
			}

			// This just verifies it can stringify without errors
			expect(jsonStringifyStable(largeObj).ok).toBe(true);
		});

		it("should handle large arrays", () => {
			const largeArray = Array(10000)
				.fill(0)
				.map((_unusedValue: number, index: number) => {
					return index;
				});
			expect(jsonStringifyStable(largeArray).ok).toBe(true);
		});
	});
});

function must_jsonStringifyStable(input: unknown): string {
	const res = jsonStringifyStable(input);
	if (!res.ok) {
		throw new Error(
			`jsonStringifyStable failed for input: ${JSON.stringify(
				input,
			)} with error: ${res.err}`,
		);
	}
	return res.val;
}
