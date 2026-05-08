// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";
import {
	EFFECT_CLIENT_BUILD_ID_FIELD,
	EFFECT_DEPLOYMENT_ID_FIELD,
	create_client_core_effect,
} from "./create_client_core_effect.ts";
import type { ClientCommit } from "./effect_runtime/client_contract.ts";
import { REVALIDATION_DEBOUNCE_MS } from "./effect_runtime/revalidation_coordinator.ts";
import { ROUTE_PAYLOAD_FIELDS } from "./effect_runtime/route_preparer.ts";
import {
	WINDOW_EVENT_BEFOREUNLOAD,
	WINDOW_EVENT_FOCUS,
	WINDOW_EVENT_POPSTATE,
} from "./effect_runtime/runtime_lifecycle.ts";

const CLIENT_BUILD_ID = "build-1";
const DEPLOYMENT_ID = "deploy-1";
const API_MOUNT_ROOT = "/api/";
const ROOT_PATTERN = "/";
const ABOUT_PATTERN = "/about";

type CommitMock = ReturnType<typeof vi.fn<(commit: ClientCommit) => void>>;

type FetchCall = {
	url: string;
	init?: RequestInit;
	resolve: (response: Response) => void;
	reject: (error: unknown) => void;
	signal: AbortSignal;
};

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (error: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

function route_payload(overrides: Record<string, unknown> = {}) {
	const fields = ROUTE_PAYLOAD_FIELDS;
	return {
		[fields.matched_patterns]: [],
		[fields.loaders_data]: [],
		[fields.import_urls]: [],
		[fields.params]: {},
		[fields.splat_values]: [],
		[fields.title]: undefined,
		[fields.meta_head_elements]: [],
		[fields.rest_head_elements]: [],
		[fields.css_bundles]: [],
		[fields.deps]: [],
		...overrides,
	};
}

function seed_payload(overrides: Record<string, unknown> = {}): void {
	const script = document.createElement("script");
	script.id = DATA_SCRIPT_ID;
	script.type = "application/json";
	script.textContent = JSON.stringify({
		[EFFECT_CLIENT_BUILD_ID_FIELD]: CLIENT_BUILD_ID,
		[EFFECT_DEPLOYMENT_ID_FIELD]: DEPLOYMENT_ID,
		...route_payload(overrides),
	});
	document.head.appendChild(script);
}

function route_response(
	overrides: Record<string, unknown> = {},
	build_id = CLIENT_BUILD_ID,
): Response {
	return new Response(JSON.stringify(route_payload(overrides)), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			[BUILD_ID_HEADER]: build_id,
		},
	});
}

function json_response(data: unknown): Response {
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: { "Content-Type": "application/json" },
	});
}

function current_history_key(): string {
	const state = window.history.state;
	if (state && typeof state === "object") {
		const value = (state as Record<string, unknown>)[HISTORY_KEY_FIELD];
		if (typeof value === "string") {
			return value;
		}
	}
	return "";
}

function simulate_popstate(key: string, href: string): void {
	window.history.replaceState({ [HISTORY_KEY_FIELD]: key }, "", href);
	window.dispatchEvent(new PopStateEvent("popstate"));
}

