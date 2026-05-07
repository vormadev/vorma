// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createFieldParts, type FieldStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Field", () => {
	setup_remix_component_test_environment();

	it("creates field parts without requiring factory call order", () => {
		const field_recipe = {
			slots: {
				description: {
					base: {
						color: "gray",
					},
				},
				error: {
					base: {
						color: "red",
					},
				},
				label: {
					base: {
						color: "black",
					},
				},
				root: {
					base: {
						display: "grid",
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
					field: field_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies FieldStyleSystem<"light", typeof field_recipe>;
		const Field = createFieldParts(style_system);
		const result = render(
			createElement(
				Field.Root,
				{
					controlID: "name-input",
					invalid: true,
				},
				createElement(Field.Label, {}, "Name"),
				createElement("input", {
					"aria-describedby": "external-hint",
					mix: Field.controlMixin<HTMLInputElement>(),
				}),
				createElement(Field.Description, {}, "Helpful description"),
				createElement(Field.Error, { match: true }, "Required"),
			),
		);

		const root = result.$("div");
		const label = result.$("label");
		const input = result.$("input");
		const paragraphs = result.$$("p");
		const description = paragraphs[0];
		const error = paragraphs[1];
		expect(root?.getAttribute("data-invalid")).toBe("true");
		expect(label?.getAttribute("data-invalid")).toBe("true");
		expect(label?.getAttribute("for")).toBe("name-input");
		expect(input?.getAttribute("id")).toBe("name-input");
		expect(input?.getAttribute("aria-invalid")).toBe("true");
		expect(input?.getAttribute("aria-describedby")).toContain(
			"external-hint",
		);
		expect(input?.getAttribute("aria-describedby")).toContain(
			description?.id,
		);
		expect(input?.getAttribute("aria-describedby")).toContain(error?.id);
		expect(description?.textContent).toBe("Helpful description");
		expect(error?.textContent).toBe("Required");

		result.cleanup();
	});
});
