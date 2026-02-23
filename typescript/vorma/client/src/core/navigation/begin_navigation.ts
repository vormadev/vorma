import {
	decideBeginNavigationExecutionPlan,
	type BeginNavigationAbortInstruction,
	type BeginNavigationCreateInstruction,
	type BeginNavigationExecutionPlan,
	type BeginNavigationPromotion,
	type BeginNavigationReuseInstruction,
} from "./begin_navigation_state_machine.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";
export { createNavigationControls } from "./navigation_controls.ts";
export type {
	CreateNavigationControlsContext,
	NavigationControls,
} from "./navigation_controls.ts";

export type BeginNavigationContext = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getRevalidationNavigation: () => NavigationEntry | null;
	setRevalidationNavigation: (entry: NavigationEntry | null) => void;
	prefetchNavigationsByTargetUrl: Map<string, NavigationEntry>;
	scheduleStatusUpdate: () => void;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
	createPrefetch: (
		props: NavigateProps,
		targetUrl: string,
	) => NavigationControl;
	createRevalidation: (props: NavigateProps) => NavigationControl;
};

function promoteEntryToActiveLane(props: {
	entry: NavigationEntry;
	promotion: BeginNavigationPromotion;
}): void {
	const { entry, promotion } = props;
	entry.targetUrl = promotion.targetUrl;
	entry.scrollToTop = promotion.scrollToTop;
	entry.replace = promotion.replace;
	entry.state = promotion.state;
	entry.type = promotion.type;
	entry.intent = promotion.intent;
}

function executeAbortInstruction(props: {
	context: BeginNavigationContext;
	abortInstruction: BeginNavigationAbortInstruction;
}): { changedStatusRelevantLane: boolean } {
	const { context, abortInstruction } = props;
	abortInstruction.entry.control.abortController?.abort();

	switch (abortInstruction.slot) {
		case "active":
			if (context.getActiveNavigation() === abortInstruction.entry) {
				context.setActiveNavigation(null);
				return { changedStatusRelevantLane: true };
			}
			return { changedStatusRelevantLane: false };
		case "revalidation":
			if (
				context.getRevalidationNavigation() === abortInstruction.entry
			) {
				context.setRevalidationNavigation(null);
				return { changedStatusRelevantLane: true };
			}
			return { changedStatusRelevantLane: false };
		case "prefetch":
			if (
				context.prefetchNavigationsByTargetUrl.get(
					abortInstruction.key,
				) === abortInstruction.entry
			) {
				context.prefetchNavigationsByTargetUrl.delete(
					abortInstruction.key,
				);
			}
			return { changedStatusRelevantLane: false };
	}
}

function createImmediatelyAbortedNavigationControl(): NavigationControl {
	const abortController = new AbortController();
	abortController.abort("navigation_not_started");
	return {
		abortController,
		promise: Promise.resolve({ type: "aborted" as const }),
	};
}

function executeBeginNavigationReuseInstruction(props: {
	context: BeginNavigationContext;
	reuseInstruction: BeginNavigationReuseInstruction;
}): { control: NavigationControl; changedStatusRelevantLane: boolean } {
	const { context, reuseInstruction } = props;
	let changedStatusRelevantLane = false;

	if (reuseInstruction.promotion) {
		promoteEntryToActiveLane({
			entry: reuseInstruction.entry,
			promotion: reuseInstruction.promotion,
		});
	}

	switch (reuseInstruction.sourceSlot) {
		case "active":
			return {
				control: reuseInstruction.entry.control,
				changedStatusRelevantLane,
			};
		case "prefetch":
			if (reuseInstruction.promotion) {
				if (
					reuseInstruction.sourcePrefetchKey &&
					context.prefetchNavigationsByTargetUrl.get(
						reuseInstruction.sourcePrefetchKey,
					) === reuseInstruction.entry
				) {
					context.prefetchNavigationsByTargetUrl.delete(
						reuseInstruction.sourcePrefetchKey,
					);
				}
				context.setActiveNavigation(reuseInstruction.entry);
				changedStatusRelevantLane = true;
			}
			return {
				control: reuseInstruction.entry.control,
				changedStatusRelevantLane,
			};
		case "revalidation":
			if (reuseInstruction.promotion) {
				if (
					context.getRevalidationNavigation() ===
					reuseInstruction.entry
				) {
					context.setRevalidationNavigation(null);
				}
				context.setActiveNavigation(reuseInstruction.entry);
				changedStatusRelevantLane = true;
			}
			return {
				control: reuseInstruction.entry.control,
				changedStatusRelevantLane,
			};
	}
}

function executeBeginNavigationCreateInstruction(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	createInstruction: BeginNavigationCreateInstruction;
	shouldScheduleStatusUpdate: boolean;
}): NavigationControl {
	const {
		context,
		navigationProps,
		createInstruction,
		shouldScheduleStatusUpdate,
	} = props;
	switch (createInstruction.slot) {
		case "active":
			return context.createActiveNavigation(
				navigationProps,
				createInstruction.intent,
			);
		case "prefetch":
			if (shouldScheduleStatusUpdate) {
				context.scheduleStatusUpdate();
			}
			return context.createPrefetch(
				navigationProps,
				createInstruction.targetUrl,
			);
		case "revalidation":
			return context.createRevalidation({
				...navigationProps,
				href: createInstruction.revalidationHref,
			});
	}
}

function executeBeginNavigationExecutionPlan(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	executionPlan: BeginNavigationExecutionPlan;
}): NavigationControl {
	const { context, navigationProps, executionPlan } = props;
	let shouldScheduleStatusUpdate = false;

	for (const abortInstruction of executionPlan.abortInstructions) {
		const abortResult = executeAbortInstruction({
			context,
			abortInstruction,
		});
		if (abortResult.changedStatusRelevantLane) {
			shouldScheduleStatusUpdate = true;
		}
	}

	if (executionPlan.reuseInstruction) {
		const reuseResult = executeBeginNavigationReuseInstruction({
			context,
			reuseInstruction: executionPlan.reuseInstruction,
		});
		if (reuseResult.changedStatusRelevantLane) {
			shouldScheduleStatusUpdate = true;
		}
		if (shouldScheduleStatusUpdate) {
			context.scheduleStatusUpdate();
		}
		return reuseResult.control;
	}

	if (executionPlan.shouldReturnImmediatelyAbortedControl) {
		if (shouldScheduleStatusUpdate) {
			context.scheduleStatusUpdate();
		}
		return createImmediatelyAbortedNavigationControl();
	}

	const createInstruction = executionPlan.createInstruction;
	if (!createInstruction) {
		throw new Error(
			"Begin navigation execution plan was missing reuse, immediate-abort, and create instructions.",
		);
	}

	return executeBeginNavigationCreateInstruction({
		context,
		navigationProps,
		createInstruction,
		shouldScheduleStatusUpdate,
	});
}

export function beginNavigation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	const executionPlan = decideBeginNavigationExecutionPlan({
		navigationProps: props,
		currentHref: window.location.href,
		lanes: {
			active: context.getActiveNavigation(),
			revalidation: context.getRevalidationNavigation(),
			prefetch: context.prefetchNavigationsByTargetUrl,
		},
	});

	return executeBeginNavigationExecutionPlan({
		context,
		navigationProps: props,
		executionPlan,
	});
}
