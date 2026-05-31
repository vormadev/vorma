import { afterEach, describe, expect, it, vi } from "vitest";
import { create_typeahead, type TypeaheadItem } from "./typeahead.ts";

const typeahead_items: readonly TypeaheadItem[] = [
	{ text: "Apple", value: "apple" },
	{ text: "Apricot", value: "apricot" },
	{ text: "Banana", value: "banana" },
];

describe("typeahead", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it("matches buffered multi-character searches", () => {
		vi.useFakeTimers();
		const typeahead = create_typeahead<TypeaheadItem>({ timeoutMs: 700 });

		expect(
			typeahead.search({
				currentValue: null,
				items: typeahead_items,
				key: "a",
			})?.value,
		).toBe("apple");
		expect(
			typeahead.search({
				currentValue: "apple",
				items: typeahead_items,
				key: "p",
			})?.value,
		).toBe("apricot");
	});

	it("cycles repeated-character searches", () => {
		vi.useFakeTimers();
		const typeahead = create_typeahead<TypeaheadItem>({ timeoutMs: 700 });

		expect(
			typeahead.search({
				currentValue: null,
				items: typeahead_items,
				key: "a",
			})?.value,
		).toBe("apple");
		expect(
			typeahead.search({
				currentValue: "apple",
				items: typeahead_items,
				key: "a",
			})?.value,
		).toBe("apricot");
	});

	it("resets the search buffer after the timeout", () => {
		vi.useFakeTimers();
		const typeahead = create_typeahead<TypeaheadItem>({ timeoutMs: 700 });

		expect(
			typeahead.search({
				currentValue: null,
				items: typeahead_items,
				key: "a",
			})?.value,
		).toBe("apple");
		vi.advanceTimersByTime(700);
		expect(
			typeahead.search({
				currentValue: "apple",
				items: typeahead_items,
				key: "b",
			})?.value,
		).toBe("banana");
	});
});
