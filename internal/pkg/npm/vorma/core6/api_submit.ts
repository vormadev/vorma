import {
	API_SUBMIT_CROSS_ORIGIN_ERROR,
	BUILD_ID_HEADER,
	CONTENT_TYPE_HEADER,
	JSON_CONTENT_TYPE,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_PROTOCOL_ENABLED,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
} from "../core/constants.ts";
import type { APIRouteKind } from "../core/types.ts";
import { create_core6_deferred, type Core6Deferred } from "./deferred.ts";
import {
	run_core6_route_navigation,
	type Core6RouteNavigationHost,
} from "./route_navigation.ts";
import { core6_route_preparation_trigger } from "./route_preparation.ts";
import { core6_route_publish_reason } from "./route_publication.ts";
import {
	core6_route_revalidation_reason,
	type Core6RouteRevalidationOwner,
	type Core6RouteRevalidationResult,
} from "./route_revalidation.ts";
import type { Core6RouteRuntime } from "./route_runtime.ts";
import {
	core6_is_http_href,
	core6_same_origin_href,
	core6_scroll_for_href,
} from "./route_url.ts";
import {
	create_core6_scope_manager,
	run_core6_scope_stage,
	type Core6Scope,
	type Core6ScopeManager,
} from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

export const CORE6_API_SUBMIT_INVALID_REDIRECT_ERROR =
	"Redirect target must use an HTTP(S) scheme. Received:";
export const CORE6_API_SUBMIT_ABORTED_ERROR = "Aborted";

export const CORE6_API_SUBMIT_REVALIDATION_OK = { ok: true } as const;

export const core6_api_submit_build_skew_default_behavior = {
	hard_reload: "hard_reload",
	notify_only: "notify_only",
} as const;

export const core6_api_submit_failure_reason = {
	aborted: "aborted",
	cross_origin: "cross_origin",
	http_error: "http_error",
	invalid_redirect: "invalid_redirect",
	invalid_url: "invalid_url",
	network_error: "network_error",
} as const;

export const core6_api_submit_outcome_kind = {
	failure: "failure",
	redirect: "redirect",
	success: "success",
} as const;

export type Core6APISubmitFailureReason =
	(typeof core6_api_submit_failure_reason)[keyof typeof core6_api_submit_failure_reason];

export type Core6APISubmitOutcomeKind =
	(typeof core6_api_submit_outcome_kind)[keyof typeof core6_api_submit_outcome_kind];

export type Core6APISubmitBuildSkewEvent = {
	active_client_build_id: string;
	default_behavior: Core6APISubmitBuildSkewDefaultBehavior;
	method: string;
	ok: boolean;
	requested_href: string;
	response: Response;
	route_kind: APIRouteKind;
	server_build_id: string;
	status: number;
};

export type Core6APISubmitBuildSkewDefaultBehavior =
	(typeof core6_api_submit_build_skew_default_behavior)[keyof typeof core6_api_submit_build_skew_default_behavior];

export type Core6APISubmitHost = Core6RouteNavigationHost & {
	current_href: () => string;
	fetch_api_response: (args: {
		href: string;
		init: RequestInit & { signal: AbortSignal };
	}) => Promise<Response>;
	notify_api_build_skew?: (event: Core6APISubmitBuildSkewEvent) => void;
};

export type Core6APISubmitRuntime = Pick<
	Core6RouteRuntime,
	| "current_route"
	| "publish_same_document"
	| "run_route_fetch"
	| "run_route_prepared"
>;

export type Core6APISubmitConfig = {
	active_client_build_id: () => string;
	deployment_id?: () => string | null | undefined;
	host: Core6APISubmitHost;
	revalidation?: Pick<Core6RouteRevalidationOwner, "request">;
	runtime: Core6APISubmitRuntime;
};

export type Core6APISubmitOptions = {
	api_route_kind?: APIRouteKind;
	dedupe_key?: string;
	revalidate?: boolean;
	skip_work_indicator?: boolean;
};

export type Core6APISubmitInput = {
	options?: Core6APISubmitOptions;
	request_init?: RequestInit;
	url: string | URL;
};

export type Core6APISubmitRequestInput = {
	base_href: string;
	deployment_id?: string | null;
	options?: Core6APISubmitOptions;
	request_init?: RequestInit;
	signal: AbortSignal;
	url: string | URL;
};

export type Core6APISubmitRequest = {
	href: string;
	init: RequestInit & { signal: AbortSignal };
	method: string;
	route_kind: APIRouteKind;
	should_revalidate: boolean;
};

