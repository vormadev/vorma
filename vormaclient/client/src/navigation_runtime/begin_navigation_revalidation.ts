import type { BeginNavigationContext } from "./begin_navigation.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";
import { hasSameDataTarget } from "./url_identity.ts";

export function beginRevalidation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	const {
		getPendingRevalidation,
		setPendingRevalidation,
		revalidationCoalesceMS,
		createRevalidation,
	} = context;
	const currentUrl = window.location.href;
	const pendingRevalidation = getPendingRevalidation();

	// Coalesce recent revalidations
	if (
		pendingRevalidation &&
		hasSameDataTarget(pendingRevalidation.targetUrl, currentUrl) &&
		Date.now() - pendingRevalidation.startTime < revalidationCoalesceMS
	) {
		return pendingRevalidation.control;
	}

	// Abort existing revalidation
	if (pendingRevalidation) {
		pendingRevalidation.control.abortController?.abort();
		setPendingRevalidation(null);
	}

	return createRevalidation({ ...props, href: currentUrl });
}
