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
		expect(new Headers(init.headers).get("Content-Type")).toBe(
			"application/json",
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

	it("passes through ArrayBufferView bodies for non-GET methods", () => {
		const bytes = new Uint8Array([1, 2, 3]);
		const init = buildRedirectRequestInit(
			{
				method: "POST",
				body: bytes,
			},
			new AbortController().signal,
		);

		expect(init.body).toBe(bytes);
	});

	it("omits null bodies for non-GET methods", () => {
		const init = buildRedirectRequestInit(
			{
				method: "POST",
				body: null,
			},
			new AbortController().signal,
		);

		expect(init.body).toBeNull();
	});

	it("omits body for HEAD requests", () => {
		const init = buildRedirectRequestInit(
			{
				method: "HEAD",
				body: JSON.stringify({ ignored: true }),
			},
			new AbortController().signal,
		);

		expect(init.body).toBeUndefined();
	});

	it("passes through Blob and ArrayBuffer bodies for non-GET methods", () => {
		const blobBody = new Blob(["blob-body"]);
		const bufferBody = new TextEncoder().encode("buffer-body").buffer;

		const blobInit = buildRedirectRequestInit(
			{
				method: "POST",
				body: blobBody,
			},
			new AbortController().signal,
		);
		const bufferInit = buildRedirectRequestInit(
			{
				method: "POST",
				body: bufferBody,
			},
			new AbortController().signal,
		);

		expect(blobInit.body).toBe(blobBody);
		expect(bufferInit.body).toBe(bufferBody);
	});

	it("passes through ReadableStream bodies for non-GET methods", () => {
		if (typeof ReadableStream === "undefined") {
			return;
		}

		const readableStreamBody = new ReadableStream();
		const init = buildRedirectRequestInit(
			{
				method: "POST",
				body: readableStreamBody,
			},
			new AbortController().signal,
		);

		expect(init.body).toBe(readableStreamBody);
	});

	it("keeps caller-provided content type when serializing object bodies", () => {
		const init = buildRedirectRequestInit(
			{
				method: "POST",
				headers: {
					"Content-Type": "application/vnd.custom+json",
				},
				body: { key: "value" } as any,
			},
			new AbortController().signal,
		);

		expect(init.body).toBe(JSON.stringify({ key: "value" }));
		expect(new Headers(init.headers).get("Content-Type")).toBe(
			"application/vnd.custom+json",
		);
	});
});
