import { panic } from "../platform/safety.ts";
import type { HeadEl } from "../app/context.ts";

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

function buildDedupedElementsFromBlocks(blocks: Array<HeadEl>): Array<Element> {
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

	return newElements;
}

function buildCurrentElementsMap(
	currentElements: Array<Element>,
): Map<string, Array<Element>> {
	const currentElementsMap = new Map<string, Array<Element>>();

	for (const el of currentElements) {
		const fingerprint = createElementFingerprint(el);
		if (!currentElementsMap.has(fingerprint)) {
			currentElementsMap.set(fingerprint, []);
		}
		currentElementsMap.get(fingerprint)?.push(el);
	}

	return currentElementsMap;
}

function reconcileHeadElements(
	currentElements: Array<Element>,
	newElements: Array<Element>,
): { finalElements: Array<Element>; usedCurrentElements: Set<Element> } {
	const currentElementsMap = buildCurrentElementsMap(currentElements);
	const finalElements: Array<Element> = [];
	const usedCurrentElements = new Set<Element>();

	for (const newEl of newElements) {
		const fingerprint = createElementFingerprint(newEl);
		const matchingCurrentElementsList =
			currentElementsMap.get(fingerprint) || [];

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

	return { finalElements, usedCurrentElements };
}

function removeStaleManagedNodes(
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

function placeReconciledHeadElements(
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
			const nextElementInDOM = (
				lastProcessedElement
					? lastProcessedElement.nextElementSibling
					: startComment.nextElementSibling
			) as Element | null;

			if (nextElementInDOM !== element) {
				parent.insertBefore(element, nextElementInDOM || endComment);
			}

			lastProcessedElement = element;
		} else {
			const insertBefore = lastProcessedElement
				? lastProcessedElement.nextSibling
				: startComment.nextSibling;

			parent.insertBefore(element, insertBefore || endComment);
			lastProcessedElement = element;
		}
	}
}

export function updateHeadEls(type: "meta" | "rest", blocks: Array<HeadEl>) {
	const { startComment, endComment } = getStartAndEndComments(type);
	if (!startComment || !endComment || !endComment.parentNode) {
		return;
	}
	const parent = endComment.parentNode;

	const currentNodes: Array<Node> = [];
	let nodePtr = startComment.nextSibling;
	while (nodePtr != null && nodePtr !== endComment) {
		currentNodes.push(nodePtr);
		nodePtr = nodePtr.nextSibling;
	}
	const currentElements = currentNodes.filter(
		(node): node is Element => node.nodeType === Node.ELEMENT_NODE,
	);

	const newElements = buildDedupedElementsFromBlocks(blocks);
	const { finalElements, usedCurrentElements } = reconcileHeadElements(
		currentElements,
		newElements,
	);
	removeStaleManagedNodes(parent, currentNodes, usedCurrentElements);
	placeReconciledHeadElements(
		parent,
		startComment,
		endComment,
		finalElements,
		usedCurrentElements,
	);
}
