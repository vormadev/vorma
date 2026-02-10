import type { BeginNavigationContext } from "./begin_navigation.ts";
import {
	createRuntimeBeginNavigation,
	type RuntimeBeginNavigation,
} from "./runtime_begin_navigation.ts";
import {
	createRuntimeNavigate,
	type RuntimeNavigate,
} from "./runtime_navigate.ts";
import {
	createRuntimeProcessSuccessfulNavigation,
	type RuntimeProcessSuccessfulNavigation,
} from "./runtime_process_successful_navigation.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
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

	const processSuccessfulNavigation =
		createRuntimeProcessSuccessfulNavigation({
			transitionPhase,
			findNavigationEntry,
			deleteNavigation,
		});

	const beginNavigation = createRuntimeBeginNavigation({
		beginNavigationContext,
		createActiveNavigation,
	});

	const navigate = createRuntimeNavigate({
		beginNavigation,
		findNavigationEntry,
		deleteNavigation,
		processSuccessfulNavigation,
		onNavigationIntentResolved,
	});

	return {
		processSuccessfulNavigation,
		beginNavigation,
		navigate,
	};
}
