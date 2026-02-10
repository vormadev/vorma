import { handleNavigationOutcome } from "./handle_navigation_outcome.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
} from "./types.ts";

export type BeginNavigationHandlers = {
	beginUserNavigation: (
		props: NavigateProps,
		targetUrl: string,
	) => NavigationControl;
	beginPrefetch: (
		props: NavigateProps,
		targetUrl: string,
	) => NavigationControl;
	beginRevalidation: (props: NavigateProps) => NavigationControl;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
};

export function beginNavigationWithHandlers(
	handlers: BeginNavigationHandlers,
	props: NavigateProps,
): NavigationControl {
	const targetUrl = new URL(props.href, window.location.href).href;

	switch (props.navigationType) {
		case "userNavigation":
			return handlers.beginUserNavigation(props, targetUrl);
		case "prefetch":
			return handlers.beginPrefetch(props, targetUrl);
		case "revalidation":
			return handlers.beginRevalidation(props);
		case "browserHistory":
		case "redirect":
		default:
			return handlers.createActiveNavigation(props, "navigate");
	}
}

export type NavigateHandlers = {
	beginNavigation: (props: NavigateProps) => NavigationControl;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
};

export async function navigateWithHandlers(
	handlers: NavigateHandlers,
	props: NavigateProps,
): Promise<{ didNavigate: boolean }> {
	const control = handlers.beginNavigation(props);

	try {
		const outcome = await control.promise;

		return await handleNavigationOutcome(
			{
				findNavigationEntry: handlers.findNavigationEntry,
				deleteNavigation: handlers.deleteNavigation,
				processSuccessfulNavigation:
					handlers.processSuccessfulNavigation,
				onNavigationIntentResolved: handlers.onNavigationIntentResolved,
			},
			props,
			outcome,
		);
	} catch {
		const targetUrl = new URL(props.href, window.location.href).href;
		handlers.deleteNavigation(targetUrl);
		return { didNavigate: false };
	}
}
