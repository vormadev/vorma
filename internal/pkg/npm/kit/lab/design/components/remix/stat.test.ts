// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createStat } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Stat", () => {
	setup_remix_component_test_environment();

	it("renders title and value slots", () => {
		const stat_recipe = {
			slots: { root: {}, title: {}, value: {} },
			variants: { valueTone: { positive: { value: {} } } },
		} as const;
		const Stat = createStat({
			metadata: {},
			modes: { light: { variables: {} } },
			token: { recipe: { stat: stat_recipe } },
			variablePrefix: "test",
		});
		const result = render(
			createElement(Stat, {
				title: "Revenue",
				value: "$100",
				valueTone: "positive",
			}),
		);

		expect(
			result.$("[data-vorma-scope='stat'][data-vorma-part='title']")
				?.textContent,
		).toBe("Revenue");
		expect(
			result.$("[data-vorma-scope='stat'][data-vorma-part='value']")
				?.textContent,
		).toBe("$100");

		result.cleanup();
	});
});
