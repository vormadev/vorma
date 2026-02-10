import { updateHeadEls } from "./head_elements/head_elements.ts";
import type { GetRouteDataOutput } from "./vorma_ctx/vorma_ctx.ts";

export function applyRouteDocumentTitle(
	title: GetRouteDataOutput["title"],
): void {
	if (title === undefined) {
		return;
	}

	// Changing the title instantly makes it feel faster.
	// The temp textarea trick decodes any HTML entities in the title.
	// This comes after history updates so history entry titles are correct.
	const tempTxt = document.createElement("textarea");
	tempTxt.innerHTML = title?.dangerousInnerHTML || "";
	if (document.title !== tempTxt.value) {
		document.title = tempTxt.value;
	}
}

export function applyRouteHeadElements(json: GetRouteDataOutput): void {
	if (json.metaHeadEls !== undefined) {
		updateHeadEls("meta", json.metaHeadEls ?? []);
	}
	if (json.restHeadEls !== undefined) {
		updateHeadEls("rest", json.restHeadEls ?? []);
	}
}
