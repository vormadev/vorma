import { describe, expect, it, vi } from "vitest";
import { panic } from "../utils/errors.ts";
import type { HeadEl } from "../vorma_ctx/vorma_ctx.ts";
import { getStartAndEndComments, updateHeadEls } from "./head_elements.ts";
import { describeUpdateHeadElsSuite } from "./head.test.helpers.ts";

vi.mock("../utils/errors.ts", () => ({
	panic: vi.fn(() => {
		throw new Error("Panic called");
	}),
}));

describeUpdateHeadElsSuite(() => {
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
});
