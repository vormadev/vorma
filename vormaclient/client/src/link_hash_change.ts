import {
	isSameDocumentHashChange,
	isSameDocumentLocation,
} from "./hash_fragment.ts";

type HashCheckAnchorDetails =
	| {
			anchor: HTMLAnchorElement;
	  }
	| null
	| undefined;

export function isJustAHashChange(
	anchorDetails: HashCheckAnchorDetails,
): boolean {
	if (!anchorDetails) return false;

	return isSameDocumentHashChange(
		anchorDetails.anchor.href,
		window.location.href,
	);
}

export function isSameDocumentNoopNavigationTarget(
	anchorDetails: HashCheckAnchorDetails,
): boolean {
	if (!anchorDetails) return false;

	return isSameDocumentLocation(
		anchorDetails.anchor.href,
		window.location.href,
	);
}
