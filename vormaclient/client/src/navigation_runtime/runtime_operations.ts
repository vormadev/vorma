import type { BeginNavigationContext } from "./begin_navigation.ts";
import {
	createRuntimeNavigationOperations,
	type RuntimeNavigationOperations,
} from "./runtime_navigation_operations.ts";
import { createRuntimeSubmit, type RuntimeSubmit } from "./runtime_submit.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

type RuntimeOperationsContext = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	beginNavigationContext: BeginNavigationContext;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
	onNavigationIntentResolved?: () => void;
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
};

export type RuntimeOperations = {
	processSuccessfulNavigation: RuntimeNavigationOperations["processSuccessfulNavigation"];
	beginNavigation: RuntimeNavigationOperations["beginNavigation"];
	navigate: RuntimeNavigationOperations["navigate"];
	submit: RuntimeSubmit;
};

export function createRuntimeOperations(
	context: RuntimeOperationsContext,
): RuntimeOperations {
	const { submissions, scheduleStatusUpdate } = context;

	const { processSuccessfulNavigation, beginNavigation, navigate } =
		createRuntimeNavigationOperations(context);

	const submit = createRuntimeSubmit({
		submissions,
		scheduleStatusUpdate,
		navigate,
	});

	return {
		processSuccessfulNavigation,
		beginNavigation,
		navigate,
		submit,
	};
}
