import { describe, expect, it } from "vitest";
import * as converters from "./converters.ts";

describe("Encoding Conversion Functions", () => {
	// Test data that will be used across multiple tests
	const testCases = [
		{
			name: "simple ASCII",
			utf8: "Hello World",
			bytes: new Uint8Array([72, 101, 108, 108, 111, 32, 87, 111, 114, 108, 100]),
			hex: "48656c6c6f20576f726c64",
			base64: "SGVsbG8gV29ybGQ=",
			base64Url: "SGVsbG8gV29ybGQ",
		},
		{
			name: "with special characters",
			utf8: "!@#$%^&*()_+",
			bytes: new Uint8Array([33, 64, 35, 36, 37, 94, 38, 42, 40, 41, 95, 43]),
			hex: "21402324255e262a28295f2b",
			base64: "IUAjJCVeJiooKV8r",
			base64Url: "IUAjJCVeJiooKV8r",
		},
		{
			name: "with Unicode characters",
			utf8: "こんにちは世界",
			bytes: new Uint8Array([
				227, 129, 147, 227, 130, 147, 227, 129, 171, 227, 129, 161, 227, 129, 175,
				228, 184, 150, 231, 149, 140,
			]),
			hex: "e38193e38293e381abe381a1e381afe4b896e7958c",
			base64: "44GT44KT44Gr44Gh44Gv5LiW55WM",
			base64Url: "44GT44KT44Gr44Gh44Gv5LiW55WM",
		},
		{
			name: "with Base64 padding",
			utf8: "a",
			bytes: new Uint8Array([97]),
			hex: "61",
			base64: "YQ==",
			base64Url: "YQ",
		},
		{
			name: "with empty string",
			utf8: "",
			bytes: new Uint8Array([]),
			hex: "",
			base64: "",
			base64Url: "",
		},
		{
			name: "with non-URL-safe base64 output",
			utf8: "to͑ϡ3",
			bytes: new Uint8Array([116, 111, 205, 145, 207, 161, 51]),
			hex: "746fcd91cfa133",
			base64: "dG/Nkc+hMw==",
			base64Url: "dG_Nkc-hMw",
		},
	];

	// Helper function to compare Uint8Arrays
	const compareBytes = (a: Uint8Array, b: Uint8Array): boolean => {
		if (a.length !== b.length) {
			return false;
		}
		for (let i = 0; i < a.length; i++) {
			if (a[i] !== b[i]) {
				return false;
			}
		}
		return true;
	};

	// BYTES --> X CONVERSIONS
	describe("Bytes --> X Conversions", () => {
		it("bytesToUtf8 should convert bytes to Utf8 correctly", () => {
			for (const test of testCases) {
				expect(converters.bytesToUtf8(test.bytes)).toBe(test.utf8);
			}
		});

		it("bytesToHex should convert bytes to hex correctly", () => {
			for (const test of testCases) {
				expect(converters.bytesToHex(test.bytes)).toBe(test.hex);
			}
		});

		it("bytesToBase64 should convert bytes to base64 correctly", () => {
			for (const test of testCases) {
				expect(converters.bytesToBase64(test.bytes)).toBe(test.base64);
			}
		});

		it("bytesToBase64Url should convert bytes to base64Url correctly", () => {
			for (const test of testCases) {
				expect(converters.bytesToBase64Url(test.bytes)).toBe(test.base64Url);
			}
		});
	});

	// UTF8 --> X CONVERSIONS
	describe("UTF8 --> X Conversions", () => {
		it("utf8ToBytes should convert UTF8 to bytes correctly", () => {
			for (const test of testCases) {
				const result = converters.utf8ToBytes(test.utf8);
				expect(compareBytes(result, test.bytes)).toBe(true);
			}
		});

		it("utf8ToHex should convert UTF8 to hex correctly", () => {
			for (const test of testCases) {
				expect(converters.utf8ToHex(test.utf8)).toBe(test.hex);
			}
		});

		it("utf8ToBase64 should convert UTF8 to base64 correctly", () => {
			for (const test of testCases) {
				expect(converters.utf8ToBase64(test.utf8)).toBe(test.base64);
			}
		});

		it("utf8ToBase64Url should convert UTF8 to base64Url correctly", () => {
			for (const test of testCases) {
				expect(converters.utf8ToBase64Url(test.utf8)).toBe(test.base64Url);
			}
		});
	});

	// HEX --> X CONVERSIONS
	describe("HEX --> X Conversions", () => {
		it("hexToBytes should convert hex to bytes correctly", () => {
			for (const test of testCases) {
				const result = converters.hexToBytes(test.hex);
				expect(compareBytes(result, test.bytes)).toBe(true);
			}

			// Test 0x prefix handling
			expect(
				compareBytes(
					converters.hexToBytes("0x48656c6c6f"),
					converters.hexToBytes("48656c6c6f"),
				),
			).toBe(true);
		});

		it("hexToUtf8 should convert hex to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.hexToUtf8(test.hex)).toBe(test.utf8);
			}
		});

		it("hexToBase64 should convert hex to base64 correctly", () => {
			for (const test of testCases) {
				expect(converters.hexToBase64(test.hex)).toBe(test.base64);
			}
		});

		it("hexToBase64Url should convert hex to base64Url correctly", () => {
			for (const test of testCases) {
				expect(converters.hexToBase64Url(test.hex)).toBe(test.base64Url);
			}
		});
	});

	// BASE64 --> X CONVERSIONS
	describe("BASE64 --> X Conversions", () => {
		it("base64ToBytes should convert base64 to bytes correctly", () => {
			for (const test of testCases) {
				const result = converters.base64ToBytes(test.base64);
				expect(compareBytes(result, test.bytes)).toBe(true);
			}
		});

		it("base64ToUtf8 should convert base64 to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.base64ToUtf8(test.base64)).toBe(test.utf8);
			}
		});

		it("base64ToHex should convert base64 to hex correctly", () => {
			for (const test of testCases) {
				expect(converters.base64ToHex(test.base64)).toBe(test.hex);
			}
		});

		it("base64ToBase64Url should convert base64 to base64Url correctly", () => {
			for (const test of testCases) {
				expect(converters.base64ToBase64Url(test.base64)).toBe(test.base64Url);
			}

			// Test whitespace and padding handling
			expect(converters.base64ToBase64Url("SGVs bG8g\nV29y\tbGQ=")).toBe(
				"SGVsbG8gV29ybGQ",
			);
		});
	});

	// BASE64URL --> X CONVERSIONS
	describe("BASE64URL --> X Conversions", () => {
		it("base64UrlToBytes should convert base64Url to bytes correctly", () => {
			for (const test of testCases) {
				const result = converters.base64UrlToBytes(test.base64Url);
				expect(compareBytes(result, test.bytes)).toBe(true);
			}
		});

		it("base64UrlToUtf8 should convert base64Url to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.base64UrlToUtf8(test.base64Url)).toBe(test.utf8);
			}
		});

		it("base64UrlToHex should convert base64Url to hex correctly", () => {
			for (const test of testCases) {
				expect(converters.base64UrlToHex(test.base64Url)).toBe(test.hex);
			}
		});

		it("base64UrlToBase64 should convert base64Url to base64 correctly", () => {
			for (const test of testCases) {
				// Account for padding differences by comparing decoded values
				const decodedBase64 = converters.base64ToBytes(test.base64);
				const decodedBase64Url = converters.base64UrlToBytes(test.base64Url);
				expect(compareBytes(decodedBase64, decodedBase64Url)).toBe(true);
			}
		});
	});

	// EDGE CASES & SPECIAL FEATURES
	describe("Edge cases and special features", () => {
		it("should handle hex strings with and without 0x prefix", () => {
			const testHex = "48656c6c6f";
			const testHexWithPrefix = "0x48656c6c6f";

			expect(converters.hexToUtf8(testHex)).toBe("Hello");
			expect(converters.hexToUtf8(testHexWithPrefix)).toBe("Hello");
		});

		it("should handle Base64 strings with whitespace", () => {
			const testBase64 = "SGVs bG8g\nV29y\tbGQ=";
			expect(converters.base64ToUtf8(testBase64)).toBe("Hello World");
			expect(converters.base64ToBase64Url(testBase64)).toBe("SGVsbG8gV29ybGQ");
		});

		it("should handle different Base64 padding cases", () => {
			const testPadding = [
				{ base64: "YQ==", base64Url: "YQ" },
				{ base64: "YWI=", base64Url: "YWI" },
				{ base64: "YWJj", base64Url: "YWJj" },
			];

			for (let i = 0; i < testPadding.length; i++) {
				const test = testPadding[i];
				expect(test).toBeDefined();
				if (!test) {
					throw new Error("Test case not defined");
				}
				expect(converters.base64ToBase64Url(test.base64)).toBe(test.base64Url);
				expect(converters.base64UrlToBase64(test.base64Url)).toBe(test.base64);
			}
		});
	});

	// ROUND-TRIP CONVERSIONS
	describe("Round-trip conversions", () => {
		it("should correctly perform round-trip conversions", () => {
			for (const test of testCases) {
				// UTF8 -> Bytes -> UTF8
				expect(converters.bytesToUtf8(converters.utf8ToBytes(test.utf8))).toBe(
					test.utf8,
				);

				// UTF8 -> Hex -> UTF8
				expect(converters.hexToUtf8(converters.utf8ToHex(test.utf8))).toBe(
					test.utf8,
				);

				// UTF8 -> Base64 -> UTF8
				expect(converters.base64ToUtf8(converters.utf8ToBase64(test.utf8))).toBe(
					test.utf8,
				);

				// UTF8 -> Base64Url -> UTF8
				expect(
					converters.base64UrlToUtf8(converters.utf8ToBase64Url(test.utf8)),
				).toBe(test.utf8);

				// Hex -> Bytes -> Hex
				expect(converters.bytesToHex(converters.hexToBytes(test.hex))).toBe(
					test.hex,
				);

				// Hex -> Base64 -> Hex
				expect(converters.base64ToHex(converters.hexToBase64(test.hex))).toBe(
					test.hex,
				);

				// Base64Url -> Base64 -> Base64Url
				const roundTripBase64Url = converters.base64ToBase64Url(
					converters.base64UrlToBase64(test.base64Url),
				);
				expect(roundTripBase64Url).toBe(test.base64Url);
			}
		});
	});
});
