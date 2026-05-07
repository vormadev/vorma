// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createSpinner } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Spinner", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const spinner_recipe = {
			defaultVariants: { size: "sm", tone: "neutral" },
			slots: { root: {} },
			variants: {
				size: { sm: { root: {} } },
				tone: { neutral: { root: {} } },
			},
		} as const;
		const Component = createSpinner({
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
					spinner: spinner_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(Component, { "data-testid": "spinner", idle: true }),
		);

		expect(
			result.$("[data-testid='spinner']")?.getAttribute("data-idle"),
		).toBe("true");

		result.cleanup();
	});
});
