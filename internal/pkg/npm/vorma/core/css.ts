import {
	CSS_BUNDLE_ATTR,
	CSS_PRELOAD_ATTR,
	CSS_PRELOAD_SETTLED_ATTR,
} from "./constants.ts";

export function preload_css(bundles: string[]): void {
	for (const path of new Set(bundles)) {
		if (find_css_bundle(path)) {
			continue;
		}
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
		const mark = (): void => {
			link.removeEventListener("load", mark);
			link.removeEventListener("error", mark);
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
	const pending_nodes: HTMLLinkElement[] = [];

	for (const path of new Set(bundles)) {
		const node = document.head.querySelector<HTMLLinkElement>(
			`link[${CSS_PRELOAD_ATTR}="${path}"]`,
		);
		if (!node || node.getAttribute(CSS_PRELOAD_SETTLED_ATTR) === "1") {
			continue;
		}
		pending_nodes.push(node);
	}

	if (signal.aborted || pending_nodes.length === 0) {
		return;
	}

	await new Promise<void>((resolve) => {
		let remaining = pending_nodes.length;
		let settled = false;
		const cleanups: Array<() => void> = [];

		const cleanup = (): void => {
			for (const remove_listener of cleanups.splice(0)) {
				remove_listener();
			}
		};

		const complete = (): void => {
			if (settled) {
				return;
			}
			settled = true;
			cleanup();
			resolve();
		};

		const on_abort = (): void => {
			complete();
		};
		signal.addEventListener("abort", on_abort, { once: true });
		cleanups.push(() => {
			signal.removeEventListener("abort", on_abort);
		});

		for (const node of pending_nodes) {
			const done = (): void => {
				node.setAttribute(CSS_PRELOAD_SETTLED_ATTR, "1");
				remaining -= 1;
				if (remaining === 0) {
					complete();
				}
			};
			node.addEventListener("load", done, { once: true });
			node.addEventListener("error", done, { once: true });
			cleanups.push(() => {
				node.removeEventListener("load", done);
				node.removeEventListener("error", done);
			});
		}
	});
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
