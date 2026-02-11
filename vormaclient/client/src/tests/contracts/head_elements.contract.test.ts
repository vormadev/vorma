import { beforeEach, describe, expect, it } from "vitest";
import {
	updateHeadEls,
	getStartAndEndComments,
} from "../../ui/head.ts";
import type { HeadEl } from "../../app/context.ts";
import { setupContractTestSuite } from "./contract_test_harness.ts";

setupContractTestSuite();

type HeadSection = "meta" | "rest";

function installSectionMarkers(): void {
	document.head.appendChild(
		document.createComment('data-vorma="meta-start"'),
	);
	document.head.appendChild(document.createComment('data-vorma="meta-end"'));
	document.head.appendChild(
		document.createComment('data-vorma="rest-start"'),
	);
	document.head.appendChild(document.createComment('data-vorma="rest-end"'));
}

function getSectionBounds(type: HeadSection): {
	startComment: Comment;
	endComment: Comment;
} {
	const comments = getStartAndEndComments(type);
	if (!comments.startComment || !comments.endComment) {
		throw new Error(`Missing ${type} section markers`);
	}
	return {
		startComment: comments.startComment,
		endComment: comments.endComment,
	};
}

function getNodesBetweenMarkers(type: HeadSection): Node[] {
	const { startComment, endComment } = getSectionBounds(type);
	const nodes: Node[] = [];
	let node: Node | null = startComment.nextSibling;
	while (node && node !== endComment) {
		nodes.push(node);
		node = node.nextSibling;
	}
	return nodes;
}

function getElementsBetweenMarkers(type: HeadSection): Element[] {
	return getNodesBetweenMarkers(type).filter(
		(node): node is Element => node.nodeType === Node.ELEMENT_NODE,
	);
}

function insertElement(type: HeadSection, element: Element): void {
	const { endComment } = getSectionBounds(type);
	document.head.insertBefore(element, endComment);
}

