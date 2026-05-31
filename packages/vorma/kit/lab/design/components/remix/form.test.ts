// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentAnatomyAttrs, createForm, type FormStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Form", () => {
	setup_remix_component_test_environment();

	it("renders a native form host", () => {
		const form_recipe = {
			defaultVariants: {
				layout: "stacked",
			},
			slots: {
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
					form: form_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies FormStyleSystem<"light", typeof form_recipe>;
		const Form = createForm(style_system);
		const result = render(
			createElement(
				Form,
				{
					action: "/submit",
					method: "post",
				},
				"Form",
			),
		);

		const form = result.$("form");
		expect(form?.getAttribute("action")).toBe("/submit");
		expect(form?.getAttribute("method")).toBe("post");
		expect(form?.getAttribute(componentAnatomyAttrs.scope)).toBe("form");
		expect(form?.getAttribute(componentAnatomyAttrs.part)).toBe("root");

		result.cleanup();
	});
});
