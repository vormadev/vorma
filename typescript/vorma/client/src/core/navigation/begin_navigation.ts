import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";
import {
	decideBeginNavigationExecutionPlan,
	type BeginNavigationAbortInstruction,
	type BeginNavigationCreateInstruction,
	type BeginNavigationExecutionPlan,
	type BeginNavigationPromotion,
	type BeginNavigationReuseInstruction,
} from "./begin_navigation_state_machine.ts";
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

export type BeginNavigationExecutionCommand =
	| {
			type: "abort_instruction";
			abortInstruction: BeginNavigationAbortInstruction;
	  }
	| {
			type: "reuse_instruction";
			reuseInstruction: BeginNavigationReuseInstruction;
	  }
	| {
			type: "return_immediately_aborted_control";
	  }
	| {
			type: "create_active_control";
			intent: NavigationIntent;
	  }
	| {
			type: "create_prefetch_control";
			targetUrl: string;
	  }
	| {
			type: "create_revalidation_control";
			revalidationHref: string;
	  };

function buildCreateCommandFromCreateInstruction(props: {
	createInstruction: BeginNavigationCreateInstruction;
}): BeginNavigationExecutionCommand {
	const { createInstruction } = props;
	switch (createInstruction.slot) {
		case "active":
			return {
				type: "create_active_control",
				intent: createInstruction.intent,
			};
		case "prefetch":
			return {
				type: "create_prefetch_control",
				targetUrl: createInstruction.targetUrl,
			};
		case "revalidation":
			return {
				type: "create_revalidation_control",
				revalidationHref: createInstruction.revalidationHref,
			};
	}
}

export function buildBeginNavigationExecutionCommands(props: {
	executionPlan: BeginNavigationExecutionPlan;
}): BeginNavigationExecutionCommand[] {
	const { executionPlan } = props;
	const commands: BeginNavigationExecutionCommand[] = [];

	for (const abortInstruction of executionPlan.abortInstructions) {
		commands.push({
			type: "abort_instruction",
			abortInstruction,
		});
	}

	if (executionPlan.reuseInstruction) {
		commands.push({
			type: "reuse_instruction",
			reuseInstruction: executionPlan.reuseInstruction,
		});
		return commands;
	}

	if (executionPlan.shouldReturnImmediatelyAbortedControl) {
		commands.push({
			type: "return_immediately_aborted_control",
		});
		return commands;
	}

	const createInstruction = executionPlan.createInstruction;
	if (!createInstruction) {
		throw new Error(
			"Begin navigation execution plan was missing reuse, immediate-abort, and create instructions.",
		);
	}

	commands.push(
		buildCreateCommandFromCreateInstruction({
			createInstruction,
		}),
	);
	return commands;
}

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

function executeBeginNavigationExecutionCommands(props: {
	context: BeginNavigationContext;
	navigationProps: NavigateProps;
	commands: BeginNavigationExecutionCommand[];
}): NavigationControl {
	const { context, navigationProps, commands } = props;
	let shouldScheduleStatusUpdate = false;

	for (const command of commands) {
		switch (command.type) {
			case "abort_instruction": {
				const abortResult = executeAbortInstruction({
					context,
					abortInstruction: command.abortInstruction,
				});
				if (abortResult.changedStatusRelevantLane) {
					shouldScheduleStatusUpdate = true;
				}
				break;
			}
			case "reuse_instruction": {
				const reuseResult = executeBeginNavigationReuseInstruction({
					context,
					reuseInstruction: command.reuseInstruction,
				});
				if (reuseResult.changedStatusRelevantLane) {
					shouldScheduleStatusUpdate = true;
				}
				if (shouldScheduleStatusUpdate) {
					context.scheduleStatusUpdate();
				}
				return reuseResult.control;
			}
			case "return_immediately_aborted_control":
				if (shouldScheduleStatusUpdate) {
					context.scheduleStatusUpdate();
				}
				return createImmediatelyAbortedNavigationControl();
			case "create_active_control":
				return context.createActiveNavigation(
					navigationProps,
					command.intent,
				);
			case "create_prefetch_control":
				if (shouldScheduleStatusUpdate) {
					context.scheduleStatusUpdate();
				}
				return context.createPrefetch(
					navigationProps,
					command.targetUrl,
				);
			case "create_revalidation_control":
				return context.createRevalidation({
					...navigationProps,
					href: command.revalidationHref,
				});
		}
	}

	throw new Error(
		"Begin navigation command plan ended without a terminal control command.",
	);
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
	const commands = buildBeginNavigationExecutionCommands({
		executionPlan,
	});

	return executeBeginNavigationExecutionCommands({
		context,
		navigationProps: props,
		commands,
	});
}
