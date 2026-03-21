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
	return `${tag.toLowerCase()}::${sorted}::${inner}`;
}

function el_fp(el: Element): string {
	const a: Record<string, string> = {};
	for (const attr of Array.from(el.attributes)) {
		a[attr.name] = attr.value;
	}
	return fingerprint(el.tagName, a, el.innerHTML);
}

function block_fp(b: HeadEl): string {
	return fingerprint(b.tag, build_attr_map(b), b.dangerousInnerHTML ?? "");
}

export function reconcile_head(type: HeadSection, blocks: HeadEl[]): void {
	const { start, end } = find_boundary_comments(type);
	if (!start || !end) {
		return;
	}
	const parent = end.parentNode!;

	const existing: Element[] = [];
	let node: Node | null = start.nextSibling;
	while (node && node !== end) {
		if (node.nodeType === Node.ELEMENT_NODE) {
			existing.push(node as Element);
		}
		node = node.nextSibling;
	}

	const used = new Set<Element>();
	const final_els: Element[] = [];

	for (const block of blocks) {
		const fp = block_fp(block);
		const matched = existing.find(
			(el) => !used.has(el) && el_fp(el) === fp,
		);
		const el = matched ?? document.createElement(block.tag);
		if (matched) {
			used.add(matched);
		}

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

	for (const el of final_els) {
		parent.insertBefore(el, end);
	}

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
		document.title = artifacts.title;
	}
	reconcile_head("meta", artifacts.meta_head_els);
	reconcile_head("rest", artifacts.rest_head_els);
}
