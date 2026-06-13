// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from "vitest";
import { apply_head_and_title, type HeadEl } from "./head.ts";

function setup_boundary(section: "meta" | "rest") {
	const start = document.createComment(` vorma-${section}-start `);
	const end = document.createComment(` vorma-${section}-end `);
	document.head.appendChild(start);
	document.head.appendChild(end);
	return { start, end };
}

function elements_between(start: Comment, end: Comment): Element[] {
	const els: Element[] = [];
	let node: Node | null = start.nextSibling;
	while (node && node !== end) {
		if (node.nodeType === Node.ELEMENT_NODE) {
			els.push(node as Element);
		}
		node = node.nextSibling;
	}
	return els;
}

function meta(
	attrs: Record<string, string>,
	opts?: { boolean_attributes?: string[]; inner_html?: string },
): HeadEl {
	return {
		tag: "meta",
		attributes_known_safe: attrs,
		boolean_attributes: opts?.boolean_attributes,
		dangerous_inner_html: opts?.inner_html,
	};
}

function script(
	attrs: Record<string, string>,
	opts?: { boolean_attributes?: string[]; inner_html?: string },
): HeadEl {
	return {
		tag: "script",
		attributes_known_safe: attrs,
		boolean_attributes: opts?.boolean_attributes,
		dangerous_inner_html: opts?.inner_html,
	};
}

function style(inner_html: string): HeadEl {
	return {
		tag: "style",
		attributes_known_safe: {},
		dangerous_inner_html: inner_html,
	};
}

beforeEach(() => {
	document.head.innerHTML = "";
	document.title = "";
});

/////////////////////////////////////////////////////////////////////
/////// Title
/////////////////////////////////////////////////////////////////////

