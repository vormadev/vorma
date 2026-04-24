import {
	CSS_BUNDLE_ATTR,
	CSS_PRELOAD_ATTR,
	CSS_PRELOAD_SETTLED_ATTR,
} from "./constants.ts";

export function preload_css(bundles: string[]): void {
	for (const path of new Set(bundles)) {
		if (
			document.head.querySelector(`link[${CSS_PRELOAD_ATTR}="${path}"]`)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "preload";
		link.setAttribute("as", "style");
		link.setAttribute(CSS_PRELOAD_ATTR, path);
		link.setAttribute(CSS_PRELOAD_SETTLED_ATTR, "0");
		const mark = () => {
			link.setAttribute(CSS_PRELOAD_SETTLED_ATTR, "1");
		};
		link.addEventListener("load", mark);
		link.addEventListener("error", mark);
		link.href = path;
		document.head.appendChild(link);
	}
}

export async function wait_for_css(
	bundles: string[],
	signal: AbortSignal,
): Promise<void> {
	const promises: Promise<void>[] = [];

	for (const path of new Set(bundles)) {
		const node = document.head.querySelector<HTMLLinkElement>(
			`link[${CSS_PRELOAD_ATTR}="${path}"]`,
		);
		if (!node || node.getAttribute(CSS_PRELOAD_SETTLED_ATTR) === "1") {
			continue;
		}
		promises.push(
			new Promise<void>((resolve) => {
				if (signal.aborted) {
					resolve();
					return;
				}
				const cleanup = () => {
					node.removeEventListener("load", done);
					node.removeEventListener("error", done);
					signal.removeEventListener("abort", on_abort);
				};
				const done = () => {
					node.setAttribute(CSS_PRELOAD_SETTLED_ATTR, "1");
					cleanup();
					resolve();
				};
				const on_abort = () => {
					cleanup();
					resolve();
				};
				node.addEventListener("load", done, { once: true });
				node.addEventListener("error", done, { once: true });
				signal.addEventListener("abort", on_abort, { once: true });
			}),
		);
	}

	if (promises.length > 0) {
		await Promise.all(promises);
	}
}

export function apply_css_bundles(bundles: string[]): void {
	for (const path of new Set(bundles)) {
		if (find_css_bundle(path)) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "stylesheet";
		link.setAttribute(CSS_BUNDLE_ATTR, path);
		link.href = path;
		document.head.appendChild(link);
	}
}

function find_css_bundle(path: string): HTMLLinkElement | null {
	return document.head.querySelector<HTMLLinkElement>(
		`link[${CSS_BUNDLE_ATTR}="${path}"]`,
	);
}
