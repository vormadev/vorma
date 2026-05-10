import { afterEach, describe, expect, it, vi } from "vitest";
import {
	API_SUBMIT_CROSS_ORIGIN_ERROR,
	BUILD_ID_HEADER,
	CONTENT_TYPE_HEADER,
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	JSON_CONTENT_TYPE,
	SCROLL_STORAGE_KEY,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import {
	apply_scroll,
	browser_fetch_api,
	browser_fetch_route,
	browser_read_boot_payload,
	read_browser_position,
	read_saved_scroll,
	run_browser_view_transition,
	save_current_scroll,
	save_scroll_position,
	write_browser_history,
} from "./browser_host.ts";
import { boot_payload_field, route_payload_field } from "./route_payload.ts";

type FetchCall = {
	init?: RequestInit;
	input: RequestInfo | URL;
};

function fetch_once(response: Response): {
	calls: FetchCall[];
	fetch_fn: typeof fetch;
} {
	const calls: FetchCall[] = [];
	return {
		calls,
		fetch_fn: ((input, init) => {
			calls.push({ init, input });
			return Promise.resolve(response);
		}) as typeof fetch,
	};
}

afterEach(() => {
	document.body.innerHTML = "";
	delete (document as Document as { startViewTransition?: unknown })
		.startViewTransition;
	sessionStorage.clear();
	vi.restoreAllMocks();
	window.history.replaceState({}, "", "/");
});

describe("browser boot payload", () => {
	it("reads boot payload facts from the data script", () => {
		window.history.replaceState(
			{ [HISTORY_USER_STATE_FIELD]: { boot: true } },
			"",
			"/boot?tab=details",
		);
		const script = document.createElement("script");
		script.id = DATA_SCRIPT_ID;
		script.type = JSON_CONTENT_TYPE;
		script.textContent = JSON.stringify({
			[boot_payload_field.client_build_id]: "build-1",
			[boot_payload_field.deployment_id]: "dpl_123",
			[route_payload_field.import_urls]: ["/boot.js"],
			[route_payload_field.loaders_data]: [{ server: true }],
			[route_payload_field.matched_patterns]: ["/boot"],
			[route_payload_field.params]: { tab: "details" },
			[route_payload_field.splat_values]: ["tail"],
		});
		document.body.append(script);

		const facts = browser_read_boot_payload({
			fallback_browser_key: "browser-1",
			operation_id: "boot-1",
			type: "read_boot_payload",
		});

		expect(facts.client_build_id).toBe("build-1");
		expect(facts.deployment_id).toBe("dpl_123");
		expect(facts.payload).toMatchObject({
			[route_payload_field.matched_patterns]: ["/boot"],
		});
		expect(facts.route).toEqual({
			client_build_id: "build-1",
			error: null,
			history_state: { boot: true },
			href: "http://localhost:3000/boot?tab=details",
			matches: [
				{
					client_loader_data: undefined,
					input: {},
					loader_data: { server: true },
					module: undefined,
					module_url: "/boot.js",
					pattern: "/boot",
				},
			],
			params: { tab: "details" },
			splat_values: ["tail"],
		});
		expect(facts.render).toEqual({
			client_build_id: "build-1",
			entries: facts.route.matches,
			error: null,
			history_state: { boot: true },
			params: { tab: "details" },
			splat_values: ["tail"],
		});
		expect(window.history.state[HISTORY_KEY_FIELD]).toBe("browser-1");
	});
});

describe("browser route fetch", () => {
	it("requests route JSON with build and deployment facts", async () => {
		const { calls, fetch_fn } = fetch_once(
			Response.json(
				{ route: true },
				{
					headers: { [BUILD_ID_HEADER]: "build-1" },
				},
			),
		);
		const signal = new AbortController().signal;

		const facts = await browser_fetch_route(
			{
				client_build_id: "build-1",
				deployment_id: "dpl_123",
				href: "/current?existing=1",
				operation_id: "reval-1",
				trigger: "revalidation",
				type: "fetch_route",
			},
			signal,
			fetch_fn,
		);

		expect(facts).toEqual({
			kind: "data",
			ok: true,
			payload: { route: true },
			server_build_id: "build-1",
			status: 200,
		});
		expect(calls).toHaveLength(1);
		const call = calls[0]!;
		const url = call.input as URL;
		expect(url.searchParams.get(VORMA_JSON_KEY)).toBe("build-1");
		expect(url.searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY)).toBe(
			"dpl_123",
		);
		expect(
			new Headers(call.init?.headers).get(X_ACCEPTS_CLIENT_REDIRECT),
		).toBe(VORMA_PROTOCOL_ENABLED);
		expect(call.init?.signal).toBe(signal);
	});

	it("turns protocol headers into route response facts", async () => {
		const { fetch_fn: build_skew_fetch } = fetch_once(
			new Response(null, {
				headers: {
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
				},
				status: 200,
			}),
		);

		await expect(
			browser_fetch_route(
				{
					client_build_id: "build-1",
					href: "/current",
					operation_id: "nav-1",
					trigger: "navigation",
					type: "fetch_route",
				},
				new AbortController().signal,
				build_skew_fetch,
			),
		).resolves.toEqual({
			kind: "build_skew",
			ok: true,
			server_build_id: "build-2",
			status: 200,
		});

		const { fetch_fn: redirect_fetch } = fetch_once(
			new Response(null, {
				headers: {
					[BUILD_ID_HEADER]: "build-1",
					[X_CLIENT_REDIRECT]: "/next",
				},
				status: 204,
			}),
		);

		await expect(
			browser_fetch_route(
				{
					client_build_id: "build-1",
					href: "/current",
					operation_id: "nav-1",
					trigger: "navigation",
					type: "fetch_route",
				},
				new AbortController().signal,
				redirect_fetch,
			),
		).resolves.toEqual({
			hard: false,
			href: "http://localhost:3000/next",
			http: true,
			kind: "redirect",
			ok: true,
			server_build_id: "build-1",
			status: 204,
		});
	});
});

