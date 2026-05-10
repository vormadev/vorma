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
import type {
	APIResponseFacts,
	BrowserKey,
	BrowserPosition,
	CoreEffect,
	RouteResponseFacts,
	ScrollState,
} from "./model.ts";
import { boot_payload_field, decode_route_payload } from "./route_payload.ts";
import type { BootPayloadFacts } from "./runner.ts";

const max_scroll_entries = 50;

type StoredScrollEntry = [BrowserKey, { x: number; y: number }];
type EffectOf<Type extends CoreEffect["type"]> = Extract<
	CoreEffect,
	{ type: Type }
>;
type FetchFunction = typeof fetch;
type ResponseRedirect = { hard: boolean; href: string; http: boolean };

type DocumentWithViewTransitions = Document & {
	startViewTransition?: (publish: () => unknown) => {
		finished?: Promise<unknown>;
		updateCallbackDone?: Promise<unknown>;
	};
};

export async function browser_fetch_route(
	effect: EffectOf<"fetch_route">,
	signal: AbortSignal,
	fetch_fn: FetchFunction = fetch,
): Promise<RouteResponseFacts> {
	const url = new URL(effect.href, window.location.href);
	url.searchParams.set(VORMA_JSON_KEY, effect.client_build_id);
	if (effect.trigger === "revalidation" && effect.deployment_id) {
		url.searchParams.set(VERCEL_DPL_QUERY_PARAM_KEY, effect.deployment_id);
	}
	const response = await fetch_fn(url, {
		headers: {
			[X_ACCEPTS_CLIENT_REDIRECT]: VORMA_PROTOCOL_ENABLED,
		},
		signal,
	});
	const server_build_id = response.headers.get(BUILD_ID_HEADER) ?? "";
	if (response.headers.get(X_VORMA_BUILD_SKEW) === VORMA_PROTOCOL_ENABLED) {
		return {
			kind: "build_skew",
			ok: response.ok,
			server_build_id,
			status: response.status,
		};
	}
	const redirect = response_redirect(response, url);
	if (redirect) {
		return {
			hard: redirect.hard,
			href: redirect.href,
			http: redirect.http,
			kind: "redirect",
			ok: response.ok,
			server_build_id,
			status: response.status,
		};
	}
	if (!response.ok) {
		return {
			kind: "error",
			ok: response.ok,
			server_build_id,
			status: response.status,
			status_text: response.statusText,
		};
	}
	try {
		return {
			kind: "data",
			ok: response.ok,
			payload: await response.json(),
			server_build_id,
			status: response.status,
		};
	} catch {
		return {
			kind: "error",
			ok: response.ok,
			server_build_id,
			status: response.status,
			status_text: response.statusText,
		};
	}
}

