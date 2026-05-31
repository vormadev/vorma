// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	componentDataAttribute,
	createFieldParts,
	fieldAnatomy,
	type FieldStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const field_recipe = {
	slots: {
		[fieldAnatomy.description]: {
			base: {
				color: "gray",
			},
		},
		[fieldAnatomy.error]: {
			base: {
				color: "red",
			},
		},
		[fieldAnatomy.label]: {
			base: {
				color: "black",
			},
		},
		[fieldAnatomy.root]: {
			base: {
				display: "grid",
			},
		},
	},
} as const;

function create_test_style_system(): FieldStyleSystem<"light", typeof field_recipe> {
	return {
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
	};
}

describe("Remix Field", () => {
	setup_remix_component_test_environment();

	it("wires label, description, error, and control relationships", () => {
		const Field = createFieldParts(create_test_style_system());
		const result = render(
			createElement(
				Field.Root,
				{
					controlId: "name-input",
					disabled: true,
					invalid: true,
					readOnly: true,
					required: true,
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
		const input = result.$("input") as HTMLInputElement | null;
		const paragraphs = result.$$("p");
		const description = paragraphs[0];
		const error = paragraphs[1];

		expect(root?.getAttribute(componentAnatomyAttrs.scope)).toBe("field");
		expect(root?.getAttribute(componentAnatomyAttrs.part)).toBe(fieldAnatomy.root);
		expect(root?.getAttribute("aria-disabled")).toBe("true");
		expect(root?.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(root?.getAttribute(componentDataAttribute.invalid)).toBe("");
		expect(root?.getAttribute(componentDataAttribute.readOnly)).toBe("");
		expect(root?.getAttribute(componentDataAttribute.required)).toBe("");
		expect(label?.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(label?.getAttribute(componentDataAttribute.invalid)).toBe("");
		expect(label?.getAttribute(componentDataAttribute.readOnly)).toBe("");
		expect(label?.getAttribute(componentDataAttribute.required)).toBe("");
		expect(label?.getAttribute("for")).toBe("name-input");
		expect(input?.id).toBe("name-input");
		expect(input?.disabled).toBe(true);
		expect(input?.readOnly).toBe(true);
		expect(input?.required).toBe(true);
		expect(input?.getAttribute("aria-invalid")).toBe("true");
		expect(input?.getAttribute("aria-describedby")).toContain("external-hint");
		expect(input?.getAttribute("aria-describedby")).toContain(description?.id);
		expect(input?.getAttribute("aria-describedby")).toContain(error?.id);
		expect(description?.getAttribute(componentAnatomyAttrs.part)).toBe(
			fieldAnatomy.description,
		);
		expect(error?.getAttribute(componentAnatomyAttrs.part)).toBe(fieldAnatomy.error);
		expect(description?.textContent).toBe("Helpful description");
		expect(error?.textContent).toBe("Required");

		result.cleanup();
	});

	it("preserves explicit control props over field defaults when appropriate", () => {
		const Field = createFieldParts(create_test_style_system());
		const result = render(
			createElement(
				Field.Root,
				{
					controlId: "fallback-id",
					invalid: true,
				},
				createElement("input", {
					"aria-describedby": "own-description",
					"aria-invalid": "spelling",
					id: "explicit-id",
					mix: Field.controlMixin<HTMLInputElement>(),
				}),
				createElement(Field.Description, {}, "Helpful description"),
				createElement(Field.Error, { match: true }, "Required"),
			),
		);

		const input = result.$("input") as HTMLInputElement;
		const description = result.$$("p")[0];
		const error = result.$$("p")[1];
		if (!description || !error) {
			throw new Error("Expected description and error nodes.");
		}

		expect(input.id).toBe("explicit-id");
		expect(input.getAttribute("aria-invalid")).toBe("spelling");
		expect(input.getAttribute("aria-describedby")).toContain("own-description");
		expect(input.getAttribute("aria-describedby")).toContain(description.id);
		expect(input.getAttribute("aria-describedby")).toContain(error.id);

		result.cleanup();
	});

	it("omits matched error content when the field is valid", () => {
		const Field = createFieldParts(create_test_style_system());
		const result = render(
			createElement(
				Field.Root,
				{},
				createElement(Field.Error, { match: true }, "Required"),
				createElement(Field.Error, {}, "Always rendered"),
			),
		);

		expect(result.$("p")?.textContent).toBe("Always rendered");

		result.cleanup();
	});

	it("keeps consumer mix on the host implied by each part", () => {
		const Field = createFieldParts(create_test_style_system());
		let label_target_tag: string | undefined;
		const result = render(
			createElement(
				Field.Root,
				{},
				createElement(
					Field.Label,
					{
						mix: on<HTMLLabelElement, "click">("click", (event) => {
							label_target_tag = event.currentTarget.tagName;
						}),
					},
					"Name",
				),
			),
		);

		const label = result.$("label");
		label?.dispatchEvent(new Event("click", { bubbles: true }));

		expect(label_target_tag).toBe("LABEL");

		result.cleanup();
	});
});
