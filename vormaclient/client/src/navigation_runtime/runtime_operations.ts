import type { BeginNavigationContext } from "./begin_navigation.ts";
import {
	createRuntimeNavigationOperations,
	type RuntimeNavigationOperations,
} from "./runtime_navigation_operations.ts";
import { executeSubmit } from "./submit.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationPhase,
	SubmitOptions,
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

type RuntimeSubmit = <T = any>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
) => Promise<{ success: true; data: T } | { success: false; error: string }>;

export function createRuntimeOperations(
	context: RuntimeOperationsContext,
): RuntimeOperations {
	const { submissions, scheduleStatusUpdate } = context;

	const { processSuccessfulNavigation, beginNavigation, navigate } =
		createRuntimeNavigationOperations(context);

	const submit: RuntimeSubmit = (url, requestInit, options) =>
		executeSubmit(
			{
				submissions,
				scheduleStatusUpdate,
				navigate,
			},
			url,
			requestInit,
			options,
		);

	return {
		processSuccessfulNavigation,
		beginNavigation,
		navigate,
		submit,
	};
}
