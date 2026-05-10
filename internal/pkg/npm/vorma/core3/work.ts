import type { CoreModel, WorkActivity, WorkProjection } from "./model.ts";

export const work_activity_kind = {
	api_request: "api_request",
	navigation: "navigation",
	prefetch: "prefetch",
	revalidation: "revalidation",
} as const;

export type WorkModelFacts = Pick<
	CoreModel,
	| "active_route_operation_id"
	| "operations"
	| "prefetch_operation_id"
	| "refresh"
	| "submissions"
>;

export type WorkDerivation = {
	activity: WorkActivity;
	projection: WorkProjection;
};

export function derive_work(model: WorkModelFacts): WorkDerivation {
	const active_operation =
		model.active_route_operation_id === null
			? undefined
			: model.operations[model.active_route_operation_id];
	const activity: WorkActivity[number][] = [];
	let navigation: WorkProjection["navigation"] = null;
	if (active_operation?.kind === "navigation") {
		navigation = {
			href: active_operation.href,
			replace: active_operation.replace,
			source: active_operation.source,
		};
		activity.push({
			kind: work_activity_kind.navigation,
			skip_work_indicator: active_operation.skip_work_indicator,
		});
	} else if (active_operation?.kind === "popstate") {
		navigation = {
			href: active_operation.browser.href,
			replace: true,
			source: active_operation.kind,
		};
		activity.push({
			kind: work_activity_kind.navigation,
			skip_work_indicator: false,
		});
	}

	let revalidation: WorkProjection["revalidation"] = null;
	if (model.refresh.kind !== "idle" && model.refresh.kind !== "pending") {
		revalidation = {
			attempt:
				model.refresh.kind === "debouncing" ? 0 : model.refresh.attempt,
			status: model.refresh.kind,
		};
		activity.push({
			kind: work_activity_kind.revalidation,
			skip_work_indicator: model.refresh.demand.skip_work_indicator,
		});
	}

	const prefetch_operation =
		model.prefetch_operation_id === null
			? undefined
			: model.operations[model.prefetch_operation_id];
	let prefetch: WorkProjection["prefetch"] = null;
	if (prefetch_operation?.kind === "route_prefetch") {
		switch (prefetch_operation.status) {
			case "requested":
			case "started":
			case "completed":
			case "classified": {
				prefetch = { href: prefetch_operation.href };
				activity.push({ kind: work_activity_kind.prefetch });
				break;
			}
			default: {
				break;
			}
		}
	}

	const submissions = Object.values(model.submissions);
	const api_requests = submissions.map((submission) => {
		return {
			href: submission.href,
			key: submission.submission_key,
			method: submission.method,
		};
	});
	for (const submission of submissions) {
		activity.push({
			kind: work_activity_kind.api_request,
			skip_work_indicator: submission.skip_work_indicator,
		});
	}

	return {
		activity,
		projection: {
			apiRequests: api_requests,
			navigation,
			prefetch,
			revalidation,
		},
	};
}
