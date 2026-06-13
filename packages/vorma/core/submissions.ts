import { is_abort_error } from "./abort_error.ts";
import type { ApiResult, Submission } from "./client_core_types.ts";
import { VERCEL_X_DEPLOYMENT_ID, X_ACCEPTS_CLIENT_REDIRECT } from "./constants.ts";
import { make_deferred } from "./deferred.ts";
import { classify_redirect_target, detect_redirect } from "./redirects.ts";
import { revalidation_ok } from "./revalidation_scheduler.ts";
import type { ResourceKind, RevalidationResult } from "./types.ts";

/*
Owns concurrent API submissions: the dedupe-keyed in-flight map, dispatch
with JSON body normalization, redirect/build-skew handling on responses, and
the revalidation scheduling tied to each submission's lifecycle. Route state
is untouched here; soft redirects and revalidation demands flow out through
deps.
*/

export interface SubmissionsDeps {
	// Whether the router has finished booting (revalidations can start).
	is_ready(): boolean;
	// Whether a route snapshot exists (submit is usable at all).
	is_booted(): boolean;
	is_same_origin(url: URL): boolean;
	current_origin(): string;
	// Deployment id stamped into request headers for platform skew protection.
	deployment_id(): string;
	// Record an apiRequest revalidation demand.
	require_revalidation(
		waiter: ReturnType<typeof make_deferred<RevalidationResult>> | undefined,
		skip_work_indicator: boolean | undefined,
	): void;
	// Run the pending demand now if nothing blocks it.
	start_pending_revalidation(): void;
	// Follow a same-origin soft redirect (or defer it until boot completes).
	redirect_to(url: URL): void;
	hard_redirect(href: string): void;
	report_resource_build_skew(input: {
		response: Response;
		resource_kind: ResourceKind;
		requested_href: string;
		method: string;
	}): void;
	on_work_update(): void;
}

/*
Resource errors arrive as the JSON error envelope the server writes; the
message is the handler's explicit client text (or the framework generic).
res.statusText is useless here: Rust never sends custom reason phrases and
HTTP/2 has none at all.
*/
async function read_error_envelope(res: Response): Promise<string> {
	try {
		const data: unknown = await res.json();
		if (
			typeof data === "object" &&
			data !== null &&
			typeof (data as { error?: unknown }).error === "string" &&
			(data as { error: string }).error !== ""
		) {
			return (data as { error: string }).error;
		}
	} catch {
		// Non-JSON error bodies fall back to the generic below.
	}
	return `Request failed (${res.status})`;
}

