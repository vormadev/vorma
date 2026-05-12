import type {
	Core6RouteFetchBuildSkewResult,
	Core6RouteFetchFailureResult,
	Core6RouteFetchRedirect,
	Core6RouteFetchRedirectResult,
} from "./route_fetch.ts";
import {
	core6_route_fetch_driver_result_kind,
	type Core6RouteFetchDriverPublishedResult,
} from "./route_fetch_driver.ts";
import type { Core6RoutePrefetchOwner } from "./route_prefetch.ts";
import { core6_route_preparation_trigger } from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RoutePublication,
} from "./route_publication.ts";
import type { Core6RouteRuntime } from "./route_runtime.ts";
import type { Core6RouteTransactionFlowIntent } from "./route_transaction_flow.ts";
import {
	core6_same_document_href,
	core6_scroll_for_href,
} from "./route_url.ts";
import {
	core6_route_transaction_kind,
	type Core6RouteTransactionKind,
} from "./transaction.ts";

export const CORE6_MAX_REDIRECTS = 10;

const core6_route_navigation_interrupted_error_reason = "error";

export const core6_route_redirect_decision_kind = {
	follow: "follow",
	hard_redirect: "hard_redirect",
	invalid_redirect: "invalid_redirect",
	redirect_loop: "redirect_loop",
	same_document: "same_document",
} as const;

export const core6_route_navigation_result_kind = {
	build_skew: "build_skew",
	failed: "failed",
	hard_redirect: "hard_redirect",
	interrupted: "interrupted",
	invalid_redirect: "invalid_redirect",
	published: "published",
	redirect_loop: "redirect_loop",
	same_document: "same_document",
} as const;

export const core6_route_navigation_build_skew_default_behavior = {
	drop_response: "drop_response",
	hard_reload: "hard_reload",
	notify_only: "notify_only",
} as const;

export type Core6RouteNavigationBuildSkewDefaultBehavior =
	(typeof core6_route_navigation_build_skew_default_behavior)[keyof typeof core6_route_navigation_build_skew_default_behavior];

export type Core6RouteRedirectDecision =
	| {
			href: string;
			kind: typeof core6_route_redirect_decision_kind.follow;
	  }
	| {
			href: string;
			kind: typeof core6_route_redirect_decision_kind.hard_redirect;
	  }
	| {
			href: string;
			kind: typeof core6_route_redirect_decision_kind.invalid_redirect;
	  }
	| {
			href: string;
			kind: typeof core6_route_redirect_decision_kind.redirect_loop;
			redirect_count: number;
	  }
	| {
			href: string;
			kind: typeof core6_route_redirect_decision_kind.same_document;
	  };

export type Core6RouteNavigationBuildSkewEvent = {
	active_client_build_id: string;
	default_behavior: Core6RouteNavigationBuildSkewDefaultBehavior;
	revalidation_reason?: string;
	response: Core6RouteFetchBuildSkewResult["response"];
	trigger: Core6RouteTransactionKind;
};

export type Core6RouteNavigationRedirectLoopEvent = {
	href: string;
	redirect_count: number;
};

export type Core6RouteNavigationHost = {
	hard_redirect: (href: string) => void;
	notify_build_skew?: (event: Core6RouteNavigationBuildSkewEvent) => void;
	warn_redirect_loop?: (event: Core6RouteNavigationRedirectLoopEvent) => void;
};

export type Core6RouteNavigationInput = {
	active_client_build_id: string;
	deployment_id?: string | null;
	host: Core6RouteNavigationHost;
	intent: Core6RouteTransactionFlowIntent;
	kind: Core6RouteTransactionKind;
	max_redirects?: number;
	prefetch?: Pick<Core6RoutePrefetchOwner, "take_prepared">;
	runtime: Pick<
		Core6RouteRuntime,
		| "current_route"
		| "publish_same_document"
		| "run_route_fetch"
		| "run_route_prepared"
	>;
};

