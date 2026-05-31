// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { create_ordered_collection } from "./ordered-collection.ts";

function append_option_node(text: string): HTMLElement {
	const node = document.createElement("div");
	node.textContent = text;
	document.body.append(node);
	return node;
}

describe("ordered collection", () => {
	it("returns items in DOM order rather than registration order", () => {
		const first_node = append_option_node("First");
		const second_node = append_option_node("Second");
		const collection = create_ordered_collection();

		collection.register({
			disabled: false,
			id: "second",
			node: second_node,
			text: "Second",
			value: "second",
		});
		collection.register({
			disabled: false,
			id: "first",
			node: first_node,
			text: "First",
			value: "first",
		});

		expect(
			collection.getItems().map((item) => {
				return item.value;
			}),
		).toEqual(["first", "second"]);

		first_node.remove();
		second_node.remove();
	});

	it("filters disabled and disconnected items", () => {
		const enabled_node = append_option_node("Enabled");
		const disabled_node = append_option_node("Disabled");
		const disconnected_node = document.createElement("div");
		const collection = create_ordered_collection();

		collection.register({
			disabled: false,
			id: "enabled",
			node: enabled_node,
			text: "Enabled",
			value: "enabled",
		});
		collection.register({
			disabled: true,
			id: "disabled",
			node: disabled_node,
			text: "Disabled",
			value: "disabled",
		});
		collection.register({
			disabled: false,
			id: "disconnected",
			node: disconnected_node,
			text: "Disconnected",
			value: "disconnected",
		});

		expect(
			collection.getItems().map((item) => {
				return item.value;
			}),
		).toEqual(["enabled", "disabled"]);
		expect(
			collection.getEnabledItems().map((item) => {
				return item.value;
			}),
		).toEqual(["enabled"]);

		enabled_node.remove();
		disabled_node.remove();
	});

	it("reports changed registrations and unregisters by ID", () => {
		const node = append_option_node("Light");
		const collection = create_ordered_collection();

		expect(
			collection.register({
				disabled: false,
				id: "light",
				node,
				text: "Light",
				value: "light",
			}),
		).toBe(true);
		expect(
			collection.register({
				disabled: false,
				id: "light",
				node,
				text: "Light",
				value: "light",
			}),
		).toBe(false);
		expect(collection.findByValue("light")?.id).toBe("light");
		expect(collection.unregister("light")?.value).toBe("light");
		expect(collection.findByValue("light")).toBeUndefined();

		node.remove();
	});
});
