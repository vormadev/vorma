import type { StatusEventDetail } from "../events.ts";
import type { RedirectData } from "../redirects/redirects.ts";
import type { ScrollState } from "../scroll_state_manager.ts";
import type { ClientLoadersResult } from "../client_loaders.ts";
import type { GetRouteDataOutput } from "../vorma_ctx/vorma_ctx.ts";

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
			cssBundlePromises: Array<Promise<any>>;
			waitFnPromise: Promise<ClientLoadersResult> | undefined;
			props: NavigateProps;
	  };

export type NavigationControl = {
	abortController: AbortController | undefined;
	promise: Promise<NavigationOutcome>;
};

export type NavigationPhase =
	| "fetching"
	| "waiting"
	| "rendering"
	| "complete";

export type NavigationIntent = "none" | "navigate" | "revalidate";

export type NavigationEntry = {
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
	control: {
		abortController: AbortController | undefined;
		promise: Promise<any>;
	};
	startTime: number;
	skipGlobalLoadingIndicator?: boolean;
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
	submit: <T = any>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: SubmitOptions,
	) => Promise<{ success: true; data: T } | { success: false; error: string }>;
	removeNavigation: (key: string) => void;
	getNavigation: (key: string) => NavigationEntry | undefined;
	hasNavigation: (key: string) => boolean;
	getNavigationsSize: () => number;
	getNavigations: () => Map<string, NavigationEntry>;
	getStatus: () => StatusEventDetail;
	clearAll: () => void;
};
