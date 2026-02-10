import type { NavigationEntry } from "./types.ts";

export function isStaleRevalidationEntry(entry: NavigationEntry): boolean {
	return (
		entry.type === "revalidation" &&
		window.location.href !== entry.originUrl
	);
}
