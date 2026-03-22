// Assertions covered: 11, 12

import { beforeEach, describe, expect, it, vi } from "vitest";
import { reset_client_runtime_for_testing } from "vorma/testing";
import {
	create_route_data_response,
	load_client,
	navigate_with_route_data,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

// ─── Head Section Helpers ────────────────────────────────────────

type HeadSection = "meta" | "rest";

function get_head_section_comments(type: HeadSection): {
	start_comment: Comment;
	end_comment: Comment;
} {
	const start_token = `data-vorma="${type}-start"`;
	const end_token = `data-vorma="${type}-end"`;
	let start_comment: Comment | undefined;
	for (const node of Array.from(document.head.childNodes)) {
		if (node.nodeType !== Node.COMMENT_NODE) {
			continue;
		}
		const value = (node as Comment).nodeValue?.trim();
		if (value === start_token) {
			start_comment = node as Comment;
			continue;
		}
		if (value === end_token && start_comment) {
			return { start_comment, end_comment: node as Comment };
		}
	}
	throw new Error(`Missing managed head markers for section "${type}".`);
}

function get_nodes_between_markers(type: HeadSection): Node[] {
	const { start_comment, end_comment } = get_head_section_comments(type);
	const nodes: Node[] = [];
	let current: Node | null = start_comment.nextSibling;
	while (current && current !== end_comment) {
		nodes.push(current);
		current = current.nextSibling;
	}
	return nodes;
}

function get_elements_between_markers(type: HeadSection): Element[] {
	return get_nodes_between_markers(type).filter(
		(node): node is Element => node.nodeType === Node.ELEMENT_NODE,
	);
}

function insert_head_element(type: HeadSection, element: Element): void {
	const { end_comment } = get_head_section_comments(type);
	document.head.insertBefore(element, end_comment);
}

// ─── Tests ───────────────────────────────────────────────────────

describe("title and head", () => {
	// ─── Assertion 11: Title Commitment ──────────────────────

	describe("title commitment", () => {
		it("commits title from server route-data", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					title: { dangerousInnerHTML: "Black Box Title" },
				}),
			);

			await client.vormaNavigate("/title-commit");
			await vi.runAllTimersAsync();

			expect(document.title).toBe("Black Box Title");
		});

		it("decodes HTML entities before updating document.title", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					title: {
						dangerousInnerHTML: "Fish &amp; Chips &lt;3",
					},
				}),
			);

			await client.vormaNavigate("/entity-title");
			await vi.runAllTimersAsync();

			expect(document.title).toBe("Fish & Chips <3");
		});

		it("clears document.title when route-data omits title payload", async () => {
			const client = await load_client();
			document.title = "Stale Old Title";
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					title: undefined,
				}),
			);

			await client.vormaNavigate("/title-cleared");
			await vi.runAllTimersAsync();

			expect(document.title).toBe("");
		});

		it("dispatches route-change after title is committed", async () => {
			const client = await load_client();
			const observed_titles: string[] = [];
			const cleanup = client.addRouteChangeListener(() => {
				observed_titles.push(document.title);
			});

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Committed Before Event",
						},
					}),
				);

				await client.vormaNavigate("/title-order");
				await vi.runAllTimersAsync();

				expect(observed_titles).toContain("Committed Before Event");
			} finally {
				cleanup();
			}
		});
	});

	// ─── Assertion 12: Head Reconciliation ───────────────────

	describe("managed head reconciliation", () => {
		it("fails loud when managed head markers are missing or invalid", async () => {
			const client = await load_client();
			document.head.innerHTML = "";
			document.head.appendChild(
				document.createComment('data-vorma="meta-end"'),
			);
			document.head.appendChild(
				document.createComment('data-vorma="meta-start"'),
			);
			const sentinel = document.createElement("meta");
			sentinel.setAttribute("name", "sentinel");
			sentinel.setAttribute("content", "keep");
			document.head.appendChild(sentinel);
			document.head.appendChild(
				document.createComment('data-vorma="rest-start"'),
			);
			document.head.appendChild(
				document.createComment('data-vorma="rest-end"'),
			);

			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				create_route_data_response({
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "new",
							},
							booleanAttributes: [],
						},
					],
				}),
			);

			await expect(
				client.vormaNavigate("/head-invalid-markers"),
			).rejects.toThrow();
			await vi.runAllTimersAsync();

			expect(
				document.head
					.querySelector('meta[name="sentinel"]')
					?.getAttribute("content"),
			).toBe("keep");
			expect(
				document.head.querySelector('meta[name="description"]'),
			).toBeNull();
		});

		it("deterministically adds, updates, removes, and reorders managed nodes", async () => {
			const client = await load_client();
			const description = document.createElement("meta");
			description.setAttribute("name", "description");
			description.setAttribute("content", "Original description");
			const keywords = document.createElement("meta");
			keywords.setAttribute("name", "keywords");
			keywords.setAttribute("content", "alpha, beta");
			const canonical = document.createElement("link");
			canonical.setAttribute("rel", "canonical");
			canonical.setAttribute("href", "/original");
			insert_head_element("meta", description);
			insert_head_element("meta", keywords);
			insert_head_element("meta", canonical);

			await navigate_with_route_data({
				client,
				href: "/head-reconcile",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "keywords",
								content: "alpha, beta",
							},
							booleanAttributes: [],
						},
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "Updated description",
							},
							booleanAttributes: [],
						},
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "viewport",
								content: "width=device-width",
							},
							booleanAttributes: [],
						},
					],
				},
			});

			const elements = get_elements_between_markers("meta");
			const { start_comment } = get_head_section_comments("meta");
			expect(elements).toHaveLength(3);
			expect(start_comment.nextElementSibling).toBe(elements[0]);
			expect(elements[0]).toBe(keywords);
			expect(elements[0]?.getAttribute("name")).toBe("keywords");
			expect(elements[1]).toBe(description);
			expect(elements[1]?.getAttribute("content")).toBe(
				"Updated description",
			);
			expect(elements[2]?.getAttribute("name")).toBe("viewport");
			expect(
				document.head.querySelector('link[rel="canonical"]'),
			).toBeNull();
		});

		it("reuses matching elements and collapses duplicates", async () => {
			const client = await load_client();
			const meta_a = document.createElement("meta");
			meta_a.setAttribute("name", "description");
			meta_a.setAttribute("content", "A");
			const meta_b = document.createElement("meta");
			meta_b.setAttribute("name", "description");
			meta_b.setAttribute("content", "A");
			insert_head_element("meta", meta_a);
			insert_head_element("meta", meta_b);

			await navigate_with_route_data({
				client,
				href: "/head-collapse",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "A",
							},
							booleanAttributes: [],
						},
					],
				},
			});

			const elements = get_elements_between_markers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta_a);
		});

		it("keeps sections unchanged when requested blocks are empty", async () => {
			const client = await load_client();
			const initial_count = document.head.childNodes.length;

			await navigate_with_route_data({
				client,
				href: "/head-empty",
				overrides: {
					meta_head_els: [],
					rest_head_els: [],
				},
			});

			expect(document.head.childNodes.length).toBe(initial_count);
			expect(get_elements_between_markers("meta")).toHaveLength(0);
			expect(get_elements_between_markers("rest")).toHaveLength(0);
		});

		it("inserts elements into empty section", async () => {
			const client = await load_client();

			await navigate_with_route_data({
				client,
				href: "/head-insert-empty",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "Inserted",
							},
							booleanAttributes: [],
						},
						{
							tag: "link",
							attributesKnownSafe: {
								rel: "stylesheet",
								href: "/inserted.css",
							},
							booleanAttributes: [],
						},
					],
				},
			});

			const elements = get_elements_between_markers("meta");
			expect(elements).toHaveLength(2);
			expect(elements[0]?.tagName.toLowerCase()).toBe("meta");
			expect(elements[0]?.getAttribute("name")).toBe("description");
			expect(elements[1]?.tagName.toLowerCase()).toBe("link");
			expect(elements[1]?.getAttribute("href")).toBe("/inserted.css");
		});

		it("keeps attribute-order-independent fingerprint matches stable", async () => {
			const client = await load_client();
			const meta = document.createElement("meta");
			meta.setAttribute("content", "Order test");
			meta.setAttribute("name", "description");
			insert_head_element("meta", meta);

			await navigate_with_route_data({
				client,
				href: "/head-attr-order",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "Order test",
							},
							booleanAttributes: [],
						},
					],
				},
			});

			const elements = get_elements_between_markers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta);
		});

		it("remains idempotent across repeated identical updates", async () => {
			const client = await load_client();
			const head_els = [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "Idempotent",
					},
					booleanAttributes: [],
				},
			];

			await navigate_with_route_data({
				client,
				href: "/head-idempotent-a",
				overrides: { meta_head_els: head_els },
			});
			const first_element = get_elements_between_markers("meta")[0];

			await navigate_with_route_data({
				client,
				href: "/head-idempotent-b",
				overrides: { meta_head_els: head_els },
			});
			const second_element = get_elements_between_markers("meta")[0];

			expect(second_element).toBe(first_element);
			expect(get_elements_between_markers("meta")).toHaveLength(1);
		});

		it("keeps meta and rest sections isolated", async () => {
			const client = await load_client();
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Meta keep");
			insert_head_element("meta", meta);

			await navigate_with_route_data({
				client,
				href: "/head-isolation",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "Meta keep",
							},
							booleanAttributes: [],
						},
					],
					rest_head_els: [
						{
							tag: "script",
							attributesKnownSafe: { src: "/rest-only.js" },
							booleanAttributes: [],
						},
					],
				},
			});

			const meta_elements = get_elements_between_markers("meta");
			const rest_elements = get_elements_between_markers("rest");
			expect(meta_elements).toHaveLength(1);
			expect(meta_elements[0]).toBe(meta);
			expect(rest_elements).toHaveLength(1);
			expect(rest_elements[0]?.getAttribute("src")).toBe("/rest-only.js");
		});

		it("cleans interleaved text nodes during reconciliation", async () => {
			const client = await load_client();
			const { end_comment } = get_head_section_comments("meta");
			document.head.insertBefore(
				document.createTextNode("\n  "),
				end_comment,
			);
			const meta = document.createElement("meta");
			meta.setAttribute("name", "description");
			meta.setAttribute("content", "Text cleanup");
			document.head.insertBefore(meta, end_comment);
			document.head.insertBefore(
				document.createTextNode("\n"),
				end_comment,
			);

			await navigate_with_route_data({
				client,
				href: "/head-clean-text",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "Text cleanup",
							},
							booleanAttributes: [],
						},
					],
				},
			});

			expect(
				get_nodes_between_markers("meta").some(
					(node) => node.nodeType === Node.TEXT_NODE,
				),
			).toBe(false);
			const elements = get_elements_between_markers("meta");
			expect(elements).toHaveLength(1);
			expect(elements[0]).toBe(meta);
		});

		it("updates style and script rest-head elements across navigations", async () => {
			const client = await load_client();

			await navigate_with_route_data({
				client,
				href: "/head-rest-a",
				overrides: {
					rest_head_els: [
						{
							tag: "style",
							attributesKnownSafe: {},
							booleanAttributes: [],
							dangerousInnerHTML: "body { color: red; }",
						},
						{
							tag: "script",
							attributesKnownSafe: { src: "/runtime-a.js" },
							booleanAttributes: [],
						},
					],
				},
			});

			let rest_elements = get_elements_between_markers("rest");
			expect(rest_elements).toHaveLength(2);
			expect(rest_elements[0]?.tagName.toLowerCase()).toBe("style");
			expect(rest_elements[0]?.innerHTML).toBe("body { color: red; }");
			expect(rest_elements[1]?.tagName.toLowerCase()).toBe("script");
			expect(rest_elements[1]?.getAttribute("src")).toBe("/runtime-a.js");

			await navigate_with_route_data({
				client,
				href: "/head-rest-b",
				overrides: {
					rest_head_els: [
						{
							tag: "style",
							attributesKnownSafe: {},
							booleanAttributes: [],
							dangerousInnerHTML: ".bar { font-weight: bold; }",
						},
						{
							tag: "script",
							attributesKnownSafe: { src: "/runtime-a.js" },
							booleanAttributes: ["async"],
						},
					],
				},
			});

			rest_elements = get_elements_between_markers("rest");
			expect(rest_elements).toHaveLength(2);
			expect(rest_elements[0]?.innerHTML).toBe(
				".bar { font-weight: bold; }",
			);
			expect(rest_elements[1]?.getAttribute("src")).toBe("/runtime-a.js");
			expect(rest_elements[1]?.hasAttribute("async")).toBe(true);

			await navigate_with_route_data({
				client,
				href: "/head-rest-c",
				overrides: { rest_head_els: [] },
			});

			rest_elements = get_elements_between_markers("rest");
			expect(rest_elements).toHaveLength(0);
		});

		it("uses nearest valid managed head marker pair and ignores stray markers", async () => {
			const client = await load_client();
			document.head.innerHTML = "";
			document.head.appendChild(
				document.createComment('data-vorma="meta-end"'),
			);
			document.head.appendChild(
				document.createComment('data-vorma="meta-start"'),
			);
			const stray = document.createElement("meta");
			stray.setAttribute("name", "stray");
			stray.setAttribute("content", "keep");
			document.head.appendChild(stray);
			document.head.appendChild(
				document.createComment('data-vorma="meta-start"'),
			);
			const stale = document.createElement("meta");
			stale.setAttribute("name", "description");
			stale.setAttribute("content", "stale");
			document.head.appendChild(stale);
			document.head.appendChild(
				document.createComment('data-vorma="meta-end"'),
			);
			const outside = document.createElement("meta");
			outside.setAttribute("name", "outside");
			outside.setAttribute("content", "keep");
			document.head.appendChild(outside);
			document.head.appendChild(
				document.createComment('data-vorma="rest-start"'),
			);
			document.head.appendChild(
				document.createComment('data-vorma="rest-end"'),
			);

			await navigate_with_route_data({
				client,
				href: "/head-nearest-markers",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "fresh",
							},
							booleanAttributes: [],
						},
					],
				},
			});

			expect(
				document.head
					.querySelector('meta[name="description"]')
					?.getAttribute("content"),
			).toBe("fresh");
			expect(
				document.head.querySelector('meta[name="stray"]'),
			).not.toBeNull();
			expect(
				document.head.querySelector('meta[name="outside"]'),
			).not.toBeNull();
		});

		it("accepts head blocks that omit booleanAttributes", async () => {
			const client = await load_client();

			await navigate_with_route_data({
				client,
				href: "/head-no-bool-attrs",
				overrides: {
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "no-bool-attrs",
							},
						},
					],
					rest_head_els: [
						{
							tag: "link",
							attributesKnownSafe: {
								rel: "canonical",
								href: "/no-bool-attrs",
							},
						},
					],
				},
			});

			expect(
				document.head.querySelector('meta[name="description"]'),
			).not.toBeNull();
			expect(
				document.head.querySelector('link[rel="canonical"]'),
			).not.toBeNull();
		});

		it("keeps css bundle stylesheets after rest-head reconciliation clears managed blocks", async () => {
			const client = await load_client();
			const existing = document.createElement("link");
			existing.setAttribute("rel", "stylesheet");
			existing.setAttribute("data-vorma-css-bundle", "/keep.css");
			existing.setAttribute("href", "/keep.css");
			insert_head_element("rest", existing);

			const append_child = document.head.appendChild.bind(document.head);
			vi.spyOn(document.head, "appendChild").mockImplementation(
				(node) => {
					if (
						node instanceof HTMLLinkElement &&
						node.rel === "preload" &&
						node.getAttribute("as") === "style"
					) {
						void Promise.resolve().then(() =>
							node.dispatchEvent(new Event("load")),
						);
					}
					return append_child(node);
				},
			);

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					css_bundles: ["/keep.css"],
					rest_head_els: [],
				}),
			);

			await client.vormaNavigate("/keep-css-bundle");
			await vi.runAllTimersAsync();

			expect(
				document.querySelector(
					'link[rel="stylesheet"][data-vorma-css-bundle="/keep.css"]',
				),
			).not.toBeNull();
			expect(get_elements_between_markers("rest")).toHaveLength(0);
		});
	});
});
