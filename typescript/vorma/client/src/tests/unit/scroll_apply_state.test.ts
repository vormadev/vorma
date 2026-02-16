import { describe, expect, it, vi } from "vitest";
import { __applyScrollState } from "../../platform/scroll.ts";

describe("scroll_apply_state", () => {
	it("does not resolve an element when current location hash is empty", () => {
		window.history.replaceState({}, "", "/scroll-empty-hash#");
		const getElementByIdSpy = vi.spyOn(document, "getElementById");

		__applyScrollState(undefined);

		expect(getElementByIdSpy).not.toHaveBeenCalled();
	});

	it("does not resolve an element when explicit hash state is empty", () => {
		const getElementByIdSpy = vi.spyOn(document, "getElementById");

		__applyScrollState({ hash: "#" });

		expect(getElementByIdSpy).not.toHaveBeenCalled();
	});
});
