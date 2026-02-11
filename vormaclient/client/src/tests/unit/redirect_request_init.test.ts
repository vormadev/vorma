import { afterEach, describe, expect, it, vi } from "vitest";
import { buildRedirectRequestInit } from "../../core/redirects.ts";

describe("buildRedirectRequestInit", () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it("serializes object bodies without throwing when body constructors are unavailable", () => {
		vi.stubGlobal("FormData", undefined as any);
		vi.stubGlobal("URLSearchParams", undefined as any);
		vi.stubGlobal("Blob", undefined as any);
		vi.stubGlobal("ArrayBuffer", undefined as any);
		vi.stubGlobal("ReadableStream", undefined as any);

		const controller = new AbortController();
		const init = buildRedirectRequestInit(
			{
				method: "POST",
				body: { key: "value" } as any,
			},
			controller.signal,
		);

		expect(init.body).toBe(JSON.stringify({ key: "value" }));
		expect(init.signal).toBe(controller.signal);
		expect(new Headers(init.headers).get("X-Accepts-Client-Redirect")).toBe(
			"1",
		);
	});

	it("omits body for implicit GET requests even when constructors are unavailable", () => {
		vi.stubGlobal("FormData", undefined as any);
		vi.stubGlobal("URLSearchParams", undefined as any);
		vi.stubGlobal("Blob", undefined as any);
		vi.stubGlobal("ArrayBuffer", undefined as any);
		vi.stubGlobal("ReadableStream", undefined as any);

		const init = buildRedirectRequestInit(
			{ body: { key: "value" } as any },
			new AbortController().signal,
		);

		expect(init.body).toBeUndefined();
	});
});
