/////////////////////////////////////////////////////////////////////
/////// PREACT
/////////////////////////////////////////////////////////////////////

// import { render } from "preact";
// import { useCallback, useEffect, useRef } from "preact/hooks";
// import { getHrefDetails } from "vorma/kit/url";
// import { highlight } from "../../setup.ts";
// import { app, Link, useLoaderData } from "../../vorma.app.ts";

// export default app.defineRoute({
// 	pattern: "/*",
// 	component: (props) => {
// 		const ld = useLoaderData(props);
// 		return <RenderedMarkdown markdown={ld.Page?.HTML || ""} />;
// 	},
// });

// function RenderedMarkdown(props: {
// 	markdown: string;
// 	stripLeadingH1?: boolean;
// }) {
// 	const containerRef = useRef<HTMLDivElement>(null);
// 	const disposersRef = useRef<Array<() => void>>([]);

// 	const cleanupPreviousRender = useCallback(() => {
// 		disposersRef.current.forEach((dispose) => dispose());
// 		disposersRef.current.length = 0;
// 	}, []);

// 	const processContent = useCallback(
// 		(content: { markdown: string; stripLeadingH1: boolean }) => {
// 			const container = containerRef.current;
// 			if (!container) return;
// 			cleanupPreviousRender();
// 			container.innerHTML = content.markdown;
// 			processHeadings(container);
// 			processCodeBlocks(container);
// 			processLinks(container, disposersRef.current);
// 		},
// 		[cleanupPreviousRender],
// 	);

// 	useEffect(() => {
// 		if (containerRef.current) {
// 			processContent({
// 				markdown: props.markdown,
// 				stripLeadingH1: props.stripLeadingH1 === true,
// 			});
// 		}
// 		return cleanupPreviousRender;
// 	}, [
// 		props.markdown,
// 		props.stripLeadingH1,
// 		processContent,
// 		cleanupPreviousRender,
// 	]);

// 	return <div ref={containerRef} class="content" />;
// }

// function processLinks(container: HTMLElement, disposers: Array<() => void>) {
// 	for (const link of container.querySelectorAll("a")) {
// 		if (link.parentElement?.classList.contains("anchor-heading")) continue;
// 		const hrefDetails = getHrefDetails(link.href);
// 		if (hrefDetails.isHTTP && hrefDetails.isExternal) {
// 			link.dataset.external = "true";
// 			link.target = "_blank";
// 			link.rel = "noopener noreferrer";
// 		} else {
// 			const href = link.href;
// 			const label = link.innerText;
// 			const placeholder = document.createElement("span");
// 			link.parentNode?.replaceChild(placeholder, link);
// 			const url = new URL(href);
// 			const path = url.pathname + url.search + url.hash;
// 			render(
// 				<Link
// 					prefetch="intent"
// 					pattern={"/*"}
// 					splatValues={path.split("/")}
// 				>
// 					{label}
// 				</Link>,
// 				placeholder,
// 			);
// 			disposers.push(() => render(null, placeholder));
// 		}
// 	}
// }

/////////////////////////////////////////////////////////////////////
/////// REACT
/////////////////////////////////////////////////////////////////////

// import { useCallback, useEffect, useRef } from "react";
// import { createRoot } from "react-dom/client";
// import { getHrefDetails } from "vorma/kit/url";
// import { highlight } from "../../setup.ts";
// import { app, Link, useLoaderData } from "../../vorma.app.ts";

// export default app.defineRoute({
// 	pattern: "/*",
// 	component: (props) => {
// 		const ld = useLoaderData(props);
// 		return <RenderedMarkdown markdown={ld.Page?.HTML || ""} />;
// 	},
// });

// function RenderedMarkdown(props: {
// 	markdown: string;
// 	stripLeadingH1?: boolean;
// }) {
// 	const containerRef = useRef<HTMLDivElement>(null);
// 	const disposersRef = useRef<Array<() => void>>([]);

// 	const cleanupPreviousRender = useCallback(() => {
// 		disposersRef.current.forEach((dispose) => dispose());
// 		disposersRef.current.length = 0;
// 	}, []);

