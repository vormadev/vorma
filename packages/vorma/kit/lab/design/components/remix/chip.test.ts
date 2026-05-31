// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createChip } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Chip", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const chip_recipe = {
			defaultVariants: {
				selected: "false",
				size: "md",
				variant: "neutral",
			},
			slots: { root: {} },
			variants: {
				selected: { false: { root: {} }, true: { root: {} } },
				size: { md: { root: {} } },
				variant: { neutral: { root: {} } },
			},
		} as const;
		const Component = createChip({
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
					chip: chip_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(Component, { "data-testid": "chip", selected: true }, "Chip"),
		);

		const chip = result.$("[data-testid='chip']");
		expect(chip?.getAttribute("aria-pressed")).toBe("true");
		expect(chip?.getAttribute("data-selected")).toBe("true");
		expect(chip?.getAttribute("type")).toBe("button");

		result.cleanup();
	});
});
