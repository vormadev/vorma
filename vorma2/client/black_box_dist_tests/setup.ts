import { afterEach, beforeEach, expect, vi } from "vitest";

// ─── Global Setup / Teardown ─────────────────────────────────────

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date("2026-01-01T00:00:00.000Z"));

	document.body.innerHTML = "";
	document.head.innerHTML = "";

	document.head.appendChild(
		document.createComment('data-vorma="meta-start"'),
	);
	document.head.appendChild(document.createComment('data-vorma="meta-end"'));
	document.head.appendChild(
		document.createComment('data-vorma="rest-start"'),
	);
	document.head.appendChild(document.createComment('data-vorma="rest-end"'));

	window.history.replaceState({}, "", "/");

	Object.defineProperty(window, "scrollTo", {
		value: vi.fn(),
		writable: true,
		configurable: true,
	});

	if (!Element.prototype.scrollIntoView) {
		Element.prototype.scrollIntoView = vi.fn();
	}

	if (!globalThis.CSS) {
		(globalThis as Record<string, unknown>).CSS = {};
	}
	if (
		!(
			globalThis as Record<string, unknown> & {
				CSS: Record<string, unknown>;
			}
		).CSS.escape
	) {
		(
			globalThis as Record<string, unknown> & {
				CSS: Record<string, unknown>;
			}
		).CSS.escape = (value: string) =>
			value.replace(/[!"#$%&'()*+,./:;<=>?@[\\\]^`{|}~]/g, "\\$&");
	}
});

afterEach(async () => {
	await vi.runOnlyPendingTimersAsync();
	vi.useRealTimers();
	vi.restoreAllMocks();

	document.body.innerHTML = "";
	document.head.innerHTML = "";
});

// ─── Types ───────────────────────────────────────────────────────

export type RouteDataOverride = {
	matched_patterns?: string[];
	loaders_data?: unknown[];
	import_urls?: string[];
	export_keys?: string[];
	error_export_keys?: string[];
	has_root_data?: boolean;
	params?: Record<string, string>;
	splat_values?: string[];
	deps?: string[];
	css_bundles?: string[];
	outermost_server_error?: unknown;
	outermost_server_error_idx?: number | null;
	title?: { dangerousInnerHTML: string };
	meta_head_els?: unknown[];
	rest_head_els?: unknown[];
};

export type StatusSnapshot = {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
};

export type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (reason?: unknown) => void;
};

export type RecordedFetchRequest = {
	input: RequestInfo | URL;
	init: RequestInit | undefined;
	url: URL;
	deferred: Deferred<Response>;
};

// ─── Module Loading ──────────────────────────────────────────────

export async function load_client() {
	vi.resetModules();
	return await import("vorma/client");
}

// ─── Route Data ──────────────────────────────────────────────────

export function create_route_data_response(
	overrides: RouteDataOverride = {},
	init: ResponseInit = {},
): Response {
	const matched_patterns = overrides.matched_patterns ?? [];
	const count = matched_patterns.length;

	const body = {
		matchedPatterns: matched_patterns,
		loadersData:
			overrides.loaders_data ?? Array.from({ length: count }, () => null),
		importURLs:
			overrides.import_urls ?? Array.from({ length: count }, () => ""),
		exportKeys:
			overrides.export_keys ?? Array.from({ length: count }, () => ""),
		errorExportKeys:
			overrides.error_export_keys ??
			Array.from({ length: count }, () => ""),
		hasRootData: overrides.has_root_data ?? false,
		params: overrides.params ?? {},
		splatValues: overrides.splat_values ?? [],
		deps: overrides.deps ?? [],
		cssBundles: overrides.css_bundles ?? [],
		metaHeadEls: overrides.meta_head_els ?? [],
		restHeadEls: overrides.rest_head_els ?? [],
		...(overrides.outermost_server_error !== undefined
			? { outermostServerError: overrides.outermost_server_error }
			: {}),
		...(overrides.outermost_server_error_idx !== undefined
			? { outermostServerErrorIdx: overrides.outermost_server_error_idx }
			: {}),
		...(overrides.title !== undefined ? { title: overrides.title } : {}),
	};

	const headers = new Headers({
		"Content-Type": "application/json",
		"X-Vorma-Client-Build-Id": "1",
	});
	if (init.headers) {
		new Headers(init.headers).forEach((value, key) => {
			headers.set(key, value);
		});
	}

	return new Response(JSON.stringify(body), {
		status: init.status ?? 200,
		statusText: init.statusText,
		headers,
	});
}

export async function navigate_with_route_data(props: {
	client: { vormaNavigate: (href: string) => Promise<unknown> };
	href: string;
	overrides?: RouteDataOverride;
	response_init?: ResponseInit;
}): Promise<void> {
	vi.spyOn(window, "fetch").mockResolvedValueOnce(
		create_route_data_response(props.overrides ?? {}, props.response_init),
	);
	await props.client.vormaNavigate(props.href);
	await vi.runAllTimersAsync();
}

// ─── Async Primitives ────────────────────────────────────────────

