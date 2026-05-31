// @vitest-environment jsdom

import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { BUILD_ID_HEADER, DATA_SCRIPT_ID } from "./constants.ts";
import type { LinkRouteState, LinkWorkState } from "./make_link_props.ts";

type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (err: unknown) => void;
};

type MockMatcher = {
	register_pattern: ReturnType<typeof vi.fn>;
	find_nested_matches: ReturnType<typeof vi.fn>;
	free: ReturnType<typeof vi.fn>;
};

function deferred<T>(): Deferred<T> {
	let resolve!: (value: T) => void;
	let reject!: (err: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

async function tick(n = 5): Promise<void> {
	for (let i = 0; i < n; i++) {
		await Promise.resolve();
	}
}

async function wait_until(check: () => boolean, message: string): Promise<void> {
	for (let i = 0; i < 50; i++) {
		if (check()) {
			return;
		}
		await tick();
		await new Promise((resolve) => {
			return setTimeout(resolve, 0);
		});
	}
	throw new Error(message);
}

function seed_payload(overrides: Record<string, unknown> = {}): void {
	const data = {
		client_build_id: "build-1",
		deployment_id: "",
		matched_patterns: [],
		loaders_data: [],
		import_urls: [],
		params: {},
		splat_values: [],
		title: undefined,
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
		...overrides,
	};
	const script = document.createElement("script");
	script.id = DATA_SCRIPT_ID;
	script.type = "application/json";
	script.textContent = JSON.stringify(data);
	document.head.appendChild(script);
}

function route_response(overrides: Record<string, unknown> = {}): Response {
	const data = {
		matched_patterns: [],
		loaders_data: [],
		import_urls: [],
		params: {},
		splat_values: [],
		title: undefined,
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
		...overrides,
	};
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			[BUILD_ID_HEADER]: "build-1",
		},
	});
}

function make_matcher(
	match: (path: string) => {
		params: Record<string, string>;
		splat_values: string[];
		patterns: string[];
	} | null,
): MockMatcher {
	return {
		register_pattern: vi.fn(),
		find_nested_matches: vi.fn(match),
		free: vi.fn(),
	};
}

beforeEach(() => {
	vi.restoreAllMocks();
	vi.resetModules();
	document.head.innerHTML = "";
	document.body.innerHTML = "";
	window.history.replaceState({}, "", "/");
	sessionStorage.clear();
});

afterEach(() => {
	vi.restoreAllMocks();
});

it("starts route fetches before matcher wasm is ready but waits to prestart client loaders", async () => {
	const matcher_ready = deferred<MockMatcher>();
	const matcher = make_matcher((path) => {
		if (path === "/users") {
			return { params: {}, splat_values: [], patterns: ["/users"] };
		}
		return null;
	});
	vi.doMock("./client_wasm/matcher.ts", () => {
		return {
			create_client_matcher: vi.fn(() => {
				return matcher_ready.promise;
			}),
		};
	});

	let captured_args: any = null;
	vi.doMock("/users-module.js", () => {
		return {
			default: {
				pattern: "/users",
				component: () => {
					return null;
				},
				client_loader: async (args: any) => {
					if (args.trigger === "navigation") {
						captured_args = args;
						await args.serverPromise;
					}
					return { client: true };
				},
			},
		};
	});

	const { create_client_core } = await import("./create_client_core.ts");
	seed_payload({
		matched_patterns: ["/users"],
		loaders_data: [{ initial: true }],
		import_urls: ["/users-module.js"],
	});

	const core_res = create_client_core({ apiMountRoot: "/api/" }, vi.fn(), {
		hard_redirect: vi.fn(),
		reload: vi.fn(),
		scroll_to: vi.fn(),
	});
	if (!core_res.ok) {
		throw new Error(`create_client_core failed with error: ${core_res.err}`);
	}
	const core = core_res.val;
	await core.boot({});

	const fetch_result = deferred<Response>();
	const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(() => {
		return fetch_result.promise;
	});

	const nav = core.navigate("/users");
	await wait_until(() => {
		return fetch.mock.calls.length === 1;
	}, "expected route fetch to start before matcher wasm resolves");

	expect(captured_args).toBeNull();
	expect(matcher.register_pattern).not.toHaveBeenCalled();

	matcher_ready.resolve(matcher);
	await wait_until(() => {
		return captured_args !== null;
	}, "expected client loader to prestart after matcher wasm resolves");

	expect(matcher.register_pattern).toHaveBeenCalledWith("/users");
	expect(captured_args.knownMatches).toEqual([{ pattern: "/users", input: {} }]);

	fetch_result.resolve(
		route_response({
			matched_patterns: ["/users"],
			loaders_data: [{ next: true }],
			import_urls: ["/users-module.js"],
		}),
	);
	await nav;
});

