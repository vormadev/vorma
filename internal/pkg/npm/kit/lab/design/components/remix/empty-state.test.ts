// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createEmptyState } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix EmptyState", () => {
	setup_remix_component_test_environment();

	it("renders indicator and title slots", () => {
		const empty_state_recipe = {
			slots: { body: {}, indicator: {}, root: {}, title: {} },
		} as const;
		const EmptyState = createEmptyState({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { emptyState: empty_state_recipe } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(
				EmptyState,
				{ indicator: "*", title: "No projects" },
				"Create one to get started.",
			),
		);

		expect(
			result
				.$(
					"[data-vorma-scope='emptyState'][data-vorma-part='indicator']",
				)
				?.getAttribute("aria-hidden"),
		).toBe("true");
		expect(
			result.$("[data-vorma-scope='emptyState'][data-vorma-part='title']")
				?.textContent,
		).toBe("No projects");

		result.cleanup();
	});
});
