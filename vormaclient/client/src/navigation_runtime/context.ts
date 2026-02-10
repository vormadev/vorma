import type { BeginNavigationContext } from "./begin_navigation.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";

export type BuildBeginNavigationContextInput = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getPendingRevalidation: () => NavigationEntry | null;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	prefetchCache: Map<string, NavigationEntry>;
	scheduleStatusUpdate: () => void;
	revalidationCoalesceMS: number;
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

export function buildBeginNavigationContext(
	input: BuildBeginNavigationContextInput,
): BeginNavigationContext {
	return {
		getActiveNavigation: input.getActiveNavigation,
		setActiveNavigation: input.setActiveNavigation,
		getPendingRevalidation: input.getPendingRevalidation,
		setPendingRevalidation: input.setPendingRevalidation,
		prefetchCache: input.prefetchCache,
		scheduleStatusUpdate: input.scheduleStatusUpdate,
		revalidationCoalesceMS: input.revalidationCoalesceMS,
		createActiveNavigation: input.createActiveNavigation,
		createPrefetch: input.createPrefetch,
		createRevalidation: input.createRevalidation,
	};
}