export async function browser_fetch_api(
	effect: EffectOf<"fetch_api">,
	signal: AbortSignal,
	fetch_fn: FetchFunction = fetch,
): Promise<APIResponseFacts> {
	const url = new URL(effect.href, window.location.href);
	if (!is_same_origin_href(url.href)) {
		return {
			dispatched: false,
			error: `${API_SUBMIT_CROSS_ORIGIN_ERROR} "${url.href}".`,
			kind: "failure",
			should_revalidate: false,
		};
	}
	const headers = new Headers();
	if (effect.deployment_id) {
		headers.set(VERCEL_X_DEPLOYMENT_ID, effect.deployment_id);
	}
	new Headers(effect.request_init?.headers ?? undefined).forEach(
		(value, key) => {
			headers.set(key, value);
		},
	);
	headers.set(X_ACCEPTS_CLIENT_REDIRECT, VORMA_PROTOCOL_ENABLED);
	const init: RequestInit = {
		...effect.request_init,
		headers,
		method: effect.method,
		signal,
	};
	const read_method = effect.method === "GET" || effect.method === "HEAD";
	if (read_method) {
		delete init.body;
	} else {
		const body = effect.request_init?.body as unknown;
		const native_body =
			body instanceof ReadableStream ||
			body instanceof FormData ||
			body instanceof URLSearchParams ||
			body instanceof Blob ||
			body instanceof ArrayBuffer ||
			ArrayBuffer.isView(body);
		if (
			body !== undefined &&
			body !== null &&
			typeof body === "object" &&
			!native_body
		) {
			init.body = JSON.stringify(body);
			if (!headers.has(CONTENT_TYPE_HEADER)) {
				headers.set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE);
			}
		}
	}
	const response = await fetch_fn(url, init);
	const server_build_id = response.headers.get(BUILD_ID_HEADER) ?? "";
	const redirect = response_redirect(response, url);
	if (redirect) {
		return {
			hard: redirect.hard,
			href: redirect.href,
			http: redirect.http,
			kind: "redirect",
			ok: response.ok,
			response,
			server_build_id,
			status: response.status,
		};
	}
	if (!response.ok) {
		return {
			dispatched: true,
			error: response.statusText,
			kind: "failure",
			ok: response.ok,
			response,
			server_build_id,
			status: response.status,
		};
	}
	return {
		data: await read_api_response_data(response),
		kind: "success",
		ok: response.ok,
		response,
		server_build_id,
		status: response.status,
	};
}

export function browser_read_boot_payload(
	effect: EffectOf<"read_boot_payload">,
): BootPayloadFacts {
	const element = document.getElementById(DATA_SCRIPT_ID);
	if (!element) {
		throw new Error(`Missing element: #${DATA_SCRIPT_ID}`);
	}
	const payload = JSON.parse(element.textContent ?? "{}") as unknown;
	const raw =
		payload && typeof payload === "object"
			? (payload as Record<string, unknown>)
			: {};
	const raw_client_build_id = raw[boot_payload_field.client_build_id];
	const raw_deployment_id = raw[boot_payload_field.deployment_id];
	const client_build_id =
		typeof raw_client_build_id === "string" ? raw_client_build_id : "";
	const deployment_id =
		typeof raw_deployment_id === "string" ? raw_deployment_id : "";
	const browser = read_browser_position(effect.fallback_browser_key);
	return {
		client_build_id,
		deployment_id,
		payload,
		...decode_route_payload({
			client_build_id,
			history_state: browser.state,
			href: browser.href,
			payload,
		}),
	};
}

export function read_browser_position(
	fallback_key?: BrowserKey,
): BrowserPosition {
	const state_record = browser_history_state_record();
	const existing_key = state_record[HISTORY_KEY_FIELD];
	const key =
		typeof existing_key === "string" ? existing_key : (fallback_key ?? "");
	if (
		key &&
		(typeof existing_key !== "string" || existing_key.length === 0)
	) {
		window.history.replaceState(
			{
				...state_record,
				[HISTORY_KEY_FIELD]: key,
			},
			"",
			window.location.href,
		);
	}
	return {
		href: window.location.href,
		key,
		state: state_record[HISTORY_USER_STATE_FIELD],
	};
}

export function write_browser_history(input: {
	position: BrowserPosition;
	replace: boolean;
}): void {
	const next_state = {
		...(input.replace ? browser_history_state_record() : {}),
		[HISTORY_KEY_FIELD]: input.position.key,
		[HISTORY_USER_STATE_FIELD]: input.position.state,
	};
	if (input.replace) {
		window.history.replaceState(next_state, "", input.position.href);
		return;
	}
	window.history.pushState(next_state, "", input.position.href);
}

export function read_current_scroll(): ScrollState {
	return { x: window.scrollX, y: window.scrollY };
}

export function save_current_scroll(): void {
	const position = read_browser_position();
	if (!position.key) {
		return;
	}
	save_scroll_position(position.key, read_current_scroll());
}

