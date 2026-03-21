import { createEffect, onCleanup } from "solid-js";
import { render } from "solid-js/web";
import { getHrefDetails } from "vorma/kit/url";
import { VormaLink } from "vorma/solid";
import { waveRuntimeURL } from "../../../__wave/vorma.gen/index.ts";
import { highlight } from "../highlight.ts";

export function RenderedMarkdown(props: {
	markdown: string;
	stripLeadingH1?: boolean;
}) {
	let containerRef: HTMLDivElement | null = null;
	const disposers: Array<() => void> = [];

	// Cleanup function to remove any previously rendered components
	const cleanupPreviousRender = () => {
		disposers.forEach((dispose) => dispose());
		disposers.length = 0;
	};

	// Process the markdown content
	const processContent = (content: {
		markdown: string;
		stripLeadingH1: boolean;
	}) => {
		if (!containerRef) {
			return;
		}

		cleanupPreviousRender();

		containerRef.innerHTML = content.markdown; // Set the HTML content

		if (content.stripLeadingH1) {
			for (const node of Array.from(containerRef.childNodes)) {
				if (
					node.nodeType === Node.TEXT_NODE &&
					(node.textContent ?? "").trim() === ""
				) {
					continue;
				}
				if (node.nodeType === Node.COMMENT_NODE) {
					continue;
				}
				if (node.nodeType === Node.ELEMENT_NODE) {
					const el = node as Element;
					if (el.tagName.toLowerCase() === "h1") {
						el.remove();
					}
				}
				break;
			}
		}

		// Process headings to add anchor links
		const headings = containerRef.querySelectorAll("h2, h3, h4, h5, h6");
		for (const heading of headings) {
			const id = heading.id;
			if (id) {
				const text = heading.textContent || "";
				heading.textContent = "";
				heading.classList.add("anchor-heading");

				const anchor = document.createElement("a");
				anchor.href = `#${id}`;
				anchor.textContent = "#";
				anchor.setAttribute("aria-label", `Link to ${text}`);
				anchor.classList.add("anchor");
				heading.appendChild(anchor);

				const textNode = document.createElement("span");
				textNode.textContent = text;
				heading.appendChild(textNode);
			}
		}

		// Process code blocks
		const codeBlocks = containerRef.querySelectorAll("pre code");
		for (const codeBlock of codeBlocks) {
			highlight.highlightElement(codeBlock as HTMLElement);
		}

		// Process links
		for (const link of containerRef.querySelectorAll("a")) {
			// Skip anchor links we just created
			if (link.parentElement?.classList.contains("anchor-heading")) {
				continue;
			}

			const hrefDetails = getHrefDetails(link.href);

			if (hrefDetails.isHTTP && hrefDetails.isExternal) {
				link.dataset.external = "true";
				link.target = "_blank";
				link.rel = "noopener noreferrer";
			} else {
				const href = link.href;
				const label = link.innerText;
				const placeholder = document.createElement("span");
				link.parentNode?.replaceChild(placeholder, link);

				const dispose = render(
					() => (
						<VormaLink prefetch="intent" href={href}>
							{label}
						</VormaLink>
					),
					placeholder,
				);
				disposers.push(dispose);
			}
		}

		// Process images
		for (const img of containerRef.querySelectorAll("img")) {
			// if data-src is set, grab value
			const src = img.getAttribute("data-src");
			if (src) {
				img.src = waveRuntimeURL(src as any);
				img.removeAttribute("data-src");
			}

			const width = img.getAttribute("data-width");
			const height = img.getAttribute("data-height");
			if (width && height) {
				img.style.aspectRatio = `${width}/${height}`;
			}
		}
	};

	// Set up ref callback to store the container element
	const ref = (el: HTMLDivElement | null) => {
		containerRef = el;
		if (el) {
			processContent({
				markdown: props.markdown,
				stripLeadingH1: props.stripLeadingH1 === true,
			});
		}
	};

	// Create effect to run processContent when markdown changes
	createEffect(() => {
		const markdown = props.markdown;
		const stripLeadingH1 = props.stripLeadingH1 === true;
		if (containerRef) {
			processContent({
				markdown,
				stripLeadingH1,
			});
		}
	});

	onCleanup(cleanupPreviousRender); // Clean up all disposers when component unmounts

	return <div ref={ref} class="content" />;
}
