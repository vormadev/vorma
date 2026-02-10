import { panic } from "../utils/errors.ts";
import type { HeadEl } from "../vorma_ctx/vorma_ctx.ts";
import { createElementFingerprint } from "./head_element_fingerprint.ts";

export function buildDedupedElementsFromBlocks(
	blocks: Array<HeadEl>,
): Array<Element> {
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

export function buildCurrentElementsMap(
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

export function matchElementsByFingerprint(
	newElements: Array<Element>,
	currentElementsMap: Map<string, Array<Element>>,
): { finalElements: Array<Element>; usedCurrentElements: Set<Element> } {
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

	return { finalElements, usedCurrentElements };
}
