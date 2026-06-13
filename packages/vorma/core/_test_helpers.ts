import { expect, vi } from "vitest";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	SCROLL_STORAGE_KEY,
} from "./constants.ts";
import {
	create_client_core,
	type ClientCommit,
	type RouteRenderState,
	type ScrollIntent,
} from "./create_client_core.ts";

/////////////////////////////////////////////////////////////////////
/////// Payload helpers
/////////////////////////////////////////////////////////////////////

export function seed_payload(overrides: Record<string, unknown> = {}) {
	const data = {
		client_build_id: "build-1",
		deployment_id: "",
		matched_patterns: [],
		views_data: [],
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

export function route_response(
	overrides: Record<string, unknown> = {},
	build_id = "build-1",
): Response {
	const data = {
		matched_patterns: [],
		views_data: [],
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
			[BUILD_ID_HEADER]: build_id,
		},
	});
}

export function json_response(data: unknown = {}): Response {
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: { "Content-Type": "application/json" },
	});
}

export function text_response(text: string, status = 200): Response {
	return new Response(text, {
		status,
		headers: { "Content-Type": "text/plain" },
	});
}

export function no_content_response(): Response {
	return new Response(null, { status: 204 });
}

export function redirect_response(headers: Record<string, string>): Response {
	return new Response("", { status: 200, headers });
}

export function non_ok_redirect_response(
	headers: Record<string, string>,
	status = 500,
): Response {
	return new Response("error", { status, headers });
}

export function native_redirect_response(url: string): Response {
	const res = route_response();
	Object.defineProperty(res, "redirected", { value: true });
	Object.defineProperty(res, "url", { value: url });
	return res;
}

/////////////////////////////////////////////////////////////////////
/////// Async helpers
/////////////////////////////////////////////////////////////////////

export async function tick(n = 5) {
	for (let i = 0; i < n; i++) {
		await Promise.resolve();
	}
}

