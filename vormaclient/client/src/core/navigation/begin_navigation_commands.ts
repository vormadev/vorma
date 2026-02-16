import type {
	BeginNavigationAbortInstruction,
	BeginNavigationCreateInstruction,
	BeginNavigationExecutionPlan,
	BeginNavigationReuseInstruction,
} from "./begin_navigation_state_machine.ts";
import type { NavigationIntent } from "./types.ts";

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
