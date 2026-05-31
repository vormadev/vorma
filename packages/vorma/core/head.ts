import { R, type Result } from "vorma/kit/result";

export type HeadEl = {
	tag: string;
	attributes_known_safe: Record<string, string>;
	boolean_attributes?: string[] | null;
	dangerous_inner_html?: string;
};

type HeadSection = "meta" | "rest";

export function apply_head_and_title(
	title: string | undefined,
	meta_els: HeadEl[],
	rest_els: HeadEl[],
): Result<void> {
	try {
		if (title !== undefined) {
			if (title !== "" || document.head.querySelector("title")) {
				document.title = title;
			}
		}
	} catch (error) {
		return R.err(error instanceof Error ? error.message : String(error));
	}

	const meta_res = reconcile_section("meta", meta_els);
	if (!meta_res.ok) {
		return R.err(meta_res.err);
	}
	const res_res = reconcile_section("rest", rest_els);
	if (!res_res.ok) {
		return R.err(res_res.err);
	}

	return R.ok(undefined);
}

function reconcile_section(section: HeadSection, els: HeadEl[]): Result<void> {
	const boundary_res = find_boundary_comments(section);
	if (!boundary_res.ok) {
		return R.err(boundary_res.err);
	}
	const { start, end } = boundary_res.val;
	if (!end.parentNode) {
		return R.err(`Invalid head section boundaries for section "${section}"`);
	}
	const parent = end.parentNode;
	let first_error: string | undefined;

	try {
		const current_elements: Element[] = [];
		let node: Node | null = start.nextSibling;
		while (node && node !== end) {
			if (node.nodeType === Node.ELEMENT_NODE) {
				current_elements.push(node as Element);
			}
			node = node.nextSibling;
		}

		const new_elements: Element[] = [];
		const new_element_fingerprints = new Map<string, Element>();

		for (const block of els) {
			if (!block.tag) {
				first_error ??= `Invalid head element tag for section "${section}"`;
				continue;
			}
			const el = document.createElement(block.tag);
			if (block.attributes_known_safe) {
				for (const [key, value] of Object.entries(block.attributes_known_safe)) {
					el.setAttribute(key, value);
				}
			}
			if (block.boolean_attributes) {
				for (const key of block.boolean_attributes) {
					el.setAttribute(key, "");
				}
			}
			if (block.dangerous_inner_html) {
				el.innerHTML = block.dangerous_inner_html;
			}

			const fp = fingerprint_element(el);

			if (new_element_fingerprints.has(fp)) {
				const prev = new_element_fingerprints.get(fp)!;
				const idx = new_elements.indexOf(prev);
				if (idx > -1) {
					new_elements.splice(idx, 1);
				}
			}
			new_elements.push(el);
			new_element_fingerprints.set(fp, el);
		}

		const current_by_fp = new Map<string, Element[]>();
		for (const el of current_elements) {
			const fp = fingerprint_element(el);
			const list = current_by_fp.get(fp);
			if (list) {
				list.push(el);
			} else {
				current_by_fp.set(fp, [el]);
			}
		}

		const final_elements: Element[] = [];
		const used = new Set<Element>();

		for (const new_el of new_elements) {
			const fp = fingerprint_element(new_el);
			const candidates = current_by_fp.get(fp) ?? [];
			const matched = candidates.find((el) => {
				return !used.has(el);
			});

			if (matched) {
				used.add(matched);
				final_elements.push(matched);
			} else {
				final_elements.push(new_el);
			}
		}

		const remaining = new Set(current_elements);
		for (const el of current_elements) {
			if (!used.has(el)) {
				parent.removeChild(el);
				remaining.delete(el);
			}
		}

		node = start.nextSibling;
		while (node && node !== end) {
			const next = node.nextSibling;
			if (node.nodeType !== Node.ELEMENT_NODE) {
				parent.removeChild(node);
			}
			node = next;
		}

		let last_processed: Element | null = null;

		for (const element of final_elements) {
			const is_existing = used.has(element);

			if (is_existing) {
				const expected_next: Element | null = last_processed
					? last_processed.nextElementSibling
					: start.nextElementSibling;

				if (expected_next !== element) {
					parent.insertBefore(element, (expected_next as Node) ?? end);
				}
				remaining.delete(element);
			} else {
				const insert_before = last_processed
					? last_processed.nextSibling
					: start.nextSibling;
				parent.insertBefore(element, insert_before ?? end);
			}

			last_processed = element;
		}

		if (first_error) {
			return R.err(first_error);
		}
		return R.ok(undefined);
	} catch (error) {
		return R.err(error instanceof Error ? error.message : String(error));
	}
}

function find_boundary_comments(section: HeadSection): Result<{
	start: Comment;
	end: Comment;
}> {
	const start_text = `vorma-${section}-start`;
	const end_text = `vorma-${section}-end`;
	let start: Comment | undefined;
	for (const node of Array.from(document.head.childNodes)) {
		if (node.nodeType !== Node.COMMENT_NODE) {
			continue;
		}
		const val = (node as Comment).nodeValue?.trim();
		if (val === start_text) {
			start = node as Comment;
		} else if (val === end_text && start) {
			return R.ok({ start, end: node as Comment });
		}
	}
	return R.err(`Could not find head boundary comments for section "${section}"`);
}

function fingerprint_element(element: Element): string {
	const attrs: string[] = [];
	for (let i = 0; i < element.attributes.length; i++) {
		const attr = element.attributes[i]!;
		const value =
			element.hasAttribute(attr.name) && attr.value === "" ? "" : attr.value;
		attrs.push(`${attr.name}="${value}"`);
	}
	attrs.sort();
	return `${element.tagName.toUpperCase()}|${attrs.join(",")}|${(element.innerHTML || "").trim()}`;
}