export type Core6RouteNavigationResult =
	| {
			kind: typeof core6_route_navigation_result_kind.published;
			redirect_count: number;
			result: Core6RouteFetchDriverPublishedResult;
	  }
	| {
			behavior: Core6RouteNavigationBuildSkewDefaultBehavior;
			kind: typeof core6_route_navigation_result_kind.build_skew;
			redirect_count: number;
			result: Core6RouteFetchBuildSkewResult;
	  }
	| {
			kind: typeof core6_route_navigation_result_kind.failed;
			redirect_count: number;
			result: Core6RouteFetchFailureResult;
	  }
	| {
			href: string;
			kind: typeof core6_route_navigation_result_kind.hard_redirect;
			redirect_count: number;
			result: Core6RouteFetchRedirectResult;
	  }
	| {
			href: string;
			kind: typeof core6_route_navigation_result_kind.invalid_redirect;
			redirect_count: number;
			result: Core6RouteFetchRedirectResult;
	  }
	| {
			href: string;
			kind: typeof core6_route_navigation_result_kind.redirect_loop;
			redirect_count: number;
			result: Core6RouteFetchRedirectResult;
	  }
	| {
			href: string;
			kind: typeof core6_route_navigation_result_kind.same_document;
			publication: Core6RoutePublication;
			redirect_count: number;
			result: Core6RouteFetchRedirectResult;
	  }
	| {
			kind: typeof core6_route_navigation_result_kind.interrupted;
			reason: string;
			redirect_count: number;
	  };

export function resolve_core6_route_redirect_decision(input: {
	current_href: string | null;
	max_redirects?: number;
	redirect: Core6RouteFetchRedirect;
	redirect_count: number;
	source_href: string;
}): Core6RouteRedirectDecision {
	let target: URL;
	let source: URL;
	try {
		target = new URL(input.redirect.href);
		source = new URL(input.source_href);
	} catch {
		return {
			href: input.redirect.href,
			kind: core6_route_redirect_decision_kind.invalid_redirect,
		};
	}

	if (target.protocol !== "http:" && target.protocol !== "https:") {
		return {
			href: target.href,
			kind: core6_route_redirect_decision_kind.invalid_redirect,
		};
	}

	if (input.redirect.hard || target.origin !== source.origin) {
		return {
			href: target.href,
			kind: core6_route_redirect_decision_kind.hard_redirect,
		};
	}

	const max_redirects = input.max_redirects ?? CORE6_MAX_REDIRECTS;
	if (input.redirect_count >= max_redirects) {
		return {
			href: target.href,
			kind: core6_route_redirect_decision_kind.redirect_loop,
			redirect_count: input.redirect_count + 1,
		};
	}

	if (input.current_href) {
		if (core6_same_document_href(target.href, input.current_href)) {
			return {
				href: target.href,
				kind: core6_route_redirect_decision_kind.same_document,
			};
		}
	}

	return {
		href: target.href,
		kind: core6_route_redirect_decision_kind.follow,
	};
}

