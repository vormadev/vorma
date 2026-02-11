// READY_TO_DELETE_AFTER_SIGNOFF
import { expect, it, vi } from "vitest";
import { panic } from "../../../platform/safety.ts";
import type { HeadEl } from "../../../app/context.ts";
import {
	getStartAndEndComments,
	updateHeadEls,
} from "../../../ui/head.ts";
import { describeUpdateHeadElsSuite } from "../head.test.helpers.ts";

vi.mock("../../../platform/safety.ts", async () => {
	const { createHeadErrorsMock } =
		await import("../head.test.helpers.ts");
	return createHeadErrorsMock();
});

describeUpdateHeadElsSuite(() => {
	it('should update the "rest" section correctly', () => {
		const blocks: Array<HeadEl> = [
			{
				tag: "script",
				attributesKnownSafe: { src: "/script.js" },
			},
		];

		updateHeadEls("rest", blocks);

		const scriptEl = document.head.querySelector("script");
		expect(scriptEl).not.toBeNull();
		if (!scriptEl) {
			throw new Error("Script element not found");
		}
		expect(scriptEl.getAttribute("src")).toBe("/script.js");

		// Verify script is between rest comments
		const startComment = Array.from(document.head.childNodes).find(
			(node) => {
				return (
					node.nodeType === 8 &&
					(node as Comment).data.trim() === 'data-vorma="rest-start"'
				);
			},
		);
		const endComment = Array.from(document.head.childNodes).find((node) => {
			return (
				node.nodeType === 8 &&
				(node as Comment).data.trim() === 'data-vorma="rest-end"'
			);
		});

		expect(startComment).toBeDefined();
		expect(endComment).toBeDefined();

		let foundScriptBetweenComments = false;
		let elementCountBetweenComments = 0;
		let currentNode = startComment?.nextSibling ?? null;

		while (currentNode && currentNode !== endComment) {
			if (currentNode.nodeType === Node.ELEMENT_NODE) {
				elementCountBetweenComments++;
				if (currentNode === scriptEl) {
					foundScriptBetweenComments = true;
				}
			}
			currentNode = currentNode.nextSibling;
		}

		expect(foundScriptBetweenComments).toBe(true);
		expect(elementCountBetweenComments).toBe(1);
	});

	it("should handle text nodes between comments", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const textNode = document.createTextNode("\n  ");
		document.head.insertBefore(textNode, comments.endComment);

		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Test Description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		const metaEl = document.head.querySelector("meta");
		expect(metaEl).not.toBeNull();
		if (!metaEl) {
			throw new Error("Meta element not found");
		}
		expect(metaEl.getAttribute("name")).toBe("description");

		// Verify text nodes are removed
		const headChildren = Array.from(document.head.childNodes);
		const metaStartIndex = headChildren.findIndex((node) => {
			return (
				node.nodeType === 8 &&
				(node as Comment).data === 'data-vorma="meta-start"'
			);
		});
		const metaEndIndex = headChildren.findIndex((node) => {
			return (
				node.nodeType === 8 &&
				(node as Comment).data === 'data-vorma="meta-end"'
			);
		});

		const nodesBetweenComments = headChildren.slice(
			metaStartIndex + 1,
			metaEndIndex,
		);
		const hasTextNodes = nodesBetweenComments.some((node) => {
			return node.nodeType === Node.TEXT_NODE;
		});
		expect(hasTextNodes).toBe(false);
	});

	it("should not duplicate elements on multiple updates", () => {
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Test Description",
				},
			},
		];

		updateHeadEls("meta", blocks);
		updateHeadEls("meta", blocks); // Call twice

		expect(document.head.querySelectorAll("meta").length).toBe(1);
	});

	it("should call Panic when attribute value is null", () => {
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: null as unknown as string,
				},
			},
		];

		expect(() => updateHeadEls("meta", blocks)).toThrow();
		expect(panic).toHaveBeenCalled();
	});

	it("should not process undefined tags", () => {
		const blocks: Array<HeadEl> = [
			{
				tag: undefined,
				attributesKnownSafe: {
					name: "description",
					content: "Test Description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		expect(document.head.querySelectorAll("*").length).toBe(0);
	});

	it("should create consistent fingerprints for elements", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const initialMeta = document.createElement("meta");
		initialMeta.setAttribute("content", "Test Description"); // Different order from blocks
		initialMeta.setAttribute("name", "description");
		document.head.insertBefore(initialMeta, comments.endComment);

		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description", // Different order from DOM element
					content: "Test Description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		const metaElements = document.head.querySelectorAll("meta");
		expect(metaElements.length).toBe(1);
		expect(metaElements[0]).toBe(initialMeta); // Same DOM reference
	});

	it("should handle complex scenario with adds, updates, removes and reordering", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create initial elements
		const meta1 = document.createElement("meta");
		meta1.setAttribute("name", "description");
		meta1.setAttribute("content", "Initial Description");

		const meta2 = document.createElement("meta");
		meta2.setAttribute("name", "keywords");
		meta2.setAttribute("content", "test, vitest");

		const link1 = document.createElement("link");
		link1.setAttribute("rel", "canonical");
		link1.setAttribute("href", "/initial-page");

		// Insert initial elements between comments
		document.head.insertBefore(link1, comments.endComment);
		document.head.insertBefore(meta2, link1);
		document.head.insertBefore(meta1, meta2);

		// Define blocks for update
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "keywords",
					content: "test, vitest",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Updated Description",
				},
			},
			{
				tag: "link",
				attributesKnownSafe: { rel: "stylesheet", href: "/styles.css" },
			},
		];

		updateHeadEls("meta", blocks);

		// Get elements between comments after update
		const elementsBetweenComments: Array<Element> = [];
		let current: Node | null = comments.startComment.nextSibling;
		while (current && current !== comments.endComment) {
			if (current.nodeType === Node.ELEMENT_NODE) {
				elementsBetweenComments.push(current as Element);
			}
			current = current.nextSibling;
		}

		// Verify count and order
		expect(elementsBetweenComments.length).toBe(3);
		expect(elementsBetweenComments[0]?.tagName?.toLowerCase()).toBe("meta");
		expect(elementsBetweenComments[1]?.tagName?.toLowerCase()).toBe("meta");
		expect(elementsBetweenComments[2]?.tagName?.toLowerCase()).toBe("link");

		// Verify content
		expect(elementsBetweenComments[0]?.getAttribute("name")).toBe(
			"keywords",
		);
		expect(elementsBetweenComments[0]?.getAttribute("content")).toBe(
			"test, vitest",
		);

		expect(elementsBetweenComments[1]?.getAttribute("name")).toBe(
			"description",
		);
		expect(elementsBetweenComments[1]?.getAttribute("content")).toBe(
			"Updated Description",
		);

		expect(elementsBetweenComments[2]?.getAttribute("rel")).toBe(
			"stylesheet",
		);
		expect(elementsBetweenComments[2]?.getAttribute("href")).toBe(
			"/styles.css",
		);

		// Verify canonical link is removed
		expect(
			document.head.querySelectorAll('link[rel="canonical"]').length,
		).toBe(0);
	});
});