it("notifies links to recompute when the link matcher wasm resolves", async () => {
	const route_matcher_ready = deferred<MockMatcher>();
	const link_matcher_ready = deferred<MockMatcher>();
	const _route_matcher = make_matcher(() => {
		return null;
	});
	const link_matcher = make_matcher((path) => {
		if (path === "/products") {
			return {
				params: {} as Record<string, string>,
				splat_values: [],
				patterns: ["/products"],
			};
		}
		if (path === "/products/123") {
			return {
				params: { id: "123" },
				splat_values: [],
				patterns: ["/products", "/products/:id"],
			};
		}
		return null;
	});
	const create_client_matcher = vi
		.fn()
		.mockImplementationOnce(() => {
			return route_matcher_ready.promise;
		})
		.mockImplementationOnce(() => {
			return link_matcher_ready.promise;
		});
	vi.doMock("./client_wasm/matcher.ts", () => {
		return { create_client_matcher };
	});

	const { create_adapter_base } = await import("./ui_adapter_core.ts");
	const commits: Array<{ link_state_version?: number }> = [];
	const adapter_res = create_adapter_base(
		{ apiMountRoot: "/api/" } as any,
		(commit) => {
			commits.push(commit);
		},
	);
	if (!adapter_res.ok) {
		throw new Error(`create_adapter_base failed with error: ${adapter_res.err}`);
	}

	const { nav_fns } = adapter_res.val;
	let listener_count = 0;
	nav_fns.subscribe_link_state(() => {
		listener_count++;
	});
	nav_fns.register_link_pattern("/products");
	nav_fns.register_link_pattern("/products/:id");

	const route_state: LinkRouteState = {
		href: "/products/123",
		matched_patterns: ["/products", "/products/:id"],
	};
	const work_state: LinkWorkState = { navigation_href: null };

	expect(
		nav_fns.get_link_attribute_state("/products", undefined, route_state, work_state)
			.active_ancestor,
	).toBe(false);

	link_matcher_ready.resolve(link_matcher);
	await wait_until(() => {
		return (
			listener_count === 1 &&
			commits.some((commit) => {
				return commit.link_state_version === 1;
			})
		);
	}, "expected link state notification after matcher wasm resolves");

	expect(link_matcher.register_pattern).toHaveBeenCalledWith("/products");
	expect(link_matcher.register_pattern).toHaveBeenCalledWith("/products/:id");
	expect(
		nav_fns.get_link_attribute_state("/products", undefined, route_state, work_state)
			.active_ancestor,
	).toBe(true);
});

it("does not register duplicate route patterns after matcher wasm is ready", async () => {
	const matcher = make_matcher(() => {
		return null;
	});
	vi.doMock("./client_wasm/matcher.ts", () => {
		return {
			create_client_matcher: vi.fn(() => {
				return Promise.resolve(matcher);
			}),
		};
	});

	vi.doMock("/users-module.js", () => {
		return {
			default: {
				pattern: "/users",
				component: () => {
					return null;
				},
				client_loader: async () => {
					return {};
				},
			},
		};
	});

	const { create_client_core } = await import("./create_client_core.ts");
	seed_payload({
		matched_patterns: ["/users"],
		loaders_data: [{ initial: true }],
		import_urls: ["/users-module.js"],
	});

	const core_res = create_client_core({ apiMountRoot: "/api/" }, vi.fn(), {
		hard_redirect: vi.fn(),
		reload: vi.fn(),
		scroll_to: vi.fn(),
	});
	if (!core_res.ok) {
		throw new Error(`create_client_core failed with error: ${core_res.err}`);
	}
	const core = core_res.val;
	await core.boot({});
	await tick();

	await window.__vorma_hmr_route_update?.("/users-module.js", {
		default: {
			pattern: "/users",
			component: () => {
				return null;
			},
			client_loader: async () => {
				return {};
			},
		},
	});

	expect(matcher.register_pattern).toHaveBeenCalledTimes(1);
	expect(matcher.register_pattern).toHaveBeenCalledWith("/users");
});

it("does not register duplicate link patterns after matcher wasm is ready", async () => {
	const route_matcher = make_matcher(() => {
		return null;
	});
	const link_matcher = make_matcher(() => {
		return null;
	});
	const create_client_matcher = vi
		.fn()
		.mockResolvedValueOnce(route_matcher)
		.mockResolvedValueOnce(link_matcher);
	vi.doMock("./client_wasm/matcher.ts", () => {
		return { create_client_matcher };
	});

	const { create_adapter_base } = await import("./ui_adapter_core.ts");
	const adapter_res = create_adapter_base({ apiMountRoot: "/api/" } as any, vi.fn());
	if (!adapter_res.ok) {
		throw new Error(`create_adapter_base failed with error: ${adapter_res.err}`);
	}
	const { nav_fns } = adapter_res.val;
	await tick();

	nav_fns.register_link_pattern("/users");
	nav_fns.register_link_pattern("/users");

	expect(link_matcher.register_pattern).toHaveBeenCalledTimes(1);
	expect(link_matcher.register_pattern).toHaveBeenCalledWith("/users");
});
