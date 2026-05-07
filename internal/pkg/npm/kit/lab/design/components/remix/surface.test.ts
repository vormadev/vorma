// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createSurface } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Surface", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const surface_recipe = {
			defaultVariants: { density: "cozy", variant: "raised" },
			slots: { root: {} },
			variants: {
				density: { cozy: { root: {} } },
				layout: { panel: { root: {} } },
				variant: { raised: { root: {} } },
			},
		} as const;
		const Component = createSurface({
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
					surface: surface_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				Component,
				{ "data-testid": "surface", layout: "panel" },
				"Surface",
			),
		);

		expect(
			result
				.$("[data-testid='surface']")
				?.getAttribute(componentAnatomyAttrs.scope),
		).toBe("surface");

		result.cleanup();
	});
});