export function save_scroll_position(
	key: BrowserKey,
	scroll: ScrollState,
): void {
	if (!key || "hash" in scroll) {
		return;
	}
	const entries = read_scroll_entries().filter((entry) => {
		return entry[0] !== key;
	});
	entries.push([key, scroll]);
	if (entries.length > max_scroll_entries) {
		entries.splice(0, entries.length - max_scroll_entries);
	}
	try {
		sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
	} catch {}
}

export function read_saved_scroll(key: BrowserKey): ScrollState | undefined {
	if (!key) {
		return undefined;
	}
	for (const [entry_key, scroll] of read_scroll_entries()) {
		if (entry_key === key) {
			return scroll;
		}
	}
	return undefined;
}

export function apply_scroll(
	scroll: ScrollState,
	options?: {
		scroll_to?: (x: number, y: number) => void;
	},
): void {
	if ("hash" in scroll) {
		const raw = scroll.hash.startsWith("#")
			? scroll.hash.slice(1)
			: scroll.hash;
		let id: string;
		try {
			id = decodeURIComponent(raw);
		} catch {
			id = raw;
		}
		document.getElementById(id)?.scrollIntoView();
		return;
	}
	const scroll_to = options?.scroll_to ?? window.scrollTo.bind(window);
	scroll_to(scroll.x, scroll.y);
}

export async function run_browser_view_transition(
	publish: () => Promise<void>,
): Promise<void> {
	const start_view_transition = (document as DocumentWithViewTransitions)
		.startViewTransition;
	if (typeof start_view_transition !== "function") {
		await publish();
		return;
	}
	const transition = start_view_transition.call(document, publish);
	await (transition.updateCallbackDone ?? transition.finished);
}

function browser_history_state_record(): Record<string, unknown> {
	const state = window.history.state;
	if (!state || typeof state !== "object") {
		return {};
	}
	return state as Record<string, unknown>;
}

function read_scroll_entries(): StoredScrollEntry[] {
	try {
		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		if (!raw) {
			return [];
		}
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) {
			return [];
		}
		return parsed.filter((entry): entry is StoredScrollEntry => {
			if (!Array.isArray(entry) || entry.length !== 2) {
				return false;
			}
			const scroll = entry[1];
			return (
				typeof entry[0] === "string" &&
				!!scroll &&
				typeof scroll === "object" &&
				Number.isFinite((scroll as { x?: unknown }).x) &&
				Number.isFinite((scroll as { y?: unknown }).y)
			);
		});
	} catch {
		return [];
	}
}

function response_redirect(
	response: Response,
	base: URL,
): ResponseRedirect | null {
	const soft_redirect = response.headers.get(X_CLIENT_REDIRECT);
	if (soft_redirect) {
		const href = new URL(soft_redirect, base).href;
		const http = is_http_href(href);
		return {
			hard: http && !is_same_origin_href(href),
			href,
			http,
		};
	}
	if (response.redirected && response.url && response.url !== base.href) {
		const href = new URL(response.url, base).href;
		const http = is_http_href(href);
		return {
			hard: http && !is_same_origin_href(href),
			href,
			http,
		};
	}
	return null;
}

function is_http_href(href: string): boolean {
	try {
		const protocol = new URL(href, window.location.href).protocol;
		return protocol === "http:" || protocol === "https:";
	} catch {
		return false;
	}
}

function is_same_origin_href(href: string): boolean {
	try {
		return (
			new URL(href, window.location.href).origin ===
			window.location.origin
		);
	} catch {
		return false;
	}
}

async function read_api_response_data(response: Response): Promise<unknown> {
	if (response.status === 204) {
		return undefined;
	}
	const header = response.headers.get(CONTENT_TYPE_HEADER);
	if (header?.toLowerCase().includes(JSON_CONTENT_TYPE)) {
		return response.json();
	}
	const text = await response.text();
	if (text.length > 0) {
		return text;
	}
	return undefined;
}
