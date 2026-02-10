import type { StatusEventDetail } from "../events.ts";
import type {
	NavigationEntry,
	NavigationOutcome,
	NavigationStateManager,
	SubmitOptions,
	SubmissionEntry,
} from "./types.ts";

export type CreateRuntimeAPISurfaceInput = {
	submissions: Map<string | symbol, SubmissionEntry>;
	navigate: NavigationStateManager["navigate"];
	beginNavigation: NavigationStateManager["beginNavigation"];
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	submit: <T = any>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<
		{ success: true; data: T } | { success: false; error: string }
	>;
	removeNavigation: NavigationStateManager["removeNavigation"];
	getNavigation: NavigationStateManager["getNavigation"];
	hasNavigation: NavigationStateManager["hasNavigation"];
	getNavigationsSize: NavigationStateManager["getNavigationsSize"];
	getNavigations: NavigationStateManager["getNavigations"];
	getStatus: () => StatusEventDetail;
	clearAll: () => void;
};

export function createRuntimeAPISurface(
	input: CreateRuntimeAPISurfaceInput,
): NavigationStateManager {
	const {
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
	} = input;

	return {
		_submissions: submissions,
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
	};
}
