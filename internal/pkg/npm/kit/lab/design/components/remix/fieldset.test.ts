// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createFieldset,
	type FieldsetStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Fieldset", () => {
	setup_remix_component_test_environment();

	it("renders native fieldset and legend hosts", () => {
		const fieldset_recipe = {
			defaultVariants: {
				layout: "stacked",
			},
			slots: {
				legend: {},
				root: {},
			},
			variants: {
				layout: {
					stacked: {
						root: {},
					},
				},
			},
		} as const;
		const style_system = {
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					fieldset: fieldset_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies FieldsetStyleSystem<"light", typeof fieldset_recipe>;
		const Fieldset = createFieldset(style_system);
		const result = render(
			createElement(
				Fieldset.Root,
				{
					disabled: true,
				},
				createElement(Fieldset.Legend, {}, "Preferences"),
			),
		);

		const fieldset = result.$("fieldset") as HTMLFieldSetElement | null;
		expect(fieldset?.disabled).toBe(true);
		expect(fieldset?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"fieldset",
		);
		expect(fieldset?.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(
			result.$("legend")?.getAttribute(componentAnatomyAttrs.part),
		).toBe("legend");

		result.cleanup();
	});
});
