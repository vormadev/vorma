import { describe, expect, it } from "vitest";
import {
	ariaBoolean,
	ariaTrue,
	dataFlag,
	is_aria_invalid,
	openState,
	openStateFromBoolean,
	selectionState,
	selectionStateFromBoolean,
} from "./component-state.ts";

describe("component state helpers", () => {
	it("renders ARIA booleans explicitly", () => {
		expect(ariaBoolean(true)).toBe("true");
		expect(ariaBoolean(false)).toBe("false");
		expect(ariaTrue(true)).toBe("true");
		expect(ariaTrue(false)).toBe(undefined);
	});

	it("renders data flags by presence", () => {
		expect(dataFlag(true)).toBe("");
		expect(dataFlag(false)).toBe(undefined);
	});

	it("recognizes ARIA invalid states", () => {
		expect(is_aria_invalid(true)).toBe(true);
		expect(is_aria_invalid("true")).toBe(true);
		expect(is_aria_invalid("grammar")).toBe(true);
		expect(is_aria_invalid("spelling")).toBe(true);
		expect(is_aria_invalid(false)).toBe(false);
		expect(is_aria_invalid("false")).toBe(false);
	});

	it("maps shared state names", () => {
		expect(openStateFromBoolean(true)).toBe(openState.open);
		expect(openStateFromBoolean(false)).toBe(openState.closed);
		expect(selectionStateFromBoolean(true)).toBe(selectionState.selected);
		expect(selectionStateFromBoolean(false)).toBe(
			selectionState.unselected,
		);
	});
});