describe("head element contracts", () => {
	beforeEach(() => {
		installSectionMarkers();
	});

	describe("advanced updates", () => {
		it("updates style innerHTML content across consecutive rest updates", () => {
			const initialCSS = "body > .foo { color: red; }\n/* comment */";
			const updatedCSS = ".bar { font-weight: bold; }";

			updateHeadEls("rest", [
				{
					tag: "style",
					dangerousInnerHTML: initialCSS,
				},
			]);

			let styles = getElementsBetweenMarkers("rest");
			expect(styles).toHaveLength(1);
			expect(styles[0]?.tagName.toLowerCase()).toBe("style");
			expect(styles[0]?.innerHTML.trim()).toBe(initialCSS);

			updateHeadEls("rest", [
				{
					tag: "style",
					dangerousInnerHTML: updatedCSS,
				},
			]);

			styles = getElementsBetweenMarkers("rest");
			expect(styles).toHaveLength(1);
			expect(styles[0]?.tagName.toLowerCase()).toBe("style");
			expect(styles[0]?.innerHTML.trim()).toBe(updatedCSS);

			updateHeadEls("rest", []);
			expect(getElementsBetweenMarkers("rest")).toHaveLength(0);
		});

		it("adds and removes boolean attributes across updates", () => {
			const baseScript: HeadEl = {
				tag: "script",
				attributesKnownSafe: { src: "a.js" },
			};

			updateHeadEls("rest", [baseScript]);
			let script =
				document.head.querySelector<HTMLScriptElement>(
					"script[src='a.js']",
				);
			expect(script).not.toBeNull();
			expect(script?.hasAttribute("async")).toBe(false);

			updateHeadEls("rest", [
				{ ...baseScript, booleanAttributes: ["async"] },
			]);
			script =
				document.head.querySelector<HTMLScriptElement>(
					"script[src='a.js']",
				);
			expect(script).not.toBeNull();
			expect(script?.hasAttribute("async")).toBe(true);
			expect(script?.getAttribute("async")).toBe("");

			updateHeadEls("rest", [baseScript]);
			script =
				document.head.querySelector<HTMLScriptElement>(
					"script[src='a.js']",
				);
			expect(script).not.toBeNull();
			expect(script?.hasAttribute("async")).toBe(false);
		});

		it("collapses duplicate existing elements down to requested count", () => {
			const metaA = document.createElement("meta");
			metaA.setAttribute("name", "description");
			metaA.setAttribute("content", "A");

			const metaB = document.createElement("meta");
			metaB.setAttribute("name", "description");
			metaB.setAttribute("content", "A");

			insertElement("meta", metaA);
			insertElement("meta", metaB);
			expect(getElementsBetweenMarkers("meta")).toHaveLength(2);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: { name: "description", content: "A" },
				},
			]);

			const finalElements = getElementsBetweenMarkers("meta");
			expect(finalElements).toHaveLength(1);
			expect(finalElements[0]?.tagName.toLowerCase()).toBe("meta");
			expect(finalElements[0]?.getAttribute("name")).toBe("description");
			expect(finalElements[0]?.getAttribute("content")).toBe("A");
		});

		it("keeps updated elements positioned immediately after section start", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Initial description");
			insertElement("meta", meta);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Updated description",
					},
				},
			]);

			const { startComment } = getSectionBounds("meta");
			const updated = document.head.querySelector(
				'meta[name="description"]',
			);
			expect(updated?.getAttribute("content")).toBe(
				"Updated description",
			);
			expect(startComment.nextElementSibling).toBe(updated);
		});

		it("reorders elements to match requested block order", () => {
			const description = document.createElement("meta");
			description.setAttribute("name", "description");
			description.setAttribute("content", "Description");

			const viewport = document.createElement("meta");
			viewport.setAttribute("name", "viewport");
			viewport.setAttribute("content", "width=device-width");

			const robots = document.createElement("meta");
			robots.setAttribute("name", "robots");
			robots.setAttribute("content", "index, follow");

			insertElement("meta", description);
			insertElement("meta", viewport);
			insertElement("meta", robots);

			updateHeadEls("meta", [
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
			]);

			const names = getElementsBetweenMarkers("meta").map((element) =>
				element.getAttribute("name"),
			);
			expect(names).toEqual(["robots", "description", "viewport"]);
		});

		it("removes elements that are no longer part of requested blocks", () => {
			const description = document.createElement("meta");
			description.setAttribute("name", "description");
			description.setAttribute("content", "Description");

			const viewport = document.createElement("meta");
			viewport.setAttribute("name", "viewport");
			viewport.setAttribute("content", "width=device-width");

			const robots = document.createElement("meta");
			robots.setAttribute("name", "robots");
			robots.setAttribute("content", "index, follow");

			insertElement("meta", description);
			insertElement("meta", viewport);
			insertElement("meta", robots);

			updateHeadEls("meta", [
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
			]);

			const names = getElementsBetweenMarkers("meta").map((element) =>
				element.getAttribute("name"),
			);
			expect(names).toEqual(["description", "robots"]);
			expect(
				document.head.querySelector('meta[name="viewport"]'),
			).toBeNull();
		});
	});

	describe("basic operations", () => {
		it("keeps head unchanged when blocks array is empty", () => {
			const initialNodeCount = document.head.childNodes.length;

			updateHeadEls("meta", []);

			expect(document.head.childNodes.length).toBe(initialNodeCount);
			expect(getElementsBetweenMarkers("meta")).toHaveLength(0);
		});

		it("inserts requested elements between meta markers when none exist", () => {
			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Test Description",
					},
				},
				{
					tag: "link",
					attributesKnownSafe: {
						rel: "stylesheet",
						href: "/styles.css",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(2);
			expect(elements[0]?.tagName.toLowerCase()).toBe("meta");
			expect(elements[1]?.tagName.toLowerCase()).toBe("link");
			expect(elements[0]?.getAttribute("name")).toBe("description");
			expect(elements[0]?.getAttribute("content")).toBe(
				"Test Description",
			);
			expect(elements[1]?.getAttribute("rel")).toBe("stylesheet");
			expect(elements[1]?.getAttribute("href")).toBe("/styles.css");
		});

		it("updates changed element attributes", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Old Description");
			insertElement("meta", meta);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "New Description",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]?.getAttribute("content")).toBe(
				"New Description",
			);
		});

		it("removes obsolete elements from section", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Old Description");

			const link = document.createElement("link");
			link.setAttribute("rel", "stylesheet");
			link.setAttribute("href", "/styles.css");

			insertElement("meta", meta);
			insertElement("meta", link);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Old Description",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]?.tagName.toLowerCase()).toBe("meta");
			expect(document.head.querySelectorAll("link")).toHaveLength(0);
		});

		it("reuses existing element when fingerprint is unchanged", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Test Description");
			insertElement("meta", meta);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Test Description",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta);
		});

		it("reorders existing elements to requested sequence", () => {
			const description = document.createElement("meta");
			description.setAttribute("name", "description");
			description.setAttribute("content", "Description");

			const viewport = document.createElement("meta");
			viewport.setAttribute("name", "viewport");
			viewport.setAttribute("content", "width=device-width");

			insertElement("meta", description);
			insertElement("meta", viewport);

			updateHeadEls("meta", [
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
			]);

			const names = getElementsBetweenMarkers("meta").map((element) =>
				element.getAttribute("name"),
			);
			expect(names).toEqual(["viewport", "description"]);
		});

		it("applies boolean attributes to inserted elements", () => {
			updateHeadEls("meta", [
				{
					tag: "script",
					attributesKnownSafe: { src: "/script.js" },
					booleanAttributes: ["async", "defer"],
				},
			]);

			const script = document.head.querySelector<HTMLScriptElement>(
				"script[src='/script.js']",
			);
			expect(script).not.toBeNull();
			expect(script?.hasAttribute("async")).toBe(true);
			expect(script?.hasAttribute("defer")).toBe(true);
			expect(script?.getAttribute("async")).toBe("");
			expect(script?.getAttribute("defer")).toBe("");
		});

		it("applies dangerousInnerHTML to inserted elements", () => {
			updateHeadEls("meta", [
				{
					tag: "style",
					dangerousInnerHTML: ".demo { color: red; }",
				},
			]);

			const style = getElementsBetweenMarkers("meta")[0];
			expect(style?.tagName.toLowerCase()).toBe("style");
			expect(style?.innerHTML).toBe(".demo { color: red; }");
		});

		it("does nothing when requested section markers are missing", () => {
			document.head.innerHTML = "";
			document.head.appendChild(
				document.createComment('data-vorma="rest-start"'),
			);
			document.head.appendChild(
				document.createComment('data-vorma="rest-end"'),
			);

			expect(() =>
				updateHeadEls("meta", [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Test Description",
						},
					},
				]),
			).not.toThrow();
			expect(document.head.querySelector("meta")).toBeNull();
		});

		it("ignores blocks without tag while still applying valid blocks", () => {
			updateHeadEls("meta", [
				{
					attributesKnownSafe: {
						name: "description",
						content: "Ignored because tag is missing",
					},
				},
				{
					tag: "link",
					attributesKnownSafe: {
						rel: "stylesheet",
						href: "/styles.css",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]?.tagName.toLowerCase()).toBe("link");
		});
	});

	describe("minimal DOM changes", () => {
		it("handles add update remove and reorder in a single reconciliation", () => {
			const description = document.createElement("meta");
			description.setAttribute("name", "description");
			description.setAttribute("content", "Original description");

			const keywords = document.createElement("meta");
			keywords.setAttribute("name", "keywords");
			keywords.setAttribute("content", "original, keywords");

			const canonical = document.createElement("link");
			canonical.setAttribute("rel", "canonical");
			canonical.setAttribute("href", "/original-url");

			insertElement("meta", description);
			insertElement("meta", keywords);
			insertElement("meta", canonical);

			updateHeadEls("meta", [
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
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(3);
			expect(elements[0]).toBe(keywords);
			expect(elements[0]?.getAttribute("name")).toBe("keywords");
			expect(elements[1]?.getAttribute("name")).toBe("description");
			expect(elements[1]?.getAttribute("content")).toBe(
				"Updated description",
			);
			expect(elements[2]?.getAttribute("name")).toBe("viewport");
			expect(
				document.head.querySelector('link[rel="canonical"]'),
			).toBeNull();
		});

		it("removes interleaved text nodes while preserving element placement", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Test Description");

			const { endComment, startComment } = getSectionBounds("meta");
			document.head.insertBefore(
				document.createTextNode("\n  "),
				endComment,
			);
			document.head.insertBefore(meta, endComment);
			document.head.insertBefore(
				document.createTextNode("\n"),
				endComment,
			);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Test Description",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta);
			expect(startComment.nextElementSibling).toBe(meta);
			expect(
				getNodesBetweenMarkers("meta").some(
					(node) => node.nodeType === Node.TEXT_NODE,
				),
			).toBe(false);
		});

		it("reuses identical elements instead of recreating nodes", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Identical content");
			insertElement("meta", meta);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Identical content",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta);
		});
	});

	describe("rest and edge cases", () => {
		it("updates rest section without modifying meta section", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Meta section entry");
			insertElement("meta", meta);

			updateHeadEls("rest", [
				{
					tag: "script",
					attributesKnownSafe: { src: "/script.js" },
				},
			]);

			const restElements = getElementsBetweenMarkers("rest");
			expect(restElements).toHaveLength(1);
			expect(restElements[0]?.tagName.toLowerCase()).toBe("script");
			expect(restElements[0]?.getAttribute("src")).toBe("/script.js");

			const metaElements = getElementsBetweenMarkers("meta");
			expect(metaElements).toHaveLength(1);
			expect(metaElements[0]).toBe(meta);
		});

		it("cleans text nodes between markers during updates", () => {
			const { endComment } = getSectionBounds("meta");
			document.head.insertBefore(
				document.createTextNode("\n  "),
				endComment,
			);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Test Description",
					},
				},
			]);

			expect(
				getNodesBetweenMarkers("meta").some(
					(node) => node.nodeType === Node.TEXT_NODE,
				),
			).toBe(false);
			expect(getElementsBetweenMarkers("meta")).toHaveLength(1);
		});

		it("remains idempotent across repeated identical updates", () => {
			const blocks: HeadEl[] = [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Test Description",
					},
				},
			];

			updateHeadEls("meta", blocks);
			updateHeadEls("meta", blocks);

			expect(getElementsBetweenMarkers("meta")).toHaveLength(1);
		});

		it("throws for null attribute values", () => {
			const blocks: HeadEl[] = [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: null as unknown as string,
					},
				},
			];

			expect(() => updateHeadEls("meta", blocks)).toThrow(
				/cannot be null or undefined/i,
			);
		});

		it("ignores entries whose tag is undefined", () => {
			updateHeadEls("meta", [
				{
					tag: undefined,
					attributesKnownSafe: {
						name: "description",
						content: "Ignored",
					},
				},
			]);

			expect(getElementsBetweenMarkers("meta")).toHaveLength(0);
		});

		it("matches existing elements regardless of attribute insertion order", () => {
			const meta = document.createElement("meta");
			meta.setAttribute("content", "Test Description");
			meta.setAttribute("name", "description");
			insertElement("meta", meta);

			updateHeadEls("meta", [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Test Description",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta);
		});

		it("handles combined add update remove and reorder deterministically", () => {
			const description = document.createElement("meta");
			description.setAttribute("name", "description");
			description.setAttribute("content", "Initial Description");

			const keywords = document.createElement("meta");
			keywords.setAttribute("name", "keywords");
			keywords.setAttribute("content", "test, vitest");

			const canonical = document.createElement("link");
			canonical.setAttribute("rel", "canonical");
			canonical.setAttribute("href", "/initial-page");

			const { endComment } = getSectionBounds("meta");
			document.head.insertBefore(canonical, endComment);
			document.head.insertBefore(keywords, canonical);
			document.head.insertBefore(description, keywords);

			updateHeadEls("meta", [
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
					attributesKnownSafe: {
						rel: "stylesheet",
						href: "/styles.css",
					},
				},
			]);

			const elements = getElementsBetweenMarkers("meta");
			expect(elements).toHaveLength(3);
			expect(elements[0]?.tagName.toLowerCase()).toBe("meta");
			expect(elements[0]?.getAttribute("name")).toBe("keywords");
			expect(elements[0]?.getAttribute("content")).toBe("test, vitest");
			expect(elements[1]?.tagName.toLowerCase()).toBe("meta");
			expect(elements[1]?.getAttribute("name")).toBe("description");
			expect(elements[1]?.getAttribute("content")).toBe(
				"Updated Description",
			);
			expect(elements[2]?.tagName.toLowerCase()).toBe("link");
			expect(elements[2]?.getAttribute("rel")).toBe("stylesheet");
			expect(elements[2]?.getAttribute("href")).toBe("/styles.css");
			expect(
				document.head.querySelector('link[rel="canonical"]'),
			).toBeNull();
		});
	});
});