function set_scroll_position(x: number, y: number): void {
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

function route_render_commits(commit: CommitMock): ClientCommit[] {
	return commit.mock.calls
		.map((call) => {
			return call[0];
		})
		.filter((client_commit): client_commit is ClientCommit => {
			return client_commit.route_render !== undefined;
		});
}

function route_render_commit_at(
	commit: CommitMock,
	index: number,
): NonNullable<ClientCommit["route_render"]> {
	const route_render = route_render_commits(commit)[index]?.route_render;
	if (!route_render) {
		throw new Error(`expected route render commit at index ${index}`);
	}
	return route_render;
}

function mock_fetch() {
	const calls: FetchCall[] = [];

	function call(index: number): FetchCall {
		const fetch_call = calls[index];
		if (!fetch_call) {
			throw new Error(`expected fetch call at index ${index}`);
		}
		return fetch_call;
	}

	async function wait_for(count: number): Promise<void> {
		for (let i = 0; i < 50; i++) {
			if (calls.length >= count) {
				return;
			}
			await Promise.resolve();
		}
		throw new Error(`expected ${count} fetch calls`);
	}

	vi.spyOn(globalThis, "fetch").mockImplementation(
		(input: string | URL | Request, init?: RequestInit) => {
			const pending = deferred<Response>();
			const signal = init?.signal;
			if (!signal) {
				throw new Error("expected fetch signal");
			}
			signal.addEventListener(
				"abort",
				() => {
					pending.reject(new DOMException("Aborted", "AbortError"));
				},
				{ once: true },
			);
			calls.push({
				url:
					input instanceof URL
						? input.href
						: typeof input === "string"
							? input
							: input.url,
				init,
				resolve: pending.resolve,
				reject: pending.reject,
				signal,
			});
			return pending.promise;
		},
	);

	return { calls, call, wait_for };
}

async function wait_until(
	predicate: () => boolean,
	message: string,
): Promise<void> {
	for (let i = 0; i < 50; i++) {
		if (predicate()) {
			return;
		}
		await Promise.resolve();
	}
	throw new Error(message);
}

beforeEach(() => {
	vi.restoreAllMocks();
	document.head.innerHTML = "";
	document.body.innerHTML = "";
	window.history.replaceState({}, "", "/");
	sessionStorage.clear();
});

afterEach(() => {
	vi.restoreAllMocks();
	vi.useRealTimers();
});

describe("ccc Effect client core adapter experiment", () => {
	it("boots through the Effect route preparer and publisher", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
			[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ root: true }],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		expect(core_result.ok).toBe(true);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}

		const boot_result = await core_result.val.boot({});

		expect(boot_result).toEqual({ ok: true, val: undefined });
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries).toHaveLength(1);
		expect(route_render.state.entries[0]?.pattern).toBe(ROOT_PATTERN);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			root: true,
		});
		expect(core_result.val.getClientBuildID()).toBe(CLIENT_BUILD_ID);
		expect(core_result.val.getRouteState()).toMatchObject({
			href: window.location.href,
			clientBuildID: CLIENT_BUILD_ID,
		});
	});

	it("boots with Effect-owned refresh scroll restoration", async () => {
		sessionStorage.setItem(
			SCROLL_STORAGE_RELOAD_KEY,
			JSON.stringify({
				x: 55,
				y: 77,
				unix: Date.now(),
				href: window.location.href,
			}),
		);
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}

		await core_result.val.boot({});

		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.scroll_intent?.scroll).toEqual({
			x: 55,
			y: 77,
		});
		expect(sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY)).toBeNull();
	});

	it("boots with a hash scroll intent from the Effect scroll service", async () => {
		window.history.replaceState({}, "", "/page#section");
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: ["/page"],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}

		await core_result.val.boot({});

		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.scroll_intent?.scroll).toEqual({
			hash: "#section",
		});
	});

	it("saves current scroll through the Effect scroll service", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		const key = current_history_key();

		set_scroll_position(31, 41);
		core_result.val.save_current_scroll();

		expect(sessionStorage.getItem(SCROLL_STORAGE_KEY)).toContain(key);
		expect(sessionStorage.getItem(SCROLL_STORAGE_KEY)).toContain("31");
	});

	it("shuts down the previous Effect lifecycle when a new core boots", async () => {
		const remove_listener = vi.spyOn(window, "removeEventListener");
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const first_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!first_result.ok) {
			throw new Error("expected first core creation to succeed");
		}
		await first_result.val.boot({
			revalidateOnWindowFocus: true,
		});
		remove_listener.mockClear();
		document.head.innerHTML = "";
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const second_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!second_result.ok) {
			throw new Error("expected second core creation to succeed");
		}

		await second_result.val.boot({});

		const removed_events = remove_listener.mock.calls.map((call) => {
			return call[0];
		});
		expect(removed_events).toContain(WINDOW_EVENT_FOCUS);
		expect(removed_events).toContain(WINDOW_EVENT_POPSTATE);
		expect(removed_events).toContain(WINDOW_EVENT_BEFOREUNLOAD);
	});

	it("shuts down the Effect lifecycle when render fails during boot", async () => {
		const remove_listener = vi.spyOn(window, "removeEventListener");
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}

		const boot_result = await core_result.val.boot({
			revalidateOnWindowFocus: true,
			render: async () => {
				throw new Error("render failed");
			},
		});

		expect(boot_result.ok).toBe(false);
		if (!boot_result.ok) {
			expect(boot_result.err).toContain("render failed");
		}
		const removed_events = remove_listener.mock.calls.map((call) => {
			return call[0];
		});
		expect(removed_events).toContain(WINDOW_EVENT_FOCUS);
		expect(removed_events).toContain(WINDOW_EVENT_POPSTATE);
		expect(removed_events).toContain(WINDOW_EVENT_BEFOREUNLOAD);
		expect(() => {
			core_result.val.getRouteState();
		}).toThrow("Vorma not booted");
	});

	it("tracks app-owned work through the Effect work indicator", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const work = deferred<number>();

		const tracked = core_result.val.workIndicator.track(work.promise);

		expect(core_result.val.workIndicator.isActive()).toBe(true);
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.start).toHaveBeenCalledTimes(1);
		work.resolve(42);
		await expect(tracked).resolves.toBe(42);
		expect(core_result.val.workIndicator.isActive()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.stop).toHaveBeenCalledTimes(1);
	});

	it("drives the Effect work indicator from navigation work", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.start).toHaveBeenCalledTimes(1);
		call(0).resolve(route_response());
		await navigation;
		await vi.advanceTimersByTimeAsync(1);

		expect(work_indicator.stop).toHaveBeenCalledTimes(1);
	});

	it("keeps app-owned Effect work active after navigation settles", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const app_work = deferred<void>();
		const tracked = core_result.val.workIndicator.track(app_work.promise);
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.start).toHaveBeenCalledTimes(1);
		call(0).resolve(route_response());
		await navigation;
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.stop).not.toHaveBeenCalled();

		app_work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.stop).toHaveBeenCalledTimes(1);
	});

	it("moves visible app-owned Effect work across work indicator replacement", async () => {
		vi.useFakeTimers();
		const first_work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		const second_work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({
			workIndicator: first_work_indicator,
		});
		const app_work = deferred<void>();
		const tracked = core_result.val.workIndicator.track(app_work.promise);
		await vi.advanceTimersByTimeAsync(1);
		expect(first_work_indicator.start).toHaveBeenCalledTimes(1);

		await core_result.val.boot({
			workIndicator: second_work_indicator,
		});
		await vi.advanceTimersByTimeAsync(1);
		expect(first_work_indicator.stop).toHaveBeenCalledTimes(1);
		expect(second_work_indicator.start).toHaveBeenCalledTimes(1);

		app_work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);
		expect(second_work_indicator.stop).toHaveBeenCalledTimes(1);
	});

	it("drives the Effect work indicator from revalidation work", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const { call, wait_for } = mock_fetch();

		const revalidation = core_result.val.revalidate();
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.start).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(route_response());
		await revalidation;
		await vi.advanceTimersByTimeAsync(1);

		expect(work_indicator.stop).toHaveBeenCalledTimes(1);
	});

	it("drives the Effect work indicator from API request work", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const { call, wait_for } = mock_fetch();

		const submission = core_result.val.submit_inner(
			"/api/action",
			{ method: "POST" },
			{ revalidate: false },
		);
		await wait_for(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(work_indicator.start).toHaveBeenCalledTimes(1);
		call(0).resolve(json_response({ ok: true }));
		await submission;
		await vi.advanceTimersByTimeAsync(1);

		expect(work_indicator.stop).toHaveBeenCalledTimes(1);
	});

	it("respects Effect work indicator navigation category skips", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
			skipNavigations: true,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		await vi.advanceTimersByTimeAsync(10);
		call(0).resolve(route_response());
		await navigation;
		await vi.advanceTimersByTimeAsync(10);

		expect(work_indicator.start).not.toHaveBeenCalled();
	});

	it("respects Effect work indicator per-navigation skips", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about", {
			skipWorkIndicator: true,
		});
		await wait_for(1);
		await vi.advanceTimersByTimeAsync(10);
		call(0).resolve(route_response());
		await navigation;
		await vi.advanceTimersByTimeAsync(10);

		expect(work_indicator.start).not.toHaveBeenCalled();
	});

	it("respects Effect work indicator per-submission skips", async () => {
		vi.useFakeTimers();
		const work_indicator = {
			stop: vi.fn(),
			stopDelayMS: 1,
			start: vi.fn(),
			startDelayMS: 1,
		};
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			vi.fn<(client_commit: ClientCommit) => void>(),
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ workIndicator: work_indicator });
		const { call, wait_for } = mock_fetch();

		const submission = core_result.val.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
				skipWorkIndicator: true,
			},
		);
		await wait_for(1);
		await vi.advanceTimersByTimeAsync(10);
		call(0).resolve(json_response({ ok: true }));
		await submission;
		await vi.advanceTimersByTimeAsync(10);

		expect(work_indicator.start).not.toHaveBeenCalled();
	});

	it("navigates through the Effect actor/fetcher/preparer/publisher path", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		call(0).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ABOUT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ page: "about" }],
			}),
		);
		const result = await navigation;

		expect(result).toEqual({ didNavigate: true });
		const request_url = new URL(call(0).url);
		expect(request_url.pathname).toBe(ABOUT_PATTERN);
		expect(request_url.searchParams.get(VORMA_JSON_KEY)).toBe(
			CLIENT_BUILD_ID,
		);
		expect(
			new Headers(call(0).init?.headers).get(X_ACCEPTS_CLIENT_REDIRECT),
		).toBe("1");
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.pattern).toBe(ABOUT_PATTERN);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			page: "about",
		});
		expect(core_result.val.getRouteState()).toMatchObject({
			href: new URL("/about", window.location.href).href,
			matches: [
				{
					pattern: ABOUT_PATTERN,
					loaderData: { page: "about" },
				},
			],
		});
		expect(window.location.href).toBe(
			new URL("/about", window.location.href).href,
		);
	});

	it("moves hash-only navigation through the Effect route publisher without fetching", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const { calls } = mock_fetch();

		const result = await core_result.val.navigate("/#section", {
			state: { tab: "overview" },
		});

		expect(result).toEqual({ didNavigate: true });
		expect(calls).toHaveLength(0);
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.scroll_intent?.scroll).toEqual({
			hash: "#section",
		});
		expect(core_result.val.getRouteState()).toMatchObject({
			href: new URL("/#section", window.location.href).href,
			historyState: { tab: "overview" },
		});
	});

	it("moves hash-only popstate through the Effect route publisher without fetching", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		const initial_key = current_history_key();
		commit.mockClear();
		const { calls } = mock_fetch();

		set_scroll_position(12, 34);
		await core_result.val.navigate("/#section");
		commit.mockClear();
		set_scroll_position(90, 100);
		simulate_popstate(initial_key, "/");
		await wait_until(() => {
			return route_render_commits(commit).length === 1;
		}, "expected hash-only Effect popstate to publish");

		expect(calls).toHaveLength(0);
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.scroll_intent?.scroll).toEqual({
			x: 12,
			y: 34,
		});
		expect(core_result.val.getRouteState()).toMatchObject({
			href: new URL("/", window.location.href).href,
		});
	});

	it("settles failed navigations without publishing a route render", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		call(0).resolve(
			new Response("nope", {
				status: 500,
				statusText: "Server Error",
			}),
		);
		const result = await navigation;

		expect(result).toEqual({ didNavigate: false });
		expect(route_render_commits(commit)).toHaveLength(0);
		expect(core_result.val.getRouteState()).toMatchObject({
			href: window.location.href,
		});
	});

	it("navigates on popstate through the Effect navigation actor without pushing history", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		const initial_key = current_history_key();
		const { call, wait_for } = mock_fetch();

		set_scroll_position(40, 80);
		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		call(0).resolve(route_response());
		await navigation;
		commit.mockClear();
		const push_spy = vi.spyOn(window.history, "pushState");

		simulate_popstate(initial_key, "/");
		await wait_for(2);
		call(1).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ home: true }],
			}),
		);
		await wait_until(() => {
			return route_render_commits(commit).length === 1;
		}, "expected Effect popstate navigation to publish");

		expect(push_spy).not.toHaveBeenCalled();
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			home: true,
		});
		expect(route_render.scroll_intent?.scroll).toEqual({
			x: 40,
			y: 80,
		});
		expect(core_result.val.getRouteState()).toMatchObject({
			href: new URL("/", window.location.href).href,
		});
	});

	it("reports newer-build navigation responses without blocking publish", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const on_build_skew = vi.fn();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ onBuildSkewDetected: on_build_skew });
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		call(0).resolve(
			route_response(
				{
					[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ABOUT_PATTERN],
					[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ page: "about" }],
				},
				"build-2",
			),
		);
		await expect(navigation).resolves.toEqual({ didNavigate: true });

		expect(route_render_commits(commit)).toHaveLength(1);
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: CLIENT_BUILD_ID,
				serverBuildID: "build-2",
				defaultBehavior: "notifyOnly",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "navigation",
					requestedHref: new URL("/about", window.location.href).href,
				}),
				currentWorkState: expect.objectContaining({
					navigation: expect.objectContaining({
						href: new URL("/about", window.location.href).href,
					}),
				}),
			}),
		);
	});

	it("hard redirects route-data build skew through the Effect reporter", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const hard_redirect = vi.fn();
		const on_build_skew = vi.fn();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
			{ hard_redirect },
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ onBuildSkewDetected: on_build_skew });
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		const navigation = core_result.val.navigate("/about");
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				headers: {
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: "1",
				},
			}),
		);
		await expect(navigation).resolves.toEqual({ didNavigate: false });

		expect(route_render_commits(commit)).toHaveLength(0);
		expect(hard_redirect).toHaveBeenCalledWith(
			new URL("/about", window.location.href).href,
		);
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: CLIENT_BUILD_ID,
				serverBuildID: "build-2",
				defaultBehavior: "hardReload",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "navigation",
					requestedHref: new URL("/about", window.location.href).href,
				}),
			}),
		);
	});

	it("prefetches through Effect without committing a route render", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		core_result.val.start_prefetch("/about");
		await wait_for(1);
		expect(core_result.val.getWorkState().prefetch).toEqual({
			href: new URL("/about", window.location.href).href,
		});
		call(0).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ABOUT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ prefetched: true }],
			}),
		);
		await wait_until(() => {
			return core_result.val.getWorkState().prefetch === null;
		}, "expected Effect prefetch work state to clear");

		expect(route_render_commits(commit)).toHaveLength(0);
	});

	it("promotes a prepared Effect prefetch into navigation without refetching", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const { calls, call, wait_for } = mock_fetch();

		core_result.val.start_prefetch("/about#one");
		await wait_for(1);
		call(0).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ABOUT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ prefetched: true }],
			}),
		);
		await wait_until(() => {
			return core_result.val.getWorkState().prefetch === null;
		}, "expected Effect prefetch to finish");

		const result = await core_result.val.navigate("/about#two");

		expect(result).toEqual({ didNavigate: true });
		expect(calls).toHaveLength(1);
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			prefetched: true,
		});
		expect(window.location.href).toBe(
			new URL("/about#two", window.location.href).href,
		);
	});

	it("aborts an in-flight Effect prefetch when stopped", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		const { call, wait_for } = mock_fetch();

		core_result.val.start_prefetch("/about");
		await wait_for(1);
		expect(call(0).signal.aborted).toBe(false);
		core_result.val.stop_prefetch("/about#later");

		expect(call(0).signal.aborted).toBe(true);
		await wait_until(() => {
			return core_result.val.getWorkState().prefetch === null;
		}, "expected stopped Effect prefetch work state to clear");
	});

	it("skips Effect prefetches for current or cross-origin routes", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		const { calls } = mock_fetch();

		core_result.val.start_prefetch("/");
		core_result.val.start_prefetch("https://example.test/about");
		await Promise.resolve();

		expect(calls).toHaveLength(0);
	});

	it("revalidates through the Effect coordinator without changing history", async () => {
		vi.useFakeTimers();
		seed_payload({
			[EFFECT_DEPLOYMENT_ID_FIELD]: DEPLOYMENT_ID,
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
			[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ fresh: false }],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const { call, wait_for } = mock_fetch();

		const revalidation = core_result.val.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ fresh: true }],
			}),
		);
		await expect(revalidation).resolves.toEqual({ ok: true });

		const request_url = new URL(call(0).url);
		expect(request_url.searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY)).toBe(
			DEPLOYMENT_ID,
		);
		expect(push_spy).not.toHaveBeenCalled();
		expect(replace_spy).not.toHaveBeenCalled();
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			fresh: true,
		});
		expect(core_result.val.getWorkState().revalidation).toBeNull();
	});

	it("reports build skew from Effect revalidation and drops the response", async () => {
		vi.useFakeTimers();
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const on_build_skew = vi.fn();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({
			onBuildSkewDetected: on_build_skew,
		});
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		const revalidation = core_result.val.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				headers: {
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: "1",
				},
			}),
		);
		await expect(revalidation).resolves.toEqual({
			ok: false,
			reason: "build_skew",
		});

		expect(route_render_commits(commit)).toHaveLength(0);
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: CLIENT_BUILD_ID,
				serverBuildID: "build-2",
				defaultBehavior: "dropResponse",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "manual",
					requestedHref: window.location.href,
				}),
			}),
		);
	});

	it("fires Effect revalidation on window focus after stale time elapses", async () => {
		vi.useFakeTimers();
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({
			revalidateOnWindowFocus: { staleTimeMS: 100 },
		});
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ focused: true }],
			}),
		);
		await wait_until(() => {
			return route_render_commits(commit).length === 1;
		}, "expected focused Effect revalidation to publish");

		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			focused: true,
		});
	});

	it("does not focus-revalidate before the Effect stale window", async () => {
		vi.useFakeTimers();
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({
			revalidateOnWindowFocus: { staleTimeMS: 1_000 },
		});
		const { calls } = mock_fetch();

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);

		expect(calls).toHaveLength(0);
	});

	it("reports window focus as the Effect build-skew revalidation reason", async () => {
		vi.useFakeTimers();
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const on_build_skew = vi.fn();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({
			onBuildSkewDetected: on_build_skew,
			revalidateOnWindowFocus: { staleTimeMS: 100 },
		});
		const { call, wait_for } = mock_fetch();

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				headers: {
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: "1",
				},
			}),
		);
		await wait_until(() => {
			return on_build_skew.mock.calls.length > 0;
		}, "expected focused Effect build skew report");

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: CLIENT_BUILD_ID,
				serverBuildID: "build-2",
				defaultBehavior: "dropResponse",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "windowFocus",
				}),
			}),
		);
	});

	it("turns a revalidation soft redirect into a replace navigation", async () => {
		vi.useFakeTimers();
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const { call, wait_for } = mock_fetch();

		const revalidation = core_result.val.revalidate();
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				headers: {
					[X_CLIENT_REDIRECT]: ABOUT_PATTERN,
				},
			}),
		);
		await wait_for(2);
		call(1).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ABOUT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ redirected: true }],
			}),
		);
		await expect(revalidation).resolves.toEqual({ ok: true });

		expect(push_spy).not.toHaveBeenCalled();
		expect(replace_spy).toHaveBeenCalled();
		expect(window.location.href).toBe(
			new URL(ABOUT_PATTERN, window.location.href).href,
		);
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			redirected: true,
		});
	});

	it("submits through the Effect submit manager with the existing APIResult shape", async () => {
		seed_payload({
			[EFFECT_DEPLOYMENT_ID_FIELD]: DEPLOYMENT_ID,
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		const { call, wait_for } = mock_fetch();

		const submission = core_result.val.submit_inner<{ saved: boolean }>(
			"/api/save",
			{
				method: "POST",
				body: { name: "Ada" } as unknown as BodyInit,
			},
			{ revalidate: false },
		);
		await wait_for(1);
		expect(core_result.val.getWorkState().apiRequests).toEqual([
			expect.objectContaining({
				href: new URL("/api/save", window.location.href).href,
				method: "POST",
			}),
		]);
		call(0).resolve(json_response({ saved: true }));
		const result = await submission;

		const request_headers = new Headers(call(0).init?.headers);
		expect(request_headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe("1");
		expect(request_headers.get(VERCEL_X_DEPLOYMENT_ID)).toBe(DEPLOYMENT_ID);
		expect(request_headers.get("Content-Type")).toBe("application/json");
		expect(call(0).init?.method).toBe("POST");
		const request_body = call(0).init?.body;
		if (typeof request_body !== "string") {
			throw new Error("expected JSON request body");
		}
		expect(JSON.parse(request_body)).toEqual({
			name: "Ada",
		});
		expect(result).toMatchObject({
			success: true,
			data: { saved: true },
		});
		await expect(result.revalidationPromise).resolves.toEqual({
			ok: true,
		});
		expect(core_result.val.getWorkState().apiRequests).toEqual([]);
	});

	it("reports API route build skew through the shared Effect reporter", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const on_build_skew = vi.fn();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({ onBuildSkewDetected: on_build_skew });
		const { call, wait_for } = mock_fetch();

		const submission = core_result.val.submit_inner(
			"/api/save",
			{ method: "POST" },
			{ revalidate: false },
		);
		await wait_for(1);
		call(0).resolve(
			new Response(JSON.stringify({ saved: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					[BUILD_ID_HEADER]: "build-2",
				},
			}),
		);
		await expect(submission).resolves.toMatchObject({
			success: true,
			data: { saved: true },
		});

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: CLIENT_BUILD_ID,
				serverBuildID: "build-2",
				defaultBehavior: "notifyOnly",
				triggeringResponse: expect.objectContaining({
					kind: "apiRoute",
					apiRouteKind: "mutation",
					requestedHref: new URL("/api/save", window.location.href)
						.href,
					method: "POST",
					status: 200,
					ok: true,
				}),
			}),
		);
	});

	it("auto-revalidates Effect submit mutations through the route revalidator", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
			[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ fresh: false }],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const { call, wait_for } = mock_fetch();

		const submission = core_result.val.submit_inner(
			"/api/save",
			{ method: "POST" },
			{},
		);
		await wait_for(1);
		call(0).resolve(json_response({ saved: true }));
		const result = await submission;
		await wait_for(2);
		call(1).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ fresh: true }],
			}),
		);

		await expect(result.revalidationPromise).resolves.toEqual({
			ok: true,
		});
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			fresh: true,
		});
	});

	it("turns a submit soft redirect into a replace navigation", async () => {
		seed_payload({
			[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ROOT_PATTERN],
		});
		const commit = vi.fn<(client_commit: ClientCommit) => void>();
		const core_result = create_client_core_effect(
			{ apiMountRoot: API_MOUNT_ROOT },
			commit,
		);
		if (!core_result.ok) {
			throw new Error("expected core creation to succeed");
		}
		await core_result.val.boot({});
		commit.mockClear();
		const push_spy = vi.spyOn(window.history, "pushState");
		const replace_spy = vi.spyOn(window.history, "replaceState");
		const { call, wait_for } = mock_fetch();

		const submission = core_result.val.submit_inner(
			"/api/save",
			{ method: "POST" },
			{ revalidate: false },
		);
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				headers: {
					[X_CLIENT_REDIRECT]: ABOUT_PATTERN,
				},
			}),
		);
		await wait_for(2);
		call(1).resolve(
			route_response({
				[ROUTE_PAYLOAD_FIELDS.matched_patterns]: [ABOUT_PATTERN],
				[ROUTE_PAYLOAD_FIELDS.loaders_data]: [{ redirected: true }],
			}),
		);
		const result = await submission;
		for (let i = 0; i < 10; i++) {
			if (route_render_commits(commit).length > 0) {
				break;
			}
			await Promise.resolve();
		}

		expect(result).toMatchObject({ success: true });
		expect(push_spy).not.toHaveBeenCalled();
		expect(replace_spy).toHaveBeenCalled();
		expect(window.location.href).toBe(
			new URL(ABOUT_PATTERN, window.location.href).href,
		);
		const route_render = route_render_commit_at(commit, 0);
		expect(route_render.state.entries[0]?.loader_data).toEqual({
			redirected: true,
		});
	});
});
