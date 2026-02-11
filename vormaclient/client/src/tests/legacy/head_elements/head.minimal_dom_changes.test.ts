// READY_TO_DELETE_AFTER_SIGNOFF
import { expect, it, vi } from "vitest";
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