describe("browser API fetch", () => {
	it("rejects cross-origin API submissions before fetch", async () => {
		const fetch_fn = vi.fn<typeof fetch>();

		const facts = await browser_fetch_api(
			{
				href: "https://elsewhere.example/api",
				method: "POST",
				operation_id: "api-1",
				submission_key: "submit-1",
				type: "fetch_api",
			},
			new AbortController().signal,
			fetch_fn,
		);

		expect(facts).toEqual({
			dispatched: false,
			error: `${API_SUBMIT_CROSS_ORIGIN_ERROR} "https://elsewhere.example/api".`,
			kind: "failure",
			should_revalidate: false,
		});
		expect(fetch_fn).not.toHaveBeenCalled();
	});

	it("dispatches same-origin API requests with protocol headers", async () => {
		const { calls, fetch_fn } = fetch_once(
			Response.json(
				{ ok: true },
				{
					headers: { [BUILD_ID_HEADER]: "build-1" },
				},
			),
		);
		const signal = new AbortController().signal;

		const facts = await browser_fetch_api(
			{
				deployment_id: "dpl_123",
				href: "/api/save",
				method: "POST",
				operation_id: "api-1",
				request_init: {
					body: { ok: true } as unknown as BodyInit,
					headers: { "X-Test": "yes" },
				},
				submission_key: "submit-1",
				type: "fetch_api",
			},
			signal,
			fetch_fn,
		);

		expect(facts).toMatchObject({
			data: { ok: true },
			kind: "success",
			ok: true,
			server_build_id: "build-1",
			status: 200,
		});
		expect(calls).toHaveLength(1);
		const call = calls[0]!;
		const headers = new Headers(call.init?.headers);
		expect((call.input as URL).href).toBe("http://localhost:3000/api/save");
		expect(call.init?.method).toBe("POST");
		expect(call.init?.signal).toBe(signal);
		expect(call.init?.body).toBe(JSON.stringify({ ok: true }));
		expect(headers.get(VERCEL_X_DEPLOYMENT_ID)).toBe("dpl_123");
		expect(headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe(
			VORMA_PROTOCOL_ENABLED,
		);
		expect(headers.get("X-Test")).toBe("yes");
		expect(headers.get(CONTENT_TYPE_HEADER)).toBe(JSON_CONTENT_TYPE);
	});
});

describe("browser history position", () => {
	it("reads the current browser position and installs a fallback key once", () => {
		window.history.replaceState(
			{ [HISTORY_USER_STATE_FIELD]: { from: "user" } },
			"",
			"/current",
		);

		expect(read_browser_position("browser-1")).toEqual({
			href: "http://localhost:3000/current",
			key: "browser-1",
			state: { from: "user" },
		});
		expect(window.history.state).toEqual({
			[HISTORY_KEY_FIELD]: "browser-1",
			[HISTORY_USER_STATE_FIELD]: { from: "user" },
		});

		expect(read_browser_position("browser-2")).toEqual({
			href: "http://localhost:3000/current",
			key: "browser-1",
			state: { from: "user" },
		});
	});

	it("writes user state under the history contract fields", () => {
		window.history.replaceState({ keep: true }, "", "/current");

		write_browser_history({
			position: {
				href: "http://localhost:3000/replaced",
				key: "browser-2",
				state: { next: true },
			},
			replace: true,
		});

		expect(window.location.href).toBe("http://localhost:3000/replaced");
		expect(window.history.state).toEqual({
			keep: true,
			[HISTORY_KEY_FIELD]: "browser-2",
			[HISTORY_USER_STATE_FIELD]: { next: true },
		});

		write_browser_history({
			position: {
				href: "http://localhost:3000/pushed",
				key: "browser-3",
				state: undefined,
			},
			replace: false,
		});

		expect(window.location.href).toBe("http://localhost:3000/pushed");
		expect(window.history.state).toEqual({
			[HISTORY_KEY_FIELD]: "browser-3",
			[HISTORY_USER_STATE_FIELD]: undefined,
		});
	});
});

describe("browser scroll storage", () => {
	it("saves and reads coordinate scroll by browser key", () => {
		save_scroll_position("browser-1", { x: 10, y: 20 });
		save_scroll_position("browser-2", { x: 30, y: 40 });

		expect(read_saved_scroll("browser-1")).toEqual({ x: 10, y: 20 });
		expect(read_saved_scroll("browser-2")).toEqual({ x: 30, y: 40 });
	});

	it("ignores empty keys, hash scroll, and malformed stored entries", () => {
		save_scroll_position("", { x: 10, y: 20 });
		save_scroll_position("browser-1", { hash: "#section" });
		sessionStorage.setItem(
			SCROLL_STORAGE_KEY,
			JSON.stringify([
				["bad-scroll", { x: "10", y: 20 }],
				["browser-2", { x: 30, y: 40 }],
			]),
		);

		expect(read_saved_scroll("browser-1")).toBeUndefined();
		expect(read_saved_scroll("browser-2")).toEqual({ x: 30, y: 40 });
	});

	it("saves the current scroll under the current history key", () => {
		window.history.replaceState(
			{ [HISTORY_KEY_FIELD]: "browser-1" },
			"",
			"/current",
		);
		vi.spyOn(window, "scrollX", "get").mockReturnValue(15);
		vi.spyOn(window, "scrollY", "get").mockReturnValue(25);

		save_current_scroll();

		expect(read_saved_scroll("browser-1")).toEqual({ x: 15, y: 25 });
	});
});

describe("browser scroll application", () => {
	it("scrolls to decoded hash targets", () => {
		const el = document.createElement("div");
		el.id = "hello world";
		el.scrollIntoView = vi.fn();
		document.body.append(el);

		apply_scroll({ hash: "#hello%20world" });

		expect(el.scrollIntoView).toHaveBeenCalledTimes(1);
	});

	it("uses coordinate scrolling for saved positions", () => {
		const scroll_to = vi.fn();

		apply_scroll({ x: 50, y: 75 }, { scroll_to });

		expect(scroll_to).toHaveBeenCalledWith(50, 75);
	});
});

describe("browser view transitions", () => {
	it("publishes directly when view transitions are unavailable", async () => {
		const log: string[] = [];

		await run_browser_view_transition(async () => {
			log.push("publish");
		});

		expect(log).toEqual(["publish"]);
	});

	it("waits for the transition update callback when available", async () => {
		const log: string[] = [];
		let finish!: () => void;
		const transition_done = new Promise<void>((resolve) => {
			finish = resolve;
		});
		(
			document as Document as {
				startViewTransition?: (publish: () => unknown) => {
					updateCallbackDone: Promise<void>;
				};
			}
		).startViewTransition = (publish) => {
			log.push("start");
			void publish();
			return { updateCallbackDone: transition_done };
		};

		const running = run_browser_view_transition(async () => {
			log.push("publish");
		});

		await Promise.resolve();
		expect(log).toEqual(["start", "publish"]);
		finish();
		await running;
	});
});
