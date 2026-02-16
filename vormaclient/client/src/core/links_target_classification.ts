import { getAnchorDetailsFromEvent } from "vorma/kit/url";
import {
	isSameDocumentHashChange,
	isSameDocumentLocation,
} from "../platform/url.ts";

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
