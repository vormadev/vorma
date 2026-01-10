import { createEffect, onCleanup } from "solid-js";
import { render } from "solid-js/web";
import { getHrefDetails } from "vorma/kit/url";
import { VormaLink } from "vorma/solid";
import { highlight } from "../highlight.ts";
import { waveRuntimeURL } from "../vorma.gen/index.ts";

export function RenderedMarkdown(props: { markdown: string }) {
	let containerRef: HTMLDivElement | null = null;
	const disposers: Array<() => void> = [];

	// Cleanup function to remove any previously rendered components
	const cleanupPreviousRender = () => {
		disposers.forEach((dispose) => dispose());
		disposers.length = 0;
	};

	// Process the markdown content
	const processContent = () => {
		if (!containerRef) {
			return;
		}

		cleanupPreviousRender();

		containerRef.innerHTML = props.markdown; // Set the HTML content

		// Process headings to add anchor links
		const headings = containerRef.querySelectorAll(
			"h1, h2, h3, h4, h5, h6",
		);
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
			processContent();
		}
	};

	// Create effect to run processContent when markdown changes
	createEffect(() => {
		props.markdown; // Access props.markdown to track changes
		if (containerRef) {
			processContent();
		}
	});

	onCleanup(cleanupPreviousRender); // Clean up all disposers when component unmounts

	return <div ref={ref} class="content" />;
}
