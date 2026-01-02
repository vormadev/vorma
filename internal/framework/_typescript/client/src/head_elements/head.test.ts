import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { panic } from "../utils/errors.ts";
import type { HeadEl } from "../vorma_ctx/vorma_ctx.ts";
import { getStartAndEndComments, updateHeadEls } from "./head_elements.ts";

vi.mock("../utils/errors.ts", () => ({
	panic: vi.fn(() => {
		throw new Error("Panic called");
	}),
}));

let dom: JSDOM;

describe("updateHeadEls", () => {
	beforeEach(() => {
		dom = new JSDOM(
			"<!DOCTYPE html><html><head></head><body></body></html>",
			{
				url: "http://localhost/",
			},
		);
		(global as any).window = dom.window as unknown as Window &
			typeof globalThis;
		(global as any).document = dom.window.document;
		(global as any).NodeFilter = {
			SHOW_COMMENT: 128,
			FILTER_ACCEPT: 1,
			FILTER_REJECT: 2,
		};
		(global as any).Node = {
			ELEMENT_NODE: 1,
		};

		const startMetaComment = document.createComment(
			'data-vorma="meta-start"',
		);
		const endMetaComment = document.createComment('data-vorma="meta-end"');
		const startRestComment = document.createComment(
			'data-vorma="rest-start"',
		);
		const endRestComment = document.createComment('data-vorma="rest-end"');

		document.head.appendChild(startMetaComment);
		document.head.appendChild(endMetaComment);
		document.head.appendChild(startRestComment);
		document.head.appendChild(endRestComment);

		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.resetAllMocks();
		dom.window.close();
		(global as any).window = undefined as unknown as Window &
			typeof globalThis;
		(global as any).document = undefined as unknown as Document;
		(global as any).NodeFilter = undefined as unknown as NodeFilter;
	});

	it("should not add any elements when blocks array is empty", () => {
		const blocks: Array<HeadEl> = [];
		const initialChildCount = document.head.childNodes.length;

		updateHeadEls("meta", blocks);

		expect(document.head.childNodes.length).toBe(initialChildCount);
		expect(document.head.querySelector("meta")).toBeNull();
	});

	it("should add elements when none exist", () => {
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Test Description",
				},
			},
			{
				tag: "link",
				attributesKnownSafe: { rel: "stylesheet", href: "/styles.css" },
			},
		];

		updateHeadEls("meta", blocks);

		const metaElements = document.head.querySelectorAll("meta");
		const linkElements = document.head.querySelectorAll("link");

		expect(metaElements.length).toBe(1);
		expect(linkElements.length).toBe(1);

		const metaEl = metaElements[0];
		if (!metaEl) {
			throw new Error("Meta element not found");
		}
		expect(metaEl.getAttribute("name")).toBe("description");
		expect(metaEl.getAttribute("content")).toBe("Test Description");

		const linkEl = linkElements[0];
		if (!linkEl) {
			throw new Error("Link element not found");
		}
		expect(linkEl.getAttribute("rel")).toBe("stylesheet");
		expect(linkEl.getAttribute("href")).toBe("/styles.css");

		// Verify elements are between the correct comments
		const headChildren = Array.from(document.head.childNodes);
		const startMetaIndex = headChildren.findIndex((node) => {
			return (
				node.nodeType === 8 &&
				(node as Comment).data === 'data-vorma="meta-start"'
			);
		});
		const endMetaIndex = headChildren.findIndex((node) => {
			return (
				node.nodeType === 8 &&
				(node as Comment).data === 'data-vorma="meta-end"'
			);
		});

		const elementsBetweenComments = headChildren.slice(
			startMetaIndex + 1,
			endMetaIndex,
		);
		expect(elementsBetweenComments.length).toBe(2);
		expect(elementsBetweenComments[0]).toBe(metaEl);
		expect(elementsBetweenComments[1]).toBe(linkEl);
	});

	it("should update elements that have changed", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const initialMeta = document.createElement("meta");
		initialMeta.setAttribute("name", "description");
		initialMeta.setAttribute("content", "Old Description");
		document.head.insertBefore(initialMeta, comments.endComment);

		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "New Description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		const metaElements = document.head.querySelectorAll("meta");
		expect(metaElements.length).toBe(1);

		const metaEl = metaElements[0];
		if (!metaEl) {
			throw new Error("Meta element not found");
		}
		expect(metaEl.getAttribute("name")).toBe("description");
		expect(metaEl.getAttribute("content")).toBe("New Description");
	});

	it("should remove elements that are no longer needed", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const initialMeta = document.createElement("meta");
		initialMeta.setAttribute("name", "description");
		initialMeta.setAttribute("content", "Old Description");

		const initialLink = document.createElement("link");
		initialLink.setAttribute("rel", "stylesheet");
		initialLink.setAttribute("href", "/styles.css");

		document.head.insertBefore(initialMeta, comments.endComment);
		document.head.insertBefore(initialLink, comments.endComment);

		const blocks: Array<HeadEl> = [
			// Only keep the meta, remove the link
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Old Description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		expect(document.head.querySelectorAll("meta").length).toBe(1);
		expect(document.head.querySelectorAll("link").length).toBe(0);
	});

	it("should keep existing elements that haven't changed", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const initialMeta = document.createElement("meta");
		initialMeta.setAttribute("name", "description");
		initialMeta.setAttribute("content", "Test Description");
		document.head.insertBefore(initialMeta, comments.endComment);

		const originalEl = initialMeta;

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

		const metaElements = document.head.querySelectorAll("meta");
		expect(metaElements.length).toBe(1);
		expect(metaElements[0]).toBe(originalEl);
	});

	it("should reorder elements correctly", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const initialMeta1 = document.createElement("meta");
		initialMeta1.setAttribute("name", "description");
		initialMeta1.setAttribute("content", "Description");

		const initialMeta2 = document.createElement("meta");
		initialMeta2.setAttribute("name", "viewport");
		initialMeta2.setAttribute("content", "width=device-width");

		document.head.insertBefore(initialMeta1, comments.endComment);
		document.head.insertBefore(initialMeta2, comments.endComment);

		// Request blocks in reverse order from current DOM order
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "viewport",
					content: "width=device-width",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		const metaElements = Array.from(document.head.querySelectorAll("meta"));
		expect(metaElements.length).toBe(2);

		if (!metaElements[0] || !metaElements[1]) {
			throw new Error("Meta elements not found");
		}

		// Check the order is now viewport -> description
		expect(metaElements[0].getAttribute("name")).toBe("viewport");
		expect(metaElements[1].getAttribute("name")).toBe("description");
	});

	it("should handle boolean attributes correctly", () => {
		const blocks: Array<HeadEl> = [
			{
				tag: "script",
				attributesKnownSafe: { src: "/script.js" },
				booleanAttributes: ["async", "defer"],
			},
		];

		updateHeadEls("meta", blocks);

		const scriptEl = document.head.querySelector("script");
		expect(scriptEl).not.toBeNull();
		if (!scriptEl) {
			throw new Error("Script element not found");
		}
		expect(scriptEl.getAttribute("src")).toBe("/script.js");
		expect(scriptEl.hasAttribute("async")).toBe(true);
		expect(scriptEl.hasAttribute("defer")).toBe(true);
		expect(scriptEl.getAttribute("async")).toBe("");
		expect(scriptEl.getAttribute("defer")).toBe("");
	});

	it("should handle innerHTML correctly", () => {
		const blocks: Array<HeadEl> = [
			{
				tag: "script",
				dangerousInnerHTML: 'console.log("test");',
			},
		];

		updateHeadEls("meta", blocks);

		const scriptEl = document.head.querySelector("script");
		expect(scriptEl).not.toBeNull();
		if (!scriptEl) {
			throw new Error("Script element not found");
		}
		expect(scriptEl.innerHTML).toBe('console.log("test");');
	});

	it("should handle missing start/end comments gracefully", () => {
		document.head.innerHTML = "";
		const startRestComment = document.createComment(
			'data-vorma="rest-start"',
		);
		const endRestComment = document.createComment('data-vorma="rest-end"');
		document.head.appendChild(startRestComment);
		document.head.appendChild(endRestComment);

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

		expect(document.head.querySelector("meta")).toBeNull();
	});

	it("should handle blocks with missing tag gracefully", () => {
		const blocks: Array<HeadEl> = [
			{
				// No tag property
				attributesKnownSafe: {
					name: "description",
					content: "Test Description",
				},
			},
			{
				tag: "link",
				attributesKnownSafe: { rel: "stylesheet", href: "/styles.css" },
			},
		];

		updateHeadEls("meta", blocks);

		expect(document.head.querySelectorAll("meta").length).toBe(0);
		expect(document.head.querySelectorAll("link").length).toBe(1);
	});

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

	it("should handle complex innerHTML correctly (style tag)", () => {
		const initialCSS = "body > .foo { color: red; }\n/* comment */";
		const updatedCSS = ".bar { font-weight: bold; }";
		const initialBlock: HeadEl = {
			tag: "style",
			dangerousInnerHTML: initialCSS,
		};
		const updatedBlock: HeadEl = {
			tag: "style",
			dangerousInnerHTML: updatedCSS,
		};

		updateHeadEls("rest", [initialBlock]);

		let styleEl = document.head.querySelector("style");
		expect(styleEl).not.toBeNull();
		expect(styleEl?.innerHTML.trim()).toBe(initialCSS.trim());

		updateHeadEls("rest", [updatedBlock]);

		styleEl = document.head.querySelector("style");
		expect(styleEl).not.toBeNull();
		expect(styleEl?.innerHTML.trim()).toBe(updatedCSS.trim());
		expect(document.head.querySelectorAll("style").length).toBe(1);

		updateHeadEls("rest", []);
		expect(document.head.querySelector("style")).toBeNull();
	});

	it("should add and remove boolean attributes across updates", () => {
		const scriptBlockBase: HeadEl = {
			tag: "script",
			attributesKnownSafe: { src: "a.js" },
		};
		const scriptBlockWithAsync: HeadEl = {
			...scriptBlockBase,
			booleanAttributes: ["async"],
		};

		updateHeadEls("rest", [scriptBlockBase]);

		let scriptEl =
			document.head.querySelector<HTMLScriptElement>(
				"script[src='a.js']",
			);
		expect(scriptEl).not.toBeNull();
		expect(scriptEl?.hasAttribute("async")).toBe(false);

		updateHeadEls("rest", [scriptBlockWithAsync]);

		scriptEl =
			document.head.querySelector<HTMLScriptElement>(
				"script[src='a.js']",
			);
		expect(scriptEl).not.toBeNull();
		expect(scriptEl?.hasAttribute("async")).toBe(true);
		expect(scriptEl?.getAttribute("async")).toBe("");

		updateHeadEls("rest", [scriptBlockBase]);

		scriptEl =
			document.head.querySelector<HTMLScriptElement>(
				"script[src='a.js']",
			);
		expect(scriptEl).not.toBeNull();
		expect(scriptEl?.hasAttribute("async")).toBe(false);
	});

	it("should handle initially duplicate DOM elements correctly", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		const metaDesc1 = document.createElement("meta");
		metaDesc1.setAttribute("name", "description");
		metaDesc1.setAttribute("content", "A");

		const metaDesc2 = document.createElement("meta");
		metaDesc2.setAttribute("name", "description");
		metaDesc2.setAttribute("content", "A");

		document.head.insertBefore(metaDesc1, comments.endComment);
		document.head.insertBefore(metaDesc2, comments.endComment);

		expect(
			document.head.querySelectorAll('meta[name="description"]').length,
		).toBe(2);

		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: { name: "description", content: "A" },
			},
		];
		updateHeadEls("meta", blocks);

		const finalElements = document.head.querySelectorAll(
			'meta[name="description"]',
		);
		expect(finalElements.length).toBe(1);
		expect(finalElements[0]?.getAttribute("content")).toBe("A");

		// Verify element is between comments
		const nodesBetween: Array<Node> = [];
		let current = comments.startComment.nextSibling;
		while (current && current !== comments.endComment) {
			nodesBetween.push(current);
			current = current.nextSibling;
		}
		const elementsBetween = nodesBetween.filter(
			(n) => n.nodeType === Node.ELEMENT_NODE,
		);
		expect(elementsBetween.length).toBe(1);
		expect(elementsBetween[0]).toBe(finalElements[0]);
	});

	it("should maintain correct element positions when updating attributes", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create initial element
		const meta = document.createElement("meta");
		meta.setAttribute("name", "description");
		meta.setAttribute("content", "Initial description");
		document.head.insertBefore(meta, comments.endComment);

		// Update content attribute only
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Updated description",
				},
			},
		];

		updateHeadEls("meta", blocks);

		// Get the element after update
		const metaAfterUpdate = document.head.querySelector(
			'meta[name="description"]',
		);

		// Verify attributes updated
		expect(metaAfterUpdate?.getAttribute("content")).toBe(
			"Updated description",
		);

		// Verify it's positioned correctly (should be first element after start comment)
		expect(comments.startComment.nextElementSibling).toBe(metaAfterUpdate);
	});

	it("should maintain correct order when reordering elements", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create elements in order A, B, C
		const elementA = document.createElement("meta");
		elementA.setAttribute("name", "description");
		elementA.setAttribute("content", "Description");

		const elementB = document.createElement("meta");
		elementB.setAttribute("name", "viewport");
		elementB.setAttribute("content", "width=device-width");

		const elementC = document.createElement("meta");
		elementC.setAttribute("name", "robots");
		elementC.setAttribute("content", "index, follow");

		// Insert in order A, B, C
		document.head.insertBefore(elementA, comments.endComment);
		document.head.insertBefore(elementB, comments.endComment);
		document.head.insertBefore(elementC, comments.endComment);

		// Update to order C, A, B
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "robots",
					content: "index, follow",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Description",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "viewport",
					content: "width=device-width",
				},
			},
		];

		updateHeadEls("meta", blocks);

		// Get elements after update
		const elements = document.head.querySelectorAll("meta");
		expect(elements.length).toBe(3);

		if (!elements[0] || !elements[1] || !elements[2]) {
			throw new Error("Meta elements not found");
		}

		// Verify order is now C, A, B
		expect(elements[0].getAttribute("name")).toBe("robots");
		expect(elements[1].getAttribute("name")).toBe("description");
		expect(elements[2].getAttribute("name")).toBe("viewport");
	});

	it("should remove elements that are no longer needed", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create three elements
		const meta1 = document.createElement("meta");
		meta1.setAttribute("name", "description");
		meta1.setAttribute("content", "Description");

		const meta2 = document.createElement("meta");
		meta2.setAttribute("name", "viewport");
		meta2.setAttribute("content", "width=device-width");

		const meta3 = document.createElement("meta");
		meta3.setAttribute("name", "robots");
		meta3.setAttribute("content", "index, follow");

		// Insert all three
		document.head.insertBefore(meta1, comments.endComment);
		document.head.insertBefore(meta2, comments.endComment);
		document.head.insertBefore(meta3, comments.endComment);

		// Update to keep only description and robots meta tags
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Description",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "robots",
					content: "index, follow",
				},
			},
		];

		updateHeadEls("meta", blocks);

		// Get elements after update
		const elements = document.head.querySelectorAll("meta");
		expect(elements.length).toBe(2);

		if (!elements[0] || !elements[1]) {
			throw new Error("Meta elements not found");
		}

		// Verify the right elements were kept (by attribute, not reference)
		expect(elements[0].getAttribute("name")).toBe("description");
		expect(elements[1].getAttribute("name")).toBe("robots");

		// Verify viewport meta is removed
		expect(document.head.querySelector('meta[name="viewport"]')).toBeNull();
	});

	it("should handle complex scenarios with minimal DOM changes", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create initial elements
		const metaDescription = document.createElement("meta");
		metaDescription.setAttribute("name", "description");
		metaDescription.setAttribute("content", "Original description");

		const metaKeywords = document.createElement("meta");
		metaKeywords.setAttribute("name", "keywords");
		metaKeywords.setAttribute("content", "original, keywords");

		const linkCanonical = document.createElement("link");
		linkCanonical.setAttribute("rel", "canonical");
		linkCanonical.setAttribute("href", "/original-url");

		// Insert in initial order
		document.head.insertBefore(metaDescription, comments.endComment);
		document.head.insertBefore(metaKeywords, comments.endComment);
		document.head.insertBefore(linkCanonical, comments.endComment);

		// Update to:
		// 1. Keep metaKeywords (unchanged)
		// 2. Update metaDescription content
		// 3. Remove linkCanonical
		// 4. Add new metaViewport
		// 5. Reorder (keywords first, then description, then viewport)
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "keywords",
					content: "original, keywords",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Updated description",
				},
			},
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "viewport",
					content: "width=device-width",
				},
			},
		];

		updateHeadEls("meta", blocks);

		// Get elements after update
		const metaElements = document.head.querySelectorAll("meta");
		const linkElements = document.head.querySelectorAll("link");

		// Verify counts
		expect(metaElements.length).toBe(3);
		expect(linkElements.length).toBe(0);

		if (!metaElements[0] || !metaElements[1] || !metaElements[2]) {
			throw new Error("Meta elements not found");
		}

		// Verify order and content
		expect(metaElements[0].getAttribute("name")).toBe("keywords");
		expect(metaElements[1].getAttribute("name")).toBe("description");
		expect(metaElements[2].getAttribute("name")).toBe("viewport");

		// Verify content updates happened
		expect(metaElements[1].getAttribute("content")).toBe(
			"Updated description",
		);

		// Verify canonical link was removed
		expect(document.head.querySelector('link[rel="canonical"]')).toBeNull();
	});

	it("should handle text nodes and preserve element positions", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create initial element
		const meta = document.createElement("meta");
		meta.setAttribute("name", "description");
		meta.setAttribute("content", "Test Description");

		// Add text nodes between elements
		const textBefore = document.createTextNode("\n  ");
		const textAfter = document.createTextNode("\n");

		// Insert with text nodes
		document.head.insertBefore(textBefore, comments.endComment);
		document.head.insertBefore(meta, comments.endComment);
		document.head.insertBefore(textAfter, comments.endComment);

		// Update with same block (no changes)
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

		// Check that meta element still exists
		const metaAfterUpdate = document.head.querySelector(
			'meta[name="description"]',
		);
		expect(metaAfterUpdate).not.toBeNull();
		expect(metaAfterUpdate?.getAttribute("content")).toBe(
			"Test Description",
		);

		// Check that element is properly positioned (first element after start comment)
		expect(comments.startComment.nextElementSibling).toBe(metaAfterUpdate);

		// Check that text nodes are removed
		let textNodesExist = false;
		let node = comments.startComment.nextSibling;

		while (node && node !== comments.endComment) {
			if (node.nodeType === Node.TEXT_NODE) {
				textNodesExist = true;
				break;
			}
			node = node.nextSibling;
		}

		expect(textNodesExist).toBe(false);
	});

	it("should not unnecessarily recreate unchanged elements with identical fingerprints", () => {
		const comments = getStartAndEndComments("meta");
		if (!comments.startComment || !comments.endComment) {
			throw new Error("Meta comments not found");
		}

		// Create initial element
		const meta = document.createElement("meta");
		meta.setAttribute("name", "description");
		meta.setAttribute("content", "Identical content");
		document.head.insertBefore(meta, comments.endComment);

		// Store original reference
		const originalElement = meta;

		// Update with identical block (no changes)
		const blocks: Array<HeadEl> = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "description",
					content: "Identical content",
				},
			},
		];

		updateHeadEls("meta", blocks);

		// Get the element after update
		const metaAfterUpdate = document.head.querySelector(
			'meta[name="description"]',
		);

		// For identical fingerprints, the element should be reused
		expect(metaAfterUpdate).toBe(originalElement);
	});
});
