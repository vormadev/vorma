// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createBox } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Box", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const box_recipe = {
			slots: { root: {} },
			variants: { layout: { panel: { root: {} } } },
		} as const;
		const Component = createBox({
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
					box: box_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				Component,
				{ as: "section", "data-testid": "box", layout: "panel" },
				"Box",
			),
		);

		const box = result.$("[data-testid='box']");
		expect(box?.tagName).toBe("SECTION");
		expect(box?.hasAttribute("layout")).toBe(false);

		result.cleanup();
	});
});
