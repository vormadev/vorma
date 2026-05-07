// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import {
	get_collection_navigation_item,
	get_first_collection_item,
	get_last_collection_item,
	get_roving_tab_index,
} from "./composite-navigation.ts";
import type { OrderedCollectionItem } from "./ordered-collection.ts";

function create_navigation_item(value: string): OrderedCollectionItem {
	const node = document.createElement("div");
	document.body.append(node);
	return {
		disabled: false,
		id: value,
		node,
		text: value,
		value,
	};
}

describe("composite navigation", () => {
	it("moves from current value without wrapping by default", () => {
		const items = [
			create_navigation_item("first"),
			create_navigation_item("second"),
		];

		expect(
			get_collection_navigation_item({
				currentValue: "first",
				items,
				offset: 1,
			})?.value,
		).toBe("second");
		expect(
			get_collection_navigation_item({
				currentValue: "first",
				items,
				offset: -1,
			}),
		).toBeUndefined();

		for (const item of items) {
			item.node.remove();
		}
	});

	it("can wrap navigation explicitly", () => {
		const items = [
			create_navigation_item("first"),
			create_navigation_item("second"),
		];

		expect(
			get_collection_navigation_item({
				currentValue: "first",
				items,
				loop: true,
				offset: -1,
			})?.value,
		).toBe("second");
		expect(
			get_collection_navigation_item({
				currentValue: "first",
				items,
				loop: true,
				offset: -3,
			})?.value,
		).toBe("second");

		for (const item of items) {
			item.node.remove();
		}
	});

	it("can clamp page navigation to the collection edges", () => {
		const items = [
			create_navigation_item("first"),
			create_navigation_item("second"),
			create_navigation_item("third"),
		];

		expect(
			get_collection_navigation_item({
				clamp: true,
				currentValue: "first",
				items,
				offset: 10,
			})?.value,
		).toBe("third");
		expect(
			get_collection_navigation_item({
				clamp: true,
				currentValue: "third",
				items,
				loop: true,
				offset: -10,
			})?.value,
		).toBe("first");

		for (const item of items) {
			item.node.remove();
		}
	});

	it("returns first and last collection items", () => {
		const items = [
			create_navigation_item("first"),
			create_navigation_item("second"),
		];

		expect(get_first_collection_item(items)?.value).toBe("first");
		expect(get_last_collection_item(items)?.value).toBe("second");

		for (const item of items) {
			item.node.remove();
		}
	});

	it("derives roving tab index from current or fallback value", () => {
		expect(
			get_roving_tab_index({
				currentValue: "second",
				itemValue: "second",
			}),
		).toBe(0);
		expect(
			get_roving_tab_index({
				currentValue: "second",
				itemValue: "first",
			}),
		).toBe(-1);
		expect(
			get_roving_tab_index({
				currentValue: null,
				fallbackValue: "first",
				itemValue: "first",
			}),
		).toBe(0);
	});
});
