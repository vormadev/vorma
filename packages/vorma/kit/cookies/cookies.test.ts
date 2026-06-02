// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from "vitest";
import { getClientCookie, setClientCookie } from "./cookies.ts";

describe("Cookie Utilities", () => {
	beforeEach(() => {
		// Clear all cookies before each test
		document.cookie.split(";").forEach((cookie) => {
			const eq_pos = cookie.indexOf("=");
			const name = eq_pos > -1 ? cookie.substr(0, eq_pos).trim() : cookie.trim();
			if (name) {
				document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
			}
		});
	});

	describe("getClientCookie", () => {
		it("should return undefined when cookie does not exist", () => {
			expect(getClientCookie("nonExistent")).toBeUndefined();
		});

		it("should return cookie value when cookie exists", () => {
			document.cookie = "testCookie=testValue";
			expect(getClientCookie("testCookie")).toBe("testValue");
		});

		it("should return empty string for empty cookie value", () => {
			document.cookie = "emptyCookie=";
			expect(getClientCookie("emptyCookie")).toBe("");
		});

		it("should handle multiple cookies correctly", () => {
			document.cookie = "cookie1=value1";
			document.cookie = "cookie2=value2";
			document.cookie = "cookie3=value3";

			expect(getClientCookie("cookie1")).toBe("value1");
			expect(getClientCookie("cookie2")).toBe("value2");
			expect(getClientCookie("cookie3")).toBe("value3");
		});

		it("should not match partial cookie names", () => {
			document.cookie = "testCookie=value1";
			document.cookie = "test=value2";

			expect(getClientCookie("test")).toBe("value2");
			expect(getClientCookie("testCookie")).toBe("value1");
		});

		it("should handle base64 values", () => {
			const base64_value = "wPrENSVOhk97/V0l6nkZrnH+DNZseEigminmJAbH0Go=";
			document.cookie = `base64Cookie=${base64_value}`;
			expect(getClientCookie("base64Cookie")).toBe(base64_value);
		});

		it("should handle cookie names containing regex metacharacters", () => {
			document.cookie = "cookie+name=value-plus";
			expect(getClientCookie("cookie+name")).toBe("value-plus");
		});
	});

	describe("setClientCookie", () => {
		it("should set a cookie that can be retrieved", () => {
			setClientCookie("newCookie", "newValue");
			expect(getClientCookie("newCookie")).toBe("newValue");
		});

		it("should update existing cookie value", () => {
			setClientCookie("updateCookie", "initialValue");
			expect(getClientCookie("updateCookie")).toBe("initialValue");

			setClientCookie("updateCookie", "updatedValue");
			expect(getClientCookie("updateCookie")).toBe("updatedValue");
		});

		it("should set empty string values", () => {
			setClientCookie("emptyCookie", "");
			expect(getClientCookie("emptyCookie")).toBe("");
		});

		it("should handle multiple cookies independently", () => {
			setClientCookie("cookie1", "value1");
			setClientCookie("cookie2", "value2");

			expect(getClientCookie("cookie1")).toBe("value1");
			expect(getClientCookie("cookie2")).toBe("value2");
		});

		it("should handle base64 values", () => {
			const base64_value = "wPrENSVOhk97/V0l6nkZrnH+DNZseEigminmJAbH0Go=";
			setClientCookie("base64Cookie", base64_value);
			expect(getClientCookie("base64Cookie")).toBe(base64_value);
		});
	});

	describe("Known limitations (no encoding)", () => {
		it("should truncate values at semicolons", () => {
			setClientCookie("semicolonCookie", "value;rest");
			expect(getClientCookie("semicolonCookie")).toBe("value");
		});

		it("should not encode or decode values", () => {
			const encoded_value = "value%20with%20encoding";
			setClientCookie("encodedCookie", encoded_value);
			expect(getClientCookie("encodedCookie")).toBe(encoded_value);
		});
	});
});
