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
});
