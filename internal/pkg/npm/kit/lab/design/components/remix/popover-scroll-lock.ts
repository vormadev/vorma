import { createMixin, on, ref, type ElementProps } from "remix/ui";

type PopoverScrollStyle = {
	overflow: string;
	scrollbarGutter: string;
};

const scroll_styles = new WeakMap<Document, PopoverScrollStyle>();

function has_open_popover(document: Document): boolean {
	return document.querySelector("[popover]:popover-open") !== null;
}

function capture_scroll_style(document: Document): void {
	if (scroll_styles.has(document)) {
		return;
	}

	const document_element = document.documentElement;
	scroll_styles.set(document, {
		overflow: document_element.style.overflow,
		scrollbarGutter: document_element.style.scrollbarGutter,
	});
}

export function releasePopoverScrollLock(document: Document): void {
	if (has_open_popover(document)) {
		return;
	}

	const style = scroll_styles.get(document);
	if (!style) {
		return;
	}

	scroll_styles.delete(document);

	const document_element = document.documentElement;
	if (document_element.style.overflow === "hidden") {
		document_element.style.overflow = style.overflow;
	}
	if (document_element.style.scrollbarGutter === "stable") {
		document_element.style.scrollbarGutter = style.scrollbarGutter;
	}
}

export const popoverScrollLockGuard = createMixin<
	HTMLElement,
	[],
	ElementProps
>((handle) => {
	let owner_document: Document | undefined;

	function release(): void {
		if (!owner_document) {
			return;
		}

		releasePopoverScrollLock(owner_document);
	}

	handle.signal.addEventListener("abort", release);

	return () => {
		return [
			ref((node) => {
				owner_document = node.ownerDocument;
			}),
			on<HTMLElement, "beforetoggle">("beforetoggle", (event) => {
				if (event.newState === "closed") {
					setTimeout(release, 0);
					return;
				}
				if (event.newState !== "open") {
					return;
				}

				const document_element =
					event.currentTarget.ownerDocument.documentElement;
				capture_scroll_style(document_element.ownerDocument);
			}),
			on<HTMLElement, "toggle">("toggle", (event) => {
				if (event.newState !== "closed") {
					return;
				}

				release();
			}),
		];
	};
});
