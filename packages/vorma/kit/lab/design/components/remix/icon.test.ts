// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createIcon } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Icon", () => {
	setup_remix_component_test_environment();

	it("renders hidden decorative icons and named accessible icons", () => {
		const icon_recipe = {
			slots: { root: {} },
			variants: {
				layout: { inline: { root: {} } },
				size: { md: { root: {} } },
				tone: { neutral: { root: {} } },
			},
		} as const;
		const Icon = createIcon({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { icon: icon_recipe } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				"div",
				{},
				createElement(Icon, { "data-testid": "icon" }, "*"),
				createElement(Icon, {
					"aria-label": "Search",
					"data-testid": "named-icon",
				}),
			),
		);

		expect(result.$("[data-testid='icon']")?.getAttribute("aria-hidden")).toBe(
			"true",
		);
		expect(
			result.$("[data-testid='named-icon']")?.getAttribute("aria-hidden"),
		).toBeNull();

		result.cleanup();
	});
});
