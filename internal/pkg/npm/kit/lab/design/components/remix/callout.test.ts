// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createCallout } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Callout", () => {
	setup_remix_component_test_environment();

	it("renders named slots", () => {
		const callout_recipe = {
			slots: { content: {}, icon: {}, root: {} },
			variants: { tone: { info: { root: {} } } },
		} as const;
		const Callout = createCallout({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { callout: callout_recipe } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				Callout,
				{ "data-testid": "callout", icon: "!", tone: "info" },
				"Heads up",
			),
		);

		expect(
			result
				.$("[data-testid='callout']")
				?.getAttribute(componentAnatomyAttrs.scope),
		).toBe("callout");
		expect(
			result
				.$("[data-vorma-scope='callout'][data-vorma-part='icon']")
				?.getAttribute("aria-hidden"),
		).toBe("true");

		result.cleanup();
	});
});
