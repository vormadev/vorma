import { createRoot, ref, type RemixNode } from "remix/ui";
import { getHrefDetails } from "vorma/kit/url";
import { defineView, Link, useLoaderData } from "../../app.tsx";
import { highlight } from "../../setup.ts";

const content_class_name = "content";
const content_route_pattern = "/*";

/////////////////////////////////////////////////////////////////////
/////// PREACT
/////////////////////////////////////////////////////////////////////

// import { createPortal } from "preact/compat";
// import { useId, useLayoutEffect, useRef, useState } from "preact/hooks";
// import { defineView, Link, useLoaderData } from "../../app.tsx";

// export default defineView({
// 	pattern: content_route_pattern,
// 	component: (props) => {
// 		const ld = useLoaderData(props);
// 		return <RenderedMarkdown markdown={ld.Page?.HTML || ""} />;
// 	},
// });

// function RenderedMarkdown(props: { markdown: string }) {
// 	const container_ref = useRef<HTMLDivElement>(null);
// 	const [link_portals, set_link_portals] = useState<LinkPortal[]>([]);
// 	const id_prefix = useId();

// 	useLayoutEffect(() => {
// 		const container = container_ref.current;
// 		if (!container) {
// 			set_link_portals([]);
// 			return;
// 		}
// 		set_link_portals(process_markdown_content(container, props.markdown));
// 	}, [props.markdown]);

// 	return (
// 		<>
// 			<div ref={container_ref} className={content_class_name} />
// 			{link_portals.map((portal, i) => {
// 				return createPortal(
// 					<ReactPortalLink portal={portal} />,
// 					portal.container,
// 					`${id_prefix}-link-${i}`,
// 				);
// 			})}
// 		</>
// 	);
// }

/////////////////////////////////////////////////////////////////////
/////// REACT
/////////////////////////////////////////////////////////////////////

// import { useId, useLayoutEffect, useRef, useState } from "react";
// import { createPortal } from "react-dom";
// import { defineView, Link, useLoaderData } from "../../app.tsx";

// export default defineView({
// 	pattern: content_route_pattern,
// 	component: (props) => {
// 		const ld = useLoaderData(props);
// 		return <RenderedMarkdown markdown={ld.Page?.HTML || ""} />;
// 	},
// });

// function RenderedMarkdown(props: { markdown: string }) {
// 	const container_ref = useRef<HTMLDivElement>(null);
// 	const [link_portals, set_link_portals] = useState<LinkPortal[]>([]);
// 	const id_prefix = useId();

// 	useLayoutEffect(() => {
// 		const container = container_ref.current;
// 		if (!container) {
// 			set_link_portals([]);
// 			return;
// 		}
// 		set_link_portals(process_markdown_content(container, props.markdown));
// 	}, [props.markdown]);

// 	return (
// 		<>
// 			<div ref={container_ref} className={content_class_name} />
// 			{link_portals.map((portal, i) => {
// 				return createPortal(
// 					<ReactPortalLink portal={portal} />,
// 					portal.container,
// 					`${id_prefix}-link-${i}`,
// 				);
// 			})}
// 		</>
// 	);
// }

// function ReactPortalLink(props: { portal: LinkPortal }) {
// 	const portal = props.portal;
// 	return (
// 		<Link
// 			aria-label={portal.aria_label}
// 			className={portal.class_name}
// 			href={portal.href}
// 			prefetch="intent"
// 			title={portal.title}
// 			dangerouslySetInnerHTML={{ __html: portal.html }}
// 		/>
// 	);
// }

/////////////////////////////////////////////////////////////////////
/////// SOLID
/////////////////////////////////////////////////////////////////////

// import { createEffect, createSignal, For } from "solid-js";
// import { Portal } from "solid-js/web";
// import { defineView, Link, useLoaderData } from "../../app.tsx";

// export default defineView({
// 	pattern: content_route_pattern,
// 	component: (props) => {
// 		const ld = useLoaderData(props);
// 		const [link_portals, set_link_portals] = createSignal<LinkPortal[]>([]);
// 		let container_ref!: HTMLDivElement;

// 		createEffect(() => {
// 			set_link_portals(
// 				process_markdown_content(
// 					container_ref,
// 					ld()?.Page?.HTML || "",
// 				),
// 			);
// 		});

// 		return (
// 			<>
// 				<div ref={container_ref} class={content_class_name} />
// 				<For each={link_portals()}>
// 					{(portal) => {
// 						return (
// 							<Portal mount={portal.container}>
// 								<SolidPortalLink portal={portal} />
// 							</Portal>
// 						);
// 					}}
// 				</For>
// 			</>
// 		);
// 	},
// });

