import type { StatusEventDetail } from "../platform/events.ts";

export type FocusRevalidationTriggerExecutionPlan =
	| {
			type: "trigger_revalidate";
			reason: "focus_revalidate_allowed";
	  }
	| {
			type: "skip";
			reason:
				| "focus_revalidate_blocked_navigating"
				| "focus_revalidate_blocked_submitting"
				| "focus_revalidate_blocked_revalidating"
				| "focus_revalidate_blocked_stale_window_not_elapsed";
	  };

export function decideFocusRevalidationTriggerExecutionPlan(props: {
	status: StatusEventDetail;
	nowTimestampMS: number;
	lastTriggeredNavOrRevalidateTimestampMS: number;
	staleTimeMS: number;
}): FocusRevalidationTriggerExecutionPlan {
	if (props.status.isNavigating) {
		return {
			type: "skip",
			reason: "focus_revalidate_blocked_navigating",
		};
	}

	if (props.status.isSubmitting) {
		return {
			type: "skip",
			reason: "focus_revalidate_blocked_submitting",
		};
	}

	if (props.status.isRevalidating) {
		return {
			type: "skip",
			reason: "focus_revalidate_blocked_revalidating",
		};
	}

	if (
		props.nowTimestampMS - props.lastTriggeredNavOrRevalidateTimestampMS <
		props.staleTimeMS
	) {
		return {
			type: "skip",
			reason: "focus_revalidate_blocked_stale_window_not_elapsed",
		};
	}

	return {
		type: "trigger_revalidate",
		reason: "focus_revalidate_allowed",
	};
}
