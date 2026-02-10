import {
	buildCurrentElementsMap,
	matchElementsByFingerprint,
} from "./head_element_candidates.ts";

export type ReconciledHeadElements = {
	finalElements: Array<Element>;
	usedCurrentElements: Set<Element>;
};

export function reconcileHeadElements(
	currentElements: Array<Element>,
	newElements: Array<Element>,
): ReconciledHeadElements {
	const currentElementsMap = buildCurrentElementsMap(currentElements);
	return matchElementsByFingerprint(newElements, currentElementsMap);
}

export function removeStaleManagedNodes(
	parent: Node,
	currentNodes: Array<Node>,
	usedCurrentElements: Set<Element>,
): void {
	for (const currentNode of currentNodes) {
		if (currentNode.nodeType !== Node.ELEMENT_NODE) {
			parent.removeChild(currentNode);
			continue;
		}

		const currentElement = currentNode as Element;
		if (!usedCurrentElements.has(currentElement)) {
			parent.removeChild(currentElement);
		}
	}
}

export function placeReconciledHeadElements(
	parent: Node,
	startComment: Comment,
	endComment: Comment,
	finalElements: Array<Element>,
	usedCurrentElements: Set<Element>,
): void {
	let lastProcessedElement: Element | null = null;

	for (let i = 0; i < finalElements.length; i++) {
		const element = finalElements[i];
		if (!element) {
			continue;
		}
		const isExistingElement = usedCurrentElements.has(element);

		if (isExistingElement) {
			// Check if this element is already in the correct position
			const nextElementInDOM = (
				lastProcessedElement
					? lastProcessedElement.nextElementSibling
					: startComment.nextElementSibling
			) as Element | null;

			if (nextElementInDOM !== element) {
				// Element exists but is in the wrong position, move it
				parent.insertBefore(element, nextElementInDOM || endComment);
			}

			lastProcessedElement = element;
		} else {
			// This is a new element, insert it
			const insertBefore = lastProcessedElement
				? lastProcessedElement.nextSibling
				: startComment.nextSibling;

			parent.insertBefore(element, insertBefore || endComment);
			lastProcessedElement = element;
		}
	}
}
