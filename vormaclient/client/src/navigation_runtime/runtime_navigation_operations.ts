import {
	beginPrefetch as executeBeginPrefetch,
	beginRevalidation as executeBeginRevalidation,
	beginUserNavigation as executeBeginUserNavigation,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import {
	beginNavigationWithHandlers,
	navigateWithHandlers,
} from "./navigate.ts";
import { processSuccessfulNavigation as executeProcessSuccessfulNavigation } from "./process_successful_navigation.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";

export type RuntimeNavigationOperationsContext = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	beginNavigationContext: BeginNavigationContext;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
	onNavigationIntentResolved?: () => void;
};

export type RuntimeNavigationOperations = {
	processSuccessfulNavigation: RuntimeProcessSuccessfulNavigation;
	beginNavigation: RuntimeBeginNavigation;
	navigate: RuntimeNavigate;
};

type RuntimeProcessSuccessfulNavigation = (
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
) => Promise<void>;

type RuntimeNavigate = (
	props: NavigateProps,
) => Promise<{ didNavigate: boolean }>;

type RuntimeBeginNavigation = (props: NavigateProps) => NavigationControl;

export function createRuntimeNavigationOperations(
	context: RuntimeNavigationOperationsContext,
): RuntimeNavigationOperations {
	const {
		transitionPhase,
		findNavigationEntry,
		deleteNavigation,
		beginNavigationContext,
		createActiveNavigation,
		onNavigationIntentResolved,
	} = context;

	const processSuccessfulNavigation: RuntimeProcessSuccessfulNavigation =
		async (outcome, entry) =>
			executeProcessSuccessfulNavigation(
				{
					transitionPhase,
					findNavigationEntry,
					deleteNavigation,
				},
				outcome,
				entry,
			);

	const beginNavigation: RuntimeBeginNavigation = (props) =>
		beginNavigationWithHandlers(
			{
				beginUserNavigation: (input, targetUrl) =>
					executeBeginUserNavigation(
						beginNavigationContext,
						input,
						targetUrl,
					),
				beginPrefetch: (input, targetUrl) =>
					executeBeginPrefetch(
						beginNavigationContext,
						input,
						targetUrl,
					),
				beginRevalidation: (input) =>
					executeBeginRevalidation(beginNavigationContext, input),
				createActiveNavigation,
			},
			props,
		);

	const navigate: RuntimeNavigate = (props) =>
		navigateWithHandlers(
			{
				beginNavigation,
				findNavigationEntry,
				deleteNavigation,
				processSuccessfulNavigation,
				onNavigationIntentResolved,
			},
			props,
		);

	return {
		processSuccessfulNavigation,
		beginNavigation,
		navigate,
	};
}
