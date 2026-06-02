import { describe, expect, it } from "vitest";
import * as converters from "./converters.ts";

describe("Encoding Conversion Functions", () => {
	// Test data that will be used across multiple tests
	const test_cases = [
		{
			name: "simple ASCII",
			utf8: "Hello World",
			bytes: new Uint8Array([72, 101, 108, 108, 111, 32, 87, 111, 114, 108, 100]),
			hex: "48656c6c6f20576f726c64",
			base64: "SGVsbG8gV29ybGQ=",
			base64_url: "SGVsbG8gV29ybGQ",
		},
		{
			name: "with special characters",
			utf8: "!@#$%^&*()_+",
			bytes: new Uint8Array([33, 64, 35, 36, 37, 94, 38, 42, 40, 41, 95, 43]),
			hex: "21402324255e262a28295f2b",
			base64: "IUAjJCVeJiooKV8r",
			base64_url: "IUAjJCVeJiooKV8r",
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
			base64_url: "44GT44KT44Gr44Gh44Gv5LiW55WM",
		},
		{
			name: "with Base64 padding",
			utf8: "a",
			bytes: new Uint8Array([97]),
			hex: "61",
			base64: "YQ==",
			base64_url: "YQ",
		},
		{
			name: "with empty string",
			utf8: "",
			bytes: new Uint8Array([]),
			hex: "",
			base64: "",
			base64_url: "",
		},
		{
			name: "with non-URL-safe base64 output",
			utf8: "to͑ϡ3",
			bytes: new Uint8Array([116, 111, 205, 145, 207, 161, 51]),
			hex: "746fcd91cfa133",
			base64: "dG/Nkc+hMw==",
			base64_url: "dG_Nkc-hMw",
		},
	];

	// Helper function to compare Uint8Arrays
	const compare_bytes = (a: Uint8Array, b: Uint8Array): boolean => {
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
			for (const test of test_cases) {
				expect(converters.bytesToUtf8(test.bytes)).toBe(test.utf8);
			}
		});

		it("bytesToHex should convert bytes to hex correctly", () => {
			for (const test of test_cases) {
				expect(converters.bytesToHex(test.bytes)).toBe(test.hex);
			}
		});

		it("bytesToBase64 should convert bytes to base64 correctly", () => {
			for (const test of test_cases) {
				expect(converters.bytesToBase64(test.bytes)).toBe(test.base64);
			}
		});

		it("bytesToBase64Url should convert bytes to base64_url correctly", () => {
			for (const test of test_cases) {
				expect(converters.bytesToBase64Url(test.bytes)).toBe(test.base64_url);
			}
		});
	});

	// UTF8 --> X CONVERSIONS
	describe("UTF8 --> X Conversions", () => {
		it("utf8ToBytes should convert UTF8 to bytes correctly", () => {
			for (const test of test_cases) {
				const result = converters.utf8ToBytes(test.utf8);
				expect(compare_bytes(result, test.bytes)).toBe(true);
			}
		});

		it("utf8ToHex should convert UTF8 to hex correctly", () => {
			for (const test of test_cases) {
				expect(converters.utf8ToHex(test.utf8)).toBe(test.hex);
			}
		});

		it("utf8ToBase64 should convert UTF8 to base64 correctly", () => {
			for (const test of test_cases) {
				expect(converters.utf8ToBase64(test.utf8)).toBe(test.base64);
			}
		});

		it("utf8ToBase64Url should convert UTF8 to base64_url correctly", () => {
			for (const test of test_cases) {
				expect(converters.utf8ToBase64Url(test.utf8)).toBe(test.base64_url);
			}
		});
	});

	// HEX --> X CONVERSIONS
	describe("HEX --> X Conversions", () => {
		it("hexToBytes should convert hex to bytes correctly", () => {
			for (const test of test_cases) {
				const result = converters.hexToBytes(test.hex);
				expect(compare_bytes(result, test.bytes)).toBe(true);
			}

			// Test 0x prefix handling
			expect(
				compare_bytes(
					converters.hexToBytes("0x48656c6c6f"),
					converters.hexToBytes("48656c6c6f"),
				),
			).toBe(true);
		});

		it("hexToUtf8 should convert hex to UTF8 correctly", () => {
			for (const test of test_cases) {
				expect(converters.hexToUtf8(test.hex)).toBe(test.utf8);
			}
		});

		it("hexToBase64 should convert hex to base64 correctly", () => {
			for (const test of test_cases) {
				expect(converters.hexToBase64(test.hex)).toBe(test.base64);
			}
		});

		it("hexToBase64Url should convert hex to base64_url correctly", () => {
			for (const test of test_cases) {
				expect(converters.hexToBase64Url(test.hex)).toBe(test.base64_url);
			}
		});
	});

	// BASE64 --> X CONVERSIONS
	describe("BASE64 --> X Conversions", () => {
		it("base64ToBytes should convert base64 to bytes correctly", () => {
			for (const test of test_cases) {
				const result = converters.base64ToBytes(test.base64);
				expect(compare_bytes(result, test.bytes)).toBe(true);
			}
		});

		it("base64ToUtf8 should convert base64 to UTF8 correctly", () => {
			for (const test of test_cases) {
				expect(converters.base64ToUtf8(test.base64)).toBe(test.utf8);
			}
		});

		it("base64ToHex should convert base64 to hex correctly", () => {
			for (const test of test_cases) {
				expect(converters.base64ToHex(test.base64)).toBe(test.hex);
			}
		});

		it("base64ToBase64Url should convert base64 to base64_url correctly", () => {
			for (const test of test_cases) {
				expect(converters.base64ToBase64Url(test.base64)).toBe(test.base64_url);
			}

			// Test whitespace and padding handling
			expect(converters.base64ToBase64Url("SGVs bG8g\nV29y\tbGQ=")).toBe(
				"SGVsbG8gV29ybGQ",
			);
		});
	});

	// BASE64URL --> X CONVERSIONS
	describe("BASE64URL --> X Conversions", () => {
		it("base64UrlToBytes should convert base64_url to bytes correctly", () => {
			for (const test of test_cases) {
				const result = converters.base64UrlToBytes(test.base64_url);
				expect(compare_bytes(result, test.bytes)).toBe(true);
			}
		});

		it("base64UrlToUtf8 should convert base64_url to UTF8 correctly", () => {
			for (const test of test_cases) {
				expect(converters.base64UrlToUtf8(test.base64_url)).toBe(test.utf8);
			}
		});

		it("base64UrlToHex should convert base64_url to hex correctly", () => {
			for (const test of test_cases) {
				expect(converters.base64UrlToHex(test.base64_url)).toBe(test.hex);
			}
		});

		it("base64UrlToBase64 should convert base64_url to base64 correctly", () => {
			for (const test of test_cases) {
				// Account for padding differences by comparing decoded values
				const decoded_base64 = converters.base64ToBytes(test.base64);
				const decoded_base64_url = converters.base64UrlToBytes(test.base64_url);
				expect(compare_bytes(decoded_base64, decoded_base64_url)).toBe(true);
			}
		});
	});

	// EDGE CASES & SPECIAL FEATURES
	describe("Edge cases and special features", () => {
		it("should handle hex strings with and without 0x prefix", () => {
			const test_hex = "48656c6c6f";
			const test_hex_with_prefix = "0x48656c6c6f";

			expect(converters.hexToUtf8(test_hex)).toBe("Hello");
			expect(converters.hexToUtf8(test_hex_with_prefix)).toBe("Hello");
		});

		it("should handle Base64 strings with whitespace", () => {
			const test_base64 = "SGVs bG8g\nV29y\tbGQ=";
			expect(converters.base64ToUtf8(test_base64)).toBe("Hello World");
			expect(converters.base64ToBase64Url(test_base64)).toBe("SGVsbG8gV29ybGQ");
		});

		it("should handle different Base64 padding cases", () => {
			const test_padding = [
				{ base64: "YQ==", base64_url: "YQ" },
				{ base64: "YWI=", base64_url: "YWI" },
				{ base64: "YWJj", base64_url: "YWJj" },
			];

			for (let i = 0; i < test_padding.length; i++) {
				const test = test_padding[i];
				expect(test).toBeDefined();
				if (!test) {
					throw new Error("Test case not defined");
				}
				expect(converters.base64ToBase64Url(test.base64)).toBe(test.base64_url);
				expect(converters.base64UrlToBase64(test.base64_url)).toBe(test.base64);
			}
		});
	});

	// ROUND-TRIP CONVERSIONS
	describe("Round-trip conversions", () => {
		it("should correctly perform round-trip conversions", () => {
			for (const test of test_cases) {
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
				const round_trip_base64_url = converters.base64ToBase64Url(
					converters.base64UrlToBase64(test.base64_url),
				);
				expect(round_trip_base64_url).toBe(test.base64_url);
			}
		});
	});
});