export function create_submissions(deps: SubmissionsDeps) {
	const submissions = new Map<string, Submission>();

	function schedule_revalidation(
		sub: Submission,
		start = true,
	): Promise<RevalidationResult> {
		if (!sub.should_revalidate) {
			return Promise.resolve(revalidation_ok);
		}
		if (deps.is_ready()) {
			const waiter = make_deferred<RevalidationResult>();
			deps.require_revalidation(waiter, sub.skip_work_indicator);
			if (start) {
				deps.start_pending_revalidation();
			}
			return waiter.promise;
		}
		deps.require_revalidation(undefined, sub.skip_work_indicator);
		return Promise.resolve(revalidation_ok);
	}

	function settle(sub: Submission, result: ApiResult<unknown>, notify = true): void {
		if (sub.settled) {
			return;
		}
		sub.settled = true;
		if (submissions.get(sub.key)?.ac === sub.ac) {
			submissions.delete(sub.key);
		}
		sub.deferred.resolve(result);
		if (notify) {
			deps.on_work_update();
		}
	}

	function replace(sub: Submission): boolean {
		sub.ac.abort();
		let scheduled_revalidation = false;
		if (sub.did_dispatch) {
			sub.revalidation_promise = schedule_revalidation(sub, false);
			scheduled_revalidation = sub.should_revalidate;
		}
		settle(
			sub,
			{
				success: false,
				error: "Aborted",
				revalidationPromise: sub.revalidation_promise,
			},
			false,
		);
		return scheduled_revalidation;
	}

	function submit<T = unknown>(
		url: string | URL,
		request_init?: RequestInit,
		options?: {
			resourceKind?: ResourceKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	): Promise<ApiResult<T>> {
		if (!deps.is_booted()) {
			return Promise.reject(new Error("Vorma not booted"));
		}
		const resolved = new URL(String(url), window.location.href);

		if (!deps.is_same_origin(resolved)) {
			return Promise.resolve({
				success: false,
				error: `submit only supports same-origin targets. Received: "${resolved.href}".`,
				revalidationPromise: Promise.resolve(revalidation_ok),
			});
		}

		const method = request_init?.method
			? request_init.method.toUpperCase().trim()
			: "GET";
		const resource_kind =
			options?.resourceKind ??
			(method === "GET" || method === "HEAD" ? "query" : "mutation");
		let should_revalidate = resource_kind === "mutation";
		if (options?.revalidate !== undefined) {
			should_revalidate = options.revalidate;
		}

		let dedupe_key = options?.dedupeKey;
		let start_replaced_revalidation = false;
		if (dedupe_key) {
			const previous = submissions.get(dedupe_key);
			if (previous) {
				start_replaced_revalidation = replace(previous);
			}
		} else {
			dedupe_key = crypto.randomUUID();
		}

		const ac = new AbortController();
		const deferred = make_deferred<ApiResult<unknown>>();
		const sub: Submission = {
			ac,
			deferred,
			did_dispatch: false,
			key: dedupe_key,
			method,
			href: resolved.href,
			revalidation_promise: Promise.resolve(revalidation_ok),
			settled: false,
			should_revalidate,
			skip_work_indicator: options?.skipWorkIndicator,
		};
		submissions.set(dedupe_key, sub);
		deps.on_work_update();

		void run(sub, resolved, request_init, resource_kind);
		if (start_replaced_revalidation) {
			deps.start_pending_revalidation();
		}
		return deferred.promise as Promise<ApiResult<T>>;
	}

	async function run<T = unknown>(
		sub: Submission,
		resolved: URL,
		request_init: RequestInit | undefined,
		resource_kind: ResourceKind,
	): Promise<void> {
		try {
			const is_get = sub.method === "GET" || sub.method === "HEAD";

			const headers = new Headers();
			const deployment_id = deps.deployment_id();
			if (deployment_id) {
				headers.set(VERCEL_X_DEPLOYMENT_ID, deployment_id);
			}
			new Headers(request_init?.headers ?? undefined).forEach((v, k) => {
				headers.set(k, v);
			});
			headers.set(X_ACCEPTS_CLIENT_REDIRECT, "1");

			const body = request_init?.body;
			const should_json =
				!is_get &&
				body &&
				typeof body === "object" &&
				!(body instanceof ReadableStream) &&
				!(body instanceof FormData) &&
				!(body instanceof URLSearchParams) &&
				!(body instanceof Blob) &&
				!(body instanceof ArrayBuffer) &&
				!ArrayBuffer.isView(body);

			const final_init: RequestInit = {
				...request_init,
				method: sub.method,
				headers,
				signal: sub.ac.signal,
			};
			if (is_get) {
				delete final_init.body;
			} else if (should_json) {
				final_init.body = JSON.stringify(body);
				if (!headers.has("Content-Type")) {
					headers.set("Content-Type", "application/json");
				}
			}

			sub.did_dispatch = true;
			const res = await fetch(resolved, final_init);
			if (sub.settled || sub.ac.signal.aborted) {
				return;
			}

			const redirect = detect_redirect(res, resolved);
			deps.report_resource_build_skew({
				response: res,
				resource_kind,
				requested_href: resolved.href,
				method: sub.method,
			});

			if (redirect) {
				const classified = classify_redirect_target(
					redirect.href,
					redirect.hard,
					deps.current_origin(),
				);
				if (classified.kind === "invalid") {
					settle(sub, {
						success: false,
						error: `Redirect target must use an HTTP(S) scheme. Received: "${redirect.href}".`,
						response: res,
						revalidationPromise: sub.revalidation_promise,
					});
					return;
				}
				if (classified.kind === "hard") {
					deps.hard_redirect(classified.href);
				} else {
					deps.redirect_to(classified.url);
				}
				settle(sub, {
					success: true,
					data: undefined as T,
					response: res,
					revalidationPromise: sub.revalidation_promise,
				});
				return;
			}

			if (!res.ok) {
				sub.revalidation_promise = schedule_revalidation(sub);
				settle(sub, {
					success: false,
					error: await read_error_envelope(res),
					response: res,
					revalidationPromise: sub.revalidation_promise,
				});
				return;
			}

			let data: unknown;
			if (res.status !== 204) {
				const ct = res.headers.get("Content-Type");
				if (ct?.toLowerCase().includes("json")) {
					data = await res.json();
				} else {
					const t = await res.text();
					data = t.length > 0 ? t : undefined;
				}
			}

			sub.revalidation_promise = schedule_revalidation(sub);
			settle(sub, {
				success: true,
				data: data as T,
				response: res,
				revalidationPromise: sub.revalidation_promise,
			});
		} catch (e) {
			if (sub.settled) {
				return;
			}
			if (is_abort_error(e)) {
				if (sub.did_dispatch) {
					sub.revalidation_promise = schedule_revalidation(sub);
				}
				settle(sub, {
					success: false,
					error: "Aborted",
					revalidationPromise: sub.revalidation_promise,
				});
				return;
			}
			if (sub.did_dispatch) {
				sub.revalidation_promise = schedule_revalidation(sub);
			}
			settle(sub, {
				success: false,
				error: String(e instanceof Error ? e.message : e),
				revalidationPromise: sub.revalidation_promise,
			});
		}
	}

	function work_sources(): Array<{
		key: string;
		method: string;
		href: string;
		skip_work_indicator?: boolean;
	}> {
		return Array.from(submissions.values(), (s) => {
			return {
				key: s.key,
				method: s.method,
				href: s.href,
				skip_work_indicator: s.skip_work_indicator,
			};
		});
	}

	return { submit, work_sources };
}

export type Submissions = ReturnType<typeof create_submissions>;
