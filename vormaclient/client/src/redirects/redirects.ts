import { type HrefDetails } from "vorma/kit/url";
import type { NavigateProps } from "../navigation_runtime/types.ts";
import { getNavigationStateAccess } from "../navigation_state_access.ts";
import {
	cleanupRedirectRelatedNavigations,
	effectuateHardRedirect,
	effectuateSoftRedirect,
} from "./redirect_effectuation.ts";
import { executeRedirectRequestFlow } from "./redirect_request_flow.ts";
import { parseResponseForRedirectData } from "./redirect_response_parsing.ts";

export type RedirectData = { href: string; hrefDetails: HrefDetails } & (
	| {
			status: "did";
	  }
	| {
			status: "should";
			shouldRedirectStrategy: "hard" | "soft";
			latestBuildID: string;
	  }
);

export function getBuildIDFromResponse(response: Response | undefined) {
	return response?.headers.get("X-Vorma-Build-Id") || "";
}

function parseFetchResponseForRedirectData(res: Response): RedirectData | null {
	const latestBuildID = getBuildIDFromResponse(res);
	return parseResponseForRedirectData(res, latestBuildID);
}

export async function effectuateRedirectDataResult(
	redirectData: RedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData | null> {
	if (redirectData.status !== "should") {
		return null;
	}

	// Clean up any active redirect or revalidations when redirecting.
	// Otherwise loading state will get stuck.
	const navigationState = getNavigationStateAccess();
	cleanupRedirectRelatedNavigations(navigationState);

	if (redirectData.shouldRedirectStrategy === "hard") {
		return effectuateHardRedirect(redirectData);
	}

	if (redirectData.shouldRedirectStrategy === "soft") {
		return effectuateSoftRedirect(
			navigationState,
			redirectData,
			redirectCount,
			originalProps,
		);
	}

	return null;
}

export async function handleRedirects(props: {
	abortController: AbortController;
	url: URL;
	requestInit?: RequestInit;
	isPrefetch?: boolean;
	redirectCount?: number;
}): Promise<{ redirectData: RedirectData | null; response?: Response }> {
	const requestFlow = await executeRedirectRequestFlow(props);
	if (requestFlow.kind === "too_many_redirects") {
		return { redirectData: null, response: undefined };
	}

	const redirectData = parseFetchResponseForRedirectData(
		requestFlow.response,
	);
	return { redirectData, response: requestFlow.response };
}
