// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createSeparator } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Separator", () => {
	setup_remix_component_test_environment();

	it("renders an hr host", () => {
		const Separator = createSeparator({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { separator: { slots: { root: {} } } } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(Separator, { "data-testid": "separator" }),
		);

		expect(result.$("[data-testid='separator']")?.tagName).toBe("HR");

		result.cleanup();
	});
});
