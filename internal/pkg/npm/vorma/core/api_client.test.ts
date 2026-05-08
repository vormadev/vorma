// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { create_typed_api_client, MutationError } from "./api_client.ts";
import { API_IDENTITY_ARRAY_PREFIX } from "./constants.ts";

type SubmitCall = {
	url: URL | string;
	init: RequestInit | undefined;
	options: Record<string, unknown> | undefined;
};

function mock_submit() {
	const calls: SubmitCall[] = [];
	const submit_fn = vi.fn(
		async (
			url: string | URL,
			init?: RequestInit,
			options?: Record<string, unknown>,
		) => {
			calls.push({ url, init, options });
			return { success: true as const, data: {} };
		},
	);
	return { submit_fn, calls };
}

function url_of(call: SubmitCall): URL {
	return call.url instanceof URL
		? call.url
		: new URL(call.url, window.location.origin);
}

function headers_of(call: SubmitCall): Headers {
	return new Headers(call.init?.headers ?? undefined);
}

describe("query and mutate", () => {
	it("defaults to GET and serializes input into URL search", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { include: "posts" },
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("GET");
		expect(calls[0]!.init?.body).toBeUndefined();
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/users/42");
		expect(url.searchParams.get("include")).toBe("posts");
	});

	it("uses explicit GET method", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			method: "GET",
			pattern: "/health",
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("GET");
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/health");
		expect(url.search).toBe("");
	});

	it("handles splat values", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			pattern: "/docs/*",
			splatValues: ["guide", "intro"],
			input: { format: "html" },
		});

		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/docs/guide/intro");
		expect(url.searchParams.get("format")).toBe("html");
	});

	it("serializes non-GET input as body", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			method: "PATCH",
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("PATCH");
		expect(calls[0]!.init?.body).toBe(JSON.stringify({ name: "Ada" }));
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/users/42");
		expect(url.search).toBe("");
	});

	it("works without input for non-GET API routes", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			method: "POST",
			pattern: "/logout",
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("POST");
		expect(calls[0]!.init?.body).toBeUndefined();
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/logout");
	});

	it("passes flattened Vorma options through", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			method: "POST",
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
			dedupeKey: "save",
			revalidate: false,
			skipWorkIndicator: true,
		});

		expect(calls[0]!.options).toEqual({
			apiRouteKind: "mutation",
			dedupeKey: "save",
			revalidate: false,
			skipWorkIndicator: true,
		});
	});

	it("passes skipWorkIndicator through for queries", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			dedupeKey: "users:42",
			pattern: "/users/:id",
			params: { id: "42" },
			input: { include: "posts" },
			skipWorkIndicator: true,
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.options).toEqual({
			apiRouteKind: "query",
			dedupeKey: "users:42",
			skipWorkIndicator: true,
		});
	});

	it("passes flattened RequestInit fields through", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			method: "POST",
			pattern: "/sessions",
			input: { email: "a@b.com" },
			credentials: "include",
			headers: { "X-Trace": "1" },
		});

		expect(calls[0]!.init?.credentials).toBe("include");
		expect(headers_of(calls[0]!).get("X-Trace")).toBe("1");
	});
	it("passes query semantics from query", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			method: "POST",
			pattern: "/rpc",
			input: { op: "quote" },
		});

		expect(calls[0]!.options).toMatchObject({ apiRouteKind: "query" });
	});

	it("passes mutation semantics from mutate", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/health",
		});

		expect(calls[0]!.options).toMatchObject({ apiRouteKind: "mutation" });
	});

	it("mutateOrThrow returns data for successful mutation", async () => {
		const { submit_fn } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await expect(
			(client as any).mutateOrThrow({
				method: "POST",
				pattern: "/sessions",
				input: { email: "a@b.com", password: "pw" },
			}),
		).resolves.toEqual({});
	});

	it("mutateOrThrow throws MutationError with preserved result", async () => {
		const submit_fn = vi.fn(async () => {
			return {
				success: false as const,
				error: "Bad Request",
				response: new Response("", {
					status: 400,
					statusText: "Bad Request",
				}),
				revalidationPromise: Promise.resolve({ ok: true as const }),
			};
		});
		const client = create_typed_api_client("/api/", submit_fn as any);

		await expect(
			(client as any).mutateOrThrow({
				method: "POST",
				pattern: "/sessions",
				input: { email: "a@b.com", password: "pw" },
			}),
		).rejects.toBeInstanceOf(MutationError);

		try {
			await (client as any).mutateOrThrow({
				method: "POST",
				pattern: "/sessions",
				input: { email: "a@b.com", password: "pw" },
			});
		} catch (err) {
			expect(err).toBeInstanceOf(MutationError);
			expect((err as MutationError).message).toBe("Bad Request");
			expect((err as MutationError).result.success).toBe(false);
			if (err instanceof MutationError) {
				expect(err.result.error).toBe("Bad Request");
				expect(err.result.response?.status).toBe(400);
			}
		}
	});
});