export type Core6APISubmitResult<T = unknown> =
	| {
			data: T;
			response: Response;
			revalidation_promise: Promise<Core6RouteRevalidationResult>;
			success: true;
	  }
	| {
			error: string;
			response?: Response;
			revalidation_promise: Promise<Core6RouteRevalidationResult>;
			success: false;
	  };

export type Core6APISubmitRedirect = {
	hard: boolean;
	href: string;
};

export type Core6APISubmitSuccessOutcome = {
	data: unknown;
	kind: typeof core6_api_submit_outcome_kind.success;
	redirect: null;
	response: Response;
};

export type Core6APISubmitRedirectOutcome = {
	kind: typeof core6_api_submit_outcome_kind.redirect;
	redirect: Core6APISubmitRedirect;
	response: Response;
};

export type Core6APISubmitFailureOutcome = {
	error: string;
	kind: typeof core6_api_submit_outcome_kind.failure;
	reason: Core6APISubmitFailureReason;
	response?: Response;
};

export type Core6APISubmitOutcome =
	| Core6APISubmitSuccessOutcome
	| Core6APISubmitRedirectOutcome
	| Core6APISubmitFailureOutcome;

export type Core6APISubmitStatus = {
	dedupe_key: string | null;
	did_dispatch: boolean;
	href: string;
	method: string;
	route_kind: APIRouteKind;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
};

export type Core6APISubmitOwner = {
	cancel: (dedupe_key: string) => boolean;
	cancel_all: () => boolean;
	current_statuses: () => Core6APISubmitStatus[];
	submit: <T = unknown>(
		input: Core6APISubmitInput,
	) => Promise<Core6APISubmitResult<T>>;
};

type Core6APISubmitScopeKind = "api_submit";

const core6_api_submit_abort_error_name = "AbortError";

type Core6APISubmitIdentity = string | symbol;

type Core6APISubmission = {
	dedupe_key: string | null;
	deferred: Core6Deferred<Core6APISubmitResult<unknown>>;
	did_dispatch: boolean;
	href: string;
	method: string;
	revalidation_promise: Promise<Core6RouteRevalidationResult>;
	route_kind: APIRouteKind;
	scope: Core6Scope<Core6APISubmitScopeKind>;
	settled: boolean;
	should_revalidate: boolean;
	skip_work_indicator: boolean;
};

type Core6APISubmitSlot = {
	manager: Core6ScopeManager<Core6APISubmitScopeKind>;
	submission: Core6APISubmission | null;
};

export function build_core6_api_submit_request(
	input: Core6APISubmitRequestInput,
): Core6APISubmitRequest | Core6APISubmitFailureOutcome {
	const href = resolve_core6_api_submit_href({
		base_href: input.base_href,
		url: input.url,
	});
	if (typeof href !== "string") {
		return href;
	}

	const method = (input.request_init?.method ?? "GET").trim().toUpperCase();
	const is_get = method === "GET" || method === "HEAD";
	const route_kind =
		input.options?.api_route_kind ?? (is_get ? "query" : "mutation");
	const should_revalidate =
		input.options?.revalidate ?? route_kind === "mutation";
	const headers = new Headers(input.request_init?.headers);
	const deployment_id = input.deployment_id;
	if (deployment_id) {
		headers.set(VERCEL_X_DEPLOYMENT_ID, deployment_id);
	}
	headers.set(X_ACCEPTS_CLIENT_REDIRECT, VORMA_PROTOCOL_ENABLED);

	const body = input.request_init?.body;
	const init: RequestInit & { signal: AbortSignal } = {
		...input.request_init,
		headers,
		method,
		signal: input.signal,
	};
	if (is_get) {
		delete init.body;
	} else if (should_jsonify_core6_api_submit_body(body)) {
		init.body = JSON.stringify(body);
		if (!headers.has(CONTENT_TYPE_HEADER)) {
			headers.set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE);
		}
	}

	return {
		href,
		init,
		method,
		route_kind,
		should_revalidate,
	};
}

