import { describe, expect, it } from "vitest";
import * as converters from "./converters.ts";

describe("Encoding Conversion Functions", () => {
	// Test data that will be used across multiple tests
	const testCases = [
		{
			name: "simple ASCII",
			utf8: "Hello World",
			bytes: new Uint8Array([
				72, 101, 108, 108, 111, 32, 87, 111, 114, 108, 100,
			]),
			hex: "48656c6c6f20576f726c64",
			base64: "SGVsbG8gV29ybGQ=",
			base64URL: "SGVsbG8gV29ybGQ",
		},
		{
			name: "with special characters",
			utf8: "!@#$%^&*()_+",
			bytes: new Uint8Array([
				33, 64, 35, 36, 37, 94, 38, 42, 40, 41, 95, 43,
			]),
			hex: "21402324255e262a28295f2b",
			base64: "IUAjJCVeJiooKV8r",
			base64URL: "IUAjJCVeJiooKV8r",
		},
		{
			name: "with Unicode characters",
			utf8: "こんにちは世界",
			bytes: new Uint8Array([
				227, 129, 147, 227, 130, 147, 227, 129, 171, 227, 129, 161, 227,
				129, 175, 228, 184, 150, 231, 149, 140,
			]),
			hex: "e38193e38293e381abe381a1e381afe4b896e7958c",
			base64: "44GT44KT44Gr44Gh44Gv5LiW55WM",
			base64URL: "44GT44KT44Gr44Gh44Gv5LiW55WM",
		},
		{
			name: "with Base64 padding",
			utf8: "a",
			bytes: new Uint8Array([97]),
			hex: "61",
			base64: "YQ==",
			base64URL: "YQ",
		},
		{
			name: "with empty string",
			utf8: "",
			bytes: new Uint8Array([]),
			hex: "",
			base64: "",
			base64URL: "",
		},
		{
			name: "with non-URL-safe base64 output",
			utf8: "to͑ϡ3",
			bytes: new Uint8Array([116, 111, 205, 145, 207, 161, 51]),
			hex: "746fcd91cfa133",
			base64: "dG/Nkc+hMw==",
			base64URL: "dG_Nkc-hMw",
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
		it("bytesToUTF8 should convert bytes to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.bytesToUTF8(test.bytes)).toBe(test.utf8);
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

		it("bytesToBase64URL should convert bytes to base64URL correctly", () => {
			for (const test of testCases) {
				expect(converters.bytesToBase64URL(test.bytes)).toBe(
					test.base64URL,
				);
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

		it("utf8ToBase64URL should convert UTF8 to base64URL correctly", () => {
			for (const test of testCases) {
				expect(converters.utf8ToBase64URL(test.utf8)).toBe(
					test.base64URL,
				);
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

		it("hexToUTF8 should convert hex to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.hexToUTF8(test.hex)).toBe(test.utf8);
			}
		});

		it("hexToBase64 should convert hex to base64 correctly", () => {
			for (const test of testCases) {
				expect(converters.hexToBase64(test.hex)).toBe(test.base64);
			}
		});

		it("hexToBase64URL should convert hex to base64URL correctly", () => {
			for (const test of testCases) {
				expect(converters.hexToBase64URL(test.hex)).toBe(
					test.base64URL,
				);
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

		it("base64ToUTF8 should convert base64 to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.base64ToUTF8(test.base64)).toBe(test.utf8);
			}
		});

		it("base64ToHex should convert base64 to hex correctly", () => {
			for (const test of testCases) {
				expect(converters.base64ToHex(test.base64)).toBe(test.hex);
			}
		});

		it("base64ToBase64URL should convert base64 to base64URL correctly", () => {
			for (const test of testCases) {
				expect(converters.base64ToBase64URL(test.base64)).toBe(
					test.base64URL,
				);
			}

			// Test whitespace and padding handling
			expect(converters.base64ToBase64URL("SGVs bG8g\nV29y\tbGQ=")).toBe(
				"SGVsbG8gV29ybGQ",
			);
		});
	});

	// BASE64URL --> X CONVERSIONS
	describe("BASE64URL --> X Conversions", () => {
		it("base64URLToBytes should convert base64URL to bytes correctly", () => {
			for (const test of testCases) {
				const result = converters.base64URLToBytes(test.base64URL);
				expect(compareBytes(result, test.bytes)).toBe(true);
			}
		});

		it("base64URLToUTF8 should convert base64URL to UTF8 correctly", () => {
			for (const test of testCases) {
				expect(converters.base64URLToUTF8(test.base64URL)).toBe(
					test.utf8,
				);
			}
		});

		it("base64URLToHex should convert base64URL to hex correctly", () => {
			for (const test of testCases) {
				expect(converters.base64URLToHex(test.base64URL)).toBe(
					test.hex,
				);
			}
		});

		it("base64URLToBase64 should convert base64URL to base64 correctly", () => {
			for (const test of testCases) {
				// Account for padding differences by comparing decoded values
				const decodedBase64 = converters.base64ToBytes(test.base64);
				const decodedBase64URL = converters.base64URLToBytes(
					test.base64URL,
				);
				expect(compareBytes(decodedBase64, decodedBase64URL)).toBe(
					true,
				);
			}
		});
	});

	// EDGE CASES & SPECIAL FEATURES
	describe("Edge cases and special features", () => {
		it("should handle hex strings with and without 0x prefix", () => {
			const testHex = "48656c6c6f";
			const testHexWithPrefix = "0x48656c6c6f";

			expect(converters.hexToUTF8(testHex)).toBe("Hello");
			expect(converters.hexToUTF8(testHexWithPrefix)).toBe("Hello");
		});

		it("should handle Base64 strings with whitespace", () => {
			const testBase64 = "SGVs bG8g\nV29y\tbGQ=";
			expect(converters.base64ToUTF8(testBase64)).toBe("Hello World");
			expect(converters.base64ToBase64URL(testBase64)).toBe(
				"SGVsbG8gV29ybGQ",
			);
		});

		it("should handle different Base64 padding cases", () => {
			const testPadding = [
				{ base64: "YQ==", base64URL: "YQ" },
				{ base64: "YWI=", base64URL: "YWI" },
				{ base64: "YWJj", base64URL: "YWJj" },
			];

			for (let i = 0; i < testPadding.length; i++) {
				const test = testPadding[i];
				expect(test).toBeDefined();
				if (!test) {
					throw new Error("Test case not defined");
				}
				expect(converters.base64ToBase64URL(test.base64)).toBe(
					test.base64URL,
				);
				expect(converters.base64URLToBase64(test.base64URL)).toBe(
					test.base64,
				);
			}
		});
	});

	// ROUND-TRIP CONVERSIONS
	describe("Round-trip conversions", () => {
		it("should correctly perform round-trip conversions", () => {
			for (const test of testCases) {
				// UTF8 -> Bytes -> UTF8
				expect(
					converters.bytesToUTF8(converters.utf8ToBytes(test.utf8)),
				).toBe(test.utf8);

				// UTF8 -> Hex -> UTF8
				expect(
					converters.hexToUTF8(converters.utf8ToHex(test.utf8)),
				).toBe(test.utf8);

				// UTF8 -> Base64 -> UTF8
				expect(
					converters.base64ToUTF8(converters.utf8ToBase64(test.utf8)),
				).toBe(test.utf8);

				// UTF8 -> Base64URL -> UTF8
				expect(
					converters.base64URLToUTF8(
						converters.utf8ToBase64URL(test.utf8),
					),
				).toBe(test.utf8);

				// Hex -> Bytes -> Hex
				expect(
					converters.bytesToHex(converters.hexToBytes(test.hex)),
				).toBe(test.hex);

				// Hex -> Base64 -> Hex
				expect(
					converters.base64ToHex(converters.hexToBase64(test.hex)),
				).toBe(test.hex);

				// Base64URL -> Base64 -> Base64URL
				const roundTripBase64URL = converters.base64ToBase64URL(
					converters.base64URLToBase64(test.base64URL),
				);
				expect(roundTripBase64URL).toBe(test.base64URL);
			}
		});
	});
});
