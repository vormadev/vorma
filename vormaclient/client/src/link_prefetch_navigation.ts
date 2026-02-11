import { navigationStateManager } from "./client.ts";

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
	const nav = navigationStateManager.getNavigation(targetHref);
	if (nav && nav.type === "prefetch" && nav.intent === "none") {
		nav.control.abortController?.abort();
		navigationStateManager.removeNavigation(targetHref);
	}
}