export async function classify_core6_api_submit_response(input: {
	request_href: string;
	response: Response;
}): Promise<Core6APISubmitOutcome> {
	const redirect = core6_api_submit_response_redirect(
		input.request_href,
		input.response,
	);
	if (is_core6_api_submit_failure_outcome(redirect)) {
		return {
			...redirect,
			response: input.response,
		};
	}
	if (redirect.redirect) {
		return {
			kind: core6_api_submit_outcome_kind.redirect,
			redirect: redirect.redirect,
			response: input.response,
		};
	}
	if (!input.response.ok) {
		return {
			error: input.response.statusText,
			kind: core6_api_submit_outcome_kind.failure,
			reason: core6_api_submit_failure_reason.http_error,
			response: input.response,
		};
	}
	return {
		data: await parse_core6_api_submit_data(input.response),
		kind: core6_api_submit_outcome_kind.success,
		redirect: null,
		response: input.response,
	};
}

export function create_core6_api_submit_owner(
	config: Core6APISubmitConfig,
): Core6APISubmitOwner {
	const slots = new Map<Core6APISubmitIdentity, Core6APISubmitSlot>();

	function revalidation_ok(): Promise<Core6RouteRevalidationResult> {
		return Promise.resolve(CORE6_API_SUBMIT_REVALIDATION_OK);
	}

	function submission_identity(
		options: Core6APISubmitOptions | undefined,
	): Core6APISubmitIdentity {
		return options?.dedupe_key ?? Symbol();
	}

	function submit_revalidation(
		submission: Core6APISubmission,
	): Promise<Core6RouteRevalidationResult> {
		if (!submission.should_revalidate || !config.revalidation) {
			return revalidation_ok();
		}
		return config.revalidation.request({
			debounce: false,
			reason: core6_route_revalidation_reason.mutation,
			skip_work_indicator: submission.skip_work_indicator,
		});
	}

	function settle_submission(
		submission: Core6APISubmission,
		result: Core6APISubmitResult<unknown>,
	): void {
		if (submission.settled) {
			return;
		}
		submission.settled = true;
		submission.deferred.resolve(result);
	}

	function settle_aborted_submission(submission: Core6APISubmission): void {
		if (submission.settled) {
			return;
		}
		if (submission.did_dispatch) {
			submission.revalidation_promise = submit_revalidation(submission);
		}
		settle_submission(submission, {
			error: CORE6_API_SUBMIT_ABORTED_ERROR,
			revalidation_promise: submission.revalidation_promise,
			success: false,
		});
	}

	function slot_for(identity: Core6APISubmitIdentity): Core6APISubmitSlot {
		const existing = slots.get(identity);
		if (existing) {
			return existing;
		}
		const created = {
			manager: create_core6_scope_manager<Core6APISubmitScopeKind>(),
			submission: null,
		};
		slots.set(identity, created);
		return created;
	}

	function notify_api_build_skew(input: {
		outcome: Core6APISubmitOutcome & { response: Response };
		request: Core6APISubmitRequest;
	}): void {
		const server_build_id =
			input.outcome.response.headers.get(BUILD_ID_HEADER) ?? "";
		const active_client_build_id = config.active_client_build_id();
		if (!server_build_id || server_build_id === active_client_build_id) {
			return;
		}
		const default_behavior =
			input.outcome.kind === core6_api_submit_outcome_kind.redirect &&
			core6_is_http_href(input.outcome.redirect.href) &&
			!core6_same_origin_href(
				input.outcome.redirect.href,
				input.request.href,
			)
				? core6_api_submit_build_skew_default_behavior.hard_reload
				: core6_api_submit_build_skew_default_behavior.notify_only;
		config.host.notify_api_build_skew?.({
			active_client_build_id,
			default_behavior,
			method: input.request.method,
			ok: input.outcome.response.ok,
			requested_href: input.request.href,
			response: input.outcome.response,
			route_kind: input.request.route_kind,
			server_build_id,
			status: input.outcome.response.status,
		});
	}

	function follow_submit_redirect(input: {
		redirect: Core6APISubmitRedirect;
		request: Core6APISubmitRequest;
	}): string | null {
		const redirect = input.redirect;
		if (!core6_is_http_href(redirect.href)) {
			return `${CORE6_API_SUBMIT_INVALID_REDIRECT_ERROR} "${redirect.href}".`;
		}
		if (
			redirect.hard ||
			!core6_same_origin_href(redirect.href, input.request.href)
		) {
			config.host.hard_redirect(redirect.href);
			return null;
		}
		const target = new URL(redirect.href);
		void run_core6_route_navigation({
			active_client_build_id: config.active_client_build_id(),
			deployment_id: config.deployment_id?.(),
			host: config.host,
			intent: {
				history: {
					href: target.href,
					replace: true,
					state: undefined,
				},
				history_state: undefined,
				href: target.href,
				preparation_trigger: core6_route_preparation_trigger.navigation,
				publish_reason: core6_route_publish_reason.navigation,
				scroll: core6_scroll_for_href(target.href),
				search_params: target.searchParams,
			},
			kind: core6_route_transaction_kind.navigation,
			runtime: config.runtime,
		}).catch(() => {});
		return null;
	}

	async function run_submission(input: {
		request: Core6APISubmitRequest;
		submission: Core6APISubmission;
	}): Promise<void> {
		const submission = input.submission;
		try {
			const stage = await run_core6_scope_stage(
				submission.scope,
				async () => {
					submission.did_dispatch = true;
					const response = await config.host.fetch_api_response({
						href: input.request.href,
						init: input.request.init,
					});
					return await classify_core6_api_submit_response({
						request_href: input.request.href,
						response,
					});
				},
			);
			if (!stage.ok) {
				return;
			}
			const outcome = stage.value;
			const result = submission.scope.complete(() => {
				if (outcome.kind === core6_api_submit_outcome_kind.redirect) {
					notify_api_build_skew({
						outcome,
						request: input.request,
					});
					const redirect_error = follow_submit_redirect({
						redirect: outcome.redirect,
						request: input.request,
					});
					if (redirect_error) {
						settle_submission(submission, {
							error: redirect_error,
							response: outcome.response,
							revalidation_promise:
								submission.revalidation_promise,
							success: false,
						});
						return;
					}
					settle_submission(submission, {
						data: undefined,
						response: outcome.response,
						revalidation_promise: submission.revalidation_promise,
						success: true,
					});
					return;
				}
				if (outcome.kind === core6_api_submit_outcome_kind.failure) {
					if (outcome.response) {
						notify_api_build_skew({
							outcome: {
								...outcome,
								response: outcome.response,
							},
							request: input.request,
						});
					}
					if (
						outcome.reason ===
						core6_api_submit_failure_reason.http_error
					) {
						submission.revalidation_promise =
							submit_revalidation(submission);
					}
					settle_submission(submission, {
						error: outcome.error,
						response: outcome.response,
						revalidation_promise: submission.revalidation_promise,
						success: false,
					});
					return;
				}
				notify_api_build_skew({ outcome, request: input.request });
				submission.revalidation_promise =
					submit_revalidation(submission);
				settle_submission(submission, {
					data: outcome.data,
					response: outcome.response,
					revalidation_promise: submission.revalidation_promise,
					success: true,
				});
			});
			if (!result.ok && !submission.settled) {
				settle_aborted_submission(submission);
			}
		} catch (error) {
			if (submission.settled) {
				return;
			}
			if (submission.did_dispatch) {
				submission.revalidation_promise =
					submit_revalidation(submission);
			}
			const aborted =
				submission.scope.signal.aborted ||
				is_core6_api_submit_abort_error(error);
			submission.scope.complete(() => {
				settle_submission(submission, {
					error: aborted
						? CORE6_API_SUBMIT_ABORTED_ERROR
						: error instanceof Error
							? error.message
							: String(error),
					revalidation_promise: submission.revalidation_promise,
					success: false,
				});
			});
		}
	}

	return {
		cancel: (dedupe_key) => {
			const slot = slots.get(dedupe_key);
			if (!slot) {
				return false;
			}
			return slot.manager.cancel_current();
		},
		cancel_all: () => {
			let cancelled = false;
			for (const slot of slots.values()) {
				cancelled = slot.manager.cancel_current() || cancelled;
			}
			return cancelled;
		},
		current_statuses: () => {
			const statuses: Core6APISubmitStatus[] = [];
			for (const slot of slots.values()) {
				const submission = slot.submission;
				if (!submission || submission.settled) {
					continue;
				}
				statuses.push({
					dedupe_key: submission.dedupe_key,
					did_dispatch: submission.did_dispatch,
					href: submission.href,
					method: submission.method,
					route_kind: submission.route_kind,
					should_revalidate: submission.should_revalidate,
					skip_work_indicator: submission.skip_work_indicator,
				});
			}
			return statuses;
		},
		submit: <T = unknown>(
			input: Core6APISubmitInput,
		): Promise<Core6APISubmitResult<T>> => {
			const base_href = config.host.current_href();
			const href = resolve_core6_api_submit_href({
				base_href,
				url: input.url,
			});
			if (typeof href !== "string") {
				return Promise.resolve({
					error: href.error,
					revalidation_promise: revalidation_ok(),
					success: false,
				});
			}
			const identity = submission_identity(input.options);
			const slot = slot_for(identity);
			const scope = slot.manager.start("api_submit");
			const request = build_core6_api_submit_request({
				base_href,
				deployment_id: config.deployment_id?.(),
				options: input.options,
				request_init: input.request_init,
				signal: scope.signal,
				url: input.url,
			});
			if (is_core6_api_submit_failure_outcome(request)) {
				scope.complete(() => {});
				return Promise.resolve({
					error: request.error,
					revalidation_promise: revalidation_ok(),
					success: false,
				});
			}

			const submission: Core6APISubmission = {
				dedupe_key: input.options?.dedupe_key ?? null,
				deferred:
					create_core6_deferred<Core6APISubmitResult<unknown>>(),
				did_dispatch: false,
				href: request.href,
				method: request.method,
				revalidation_promise: revalidation_ok(),
				route_kind: request.route_kind,
				scope,
				settled: false,
				should_revalidate: request.should_revalidate,
				skip_work_indicator:
					input.options?.skip_work_indicator === true,
			};
			slot.submission = submission;
			scope.on_cancel(() => {
				settle_aborted_submission(submission);
			});
			scope.on_cleanup(() => {
				if (slot.submission === submission) {
					slot.submission = null;
				}
				if (!slot.manager.current()) {
					slots.delete(identity);
				}
			});
			void run_submission({ request, submission });
			return submission.deferred.promise as Promise<
				Core6APISubmitResult<T>
			>;
		},
	};
}

