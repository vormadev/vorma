// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from "vitest";
import { CSS_BUNDLE_ATTR } from "./constants.ts";
import { apply_css_bundles, preload_css, wait_for_css } from "./css.ts";

beforeEach(() => {
	document.head.innerHTML = "";
});

/////////////////////////////////////////////////////////////////////
/////// preload_css
/////////////////////////////////////////////////////////////////////

describe("preload_css", () => {
	it("creates preload link elements", () => {
		preload_css(["/styles/a.css"]);

		const link = document.head.querySelector(
			'link[data-vorma-css-preload="/styles/a.css"]',
		) as HTMLLinkElement;
		expect(link).not.toBeNull();
		expect(link.rel).toBe("preload");
		expect(link.getAttribute("as")).toBe("style");
		expect(link.href).toContain("/styles/a.css");
	});

	it("creates multiple preload links", () => {
		preload_css(["/a.css", "/b.css"]);

		const links = document.head.querySelectorAll(
			"link[data-vorma-css-preload]",
		);
		expect(links).toHaveLength(2);
	});

	it("deduplicates same path", () => {
		preload_css(["/a.css", "/a.css"]);

		const links = document.head.querySelectorAll(
			'link[data-vorma-css-preload="/a.css"]',
		);
		expect(links).toHaveLength(1);
	});

	it("deduplicates across multiple calls", () => {
		preload_css(["/a.css"]);
		preload_css(["/a.css"]);

		const links = document.head.querySelectorAll(
			'link[data-vorma-css-preload="/a.css"]',
		);
		expect(links).toHaveLength(1);
	});

	it("marks settled on load", () => {
		preload_css(["/a.css"]);

		const link = document.head.querySelector(
			'link[data-vorma-css-preload="/a.css"]',
		) as HTMLLinkElement;
		expect(link.getAttribute("data-vorma-css-settled")).toBe("0");

		link.dispatchEvent(new Event("load"));
		expect(link.getAttribute("data-vorma-css-settled")).toBe("1");
	});

	it("marks settled on error", () => {
		preload_css(["/a.css"]);

		const link = document.head.querySelector(
			'link[data-vorma-css-preload="/a.css"]',
		) as HTMLLinkElement;
		expect(link.getAttribute("data-vorma-css-settled")).toBe("0");

		link.dispatchEvent(new Event("error"));
		expect(link.getAttribute("data-vorma-css-settled")).toBe("1");
	});
});

/////////////////////////////////////////////////////////////////////
/////// wait_for_css
/////////////////////////////////////////////////////////////////////

describe("wait_for_css", () => {
	it("resolves when all preloads settle", async () => {
		preload_css(["/a.css", "/b.css"]);

		const ac = new AbortController();
		const promise = wait_for_css(["/a.css", "/b.css"], ac.signal);

		const linkA = document.head.querySelector(
			'link[data-vorma-css-preload="/a.css"]',
		) as HTMLLinkElement;
		const linkB = document.head.querySelector(
			'link[data-vorma-css-preload="/b.css"]',
		) as HTMLLinkElement;

		linkA.dispatchEvent(new Event("load"));
		linkB.dispatchEvent(new Event("load"));

		await promise;
	});

	it("resolves immediately for already-settled preloads", async () => {
		preload_css(["/a.css"]);

		const link = document.head.querySelector(
			'link[data-vorma-css-preload="/a.css"]',
		) as HTMLLinkElement;
		link.dispatchEvent(new Event("load"));

		const ac = new AbortController();
		await wait_for_css(["/a.css"], ac.signal);
	});

	it("resolves immediately when no matching preloads exist", async () => {
		const ac = new AbortController();
		await wait_for_css(["/nonexistent.css"], ac.signal);
	});

	it("resolves immediately for empty bundle list", async () => {
		const ac = new AbortController();
		await wait_for_css([], ac.signal);
	});

	it("respects abort signal", async () => {
		preload_css(["/a.css"]);

		const ac = new AbortController();
		const promise = wait_for_css(["/a.css"], ac.signal);

		ac.abort();
		await promise;
	});

	it("resolves immediately when signal is already aborted", async () => {
		preload_css(["/a.css"]);

		const ac = new AbortController();
		ac.abort();
		await wait_for_css(["/a.css"], ac.signal);
	});

	it("resolves with mix of settled and pending preloads", async () => {
		preload_css(["/a.css", "/b.css"]);

		const linkA = document.head.querySelector(
			'link[data-vorma-css-preload="/a.css"]',
		) as HTMLLinkElement;
		linkA.dispatchEvent(new Event("load"));

		const ac = new AbortController();
		const promise = wait_for_css(["/a.css", "/b.css"], ac.signal);

		const linkB = document.head.querySelector(
			'link[data-vorma-css-preload="/b.css"]',
		) as HTMLLinkElement;
		linkB.dispatchEvent(new Event("load"));

		await promise;
	});
});

/////////////////////////////////////////////////////////////////////
/////// apply_css_bundles
/////////////////////////////////////////////////////////////////////

describe("apply_css_bundles", () => {
	it("creates stylesheet link elements", () => {
		apply_css_bundles(["/styles/a.css"]);

		const link = document.head.querySelector(
			'link[data-vorma-css-bundle="/styles/a.css"]',
		) as HTMLLinkElement;
		expect(link).not.toBeNull();
		expect(link.rel).toBe("stylesheet");
		expect(link.href).toContain("/styles/a.css");
	});

	it("creates multiple stylesheet links", () => {
		apply_css_bundles(["/a.css", "/b.css"]);

		const links = document.head.querySelectorAll(
			"link[data-vorma-css-bundle]",
		);
		expect(links).toHaveLength(2);
	});

	it("deduplicates same path", () => {
		apply_css_bundles(["/a.css", "/a.css"]);

		const links = document.head.querySelectorAll(
			'link[data-vorma-css-bundle="/a.css"]',
		);
		expect(links).toHaveLength(1);
	});

	it("deduplicates across multiple calls", () => {
		apply_css_bundles(["/a.css"]);
		apply_css_bundles(["/a.css"]);

		const links = document.head.querySelectorAll(
			'link[data-vorma-css-bundle="/a.css"]',
		);
		expect(links).toHaveLength(1);
	});

	it("deduplicates server-rendered bundle links", () => {
		const link = document.createElement("link");
		link.rel = "stylesheet";
		link.setAttribute(CSS_BUNDLE_ATTR, "/a.css");
		link.href = "/a.css";
		document.head.appendChild(link);

		apply_css_bundles(["/a.css"]);

		const links = document.head.querySelectorAll(
			'link[data-vorma-css-bundle="/a.css"]',
		);
		expect(links).toHaveLength(1);
	});
});
