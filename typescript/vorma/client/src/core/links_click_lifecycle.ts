import { getAnchorDetailsFromEvent, resolveAbsoluteHref } from "vorma/kit/url";
import { navigationStateManager } from "../client.ts";
import {
	hasNavigationOperationOwnership,
	type NavigationControl,
	type NavigationOutcome,
} from "./navigation/types.ts";
import {
	effectuateRedirectDataResult,
	syncBuildIDFromRedirectData,
} from "./redirects.ts";
import { saveScrollState } from "../platform/scroll.ts";
import { logError } from "../platform/safety.ts";
import {
	isSameDocumentHashChange,
	isSameDocumentLocation,
} from "../platform/url.ts";

type LinkOnClickCallback<E extends Event> = (event: E) => void | Promise<void>;

export type LinkOnClickCallbacks<E extends Event> = {
	beforeBegin?: LinkOnClickCallback<E>;
	beforeRender?: LinkOnClickCallback<E>;
	afterRender?: LinkOnClickCallback<E>;
};

export type LinkLifecycleCallbacks<E extends Event> = {
	beforeRender?: (event: E) => void | Promise<void>;
	afterRender?: (event: E) => void | Promise<void>;
};

export type ClickNavigationOptions = {
	scrollToTop?: boolean;
	replace?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};

export type EligibleInternalAnchorDetails = Exclude<
	ReturnType<typeof getAnchorDetailsFromEvent>,
	null
>;

function isJustAHashChange(
	anchorDetails: EligibleInternalAnchorDetails,
): boolean {
	return isSameDocumentHashChange({
		targetHref: anchorDetails.anchor.href,
		currentHref: window.location.href,
	});
}

function isSameDocumentNoopNavigationTarget(
	anchorDetails: EligibleInternalAnchorDetails,
): boolean {
	return isSameDocumentLocation({
		targetHref: anchorDetails.anchor.href,
		currentHref: window.location.href,
	});
}

export type EligibleAnchorTargetClassification =
	| "same-document-noop"
	| "hash-change"
	| "navigate";

export function classifyEligibleAnchorTarget(
	anchorDetails: EligibleInternalAnchorDetails,
): EligibleAnchorTargetClassification {
	if (isSameDocumentNoopNavigationTarget(anchorDetails)) {
		return "same-document-noop";
	}
	if (isJustAHashChange(anchorDetails)) {
		return "hash-change";
	}
	return "navigate";
}

export function getEligibleInternalAnchorDetails(
	event: Event,
): EligibleInternalAnchorDetails | null {
	if (event.defaultPrevented) return null;

	const anchorDetails = getAnchorDetailsFromEvent(
		event as unknown as MouseEvent,
	);
	if (!anchorDetails) return null;

	if (
		!anchorDetails.isEligibleForDefaultPrevention ||
		!anchorDetails.isInternal
	) {
		return null;
	}

	return anchorDetails;
}

function doesNavigationEntryBelongToControl(props: {
	entry: ReturnType<typeof navigationStateManager.getNavigation>;
	control: NavigationControl;
}): boolean {
	const { entry, control } = props;
	if (!entry) {
		return false;
	}

	return hasNavigationOperationOwnership({
		entry,
		expectedOperationID: control.operationID,
	});
}

function getCurrentNavigationEntryForControl(props: {
	targetUrl: string;
	control: NavigationControl;
}): ReturnType<typeof navigationStateManager.getNavigation> {
	const { targetUrl, control } = props;
	const currentEntry = navigationStateManager.getNavigation(targetUrl);
	if (
		!doesNavigationEntryBelongToControl({
			entry: currentEntry,
			control,
		})
	) {
		return undefined;
	}

	return currentEntry;
}

async function handleLinkNavigationOutcome<E extends Event>(props: {
	event: E;
	outcome: NavigationOutcome;
	targetUrl: string;
	control: NavigationControl;
	callbacks: LinkLifecycleCallbacks<E>;
}): Promise<void> {
	const { event, outcome, targetUrl, control, callbacks } = props;
	const currentEntry = getCurrentNavigationEntryForControl({
		targetUrl,
		control,
	});
	if (!currentEntry) {
		return;
	}

	if (outcome.type === "aborted") {
		navigationStateManager.removeNavigation(targetUrl);
		return;
	}

	if (outcome.type === "redirect") {
		await callbacks.beforeRender?.(event);
		if (
			!getCurrentNavigationEntryForControl({
				targetUrl,
				control,
			})
		) {
			return;
		}

		syncBuildIDFromRedirectData(outcome.redirectData);
		navigationStateManager.removeNavigation(targetUrl);
		const redirectResult = await effectuateRedirectDataResult(
			outcome.redirectData,
			outcome.props.redirectCount || 0,
			outcome.props,
		);
		if (redirectResult?.status !== "did") {
			return;
		}
		await callbacks.afterRender?.(event);
		return;
	}

	await callbacks.beforeRender?.(event);
	const currentEntryAfterBeforeRender = getCurrentNavigationEntryForControl({
		targetUrl,
		control,
	});
	if (!currentEntryAfterBeforeRender) {
		return;
	}

	await navigationStateManager.processSuccessfulNavigation(
		outcome,
		currentEntryAfterBeforeRender,
	);
	if (currentEntryAfterBeforeRender.phase !== "complete") {
		return;
	}
	await callbacks.afterRender?.(event);
}

export function createLinkOnClickFn<E extends Event>(
	callbacks: LinkOnClickCallbacks<E> & {
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
	},
) {
	return async (event: E) => {
		const anchorDetails = getEligibleInternalAnchorDetails(event);
		if (!anchorDetails) return;

		const targetType = classifyEligibleAnchorTarget(anchorDetails);
		if (targetType === "same-document-noop") {
			event.preventDefault();
			return;
		}
		if (targetType === "hash-change") {
			saveScrollState();
			return;
		}

		const { anchor } = anchorDetails;
		event.preventDefault();
		await callbacks.beforeBegin?.(event);

		const control = navigationStateManager.beginNavigation({
			href: anchor.href,
			navigationType: "userNavigation",
			scrollToTop: callbacks.scrollToTop,
			replace: callbacks.replace,
			state: callbacks.state,
		});
		let outcome: NavigationOutcome;
		try {
			outcome = await control.promise;
		} catch (error) {
			const targetUrl = resolveAbsoluteHref({ href: anchor.href });
			const currentEntry =
				navigationStateManager.getNavigation(targetUrl);
			if (
				doesNavigationEntryBelongToControl({
					entry: currentEntry,
					control,
				})
			) {
				navigationStateManager.removeNavigation(targetUrl);
			}
			logError("Link navigation failed", error);
			return;
		}

		const targetUrl = resolveAbsoluteHref({ href: anchor.href });
		await handleLinkNavigationOutcome({
			event,
			outcome,
			targetUrl,
			control,
			callbacks: {
				beforeRender: callbacks.beforeRender,
				afterRender: callbacks.afterRender,
			},
		});
	};
}
