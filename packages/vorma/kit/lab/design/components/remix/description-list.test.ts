// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createDescriptionList, createDescriptionListItem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix DescriptionList", () => {
	setup_remix_component_test_environment();

	it("renders native description list semantics", () => {
		const description_list_recipe = {
			slots: { description: {}, item: {}, root: {}, term: {} },
			variants: {
				descriptionTone: { muted: { description: {} } },
				layout: { stacked: { item: {} } },
				tone: { neutral: { term: {} } },
			},
		} as const;
		const style_system = {
			metadata: { breakpoint: { md: "48rem" } },
			modes: { light: { variables: {} } },
			token: { recipe: { descriptionList: description_list_recipe } },
			variablePrefix: "test",
		};
		const DescriptionList = createDescriptionList(style_system);
		const DescriptionListItem = createDescriptionListItem(style_system);
		const result = render(
			createElement(
				DescriptionList,
				{ "data-testid": "description-list" },
				createElement(
					DescriptionListItem,
					{
						descriptionTone: "muted",
						layout: "stacked",
						term: "Plan",
						tone: "neutral",
					},
					"Pro",
				),
			),
		);

		expect(result.$("[data-testid='description-list']")?.tagName).toBe("DL");
		expect(result.$("dt")?.textContent).toBe("Plan");
		expect(result.$("dd")?.textContent).toBe("Pro");

		result.cleanup();
	});
});
