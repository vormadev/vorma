// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createGrid } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Grid", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const grid_recipe = {
			slots: { root: {} },
			variants: {
				columns: { two: { root: {} } },
				gap: { md: { root: {} } },
				layout: { dashboard: { root: {} } },
			},
		} as const;
		const Component = createGrid({
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
					grid: grid_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				Component,
				{
					as: "section",
					columns: "two",
					"data-testid": "grid",
					gap: "md",
					layout: "dashboard",
				},
				"Grid",
			),
		);

		const grid = result.$("[data-testid='grid']");
		expect(grid?.tagName).toBe("SECTION");
		expect(grid?.hasAttribute("columns")).toBe(false);

		result.cleanup();
	});
});
