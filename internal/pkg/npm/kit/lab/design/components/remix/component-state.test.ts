import { describe, expect, it } from "vitest";
import {
	ariaBoolean,
	ariaTrue,
	dataFlag,
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

	it("maps shared state names", () => {
		expect(openStateFromBoolean(true)).toBe(openState.open);
		expect(openStateFromBoolean(false)).toBe(openState.closed);
		expect(selectionStateFromBoolean(true)).toBe(selectionState.selected);
		expect(selectionStateFromBoolean(false)).toBe(
			selectionState.unselected,
		);
	});
});
