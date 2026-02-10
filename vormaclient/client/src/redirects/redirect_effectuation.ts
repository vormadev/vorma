import type {
	NavigateProps,
	NavigationEntry,
} from "../navigation_runtime/types.ts";
import { VORMA_HARD_RELOAD_QUERY_PARAM } from "../hard_reload.ts";
import type { RedirectData } from "./redirects.ts";

type RedirectNavigationState = {
	getNavigations: () => Map<string, NavigationEntry>;
	removeNavigation: (key: string) => void;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

type ShouldRedirectData = Extract<RedirectData, { status: "should" }>;

function toDidRedirectData(redirectData: ShouldRedirectData): RedirectData {
	return {
		hrefDetails: redirectData.hrefDetails,
		status: "did",
		href: redirectData.href,
	};
}

export function cleanupRedirectRelatedNavigations(
	navigationState: RedirectNavigationState,
): void {
	const navEntries = navigationState.getNavigations().entries();
	for (const [key, nav] of navEntries) {
		if (nav.type === "redirect" || nav.type === "revalidation") {
			nav.control.abortController?.abort();
			navigationState.removeNavigation(key);
		}
	}
}

export function effectuateHardRedirect(
	redirectData: ShouldRedirectData,
): RedirectData | null {
	if (!redirectData.hrefDetails.isHTTP) {
		return null;
	}

	if (redirectData.hrefDetails.isExternal) {
		window.location.href = redirectData.href;
	} else {
		const url = new URL(redirectData.href, window.location.href);
		url.searchParams.set(
			VORMA_HARD_RELOAD_QUERY_PARAM,
			redirectData.latestBuildID,
		);
		window.location.href = url.href;
	}

	return toDidRedirectData(redirectData);
}

export async function effectuateSoftRedirect(
	navigationState: RedirectNavigationState,
	redirectData: ShouldRedirectData,
	redirectCount: number,
	originalProps?: NavigateProps,
): Promise<RedirectData> {
	await navigationState.navigate({
		href: redirectData.href,
		navigationType: "redirect",
		redirectCount: redirectCount + 1,
		state: originalProps?.state,
		replace: originalProps?.replace,
		scrollToTop: originalProps?.scrollToTop,
	});

	return toDidRedirectData(redirectData);
}
