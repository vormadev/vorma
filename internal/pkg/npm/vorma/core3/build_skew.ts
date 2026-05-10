import type {
	BuildSkewDefaultBehavior,
	BuildSkewDetectedEvent,
	BuildSkewReport,
	BuildSkewTriggeringResponse,
	CoreEffect,
	CoreModel,
	CoreOperation,
} from "./model.ts";
import {
	build_skew_default_behavior,
	build_skew_trigger_kind,
} from "./model.ts";
import { route_facts_to_state } from "./route.ts";
import { derive_work } from "./work.ts";

export type RouteBuildSkewOperation = Extract<
	CoreOperation,
	{
		kind:
			| "boot"
			| "navigation"
			| "popstate"
			| "route_prefetch"
			| "route_revalidation";
	}
>;

export type RouteBuildSkewEffectInput = {
	model: CoreModel;
	operation: RouteBuildSkewOperation;
	report: BuildSkewReport | undefined;
};

export function route_build_skew_effect(
	input: RouteBuildSkewEffectInput,
): CoreEffect | undefined {
	if (!input.report || !input.model.current) {
		return undefined;
	}
	const triggering_response = route_build_skew_triggering_response(
		input.operation,
		input.report,
	);
	if (!triggering_response) {
		return undefined;
	}
	const event = {
		activeClientBuildID: input.model.client_build_id,
		currentRouteState: route_facts_to_state(input.model.current.route),
		currentWorkState: derive_work(input.model).projection,
		defaultBehavior: build_skew_default_behavior_for_report(input.report),
		serverBuildID: input.report.server_build_id,
		triggeringResponse: triggering_response,
	} satisfies BuildSkewDetectedEvent;
	return {
		event,
		type: "report_build_skew",
	};
}

function build_skew_default_behavior_for_report(
	report: BuildSkewReport,
): BuildSkewDefaultBehavior {
	if (report.behavior === "drop") {
		return build_skew_default_behavior.drop_response;
	}
	if (report.behavior === "reload") {
		return build_skew_default_behavior.hard_reload;
	}
	return build_skew_default_behavior.notify_only;
}

function route_build_skew_triggering_response(
	operation: RouteBuildSkewOperation,
	report: BuildSkewReport,
): BuildSkewTriggeringResponse | undefined {
	const base = {
		kind: build_skew_trigger_kind.route,
		ok: report.ok,
		requestedHref:
			operation.kind === "popstate"
				? operation.browser.href
				: operation.href,
		status: report.status,
	};
	if (operation.kind === "route_revalidation") {
		return {
			...base,
			revalidationReason: operation.reason,
			trigger: "revalidation",
		};
	}
	if (operation.kind === "boot") {
		return undefined;
	}
	if (operation.kind === "route_prefetch") {
		return {
			...base,
			trigger: "prefetch",
		};
	}
	return {
		...base,
		trigger: operation.kind === "popstate" ? "popstate" : "navigation",
	};
}