function resolve_core6_api_submit_href(input: {
	base_href: string;
	url: string | URL;
}): string | Core6APISubmitFailureOutcome {
	let url: URL;
	try {
		url = new URL(String(input.url), input.base_href);
	} catch (error) {
		return {
			error: error instanceof Error ? error.message : String(error),
			kind: core6_api_submit_outcome_kind.failure,
			reason: core6_api_submit_failure_reason.invalid_url,
		};
	}
	if (url.origin !== new URL(input.base_href).origin) {
		return {
			error: `${API_SUBMIT_CROSS_ORIGIN_ERROR} "${url.href}".`,
			kind: core6_api_submit_outcome_kind.failure,
			reason: core6_api_submit_failure_reason.cross_origin,
		};
	}
	return url.href;
}

function should_jsonify_core6_api_submit_body(
	body: BodyInit | null | undefined,
): boolean {
	return (
		body !== null &&
		body !== undefined &&
		typeof body === "object" &&
		!(body instanceof ReadableStream) &&
		!(body instanceof FormData) &&
		!(body instanceof URLSearchParams) &&
		!(body instanceof Blob) &&
		!(body instanceof ArrayBuffer) &&
		!ArrayBuffer.isView(body)
	);
}

function core6_api_submit_response_redirect(
	request_href: string,
	response: Response,
): { redirect: Core6APISubmitRedirect | null } | Core6APISubmitFailureOutcome {
	const soft_redirect = response.headers.get(X_CLIENT_REDIRECT);
	if (soft_redirect) {
		try {
			return {
				redirect: {
					hard: false,
					href: new URL(soft_redirect, request_href).href,
				},
			};
		} catch (error) {
			return {
				error: error instanceof Error ? error.message : String(error),
				kind: core6_api_submit_outcome_kind.failure,
				reason: core6_api_submit_failure_reason.invalid_redirect,
			};
		}
	}
	if (response.redirected && response.url && response.url !== request_href) {
		return {
			redirect: {
				hard: false,
				href: response.url,
			},
		};
	}
	return { redirect: null };
}

async function parse_core6_api_submit_data(
	response: Response,
): Promise<unknown> {
	if (response.status === 204) {
		return undefined;
	}
	const content_type = response.headers
		.get(CONTENT_TYPE_HEADER)
		?.toLowerCase();
	if (content_type?.includes("json")) {
		return await response.json();
	}
	const text = await response.text();
	if (text.length === 0) {
		return undefined;
	}
	return text;
}

function is_core6_api_submit_abort_error(error: unknown): boolean {
	return (
		error instanceof DOMException &&
		(error.name === core6_api_submit_abort_error_name ||
			error.message === CORE6_API_SUBMIT_ABORTED_ERROR)
	);
}

function is_core6_api_submit_failure_outcome(
	value:
		| Core6APISubmitRequest
		| Core6APISubmitOutcome
		| { redirect: Core6APISubmitRedirect | null },
): value is Core6APISubmitFailureOutcome {
	return (
		"kind" in value && value.kind === core6_api_submit_outcome_kind.failure
	);
}
