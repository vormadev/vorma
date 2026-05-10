import { parse_href, route_hrefs_share_document } from "./href.ts";
import type {
	APIOutcome,
	APIResponseFacts,
	BuildSkewReport,
	NavigationRequestOutcome,
	OperationID,
	OperationKind,
	PreparationOutcome,
	PreparedRoute,
	RouteHooksOutcome,
	RouteOutcome,
	RouteResponseFacts,
	SubmissionKey,
} from "./model.ts";

export type RouteClassificationInput = {
	active_client_build_id: string;
	is_current_operation: boolean;
	max_redirects: number;
	max_revalidation_retries: number;
	operation_id: OperationID;
	operation_kind: OperationKind;
	redirect_count: number;
	revalidation_attempt: number;
	response: RouteResponseFacts;
};

export type RouteClassification = {
	build_skew_report?: BuildSkewReport;
	outcome: RouteOutcome;
};

export type APIClassificationInput = {
	active_client_build_id: string;
	is_current_submission: boolean;
	operation_id: OperationID;
	response: APIResponseFacts;
	revalidate: boolean;
	submission_key: SubmissionKey;
};

export type APIClassification = {
	build_skew_report?: BuildSkewReport;
	outcome: APIOutcome;
};

export type NavigationRequestClassificationInput = {
	current_browser_href: string;
	current_route_href: string | null;
	href: string;
};

export type NavigationRequestClassification = {
	outcome: NavigationRequestOutcome;
};

export type RoutePreparationFacts =
	| {
			kind: "aborted";
	  }
	| {
			cause: unknown;
			kind: "failed";
	  }
	| {
			kind: "prepared";
			prepared: PreparedRoute;
	  };

export type RoutePreparationClassificationInput = {
	is_current_operation: boolean;
	operation_id: OperationID;
	result: RoutePreparationFacts;
};

export type RouteHooksFacts =
	| {
			kind: "aborted";
	  }
	| {
			cause: unknown;
			kind: "failed";
	  }
	| {
			kind: "completed";
			prepared: PreparedRoute;
	  };

export type RouteHooksClassificationInput = {
	is_current_operation: boolean;
	operation_id: OperationID;
	result: RouteHooksFacts;
};

export function classify_route_response(
	input: RouteClassificationInput,
): RouteClassification {
	if (!input.is_current_operation) {
		return {
			outcome: {
				kind: "stale",
				operation_id: input.operation_id,
			},
		};
	}

	if (input.response.kind === "build_skew") {
		const behavior =
			input.operation_kind === "route_revalidation" ||
			input.operation_kind === "route_prefetch"
				? "drop"
				: "reload";
		return {
			build_skew_report: build_skew_report(
				input.active_client_build_id,
				input.response.server_build_id,
				behavior,
				input.response.ok,
				input.response.status,
			),
			outcome: {
				behavior,
				kind:
					behavior === "drop"
						? "build_skew_drop"
						: "build_skew_reload",
				operation_id: input.operation_id,
				server_build_id: input.response.server_build_id,
			},
		};
	}

	if (input.response.kind === "data") {
		return {
			build_skew_report: build_skew_report(
				input.active_client_build_id,
				input.response.server_build_id,
				"notify",
				input.response.ok,
				input.response.status,
			),
			outcome: {
				kind: "route_data",
				operation_id: input.operation_id,
				payload: input.response.payload,
			},
		};
	}

	if (input.response.kind === "error") {
		let outcome: RouteOutcome;
		if (input.operation_kind !== "route_revalidation") {
			outcome = {
				kind: "route_error",
				operation_id: input.operation_id,
				status_text: input.response.status_text,
			};
		} else if (
			input.revalidation_attempt >= input.max_revalidation_retries
		) {
			outcome = {
				kind: "terminal_revalidation_failure",
				operation_id: input.operation_id,
				status_text: input.response.status_text,
			};
		} else {
			outcome = {
				kind: "retryable_revalidation_failure",
				operation_id: input.operation_id,
				status_text: input.response.status_text,
			};
		}
		return {
			build_skew_report: build_skew_report(
				input.active_client_build_id,
				input.response.server_build_id,
				"notify",
				input.response.ok,
				input.response.status,
			),
			outcome,
		};
	}

	const behavior =
		input.response.hard && input.response.http
			? input.operation_kind === "route_revalidation" ||
				input.operation_kind === "route_prefetch"
				? "drop"
				: "reload"
			: "notify";
	const report = build_skew_report(
		input.active_client_build_id,
		input.response.server_build_id,
		behavior,
		input.response.ok,
		input.response.status,
	);
	if (!input.response.http) {
		return {
			build_skew_report: report,
			outcome: {
				href: input.response.href,
				kind: "invalid_redirect",
				operation_id: input.operation_id,
			},
		};
	}
	if (!input.response.hard && input.redirect_count >= input.max_redirects) {
		return {
			build_skew_report: report,
			outcome: {
				href: input.response.href,
				kind: "redirect_loop",
				operation_id: input.operation_id,
			},
		};
	}
	return {
		build_skew_report: report,
		outcome: {
			hard: input.response.hard,
			href: input.response.href,
			kind: input.response.hard ? "hard_redirect" : "soft_redirect",
			operation_id: input.operation_id,
		},
	};
}