describe("title", () => {
	it("sets title from string", () => {
		apply_head_and_title("Hello", [], []);
		expect(document.title).toBe("Hello");
	});

	it("does not change title when undefined", () => {
		document.title = "Existing";
		apply_head_and_title(undefined, [], []);
		expect(document.title).toBe("Existing");
	});

	it("clears title with empty string when title element exists", () => {
		document.title = "Existing";
		apply_head_and_title("", [], []);
		expect(document.title).toBe("");
	});

	it("does not create title element for empty string when none exists", () => {
		const title_el = document.head.querySelector("title");
		if (title_el) {
			title_el.remove();
		}
		apply_head_and_title("", [], []);
		// title should remain empty, no element created
		expect(document.title).toBe("");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Meta section
/////////////////////////////////////////////////////////////////////

describe("meta section", () => {
	it("adds elements into empty section", () => {
		const { start, end } = setup_boundary("meta");

		apply_head_and_title(
			undefined,
			[
				meta({ name: "description", content: "A page" }),
				meta({ property: "og:title", content: "OG Title" }),
			],
			[],
		);

		const els = elements_between(start, end);
		expect(els).toHaveLength(2);
		expect(els[0]!.getAttribute("name")).toBe("description");
		expect(els[0]!.getAttribute("content")).toBe("A page");
		expect(els[1]!.getAttribute("property")).toBe("og:title");
	});

	it("updates existing element attributes", () => {
		const { start, end } = setup_boundary("meta");

		apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "Old" })],
			[],
		);

		apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "New" })],
			[],
		);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
		expect(els[0]!.getAttribute("content")).toBe("New");
	});

	it("removes stale elements", () => {
		const { start, end } = setup_boundary("meta");

		apply_head_and_title(
			undefined,
			[
				meta({ name: "description", content: "A page" }),
				meta({ name: "keywords", content: "test" }),
			],
			[],
		);

		apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "A page" })],
			[],
		);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
		expect(els[0]!.getAttribute("name")).toBe("description");
	});

	it("reorders to match server order", () => {
		const { start, end } = setup_boundary("meta");

		apply_head_and_title(
			undefined,
			[meta({ name: "a", content: "1" }), meta({ name: "b", content: "2" })],
			[],
		);

		apply_head_and_title(
			undefined,
			[meta({ name: "b", content: "2" }), meta({ name: "a", content: "1" })],
			[],
		);

		const els = elements_between(start, end);
		expect(els).toHaveLength(2);
		expect(els[0]!.getAttribute("name")).toBe("b");
		expect(els[1]!.getAttribute("name")).toBe("a");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Rest section
/////////////////////////////////////////////////////////////////////

describe("rest section", () => {
	it("adds script and style elements", () => {
		setup_boundary("meta");
		const { start, end } = setup_boundary("rest");

		apply_head_and_title(
			undefined,
			[],
			[script({ src: "/app.js" }), style("body { color: red; }")],
		);

		const els = elements_between(start, end);
		expect(els).toHaveLength(2);
		expect(els[0]!.tagName).toBe("SCRIPT");
		expect(els[0]!.getAttribute("src")).toBe("/app.js");
		expect(els[1]!.tagName).toBe("STYLE");
		expect(els[1]!.innerHTML).toBe("body { color: red; }");
	});

	it("removes stale rest elements", () => {
		setup_boundary("meta");
		const { start, end } = setup_boundary("rest");

		apply_head_and_title(
			undefined,
			[],
			[script({ src: "/a.js" }), script({ src: "/b.js" })],
		);

		apply_head_and_title(undefined, [], [script({ src: "/a.js" })]);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
		expect(els[0]!.getAttribute("src")).toBe("/a.js");
	});

	it("sets dangerous_inner_html for style content", () => {
		setup_boundary("meta");
		const { start, end } = setup_boundary("rest");

		apply_head_and_title(undefined, [], [style(".cls { display: flex; }")]);

		const els = elements_between(start, end);
		expect(els[0]!.innerHTML).toBe(".cls { display: flex; }");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Fingerprint matching
/////////////////////////////////////////////////////////////////////

describe("fingerprint matching", () => {
	it("reuses matching DOM nodes regardless of attribute order", () => {
		const { start, end } = setup_boundary("meta");

		apply_head_and_title(undefined, [meta({ name: "x", content: "y" })], []);

		const original = elements_between(start, end)[0]!;

		// Same attributes, conceptually same element
		apply_head_and_title(undefined, [meta({ content: "y", name: "x" })], []);

		const after = elements_between(start, end)[0]!;
		expect(after).toBe(original);
	});

	it("collapses duplicates with same fingerprint", () => {
		const { start, end } = setup_boundary("meta");

		apply_head_and_title(
			undefined,
			[meta({ name: "x", content: "y" }), meta({ name: "x", content: "y" })],
			[],
		);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Idempotency
/////////////////////////////////////////////////////////////////////

describe("idempotency", () => {
	it("repeated identical updates do not create new DOM nodes", () => {
		const { start, end } = setup_boundary("meta");

		const els_def = [
			meta({ name: "description", content: "Same" }),
			meta({ property: "og:title", content: "Same OG" }),
		];

		apply_head_and_title(undefined, els_def, []);
		const first_pass = elements_between(start, end).slice();

		apply_head_and_title(undefined, els_def, []);
		const second_pass = elements_between(start, end);

		expect(second_pass).toHaveLength(first_pass.length);
		for (let i = 0; i < first_pass.length; i++) {
			expect(second_pass[i]).toBe(first_pass[i]);
		}
	});
});

/////////////////////////////////////////////////////////////////////
/////// Section isolation
/////////////////////////////////////////////////////////////////////

describe("section isolation", () => {
	it("meta changes do not affect rest section", () => {
		setup_boundary("meta");
		const { start: rs, end: re } = setup_boundary("rest");

		apply_head_and_title(undefined, [], [script({ src: "/app.js" })]);

		apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "New" })],
			[script({ src: "/app.js" })],
		);

		const rest_els = elements_between(rs, re);
		expect(rest_els).toHaveLength(1);
		expect(rest_els[0]!.getAttribute("src")).toBe("/app.js");
	});

	it("rest changes do not affect meta section", () => {
		const { start: ms, end: me } = setup_boundary("meta");
		setup_boundary("rest");

		apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "Stay" })],
			[],
		);

		apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "Stay" })],
			[script({ src: "/new.js" })],
		);

		const meta_els = elements_between(ms, me);
		expect(meta_els).toHaveLength(1);
		expect(meta_els[0]!.getAttribute("name")).toBe("description");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Text node cleanup
/////////////////////////////////////////////////////////////////////

describe("text node cleanup", () => {
	it("removes interleaved whitespace text nodes during reconciliation", () => {
		const { start, end } = setup_boundary("meta");

		// Manually insert text nodes between markers
		const text1 = document.createTextNode("\n  ");
		const text2 = document.createTextNode("  \n");
		const existing = document.createElement("meta");
		existing.setAttribute("name", "old");
		document.head.insertBefore(text1, end);
		document.head.insertBefore(existing, end);
		document.head.insertBefore(text2, end);

		apply_head_and_title(undefined, [meta({ name: "new", content: "val" })], []);

		// Walk between markers: should only find element nodes
		let node: Node | null = start.nextSibling;
		while (node && node !== end) {
			expect(node.nodeType).toBe(Node.ELEMENT_NODE);
			node = node.nextSibling;
		}

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
		expect(els[0]!.getAttribute("name")).toBe("new");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Boolean attributes
/////////////////////////////////////////////////////////////////////

describe("boolean attributes", () => {
	it("sets async and defer as empty-string attributes", () => {
		setup_boundary("meta");
		const { start, end } = setup_boundary("rest");

		apply_head_and_title(
			undefined,
			[],
			[script({ src: "/app.js" }, { boolean_attributes: ["async", "defer"] })],
		);

		const el = elements_between(start, end)[0]!;
		expect(el.hasAttribute("async")).toBe(true);
		expect(el.getAttribute("async")).toBe("");
		expect(el.hasAttribute("defer")).toBe(true);
		expect(el.getAttribute("defer")).toBe("");
	});

	it("handles missing boolean_attributes field gracefully", () => {
		setup_boundary("meta");
		const { start, end } = setup_boundary("rest");

		const el: HeadEl = {
			tag: "script",
			attributes_known_safe: { src: "/app.js" },
			// boolean_attributes intentionally omitted
		};

		apply_head_and_title(undefined, [], [el]);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
		expect(els[0]!.getAttribute("src")).toBe("/app.js");
	});

	it("handles absent boolean_attributes field gracefully", () => {
		setup_boundary("meta");
		const { start, end } = setup_boundary("rest");

		// The server omits empty fields entirely; absence is the wire-real case.
		const el: HeadEl = {
			tag: "script",
			attributes_known_safe: { src: "/app.js" },
		};

		apply_head_and_title(undefined, [], [el]);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Invalid/missing boundary markers
/////////////////////////////////////////////////////////////////////

describe("invalid/missing boundary markers", () => {
	it("returns err when meta markers are missing", () => {
		const res = apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "test" })],
			[],
		);
		expect(res.ok).toBe(false);
	});

	it("returns err when rest markers are missing", () => {
		const res = apply_head_and_title(undefined, [], [script({ src: "/app.js" })]);
		expect(res.ok).toBe(false);
	});

	it("does not corrupt surrounding DOM when markers are missing", () => {
		const existing = document.createElement("link");
		existing.rel = "icon";
		existing.href = "/favicon.ico";
		document.head.appendChild(existing);

		const res = apply_head_and_title(
			"Title",
			[meta({ name: "description", content: "test" })],
			[script({ src: "/app.js" })],
		);
		expect(res.ok).toBe(false);

		expect(document.head.querySelector('link[rel="icon"]')).toBe(existing);
	});

	it("returns err when only start marker exists", () => {
		document.head.appendChild(document.createComment(" vorma-meta-start "));

		const res = apply_head_and_title(
			undefined,
			[meta({ name: "description", content: "test" })],
			[],
		);

		expect(res.ok).toBe(false);
	});

	it("returns error for elements with empty tag", () => {
		const { start, end } = setup_boundary("meta");

		const res = apply_head_and_title(
			undefined,
			[
				{ tag: "", attributes_known_safe: {} },
				meta({ name: "valid", content: "yes" }),
			],
			[],
		);

		expect(res.ok).toBe(false);

		const els = elements_between(start, end);
		expect(els).toHaveLength(1);
		expect(els[0]!.getAttribute("name")).toBe("valid");
	});
});
