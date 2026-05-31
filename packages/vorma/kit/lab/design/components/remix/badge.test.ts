// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createBadge } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Badge", () => {
	setup_remix_component_test_environment();

	it("renders semantic host and anatomy", () => {
		const badge_recipe = {
			defaultVariants: {
				size: "sm",
				tone: "info",
			},
			slots: {
				root: {},
			},
			variants: {
				size: { sm: { root: {} } },
				tone: { info: { root: {} } },
			},
		} as const;
		const Component = createBadge({
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
					badge: badge_recipe,
				},
			},
			variablePrefix: "test",
		});
		const result = render(
			createElement(Component, { "data-testid": "badge" }, "Beta"),
		);

		const badge = result.$("[data-testid='badge']");
		expect(badge?.tagName).toBe("SPAN");
		expect(badge?.getAttribute(componentAnatomyAttrs.scope)).toBe("badge");

		result.cleanup();
	});
});
