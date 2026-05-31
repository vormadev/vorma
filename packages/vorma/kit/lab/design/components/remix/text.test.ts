// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createText } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Text", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const text_recipe = {
			slots: { root: {} },
			variants: {
				tone: { muted: { root: {} } },
				variant: { heading: { root: {} } },
			},
		} as const;
		const Component = createText({
			metadata: {
				breakpoint: {
					md: "48rem",
				},
			},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					text: text_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				Component,
				{
					as: "h2",
					"data-testid": "text",
					tone: "muted",
					variant: "heading",
				},
				"Heading",
			),
		);

		expect(result.$("[data-testid='text']")?.tagName).toBe("H2");

		result.cleanup();
	});
});