export async function run_core6_route_navigation(
	input: Core6RouteNavigationInput,
): Promise<Core6RouteNavigationResult> {
	let redirect_count = 0;
	let intent = input.intent;
	let kind = input.kind;

	while (true) {
		if (
			kind === core6_route_transaction_kind.navigation ||
			kind === core6_route_transaction_kind.popstate
		) {
			const promotion = input.prefetch?.take_prepared(intent.href);
			if (promotion) {
				const result = input.runtime.run_route_prepared({
					intent,
					kind,
					prepared: {
						...promotion.prepared,
						history_state: intent.history_state,
						href: intent.href,
					},
				});
				if (!result.ok) {
					return {
						kind: core6_route_navigation_result_kind.interrupted,
						reason: result.reason,
						redirect_count,
					};
				}
				return {
					kind: core6_route_navigation_result_kind.published,
					redirect_count,
					result: {
						kind: core6_route_fetch_driver_result_kind.published,
						publication: result.value,
						response: promotion.response,
					},
				};
			}
		}

		let result;
		try {
			result = await input.runtime.run_route_fetch({
				active_client_build_id: input.active_client_build_id,
				deployment_id: input.deployment_id,
				intent,
				kind,
			});
		} catch {
			return {
				kind: core6_route_navigation_result_kind.interrupted,
				reason: core6_route_navigation_interrupted_error_reason,
				redirect_count,
			};
		}
		if (!result.ok) {
			return {
				kind: core6_route_navigation_result_kind.interrupted,
				reason: result.reason,
				redirect_count,
			};
		}

		if (
			result.value.kind === core6_route_fetch_driver_result_kind.published
		) {
			return {
				kind: core6_route_navigation_result_kind.published,
				redirect_count,
				result: result.value,
			};
		}

		if (
			result.value.kind === core6_route_fetch_driver_result_kind.failure
		) {
			return {
				kind: core6_route_navigation_result_kind.failed,
				redirect_count,
				result: result.value,
			};
		}

		if (
			result.value.kind ===
			core6_route_fetch_driver_result_kind.build_skew
		) {
			const behavior =
				kind === core6_route_transaction_kind.prefetch ||
				kind === core6_route_transaction_kind.revalidation
					? core6_route_navigation_build_skew_default_behavior.drop_response
					: result.value.redirect
						? core6_route_navigation_build_skew_default_behavior.hard_reload
						: core6_route_navigation_build_skew_default_behavior.notify_only;
			if (kind !== core6_route_transaction_kind.revalidation) {
				input.host.notify_build_skew?.({
					active_client_build_id: input.active_client_build_id,
					default_behavior: behavior,
					response: result.value.response,
					trigger: kind,
				});
			}
			if (
				behavior ===
				core6_route_navigation_build_skew_default_behavior.hard_reload
			) {
				input.host.hard_redirect(result.value.response.requested_href);
			}
			return {
				behavior,
				kind: core6_route_navigation_result_kind.build_skew,
				redirect_count,
				result: result.value,
			};
		}

		const decision = resolve_core6_route_redirect_decision({
			current_href: input.runtime.current_route()?.href ?? null,
			max_redirects: input.max_redirects,
			redirect: result.value.redirect,
			redirect_count,
			source_href: result.value.response.requested_href,
		});
		if (
			decision.kind === core6_route_redirect_decision_kind.hard_redirect
		) {
			notify_core6_route_redirect_build_skew({
				active_client_build_id: input.active_client_build_id,
				behavior:
					core6_route_navigation_build_skew_default_behavior.hard_reload,
				host: input.host,
				response: result.value.response,
				trigger: kind,
			});
			input.host.hard_redirect(decision.href);
			return {
				href: decision.href,
				kind: core6_route_navigation_result_kind.hard_redirect,
				redirect_count,
				result: result.value,
			};
		}
		if (
			decision.kind ===
			core6_route_redirect_decision_kind.invalid_redirect
		) {
			return {
				href: decision.href,
				kind: core6_route_navigation_result_kind.invalid_redirect,
				redirect_count,
				result: result.value,
			};
		}
		if (
			decision.kind === core6_route_redirect_decision_kind.redirect_loop
		) {
			input.host.warn_redirect_loop?.({
				href: decision.href,
				redirect_count: decision.redirect_count,
			});
			return {
				href: decision.href,
				kind: core6_route_navigation_result_kind.redirect_loop,
				redirect_count: decision.redirect_count,
				result: result.value,
			};
		}
		if (
			decision.kind === core6_route_redirect_decision_kind.same_document
		) {
			const publication = input.runtime.publish_same_document({
				history: intent.history
					? {
							...intent.history,
							href: decision.href,
						}
					: undefined,
				history_state: intent.history_state,
				href: decision.href,
				reason: intent.publish_reason,
				scroll: core6_scroll_for_href(decision.href),
			});
			if (!publication) {
				throw new Error(
					"same-document navigation requires current route",
				);
			}
			return {
				href: decision.href,
				kind: core6_route_navigation_result_kind.same_document,
				publication,
				redirect_count,
				result: result.value,
			};
		}

		redirect_count += 1;
		const target = new URL(decision.href);
		intent = {
			...intent,
			href: target.href,
			preparation_trigger: core6_route_preparation_trigger.navigation,
			publish_reason: core6_route_publish_reason.navigation,
			search_params: new URLSearchParams(target.search),
		};
		kind = core6_route_transaction_kind.navigation;
	}
}

function notify_core6_route_redirect_build_skew(input: {
	active_client_build_id: string;
	behavior: Core6RouteNavigationBuildSkewDefaultBehavior;
	host: Pick<Core6RouteNavigationHost, "notify_build_skew">;
	response: Core6RouteFetchRedirectResult["response"];
	trigger: Core6RouteTransactionKind;
}): void {
	if (
		input.response.server_build_id.length === 0 ||
		input.active_client_build_id.length === 0 ||
		input.response.server_build_id === input.active_client_build_id
	) {
		return;
	}
	input.host.notify_build_skew?.({
		active_client_build_id: input.active_client_build_id,
		default_behavior: input.behavior,
		response: input.response,
		trigger: input.trigger,
	});
}
