export function preload_css(bundles: string[]): void {
	for (const path of new Set(bundles)) {
		if (
			document.head.querySelector(
				`link[data-vorma-css-preload="${path}"]`,
			)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "preload";
		link.setAttribute("as", "style");
		link.setAttribute("data-vorma-css-preload", path);
		link.setAttribute("data-vorma-css-settled", "0");
		const mark = () => {
			link.setAttribute("data-vorma-css-settled", "1");
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
			`link[data-vorma-css-preload="${path}"]`,
		);
		if (!node || node.getAttribute("data-vorma-css-settled") === "1") {
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
					node.setAttribute("data-vorma-css-settled", "1");
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
		if (
			document.head.querySelector(`link[data-vorma-css-bundle="${path}"]`)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "stylesheet";
		link.setAttribute("data-vorma-css-bundle", path);
		link.href = path;
		document.head.appendChild(link);
	}
}
