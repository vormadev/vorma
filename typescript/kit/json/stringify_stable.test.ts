import { describe, expect, it } from "vitest";
import { jsonDeepEquals } from "./deep_equals.ts";
import { jsonStringifyStable } from "./stringify_stable.ts";

describe("jsonStringifyStable", () => {
	describe("basic functionality", () => {
		it("should stringify primitives correctly", () => {
			expect(jsonStringifyStable(42)).toBe("42");
			expect(jsonStringifyStable("hello")).toBe('"hello"');
			expect(jsonStringifyStable(true)).toBe("true");
			expect(jsonStringifyStable(null)).toBe("null");
		});

		it("should stringify arrays correctly", () => {
			expect(jsonStringifyStable([])).toBe("[]");
			expect(jsonStringifyStable([1, 2, 3])).toBe("[1,2,3]");
			expect(jsonStringifyStable(["a", "b", "c"])).toBe('["a","b","c"]');
		});

		it("should stringify objects correctly", () => {
			expect(jsonStringifyStable({})).toBe("{}");
			expect(jsonStringifyStable({ a: 1 })).toBe('{"a":1}');
			expect(jsonStringifyStable({ a: 1, b: 2 })).toBe('{"a":1,"b":2}');
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
			expect(jsonStringifyStable(obj1)).toBe(jsonStringifyStable(obj2));
			// And the keys should be alphabetically sorted
			expect(jsonStringifyStable(obj1)).toBe('{"a":1,"b":2,"c":3}');
		});

		it("should handle objects with numeric and non-alphanumeric keys", () => {
			const obj = {
				"2": "numeric",
				"1": "also numeric",
				_a: "underscore",
				a: "alpha",
			};
			expect(jsonStringifyStable(obj)).toBe(
				'{"1":"also numeric","2":"numeric","_a":"underscore","a":"alpha"}',
			);
		});
	});

	describe("nested structures", () => {
		it("should handle nested objects consistently", () => {
			const nested1 = { a: 1, b: { d: 4, c: 3 } };
			const nested2 = { a: 1, b: { c: 3, d: 4 } };

			expect(jsonStringifyStable(nested1)).toBe(
				jsonStringifyStable(nested2),
			);
			expect(jsonStringifyStable(nested1)).toBe(
				'{"a":1,"b":{"c":3,"d":4}}',
			);
		});

		it("should handle nested arrays consistently", () => {
			const arrObj1 = { a: [1, { c: 3, b: 2 }] };
			const arrObj2 = { a: [1, { b: 2, c: 3 }] };

			expect(jsonStringifyStable(arrObj1)).toBe(
				jsonStringifyStable(arrObj2),
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
			expect(jsonStringifyStable(complex)).toBe(expected);
		});
	});

	describe("edge cases", () => {
		it("should handle undefined values", () => {
			// In standard JSON.stringify, undefined becomes null in arrays and is omitted in objects
			expect(jsonStringifyStable([undefined])).toBe("[null]");
			expect(jsonStringifyStable({ a: undefined })).toBe("{}");
		});

		it("should handle special number values", () => {
			// NaN and Infinity become null in standard JSON
			expect(jsonStringifyStable(Number.NaN)).toBe("null");
			expect(jsonStringifyStable(Number.POSITIVE_INFINITY)).toBe("null");
			expect(jsonStringifyStable(-Number.POSITIVE_INFINITY)).toBe("null");
		});

		it("should handle empty slots in arrays", () => {
			// Create an array with empty slots
			const sparseArray = Array(3);
			sparseArray[0] = 1;
			sparseArray[2] = 3;

			// Empty slots should become null in JSON
			expect(jsonStringifyStable(sparseArray)).toBe("[1,null,3]");
		});

		it("should handle special characters in strings", () => {
			expect(jsonStringifyStable("Line1\nLine2")).toBe('"Line1\\nLine2"');
			expect(jsonStringifyStable("Tab\t")).toBe('"Tab\\t"');
			expect(jsonStringifyStable('Quote"')).toBe('"Quote\\""');
		});

		it("should handle circular references", () => {
			const circular: any = { name: "circular" };
			circular.self = circular;

			// Expect an error for circular references
			expect(() => jsonStringifyStable(circular)).toThrow();
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
				const str1 = jsonStringifyStable(obj1);
				const str2 = jsonStringifyStable(obj2);
				expect(str1).toBe(str2);
			}
		});

		it("should maintain array order", () => {
			// Arrays should maintain their order
			const arr1 = [3, 1, 2];
			const arr2 = [3, 1, 2]; // Same order
			const arr3 = [1, 2, 3]; // Different order

			expect(jsonStringifyStable(arr1)).toBe(jsonStringifyStable(arr2));
			expect(jsonStringifyStable(arr1)).not.toBe(
				jsonStringifyStable(arr3),
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

			const str1 = jsonStringifyStable(arr1);
			const str2 = jsonStringifyStable(arr2);

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

			const str1 = jsonStringifyStable(nested1);
			const str2 = jsonStringifyStable(nested2);

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

			const str1 = jsonStringifyStable(mixed1);
			const str2 = jsonStringifyStable(mixed2);

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

			const str1 = jsonStringifyStable(large1);
			const str2 = jsonStringifyStable(large2);

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
				const jsonString = jsonStringifyStable(obj);
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
			expect(() => jsonStringifyStable(largeObj)).not.toThrow();
		});

		it("should handle large arrays", () => {
			const largeArray = Array(10000)
				.fill(0)
				.map((_unusedValue: number, index: number) => index);
			expect(() => jsonStringifyStable(largeArray)).not.toThrow();
		});
	});
});