// function SolidPortalLink(props: { portal: LinkPortal }) {
// 	const portal = props.portal;
// 	return (
// 		<Link
// 			aria-label={portal.aria_label}
// 			class={portal.class_name}
// 			href={portal.href}
// 			prefetch="intent"
// 			title={portal.title}
// 			innerHTML={portal.html}
// 		/>
// 	);
// }

/////////////////////////////////////////////////////////////////////
/////// REMIX
/////////////////////////////////////////////////////////////////////

type LinkPortal = {
	aria_label?: string;
	class_name?: string;
	container: HTMLElement;
	href: string;
	html: string;
	title?: string;
};

type LinkRoot = ReturnType<typeof createRoot>;

const anchor_class = "anchor";
const anchor_heading_class = "anchor-heading";
const heading_selector = "h2, h3, h4, h5, h6";

export default defineView({
	pattern: content_route_pattern,
	component: (handle) => {
		let container: HTMLElement | undefined;
		let link_roots: LinkRoot[] = [];
		let markdown = "";

		function dispose_link_roots(): void {
			for (const root of link_roots) {
				root.dispose();
			}
			link_roots = [];
		}

		function render_markdown_content(): void {
			if (!container) {
				return;
			}

			dispose_link_roots();
			const portals = process_markdown_content(container, markdown);
			link_roots = portals.map((portal) => {
				const root = createRoot(portal.container);
				root.render(
					<Link
						aria-label={portal.aria_label}
						className={portal.class_name}
						href={portal.href}
						prefetch="intent"
						title={portal.title}
						innerHTML={portal.html}
					/>,
				);
				root.flush();
				return root;
			});
		}

		handle.signal.addEventListener("abort", dispose_link_roots);

		return (props): RemixNode => {
			const ld = useLoaderData(props);
			const next_markdown = ld.Page?.HTML || "";
			if (next_markdown !== markdown) {
				markdown = next_markdown;
				handle.queueTask((signal) => {
					if (signal.aborted) {
						return;
					}
					render_markdown_content();
				});
			}

			return (
				<div
					mix={ref((node, signal) => {
						const next_container = node as HTMLElement;
						container = next_container;
						render_markdown_content();
						signal.addEventListener(
							"abort",
							() => {
								if (container === next_container) {
									container = undefined;
								}
								dispose_link_roots();
							},
							{ once: true },
						);
					})}
					className={content_class_name}
				/>
			);
		};
	},
});

function process_markdown_content(
	container: HTMLElement,
	markdown: string,
): LinkPortal[] {
	container.innerHTML = markdown;
	process_headings(container);
	process_code_blocks(container);
	return process_links(container);
}

function process_headings(container: HTMLElement): void {
	const headings = container.querySelectorAll(heading_selector);
	for (const heading of headings) {
		const id = heading.id;
		if (!id) {
			continue;
		}

		const label = heading.textContent || "";
		const content = document.createElement("span");
		content.append(...Array.from(heading.childNodes));

		const anchor = document.createElement("a");
		anchor.href = `#${id}`;
		anchor.textContent = "#";
		anchor.setAttribute("aria-label", `Link to ${label}`);
		anchor.classList.add(anchor_class);

		heading.textContent = "";
		heading.classList.add(anchor_heading_class);
		heading.append(anchor, content);
	}
}

function process_code_blocks(container: HTMLElement): void {
	const code_blocks = container.querySelectorAll("pre code");
	for (const code_block of code_blocks) {
		highlight.highlightElement(code_block as HTMLElement);
	}
}

function process_links(container: HTMLElement): LinkPortal[] {
	const portals: LinkPortal[] = [];
	for (const link of container.querySelectorAll("a")) {
		if (is_heading_anchor(link)) {
			continue;
		}

		const href_details = getHrefDetails(link.href);
		if (!href_details.isHTTP) {
			continue;
		}
		if (href_details.isExternal) {
			link.dataset.external = "true";
			link.target = "_blank";
			link.rel = "noopener noreferrer";
			continue;
		}

		const placeholder = document.createElement("span");
		link.replaceWith(placeholder);
		portals.push({
			aria_label: attr_or_undefined(link, "aria-label"),
			class_name: attr_or_undefined(link, "class"),
			container: placeholder,
			href: href_details.relativeURL,
			html: link.innerHTML,
			title: attr_or_undefined(link, "title"),
		});
	}
	return portals;
}

function attr_or_undefined(el: Element, name: string): string | undefined {
	return el.getAttribute(name) || undefined;
}

function is_heading_anchor(anchor: HTMLAnchorElement): boolean {
	return (
		anchor.classList.contains(anchor_class) &&
		anchor.parentElement?.classList.contains(anchor_heading_class) === true
	);
}