export function classify_navigation_request(
	input: NavigationRequestClassificationInput,
): NavigationRequestClassification {
	const current_browser_url = parse_href(input.current_browser_href);
	if (!current_browser_url) {
		return {
			outcome: {
				href: input.href,
				kind: "invalid_href",
			},
		};
	}
	const target_url = parse_href(input.href, current_browser_url.href);
	if (!target_url) {
		return {
			outcome: {
				href: input.href,
				kind: "invalid_href",
			},
		};
	}
	if (target_url.origin !== current_browser_url.origin) {
		return {
			outcome: {
				href: target_url.href,
				kind: "hard_redirect",
			},
		};
	}
	if (
		input.current_route_href &&
		route_hrefs_share_document(input.current_route_href, target_url.href)
	) {
		return {
			outcome: {
				href: target_url.href,
				kind: "same_document",
			},
		};
	}
	return {
		outcome: {
			href: target_url.href,
			kind: "route_navigation",
		},
	};
}

export function classify_api_response(
	input: APIClassificationInput,
): APIClassification {
	if (!input.is_current_submission) {
		return {
			outcome: {
				kind: "stale",
				operation_id: input.operation_id,
				submission_key: input.submission_key,
			},
		};
	}

	if (input.response.kind === "success") {
		return {
			build_skew_report: build_skew_report(
				input.active_client_build_id,
				input.response.server_build_id,
				"notify",
				input.response.ok,
				input.response.status,
			),
			outcome: {
				data: input.response.data,
				kind: "success",
				operation_id: input.operation_id,
				revalidation_required: input.revalidate,
				response: input.response.response,
				submission_key: input.submission_key,
			},
		};
	}

	if (input.response.kind === "failure") {
		let report: BuildSkewReport | undefined;
		if (
			input.response.server_build_id !== undefined &&
			input.response.status !== undefined
		) {
			report = build_skew_report(
				input.active_client_build_id,
				input.response.server_build_id,
				"notify",
				input.response.ok === true,
				input.response.status,
			);
		}
		const outcome: APIOutcome = {
			error: input.response.error,
			kind: "failure",
			operation_id: input.operation_id,
			revalidation_required:
				input.revalidate &&
				input.response.dispatched &&
				input.response.should_revalidate !== false,
			submission_key: input.submission_key,
		};
		if (input.response.response !== undefined) {
			outcome.response = input.response.response;
		}
		return {
			build_skew_report: report,
			outcome,
		};
	}

	if (!input.response.http) {
		return {
			build_skew_report: build_skew_report(
				input.active_client_build_id,
				input.response.server_build_id,
				"notify",
				input.response.ok,
				input.response.status,
			),
			outcome: {
				error: `Redirect target must use an HTTP(S) scheme. Received: "${input.response.href}".`,
				kind: "failure",
				operation_id: input.operation_id,
				revalidation_required: false,
				response: input.response.response,
				submission_key: input.submission_key,
			},
		};
	}
	return {
		build_skew_report: build_skew_report(
			input.active_client_build_id,
			input.response.server_build_id,
			input.response.hard ? "reload" : "notify",
			input.response.ok,
			input.response.status,
		),
		outcome: {
			href: input.response.href,
			kind: input.response.hard ? "hard_redirect" : "soft_redirect",
			operation_id: input.operation_id,
			response: input.response.response,
			submission_key: input.submission_key,
		},
	};
}

export function classify_route_preparation(
	input: RoutePreparationClassificationInput,
): PreparationOutcome {
	if (!input.is_current_operation) {
		return {
			kind: "stale",
			operation_id: input.operation_id,
		};
	}
	if (input.result.kind === "aborted") {
		return {
			kind: "aborted",
			operation_id: input.operation_id,
		};
	}
	if (input.result.kind === "failed") {
		return {
			cause: input.result.cause,
			kind: "failed",
			operation_id: input.operation_id,
		};
	}
	return {
		kind: "prepared",
		operation_id: input.operation_id,
		prepared: input.result.prepared,
	};
}

export function classify_route_hooks(
	input: RouteHooksClassificationInput,
): RouteHooksOutcome {
	if (!input.is_current_operation) {
		return {
			kind: "stale",
			operation_id: input.operation_id,
		};
	}
	if (input.result.kind === "aborted") {
		return {
			kind: "aborted",
			operation_id: input.operation_id,
		};
	}
	if (input.result.kind === "failed") {
		return {
			cause: input.result.cause,
			kind: "failed",
			operation_id: input.operation_id,
		};
	}
	return {
		kind: "publishable",
		operation_id: input.operation_id,
		prepared: input.result.prepared,
	};
}

function build_skew_report(
	active_client_build_id: string,
	server_build_id: string,
	behavior: BuildSkewReport["behavior"],
	ok: boolean,
	status: number,
): BuildSkewReport | undefined {
	if (
		server_build_id.length === 0 ||
		server_build_id === active_client_build_id
	) {
		return undefined;
	}
	return {
		behavior,
		ok,
		server_build_id,
		status,
	};
}
