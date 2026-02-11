import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, vi } from "vitest";

let dom: JSDOM;

export const createHeadErrorsMock = () => ({
	panic: vi.fn(() => {
		throw new Error("Panic called");
	}),
});

export const describeUpdateHeadElsSuite = (registerTests: () => void) => {
	describe("updateHeadEls", () => {
		beforeEach(() => {
			dom = new JSDOM(
				"<!DOCTYPE html><html><head></head><body></body></html>",
				{
					url: "http://localhost/",
				},
			);
			(global as any).window = dom.window as unknown as Window &
				typeof globalThis;
			(global as any).document = dom.window.document;
			(global as any).NodeFilter = {
				SHOW_COMMENT: 128,
				FILTER_ACCEPT: 1,
				FILTER_REJECT: 2,
			};
			(global as any).Node = {
				ELEMENT_NODE: 1,
			};

			const startMetaComment = document.createComment(
				'data-vorma="meta-start"',
			);
			const endMetaComment = document.createComment(
				'data-vorma="meta-end"',
			);
			const startRestComment = document.createComment(
				'data-vorma="rest-start"',
			);
			const endRestComment = document.createComment(
				'data-vorma="rest-end"',
			);

			document.head.appendChild(startMetaComment);
			document.head.appendChild(endMetaComment);
			document.head.appendChild(startRestComment);
			document.head.appendChild(endRestComment);

			vi.clearAllMocks();
		});

		afterEach(() => {
			vi.resetAllMocks();
			dom.window.close();
			(global as any).window = undefined as unknown as Window &
				typeof globalThis;
			(global as any).document = undefined as unknown as Document;
			(global as any).NodeFilter = undefined as unknown as NodeFilter;
		});

		registerTests();
	});
};
