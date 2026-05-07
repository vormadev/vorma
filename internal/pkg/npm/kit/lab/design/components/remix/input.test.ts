// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createInput } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Input", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const input_recipe = {
			defaultVariants: { size: "md", variant: "default" },
			slots: { root: {} },
			variants: {
				size: { md: { root: {} } },
				variant: { default: { root: {} } },
			},
		} as const;
		const Component = createInput({
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
					input: input_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(Component, {
				"data-testid": "input",
				placeholder: "Name",
			}),
		);

		const input = result.$("[data-testid='input']");
		expect(input?.tagName).toBe("INPUT");
		expect(input?.getAttribute("placeholder")).toBe("Name");
		expect(input?.getAttribute(componentAnatomyAttrs.scope)).toBe("input");

		result.cleanup();
	});
});