describe("toIdentityArray", () => {
	it("builds a stable action identity array", () => {
		const { submit_fn } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		const key = (client as any).toIdentityArray({
			method: " post ",
			pattern: " /users/:id ",
			params: { id: "42" },
			splatValues: ["profile"],
			input: { z: 1, a: { d: 4, c: 3 } },
		});

		expect(key).toEqual([
			API_IDENTITY_ARRAY_PREFIX,
			"/api/",
			"POST",
			"/users/:id",
			`{"id":"42"}`,
			`["profile"]`,
			`{"a":{"c":3,"d":4},"z":1}`,
		]);
	});

	it("defaults method and missing optional identity parts", () => {
		const { submit_fn } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		const key = (client as any).toIdentityArray({
			pattern: "/health",
		});

		expect(key).toEqual([
			API_IDENTITY_ARRAY_PREFIX,
			"/api/",
			"GET",
			"/health",
			"null",
			"[]",
			"null",
		]);
	});
});

describe("decorator", () => {
	it("calls decorator with method and pattern", async () => {
		const decorator = vi.fn().mockResolvedValue(undefined);
		const { submit_fn } = mock_submit();
		const client = create_typed_api_client(
			"/api/",
			submit_fn as any,
			decorator,
		);

		await (client as any).mutate({
			method: "PATCH",
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
			headers: { "X-Trace": "1" },
		});

		expect(decorator).toHaveBeenCalledTimes(1);
		expect(decorator.mock.calls[0]![0]).toMatchObject({
			method: "PATCH",
			pattern: "/users/:id",
			input: { name: "Ada" },
			requestInit: {
				headers: { "X-Trace": "1" },
			},
		});
	});

	it("merges decorator headers onto request", async () => {
		const decorator = vi.fn().mockResolvedValue({
			headers: { Authorization: "Bearer token" },
		});
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client(
			"/api/",
			submit_fn as any,
			decorator,
		);

		await (client as any).query({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { q: "test" },
		});

		expect(headers_of(calls[0]!).get("Authorization")).toBe("Bearer token");
	});

	it("per-call headers override decorator headers", async () => {
		const decorator = vi.fn().mockResolvedValue({
			headers: { "X-Default": "decorator", "X-Override": "decorator" },
		});
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client(
			"/api/",
			submit_fn as any,
			decorator,
		);

		await (client as any).query({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { q: "test" },
			headers: { "X-Override": "per-call" },
		});

		const headers = headers_of(calls[0]!);
		expect(headers.get("X-Default")).toBe("decorator");
		expect(headers.get("X-Override")).toBe("per-call");
	});

	it("merges decorator credentials onto request", async () => {
		const decorator = vi.fn().mockResolvedValue({
			credentials: "include",
		});
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client(
			"/api/",
			submit_fn as any,
			decorator,
		);

		await (client as any).mutate({
			method: "POST",
			pattern: "/sessions",
			input: { email: "a@b.com" },
		});

		expect(calls[0]!.init?.credentials).toBe("include");
	});
});
