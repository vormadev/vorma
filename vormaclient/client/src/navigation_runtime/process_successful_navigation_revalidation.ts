import type { NavigationEntry } from "./types.ts";
import { hasSameDataTarget } from "./url_identity.ts";

export function isStaleRevalidationEntry(entry: NavigationEntry): boolean {
	return (
		entry.type === "revalidation" &&
		!hasSameDataTarget(window.location.href, entry.originUrl)
	);
}
