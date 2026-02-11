import { navigationStateManager } from "./client.ts";
import { hasSameDataTarget } from "./navigation_runtime/url_identity.ts";

function findIdlePrefetchNavigationByDataTarget(targetHref: string) {
	const exact = navigationStateManager.getNavigation(targetHref);
	if (exact && exact.type === "prefetch" && exact.intent === "none") {
		return exact;
	}

	for (const nav of navigationStateManager.getNavigations().values()) {
		if (
			nav.type === "prefetch" &&
			nav.intent === "none" &&
			hasSameDataTarget(nav.targetUrl, targetHref)
		) {
			return nav;
		}
	}

	return undefined;
}

export function hasIdlePrefetchNavigation(targetHref: string): boolean {
	return !!findIdlePrefetchNavigationByDataTarget(targetHref);
}

export async function startPrefetchNavigation(props: {
	targetHref: string;
	state?: unknown;
}): Promise<void> {
	await navigationStateManager.navigate({
		href: props.targetHref,
		navigationType: "prefetch",
		state: props.state,
	});
}

export function abortIdlePrefetchNavigation(targetHref: string): void {
	const nav = findIdlePrefetchNavigationByDataTarget(targetHref);
	if (nav) {
		nav.control.abortController?.abort();
		navigationStateManager.removeNavigation(nav.targetUrl);
	}
}
