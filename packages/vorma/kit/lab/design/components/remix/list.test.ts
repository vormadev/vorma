// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createList, createListItem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix List", () => {
	setup_remix_component_test_environment();

	it("renders configured list and item hosts", () => {
		const list_recipe = { slots: { item: {}, root: {} } } as const;
		const style_system = {
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { list: list_recipe } },
			variablePrefix: "test",
		};
		const List = createList(style_system);
		const ListItem = createListItem(style_system);
		const result = render(
			createElement(
				List,
				{ as: "ol", "data-testid": "list" },
				createElement(ListItem, {}, "First"),
			),
		);

		expect(result.$("[data-testid='list']")?.tagName).toBe("OL");
		expect(result.$("li")?.textContent).toBe("First");

		result.cleanup();
	});
});
