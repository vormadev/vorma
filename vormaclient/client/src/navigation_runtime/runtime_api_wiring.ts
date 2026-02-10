import type { BeginNavigationContext } from "./begin_navigation.ts";
import { createRuntimeOperations } from "./runtime_operations.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

export type RuntimeAPIWiringContext = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
	beginNavigationContext: BeginNavigationContext;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
	onNavigationIntentResolved?: () => void;
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
};

export function createRuntimeAPIWiring(context: RuntimeAPIWiringContext) {
	const {
		submissions,
		scheduleStatusUpdate,
		beginNavigationContext,
		createActiveNavigation,
		onNavigationIntentResolved,
		transitionPhase,
		findNavigationEntry,
		deleteNavigation,
	} = context;

	return createRuntimeOperations({
		transitionPhase,
		findNavigationEntry,
		deleteNavigation,
		beginNavigationContext,
		createActiveNavigation,
		onNavigationIntentResolved,
		submissions,
		scheduleStatusUpdate,
	});
}
