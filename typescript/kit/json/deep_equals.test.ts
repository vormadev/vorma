import { describe, expect, it } from "vitest";
import { jsonDeepEquals } from "./deep_equals.ts";

describe("jsonDeepEquals", () => {
	describe("primitives", () => {
		it("should handle numbers", () => {
			expect(jsonDeepEquals(1, 1)).toBe(true);
			expect(jsonDeepEquals(1, 2)).toBe(false);
			expect(jsonDeepEquals(0, -0)).toBe(true);
			expect(jsonDeepEquals(Number.NaN, Number.NaN)).toBe(false); // NaN !== NaN
		});

		it("should handle strings", () => {
			expect(jsonDeepEquals("hello", "hello")).toBe(true);
			expect(jsonDeepEquals("hello", "world")).toBe(false);
			expect(jsonDeepEquals("", "")).toBe(true);
		});

		it("should handle booleans", () => {
			expect(jsonDeepEquals(true, true)).toBe(true);
			expect(jsonDeepEquals(false, false)).toBe(true);
			expect(jsonDeepEquals(true, false)).toBe(false);
		});

		it("should handle null and undefined", () => {
			expect(jsonDeepEquals(null, null)).toBe(true);
			expect(jsonDeepEquals(undefined, undefined)).toBe(true);
			expect(jsonDeepEquals(null, undefined)).toBe(false);
			expect(jsonDeepEquals(undefined, null)).toBe(false);
		});
	});

	describe("arrays", () => {
		it("should handle empty arrays", () => {
			expect(jsonDeepEquals([], [])).toBe(true);
		});

		it("should handle simple arrays", () => {
			expect(jsonDeepEquals([1, 2, 3], [1, 2, 3])).toBe(true);
			expect(jsonDeepEquals([1, 2, 3], [1, 2, 4])).toBe(false);
			expect(jsonDeepEquals([1, 2], [1, 2, 3])).toBe(false);
		});

		it("should handle nested arrays", () => {
			expect(jsonDeepEquals([1, [2, 3]], [1, [2, 3]])).toBe(true);
			expect(jsonDeepEquals([1, [2, 3]], [1, [2, 4]])).toBe(false);
			expect(jsonDeepEquals([1, [2, [3]]], [1, [2, [3]]])).toBe(true);
		});

		it("should handle arrays with mixed types", () => {
			expect(jsonDeepEquals([1, "two", true], [1, "two", true])).toBe(true);
			expect(jsonDeepEquals([1, "two", true], [1, "two", false])).toBe(false);
		});
	});

	describe("objects", () => {
		it("should handle empty objects", () => {
			expect(jsonDeepEquals({}, {})).toBe(true);
		});

		it("should handle simple objects", () => {
			expect(jsonDeepEquals({ a: 1, b: 2 }, { a: 1, b: 2 })).toBe(true);
			expect(jsonDeepEquals({ a: 1, b: 2 }, { a: 1, b: 3 })).toBe(false);
			expect(jsonDeepEquals({ a: 1 }, { a: 1, b: 2 })).toBe(false);
		});

		it("should handle nested objects", () => {
			expect(jsonDeepEquals({ a: { b: 1 } }, { a: { b: 1 } })).toBe(true);
			expect(jsonDeepEquals({ a: { b: 1 } }, { a: { b: 2 } })).toBe(false);
			expect(jsonDeepEquals({ a: { b: { c: 3 } } }, { a: { b: { c: 3 } } })).toBe(
				true,
			);
		});

		it("should handle objects with arrays", () => {
			expect(jsonDeepEquals({ a: [1, 2], b: 3 }, { a: [1, 2], b: 3 })).toBe(true);
			expect(jsonDeepEquals({ a: [1, 2], b: 3 }, { a: [1, 3], b: 3 })).toBe(false);
		});
	});

	describe("mixed complex structures", () => {
		it("should handle complex nested structures", () => {
			const complex1 = {
				a: [1, { b: 2 }],
				c: {
					d: [3, 4],
					e: { f: 5 },
				},
			};
			const complex2 = {
				a: [1, { b: 2 }],
				c: {
					d: [3, 4],
					e: { f: 5 },
				},
			};
			expect(jsonDeepEquals(complex1, complex2)).toBe(true);

			const complex3 = {
				a: [1, { b: 2 }],
				c: {
					d: [3, 4],
					e: { f: 6 }, // Different value
				},
			};
			expect(jsonDeepEquals(complex1, complex3)).toBe(false);
		});
	});

	describe("type comparisons", () => {
		it("should handle different types", () => {
			expect(jsonDeepEquals([], {})).toBe(false);
			expect(jsonDeepEquals(1, "1")).toBe(false);
			expect(jsonDeepEquals(true, 1)).toBe(false);
			expect(jsonDeepEquals([1], { 0: 1 })).toBe(false);
		});
	});

	describe("edge cases", () => {
		it("should handle empty values correctly", () => {
			expect(jsonDeepEquals({}, [])).toBe(false);
			expect(jsonDeepEquals(null, {})).toBe(false);
			expect(jsonDeepEquals(undefined, [])).toBe(false);
		});

		it("should handle objects with undefined values", () => {
			expect(jsonDeepEquals({ a: undefined }, { a: undefined })).toBe(true);
			expect(jsonDeepEquals({ a: undefined }, { b: undefined })).toBe(false);
		});

		it("should handle arrays with undefined values", () => {
			expect(jsonDeepEquals([undefined], [undefined])).toBe(true);
			expect(jsonDeepEquals([undefined, 1], [undefined, 2])).toBe(false);
		});
	});
});
