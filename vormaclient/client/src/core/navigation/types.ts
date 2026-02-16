import type { GetRouteDataOutput } from "../../app/context.ts";
import type { StatusEventDetail } from "../../platform/events.ts";
import type { ScrollState } from "../../platform/scroll.ts";
import type { RedirectData } from "../redirects.ts";
import type { ClientLoadersResult } from "../render_runtime.ts";
import type { ServerSuccessPreloadCommand } from "./fetch_route_data_preload_commands.ts";

export type VormaNavigationType =
	| "browserHistory"
	| "userNavigation"
	| "revalidation"
	| "redirect"
	| "prefetch"
	| "action";

export type NavigateProps = {
	href: string;
	state?: unknown;
	navigationType: VormaNavigationType;
	scrollStateToRestore?: ScrollState;
	replace?: boolean;
	redirectCount?: number;
	scrollToTop?: boolean;
};

export type NavigationOutcome =
	| { type: "aborted" }
	| { type: "redirect"; redirectData: RedirectData; props: NavigateProps }
	| {
			type: "success";
			response: Response;
			json: GetRouteDataOutput;
			preloadCommands: ServerSuccessPreloadCommand[];
			waitFnPromise: Promise<ClientLoadersResult> | undefined;
			props: NavigateProps;
	  };

export type NavigationControl = {
	abortController: AbortController | undefined;
	promise: Promise<NavigationOutcome>;
	operationID?: number;
};

export type NavigationPhase = "fetching" | "waiting" | "rendering" | "complete";

export type NavigationIntent = "none" | "navigate" | "revalidate";

export type NavigationLane =
	| "active"
	| "revalidation"
	| "prefetch"
	| "submission";

export type NavigationEntry = {
	operationID: number;
	control: NavigationControl;
	type: VormaNavigationType;
	intent: NavigationIntent;
	phase: NavigationPhase;
	startTime: number;
	targetUrl: string;
	originUrl: string;
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

export type SubmissionEntry = {
	operationID: number;
	control: {
		abortController: AbortController | undefined;
		promise: Promise<unknown>;
	};
	startTime: number;
	skipGlobalLoadingIndicator?: boolean;
};

export type NavigationDebugJournalEntry = {
	timestampMS: number;
	operationID: number | null;
	lane: NavigationLane;
	targetUrl: string;
	fromState: string;
	toState: string;
	reason: string;
	causedByOperationID: number | null;
};

export type SubmitOptions = {
	dedupeKey?: string;
	revalidate?: boolean;
	skipGlobalLoadingIndicator?: boolean;
};

export type NavigationStateManager = {
	_submissions: Map<string | symbol, SubmissionEntry>;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
	beginNavigation: (props: NavigateProps) => NavigationControl;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	submit: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<
		{ success: true; data: T } | { success: false; error: string }
	>;
	removeNavigation: (key: string) => void;
	getNavigation: (key: string) => NavigationEntry | undefined;
	hasNavigation: (key: string) => boolean;
	getNavigationsSize: () => number;
	getNavigations: () => Map<string, NavigationEntry>;
	getStatus: () => StatusEventDetail;
	getDebugJournal: () => ReadonlyArray<NavigationDebugJournalEntry>;
	clearDebugJournal: () => void;
	clearAll: () => void;
};

export function hasNavigationOperationOwnership(props: {
	entry: NavigationEntry | null | undefined;
	expectedOperationID: number | undefined;
}): props is { entry: NavigationEntry; expectedOperationID: number } {
	return (
		!!props.entry &&
		typeof props.expectedOperationID === "number" &&
		props.entry.operationID === props.expectedOperationID
	);
}

export function hasSubmissionOperationOwnership(props: {
	entry: SubmissionEntry | null | undefined;
	expectedOperationID: number | undefined;
}): props is { entry: SubmissionEntry; expectedOperationID: number } {
	return (
		!!props.entry &&
		typeof props.expectedOperationID === "number" &&
		props.entry.operationID === props.expectedOperationID
	);
}
