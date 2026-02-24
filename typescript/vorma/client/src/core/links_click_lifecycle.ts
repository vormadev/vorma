import { getAnchorDetailsFromEvent } from "vorma/kit/url";
import { navigationStateManager } from "../client.ts";
import { logError } from "../platform/safety.ts";
import {
	classifyNavigationTargetAgainstCurrentLocation,
	type NavigationTargetClassificationAgainstCurrentLocation,
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

export type EligibleAnchorTargetClassification =
	NavigationTargetClassificationAgainstCurrentLocation;

export function classifyEligibleAnchorTarget(
	anchorDetails: EligibleInternalAnchorDetails,
): EligibleAnchorTargetClassification {
	return classifyNavigationTargetAgainstCurrentLocation({
		targetHref: anchorDetails.anchor.href,
		currentHref: window.location.href,
	});
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

export async function navigateEligibleInternalAnchorClick<E extends Event>(
	props: LinkOnClickCallbacks<E> & {
		event: E;
		anchorDetails: EligibleInternalAnchorDetails;
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
		shouldRunBeforeBegin?: boolean;
	},
): Promise<void> {
	const {
		event,
		anchorDetails,
		beforeBegin,
		beforeRender,
		afterRender,
		scrollToTop,
		replace,
		state,
		shouldRunBeforeBegin = true,
	} = props;
	const targetType = classifyEligibleAnchorTarget(anchorDetails);
	const isCrossDocumentNavigation = targetType === "navigate";

	event.preventDefault();

	if (isCrossDocumentNavigation && shouldRunBeforeBegin) {
		await beforeBegin?.(event);
	}
	if (isCrossDocumentNavigation) {
		await beforeRender?.(event);
	}

	try {
		const navigationResult = await navigationStateManager.navigate({
			href: anchorDetails.anchor.href,
			navigationType: "userNavigation",
			scrollToTop,
			replace,
			state,
		});
		if (isCrossDocumentNavigation && navigationResult.didNavigate) {
			await afterRender?.(event);
		}
	} catch (error) {
		logError("Link navigation failed", error);
	}
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

		await navigateEligibleInternalAnchorClick({
			event,
			anchorDetails,
			beforeBegin: callbacks.beforeBegin,
			beforeRender: callbacks.beforeRender,
			afterRender: callbacks.afterRender,
			scrollToTop: callbacks.scrollToTop,
			replace: callbacks.replace,
			state: callbacks.state,
		});
	};
}
