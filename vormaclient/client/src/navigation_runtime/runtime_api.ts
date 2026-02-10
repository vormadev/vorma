import type { StatusEventDetail } from "../events.ts";
import type { BeginNavigationContext } from "./begin_navigation.ts";
import { createNavigationBookkeepingAdapter } from "./bookkeeping_adapter.ts";
import type { NavigationBookkeeping } from "./navigation_bookkeeping.ts";
import { createRuntimeAPISurface } from "./runtime_api_surface.ts";
import { createRuntimeAPIWiring } from "./runtime_api_wiring.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationIntent,
	NavigationStateManager,
	SubmissionEntry,
} from "./types.ts";

export type CreateNavigationRuntimeAPIOptions = {
	submissions: Map<string | symbol, SubmissionEntry>;
	getStatus: () => StatusEventDetail;
	scheduleStatusUpdate: () => void;
	navigationBookkeeping: NavigationBookkeeping;
	beginNavigationContext: BeginNavigationContext;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
	onNavigationIntentResolved?: () => void;
};

export function createNavigationRuntimeAPI(
	options: CreateNavigationRuntimeAPIOptions,
): NavigationStateManager {
	const {
		submissions,
		getStatus,
		scheduleStatusUpdate,
		navigationBookkeeping,
		beginNavigationContext,
		createActiveNavigation,
		onNavigationIntentResolved,
	} = options;

	const {
		clearAll,
		transitionPhase,
		findNavigationEntry,
		deleteNavigation,
		removeNavigation,
		getNavigation,
		hasNavigation,
		getNavigationsSize,
		getNavigations,
	} = createNavigationBookkeepingAdapter({
		navigationBookkeeping,
		submissions,
	});

	const { processSuccessfulNavigation, beginNavigation, navigate, submit } =
		createRuntimeAPIWiring({
			transitionPhase,
			findNavigationEntry,
			deleteNavigation,
			beginNavigationContext,
			createActiveNavigation,
			onNavigationIntentResolved,
			submissions,
			scheduleStatusUpdate,
		});

	return createRuntimeAPISurface({
		submissions,
		navigate,
		beginNavigation,
		processSuccessfulNavigation,
		submit,
		removeNavigation,
		getNavigation,
		hasNavigation,
		getNavigationsSize,
		getNavigations,
		getStatus,
		clearAll,
	});
}