export function create_deferred<T>(): Deferred<T> {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

export function create_deferred_fetch_call(): {
	deferred: Deferred<Response>;
	get_signal: () => AbortSignal | undefined;
	mock: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
} {
	const deferred = create_deferred<Response>();
	let signal: AbortSignal | undefined;
	function mock(
		_input: RequestInfo | URL,
		init?: RequestInit,
	): Promise<Response> {
		signal = init?.signal ?? undefined;
		return deferred.promise;
	}
	return { deferred, get_signal: () => signal, mock };
}

export function create_abort_aware_fetch_recorder(): {
	requests: RecordedFetchRequest[];
} {
	const requests: RecordedFetchRequest[] = [];
	vi.spyOn(window, "fetch").mockImplementation(
		(input: RequestInfo | URL, init?: RequestInit) => {
			const deferred = create_deferred<Response>();
			const signal = init?.signal;
			if (signal?.aborted) {
				deferred.reject(new DOMException("Aborted", "AbortError"));
				return deferred.promise;
			}
			signal?.addEventListener(
				"abort",
				() => {
					deferred.reject(new DOMException("Aborted", "AbortError"));
				},
				{ once: true },
			);
			requests.push({
				input,
				init,
				url: request_input_to_url(input),
				deferred,
			});
			return deferred.promise;
		},
	);
	return { requests };
}

// ─── Waiting ─────────────────────────────────────────────────────

export async function wait_for_request_count(props: {
	requests: unknown[];
	count: number;
	max_ticks?: number;
	tick_ms?: number;
}): Promise<void> {
	const max_ticks = props.max_ticks ?? 300;
	const tick_ms = props.tick_ms ?? 1;
	for (let i = 0; i < max_ticks; i += 1) {
		if (props.requests.length >= props.count) {
			return;
		}
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(tick_ms);
	}
	throw new Error(`Timed out waiting for request count ${props.count}`);
}

// ─── URL ─────────────────────────────────────────────────────────

export function request_input_to_url(input: RequestInfo | URL): URL {
	if (input instanceof URL) {
		return input;
	}
	if (input instanceof Request) {
		return new URL(input.url, window.location.origin);
	}
	return new URL(String(input), window.location.origin);
}

// ─── Unhandled Rejections ────────────────────────────────────────

export async function with_unhandled_rejection_capture<T>(props: {
	run: () => Promise<T>;
}): Promise<{ result: T; unhandled_rejections: unknown[] }> {
	const unhandled_rejections: unknown[] = [];
	function handler(reason: unknown): void {
		unhandled_rejections.push(reason);
	}
	process.on("unhandledRejection", handler);
	try {
		const result = await props.run();
		await Promise.resolve();
		return { result, unhandled_rejections };
	} finally {
		process.off("unhandledRejection", handler);
	}
}

// ─── Status Assertions ──────────────────────────────────────────

export function expect_status_idle(status: StatusSnapshot | undefined): void {
	expect(status).toEqual({
		isNavigating: false,
		isSubmitting: false,
		isRevalidating: false,
	});
}

export function expect_no_loading_gap(statuses: StatusSnapshot[]): void {
	const non_final = statuses.slice(0, -1);
	const has_gap = non_final.some(
		(s) => !s.isNavigating && !s.isSubmitting && !s.isRevalidating,
	);
	expect(has_gap).toBe(false);
}

export function collect_status_snapshots(client: {
	addStatusListener: (
		fn: (event: { detail: StatusSnapshot }) => void,
	) => () => void;
}): { statuses: StatusSnapshot[]; cleanup: () => void } {
	const statuses: StatusSnapshot[] = [];
	const cleanup = client.addStatusListener((event) => {
		statuses.push(event.detail);
	});
	return { statuses, cleanup };
}

// ─── Location Stub ───────────────────────────────────────────────

export function stub_window_location_href(
	initial_href = window.location.href,
): {
	get_href: () => string;
	restore: () => void;
} {
	const original_location = window.location;
	let href = initial_href;

	function resolve_url(): URL {
		return new URL(href, original_location.href);
	}

	const stub = {
		get href() {
			return href;
		},
		set href(value: string) {
			href = new URL(String(value), href).href;
		},
		get origin() {
			return resolve_url().origin;
		},
		get pathname() {
			return resolve_url().pathname;
		},
		get search() {
			return resolve_url().search;
		},
		get hash() {
			return resolve_url().hash;
		},
		assign(value: string | URL): void {
			href = new URL(String(value), href).href;
		},
		replace(value: string | URL): void {
			href = new URL(String(value), href).href;
		},
		toString(): string {
			return href;
		},
	} as Location;

	Object.defineProperty(window, "location", {
		value: stub,
		configurable: true,
	});

	return {
		get_href: () => href,
		restore: () => {
			Object.defineProperty(window, "location", {
				value: original_location,
				configurable: true,
			});
		},
	};
}

// ─── Seeded Random ───────────────────────────────────────────────

export function create_seeded_random(seed: number): () => number {
	let state = seed >>> 0;
	return () => {
		state = (state * 1664525 + 1013904223) >>> 0;
		return state / 0x100000000;
	};
}

export function shuffled_indices(length: number, seed: number): number[] {
	const random = create_seeded_random(seed);
	const indices = Array.from({ length }, (_, i) => i);
	for (let i = indices.length - 1; i > 0; i -= 1) {
		const j = Math.floor(random() * (i + 1));
		const val_at_i = indices[i]!;
		indices[i] = indices[j]!;
		indices[j] = val_at_i;
	}
	return indices;
}
