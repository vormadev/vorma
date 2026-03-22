/// <reference types="vite/client" />

import type { HeadEl, NavigationArtifacts } from "./types.ts";

type HeadSection = "meta" | "rest";

function find_boundary_comments(type: HeadSection): {
	start: Comment | undefined;
	end: Comment | undefined;
} {
	const st = `data-vorma="${type}-start"`;
	const et = `data-vorma="${type}-end"`;
	let start: Comment | undefined;
	for (const node of Array.from(document.head.childNodes)) {
		if (node.nodeType !== Node.COMMENT_NODE) {
			continue;
		}
		const val = (node as Comment).nodeValue?.trim();
		if (val === st) {
			start = node as Comment;
		} else if (val === et && start) {
			return { start, end: node as Comment };
		}
	}
	return { start: undefined, end: undefined };
}

function build_attr_map(block: HeadEl): Record<string, string> {
	const m: Record<string, string> = { ...block.attributesKnownSafe };
	for (const k of block.booleanAttributes ?? []) {
		m[k] = "";
	}
	return m;
}

function fingerprint(
	tag: string,
	attrs: Record<string, string>,
	inner: string,
): string {
	const sorted = Object.entries(attrs)
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([k, v]) => `${k}=${v}`)
		.join("|");
	return `fp::${tag.toLowerCase()}::${sorted}::${inner}`;
}

function semantic_key(
	tag: string,
	attrs: Record<string, string>,
): string | null {
	const t = tag.toLowerCase();
	if (t === "meta") {
		if (attrs["name"]) {
			return `meta::name=${attrs["name"]}`;
		}
		if (attrs["property"]) {
			return `meta::property=${attrs["property"]}`;
		}
		if (attrs["http-equiv"]) {
			return `meta::http-equiv=${attrs["http-equiv"]}`;
		}
		if ("charset" in attrs) {
			return "meta::charset";
		}
	}
	if (t === "link") {
		if (attrs["rel"]) {
			return `link::rel=${attrs["rel"]}`;
		}
	}
	if (t === "script") {
		if (attrs["src"]) {
			return `script::src=${attrs["src"]}`;
		}
	}
	return null;
}

function match_key_for_el(el: Element): string {
	const attrs: Record<string, string> = {};
	for (const attr of Array.from(el.attributes)) {
		attrs[attr.name] = attr.value;
	}
	const tag = el.tagName.toLowerCase();
	return semantic_key(tag, attrs) ?? fingerprint(tag, attrs, el.innerHTML);
}

function match_key_for_block(block: HeadEl): string {
	const attrs = build_attr_map(block);
	const tag = block.tag.toLowerCase();
	return (
		semantic_key(tag, attrs) ??
		fingerprint(tag, attrs, block.dangerousInnerHTML ?? "")
	);
}

export function reconcile_head(type: HeadSection, blocks: HeadEl[]): void {
	const { start, end } = find_boundary_comments(type);
	if (!start || !end) {
		throw new Error(`Missing managed head markers for section "${type}".`);
	}
	const parent = end.parentNode!;

	// Collect existing elements between markers
	const existing: Element[] = [];
	let node: Node | null = start.nextSibling;
	while (node && node !== end) {
		if (node.nodeType === Node.ELEMENT_NODE) {
			existing.push(node as Element);
		}
		node = node.nextSibling;
	}

	// Build a map of existing elements by match key
	const existing_by_key = new Map<string, Element[]>();
	for (const el of existing) {
		const key = match_key_for_el(el);
		const list = existing_by_key.get(key);
		if (list) {
			list.push(el);
		} else {
			existing_by_key.set(key, [el]);
		}
	}

	const used = new Set<Element>();
	const final_els: Element[] = [];

	for (const block of blocks) {
		const key = match_key_for_block(block);
		const candidates = existing_by_key.get(key);
		const matched = candidates?.find((el) => !used.has(el));

		let el: Element;
		if (matched) {
			used.add(matched);
			el = matched;
		} else {
			el = document.createElement(block.tag);
		}

		// Patch attributes to match desired state
		const desired = build_attr_map(block);
		for (const name of Array.from(el.attributes).map((a) => a.name)) {
			if (!(name in desired)) {
				el.removeAttribute(name);
			}
		}
		for (const [name, val] of Object.entries(desired)) {
			if (el.getAttribute(name) !== val) {
				el.setAttribute(name, val);
			}
		}
		const inner = block.dangerousInnerHTML ?? "";
		if (el.innerHTML !== inner) {
			el.innerHTML = inner;
		}

		final_els.push(el);
	}

	// Place all final elements in order before the end comment
	for (const el of final_els) {
		parent.insertBefore(el, end);
	}

	// Remove everything between start and end that isn't in the final set
	// (including stale elements and text nodes)
	node = start.nextSibling;
	const final_set = new Set(final_els);
	while (node && node !== end) {
		const next = node.nextSibling;
		if (
			node.nodeType !== Node.ELEMENT_NODE ||
			!final_set.has(node as Element)
		) {
			parent.removeChild(node);
		}
		node = next;
	}
}

export function apply_head_and_title(artifacts: NavigationArtifacts): void {
	if (artifacts.title !== undefined) {
		// Only assign when there is a non-empty title to set, or when a
		// <title> element already exists and needs to be cleared.
		// Assigning "" when no <title> exists would create one as a side
		// effect, adding an unexpected node to document.head.
		if (artifacts.title !== "" || document.head.querySelector("title")) {
			document.title = artifacts.title;
		}
	}
	reconcile_head("meta", artifacts.meta_head_els);
	reconcile_head("rest", artifacts.rest_head_els);
}
