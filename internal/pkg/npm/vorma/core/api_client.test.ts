// @vitest-environment jsdom

import { describe, expect, it, vi } from "vitest";
import { create_typed_api_client } from "./api_client.ts";

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

describe("query", () => {
	it("constructs GET URL with mount root and params", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { include: "posts" },
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("GET");
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/users/42");
		expect(url.searchParams.get("include")).toBe("posts");
	});

	it("passes options through to submit", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { q: "test" },
			options: { revalidate: false },
		});

		expect(calls[0]!.options).toEqual({ revalidate: false });
	});

	it("works without input", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			pattern: "/health",
		});

		expect(calls).toHaveLength(1);
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
});

describe("mutate", () => {
	it("constructs POST URL with mount root and params", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
			requestInit: { method: "POST" },
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("POST");
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/users/42");
		expect(url.search).toBe("");
	});

	it("serializes input as body", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
			requestInit: { method: "PATCH" },
		});

		expect(calls[0]!.init?.method).toBe("PATCH");
		expect(calls[0]!.init?.body).toBe(JSON.stringify({ name: "Ada" }));
	});

	it("defaults to POST when requestInit omits method", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/sessions",
			input: { email: "a@b.com" },
		});

		expect(calls[0]!.init?.method).toBe("POST");
	});

	it("works without input", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/logout",
			requestInit: { method: "POST" },
		});

		expect(calls).toHaveLength(1);
		const url = url_of(calls[0]!);
		expect(url.pathname).toBe("/api/logout");
	});

	it("passes options through to submit", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
			requestInit: { method: "POST" },
			options: { dedupeKey: "save" },
		});

		expect(calls[0]!.options).toEqual({ dedupeKey: "save" });
	});
});

describe("decorator", () => {
	it("calls decorator with correct query context", async () => {
		const decorator = vi.fn().mockResolvedValue(undefined);
		const { submit_fn } = mock_submit();
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

		expect(decorator).toHaveBeenCalledTimes(1);
		expect(decorator.mock.calls[0]![0]).toMatchObject({
			type: "query",
			pattern: "/users/:id",
			input: { q: "test" },
		});
	});

	it("calls decorator with correct mutation context", async () => {
		const decorator = vi.fn().mockResolvedValue(undefined);
		const { submit_fn } = mock_submit();
		const client = create_typed_api_client(
			"/api/",
			submit_fn as any,
			decorator,
		);

		await (client as any).mutate({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { name: "Ada" },
			requestInit: { method: "PATCH" },
		});

		expect(decorator).toHaveBeenCalledTimes(1);
		expect(decorator.mock.calls[0]![0]).toMatchObject({
			type: "mutation",
			pattern: "/users/:id",
			input: { name: "Ada" },
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

	it("per-call requestInit headers override decorator headers", async () => {
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
			requestInit: {
				headers: { "X-Override": "per-call" },
			},
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
			pattern: "/sessions",
			input: { email: "a@b.com" },
			requestInit: { method: "POST" },
		});

		expect(calls[0]!.init?.credentials).toBe("include");
	});
});

describe("no decorator", () => {
	it("works without a decorator for queries", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).query({
			pattern: "/health",
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("GET");
	});

	it("works without a decorator for mutations", async () => {
		const { submit_fn, calls } = mock_submit();
		const client = create_typed_api_client("/api/", submit_fn as any);

		await (client as any).mutate({
			pattern: "/logout",
			requestInit: { method: "POST" },
		});

		expect(calls).toHaveLength(1);
		expect(calls[0]!.init?.method).toBe("POST");
	});
});
