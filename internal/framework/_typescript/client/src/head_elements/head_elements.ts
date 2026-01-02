import { panic } from "../utils/errors.ts";
import type { HeadEl } from "../vorma_ctx/vorma_ctx.ts";

export function getStartAndEndComments(type: "meta" | "rest"): {
	startComment: Comment | null;
	endComment: Comment | null;
} {
	const startMarker = `data-vorma="${type}-start"`;
	const endMarker = `data-vorma="${type}-end"`;
	const start = findComment(startMarker);
	const end = findComment(endMarker);
	return { startComment: start, endComment: end };
}

function findComment(matchingText: string): Comment | null {
	const walker = document.createTreeWalker(
		document.head,
		NodeFilter.SHOW_COMMENT,
		{
			acceptNode(node: Comment) {
				return node.nodeValue?.trim() === matchingText.trim()
					? NodeFilter.FILTER_ACCEPT
					: NodeFilter.FILTER_REJECT;
			},
		},
	);
	return walker.nextNode() as Comment | null;
}

export function updateHeadEls(type: "meta" | "rest", blocks: Array<HeadEl>) {
	const { startComment, endComment } = getStartAndEndComments(type);
	if (!startComment || !endComment || !endComment.parentNode) {
		return;
	}
	const parent = endComment.parentNode;

	// Collect all current nodes between start and end comments
	const currentNodes: Array<Node> = [];
	let nodePtr = startComment.nextSibling;
	while (nodePtr != null && nodePtr !== endComment) {
		currentNodes.push(nodePtr);
		nodePtr = nodePtr.nextSibling;
	}
	const currentElements = currentNodes.filter(
		(node): node is Element => node.nodeType === Node.ELEMENT_NODE,
	);

	// Create new elements from blocks
	const newElements: Array<Element> = [];
	const newElementFingerprints = new Map<string, Element>();
	for (const block of blocks) {
		if (!block.tag) {
			continue;
		}
		const newEl = document.createElement(block.tag);
		if (block.attributesKnownSafe) {
			for (const key of Object.keys(block.attributesKnownSafe)) {
				const value = block.attributesKnownSafe[key];
				if (value === null || value === undefined) {
					panic(
						`Attribute value for '${key}' in tag '${block.tag}' cannot be null or undefined.`,
					);
				}
				newEl.setAttribute(key, value);
			}
		}
		if (block.booleanAttributes) {
			for (const key of block.booleanAttributes) {
				newEl.setAttribute(key, "");
			}
		}
		if (block.dangerousInnerHTML) {
			newEl.innerHTML = block.dangerousInnerHTML;
		}

		const fingerprint = createElementFingerprint(newEl);
		if (newElementFingerprints.has(fingerprint)) {
			const elementToRemove = newElementFingerprints.get(fingerprint);
			if (elementToRemove) {
				const indexToRemove = newElements.indexOf(elementToRemove);
				if (indexToRemove > -1) {
					newElements.splice(indexToRemove, 1);
				}
			}
		}
		newElements.push(newEl);
		newElementFingerprints.set(fingerprint, newEl);
	}

	// Build a map of current elements by fingerprint
	const currentElementsMap = new Map<string, Array<Element>>();
	for (const el of currentElements) {
		const fingerprint = createElementFingerprint(el);
		if (!currentElementsMap.has(fingerprint)) {
			currentElementsMap.set(fingerprint, []);
		}
		currentElementsMap.get(fingerprint)?.push(el);
	}

	// Match new elements with existing ones when possible
	const finalElements: Array<Element> = [];
	const usedCurrentElements = new Set<Element>();

	for (const newEl of newElements) {
		const fingerprint = createElementFingerprint(newEl);
		const matchingCurrentElementsList =
			currentElementsMap.get(fingerprint) || [];

		// Find the first matching element that hasn't been used yet
		const matchingElement = matchingCurrentElementsList.find(
			(el) => !usedCurrentElements.has(el),
		);

		if (matchingElement) {
			usedCurrentElements.add(matchingElement);
			finalElements.push(matchingElement);
		} else {
			finalElements.push(newEl);
		}
	}

	// Create a map to track which elements are in the correct position
	// and which need to be moved or added
	const desiredPositions = new Map<Element, number>();
	finalElements.forEach((el, index) => {
		desiredPositions.set(el, index);
	});

	// Track elements still in the DOM
	const remainingCurrentElements = new Set(currentElements);

	// First pass: remove elements that are no longer needed
	for (const currentElement of currentElements) {
		if (!usedCurrentElements.has(currentElement)) {
			parent.removeChild(currentElement);
			remainingCurrentElements.delete(currentElement);
		}
	}

	// Second pass: position elements in the correct order with minimal DOM operations
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

			// Mark as processed
			remainingCurrentElements.delete(element);
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

function createElementFingerprint(element: Element): string {
	const attributes: Array<string> = [];
	for (let i = 0; i < element.attributes.length; i++) {
		const attr = element.attributes[i];
		if (!attr) {
			continue;
		}
		const value =
			element.hasAttribute(attr.name) && attr.value === ""
				? ""
				: attr.value;
		attributes.push(`${attr.name}="${value}"`);
	}
	attributes.sort();
	return `${element.tagName.toUpperCase()}|${attributes.join(",")}|${(element.innerHTML || "").trim()}`;
}