// 	const processContent = useCallback(
// 		(content: { markdown: string; stripLeadingH1: boolean }) => {
// 			const container = containerRef.current;
// 			if (!container) return;
// 			cleanupPreviousRender();
// 			container.innerHTML = content.markdown;
// 			processHeadings(container);
// 			processCodeBlocks(container);
// 			processLinks(container, disposersRef.current);
// 		},
// 		[cleanupPreviousRender],
// 	);

// 	useEffect(() => {
// 		if (containerRef.current) {
// 			processContent({
// 				markdown: props.markdown,
// 				stripLeadingH1: props.stripLeadingH1 === true,
// 			});
// 		}
// 		return cleanupPreviousRender;
// 	}, [
// 		props.markdown,
// 		props.stripLeadingH1,
// 		processContent,
// 		cleanupPreviousRender,
// 	]);

// 	return <div ref={containerRef} className="content" />;
// }

// function processLinks(container: HTMLElement, disposers: Array<() => void>) {
// 	for (const link of container.querySelectorAll("a")) {
// 		if (link.parentElement?.classList.contains("anchor-heading")) continue;
// 		const hrefDetails = getHrefDetails(link.href);
// 		if (hrefDetails.isHTTP && hrefDetails.isExternal) {
// 			link.dataset.external = "true";
// 			link.target = "_blank";
// 			link.rel = "noopener noreferrer";
// 		} else {
// 			const href = link.href;
// 			const label = link.innerText;
// 			const placeholder = document.createElement("span");
// 			link.parentNode?.replaceChild(placeholder, link);
// 			const url = new URL(href);
// 			const path = url.pathname + url.search + url.hash;
// 			const root = createRoot(placeholder);
// 			root.render(
// 				<Link
// 					prefetch="intent"
// 					pattern={"/*"}
// 					splatValues={path.split("/")}
// 				>
// 					{label}
// 				</Link>,
// 			);
// 			disposers.push(() => root.unmount());
// 		}
// 	}
// }

/////////////////////////////////////////////////////////////////////
/////// SOLID
/////////////////////////////////////////////////////////////////////

import { createEffect, onCleanup } from "solid-js";
import { render } from "solid-js/web";
import { getHrefDetails } from "vorma/kit/url";
import { highlight } from "../../setup.ts";
import { app, Link, useLoaderData } from "../../vorma.app.ts";

export default app.defineRoute({
	pattern: "/*",
	component: (props) => {
		const ld = useLoaderData(props);
		let containerRef!: HTMLDivElement;
		const disposers: Array<() => void> = [];

		function cleanup() {
			disposers.forEach((dispose) => dispose());
			disposers.length = 0;
		}

		createEffect(() => {
			const markdown = ld()?.Page?.HTML || "";
			if (!containerRef) {
				return;
			}
			cleanup();
			containerRef.innerHTML = markdown;
			processHeadings(containerRef);
			processCodeBlocks(containerRef);
			processLinksSolid(containerRef, disposers);
		});

		onCleanup(cleanup);

		return <div ref={containerRef} class="content" />;
	},
});

function processLinksSolid(
	container: HTMLElement,
	disposers: Array<() => void>,
) {
	for (const link of container.querySelectorAll("a")) {
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
			const url = new URL(href);
			const path = url.pathname + url.search + url.hash;
			const dispose = render(() => {
				return (
					<Link
						prefetch="intent"
						pattern={"/*"}
						splatValues={path.split("/")}
					>
						{label}
					</Link>
				);
			}, placeholder);
			disposers.push(dispose);
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// SHARED
/////////////////////////////////////////////////////////////////////

function processHeadings(container: HTMLElement) {
	const headings = container.querySelectorAll("h2, h3, h4, h5, h6");
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
}

function processCodeBlocks(container: HTMLElement) {
	const codeBlocks = container.querySelectorAll("pre code");
	for (const codeBlock of codeBlocks) {
		highlight.highlightElement(codeBlock as HTMLElement);
	}
}