export function deferred<T>() {
	let resolve!: (v: T) => void;
	let reject!: (e: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

/////////////////////////////////////////////////////////////////////
/////// Commit helpers
/////////////////////////////////////////////////////////////////////

type CommitMock = {
	mock: {
		calls: unknown[][];
	};
};

type TestRouteRenderState = Omit<RouteRenderState, "entries" | "error"> & {
	entries: any[];
	error: any;
};

export function route_render_commit_count(commit: CommitMock): number {
	return commit.mock.calls.filter((call) => {
		const client_commit = call[0] as ClientCommit | undefined;
		return client_commit?.route_render !== undefined;
	}).length;
}

export function has_route_render_commit(commit: CommitMock): boolean {
	return route_render_commit_count(commit) > 0;
}

export function route_render_commit_at(
	commit: CommitMock,
	index: number,
): TestRouteRenderState {
	let seen = 0;
	for (const call of commit.mock.calls) {
		const client_commit = call[0] as ClientCommit | undefined;
		const state = client_commit?.route_render?.state;
		if (!state) {
			continue;
		}
		if (seen === index) {
			return state as TestRouteRenderState;
		}
		seen++;
	}
	throw new Error(`Expected route render commit at index ${index}`);
}

export function last_route_render_commit(commit: CommitMock): TestRouteRenderState {
	const count = route_render_commit_count(commit);
	if (count === 0) {
		throw new Error("Expected at least one route render commit");
	}
	return route_render_commit_at(commit, count - 1);
}

export function route_render_scroll_intent_at(
	commit: CommitMock,
	index: number,
): ScrollIntent | undefined {
	let seen = 0;
	for (const call of commit.mock.calls) {
		const client_commit = call[0] as ClientCommit | undefined;
		const route_render = client_commit?.route_render;
		if (!route_render) {
			continue;
		}
		if (seen === index) {
			return route_render.scroll_intent;
		}
		seen++;
	}
	throw new Error(`Expected route render commit at index ${index}`);
}

/////////////////////////////////////////////////////////////////////
/////// Fetch mock
/////////////////////////////////////////////////////////////////////

export type FetchCall = {
	init?: RequestInit;
	url: string;
	resolve: (r: Response) => void;
	reject: (e: unknown) => void;
	signal: AbortSignal;
};

export function mock_fetch() {
	const calls: FetchCall[] = [];

	function call(index: number): FetchCall {
		const entry = calls[index];
		if (!entry) {
			throw new Error(
				`Expected fetch call at index ${index}, but only ${calls.length} calls have been made`,
			);
		}
		return entry;
	}

	async function wait_for(count: number) {
		for (let i = 0; i < 50; i++) {
			if (calls.length >= count) {
				return;
			}
			await Promise.resolve();
		}
		throw new Error(`Expected ${count} fetch calls, got ${calls.length}`);
	}

	vi.spyOn(globalThis, "fetch").mockImplementation(
		(input: string | URL | Request, init?: RequestInit) => {
			const d = deferred<Response>();
			const signal = init?.signal;
			if (signal) {
				if (signal.aborted) {
					return Promise.reject(new DOMException("Aborted", "AbortError"));
				}
				signal.addEventListener(
					"abort",
					() => {
						d.reject(new DOMException("Aborted", "AbortError"));
					},
					{ once: true },
				);
			}
			calls.push({
				init,
				url:
					input instanceof URL
						? input.href
						: typeof input === "string"
							? input
							: input.url,
				resolve: d.resolve,
				reject: d.reject,
				signal: signal!,
			});
			return d.promise;
		},
	);

	return { calls, call, wait_for };
}

/////////////////////////////////////////////////////////////////////
/////// History / scroll helpers
/////////////////////////////////////////////////////////////////////

export function get_history_key(): string {
	const state = window.history.state;
	return state?.[HISTORY_KEY_FIELD] ?? "";
}

export function get_stored_scroll(key: string): { x: number; y: number } | undefined {
	const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
	if (!raw) {
		return undefined;
	}
	try {
		const entries = JSON.parse(raw);
		for (const [k, s] of entries) {
			if (k === key) {
				return s;
			}
		}
	} catch {}
	return undefined;
}

export function set_scroll_position(x: number, y: number) {
	Object.defineProperty(window, "scrollX", {
		value: x,
		writable: true,
		configurable: true,
	});
	Object.defineProperty(window, "scrollY", {
		value: y,
		writable: true,
		configurable: true,
	});
}

export function simulate_popstate(key: string, url: string) {
	window.history.replaceState({ [HISTORY_KEY_FIELD]: key }, "", url);
	window.dispatchEvent(new PopStateEvent("popstate"));
}

/////////////////////////////////////////////////////////////////////
/////// Core setup with listener tracking
/////////////////////////////////////////////////////////////////////

let _listener_cleanups: Array<() => void> = [];

export function cleanup_listeners() {
	for (const fn of _listener_cleanups) {
		fn();
	}
	_listener_cleanups = [];
}

export async function setup(
	opts: {
		payload?: Record<string, unknown>;
		clientOptions?: Record<string, any>;
	} = {},
) {
	seed_payload(opts.payload ?? {});
	const commit = vi.fn();
	const hard_redirect = vi.fn();
	const reload = vi.fn();
	const scroll_to = vi.fn();

	// Track listeners added during boot so we can remove them after each test
	const added_listeners: Array<[string, any]> = [];
	const orig_add = window.addEventListener.bind(window);
	const orig_remove = window.removeEventListener.bind(window);
	window.addEventListener = ((type: string, handler: any, ...rest: any[]) => {
		added_listeners.push([type, handler]);
		return orig_add(type, handler, ...rest);
	}) as any;

	const core_res = create_client_core({}, commit, {
		hard_redirect,
		reload,
		scroll_to,
	});
	if (!core_res.ok) {
		throw new Error(`create_client_core failed: ${core_res.err}`);
	}
	const core = core_res.val;
	await core.boot(opts.clientOptions ?? {});

	window.addEventListener = orig_add as any;
	_listener_cleanups.push(() => {
		for (const [type, handler] of added_listeners) {
			orig_remove(type, handler);
		}
	});

	commit.mockClear();
	return { core, commit, hard_redirect, reload, scroll_to };
}

/////////////////////////////////////////////////////////////////////
/////// Standard beforeEach / afterEach for all ccc test files
/////////////////////////////////////////////////////////////////////

export function register_ccc_lifecycle(
	before_each: (fn: () => void) => void,
	after_each: (fn: () => void) => void,
) {
	// Track every window listener added during a test (cores are often booted
	// directly, without setup()), so no listener leaks into the next test.
	let tracked_listeners: Array<[string, any]> = [];
	let orig_add: typeof window.addEventListener | null = null;

	before_each(() => {
		vi.restoreAllMocks();
		vi.resetModules();
		document.head.innerHTML = "";
		document.body.innerHTML = "";
		window.history.replaceState({}, "", "/");
		sessionStorage.clear();
		tracked_listeners = [];
		orig_add = window.addEventListener.bind(window);
		window.addEventListener = ((type: string, handler: any, ...rest: any[]) => {
			tracked_listeners.push([type, handler]);
			return orig_add!(type, handler, ...rest);
		}) as any;
	});

	after_each(() => {
		if (orig_add) {
			window.addEventListener = orig_add as any;
			orig_add = null;
		}
		for (const [type, handler] of tracked_listeners) {
			window.removeEventListener(type, handler);
		}
		tracked_listeners = [];
		cleanup_listeners();
	});
}

export const t_opts = () => {
	return {
		hard_redirect: vi.fn(),
		reload: vi.fn(),
		scroll_to: vi.fn(),
	};
};

export async function wait_until(check: () => boolean, message: string): Promise<void> {
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

export async function expect_revalidation_promise_resolves_ok(
	promise: Promise<unknown>,
): Promise<void> {
	const pending = Symbol("pending");
	const result = await Promise.race([
		promise,
		tick().then(() => {
			return pending;
		}),
	]);
	expect(result).toEqual({ ok: true });
}
