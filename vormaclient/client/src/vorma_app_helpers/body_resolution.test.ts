import { afterEach, describe, expect, it, vi } from "vitest";
import { resolveVormaRequestBody } from "./body_resolution.ts";

describe("resolveVormaRequestBody", () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it("serializes plain objects when ReadableStream is unavailable", () => {
		vi.stubGlobal("ReadableStream", undefined as any);

		expect(resolveVormaRequestBody({ key: "value" })).toBe(
			JSON.stringify({ key: "value" }),
		);
	});

	it("passes through ReadableStream bodies when available", () => {
		if (typeof ReadableStream === "undefined") {
			return;
		}

		const stream = new ReadableStream();
		expect(resolveVormaRequestBody(stream)).toBe(stream);
	});

	it("does not throw when body constructors are unavailable", () => {
		vi.stubGlobal("ReadableStream", undefined as any);
		vi.stubGlobal("Blob", undefined as any);
		vi.stubGlobal("FormData", undefined as any);
		vi.stubGlobal("URLSearchParams", undefined as any);
		vi.stubGlobal("ArrayBuffer", undefined as any);

		expect(resolveVormaRequestBody({ key: "value" })).toBe(
			JSON.stringify({ key: "value" }),
		);
	});
});
