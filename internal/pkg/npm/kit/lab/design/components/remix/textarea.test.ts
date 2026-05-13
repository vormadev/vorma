// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	componentDataAttribute,
	createTextarea,
	textareaAnatomy,
	type TextareaStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const input_event = "input";

const textarea_recipe = {
	defaultVariants: { size: "md", variant: "default" },
	slots: {
		root: {
			base: {
				display: "block",
			},
			conditions: {
				invalid: {
					borderColor: "red",
				},
				placeholder: {
					color: "gray",
				},
			},
		},
	},
	variants: {
		size: {
			md: {
				root: {
					base: {
						minBlockSize: "6rem",
					},
				},
			},
		},
		variant: {
			default: {
				root: {
					base: {
						resize: "vertical",
					},
				},
			},
		},
	},
} as const;

function create_test_style_system(): TextareaStyleSystem<
	"light",
	typeof textarea_recipe,
	{ breakpoint: { md: string } }
> {
	return {
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
				textarea: textarea_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Textarea", () => {
	setup_remix_component_test_environment();

	it("renders native textarea semantics and applies props to the textarea host", async () => {
		const Textarea = createTextarea(create_test_style_system());
		let current_target_tag: string | undefined;
		let current_target_value: string | undefined;
		const result = render(
			createElement(Textarea, {
				"data-testid": "textarea",
				mix: on<HTMLTextAreaElement, typeof input_event>(
					input_event,
					(event) => {
						current_target_tag = event.currentTarget.tagName;
						current_target_value = event.currentTarget.value;
					},
				),
				name: "message",
				placeholder: "Message",
				rows: 4,
				value: "Hello",
			}),
		);

		const textarea = result.$(
			"[data-testid='textarea']",
		) as HTMLTextAreaElement | null;
		expect(textarea).not.toBeNull();
		if (!textarea) {
			throw new Error("Expected Textarea to render a textarea");
		}
		expect(textarea.tagName).toBe("TEXTAREA");
		expect(textarea.name).toBe("message");
		expect(textarea.value).toBe("Hello");
		expect(textarea.getAttribute("placeholder")).toBe("Message");
		expect(textarea.getAttribute("rows")).toBe("4");
		expect(textarea.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"textarea",
		);
		expect(textarea.getAttribute(componentAnatomyAttrs.part)).toBe(
			textareaAnatomy.root,
		);

		await result.act(() => {
			textarea.value = "Updated";
			textarea.dispatchEvent(new Event(input_event, { bubbles: true }));
		});

		expect(current_target_tag).toBe("TEXTAREA");
		expect(current_target_value).toBe("Updated");

		result.cleanup();
	});

	it("reflects native textarea state into component data attributes", () => {
		const Textarea = createTextarea(create_test_style_system());
		const result = render(
			createElement(Textarea, {
				"aria-invalid": "grammar",
				"data-testid": "textarea",
				disabled: true,
				readOnly: true,
				required: true,
			}),
		);

		const textarea = result.$(
			"[data-testid='textarea']",
		) as HTMLTextAreaElement;
		expect(textarea.disabled).toBe(true);
		expect(textarea.readOnly).toBe(true);
		expect(textarea.required).toBe(true);
		expect(textarea.getAttribute("aria-invalid")).toBe("grammar");
		expect(textarea.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(textarea.getAttribute(componentDataAttribute.invalid)).toBe("");
		expect(textarea.getAttribute(componentDataAttribute.readOnly)).toBe("");
		expect(textarea.getAttribute(componentDataAttribute.required)).toBe("");

		result.cleanup();
	});

	it("maps textarea state conditions onto the textarea host mix", () => {
		type StyleMixNode = {
			props: {
				mix: readonly [
					{
						args: readonly [Record<string, unknown>];
					},
				];
			};
		};
		const Textarea = createTextarea(create_test_style_system());
		const node = Textarea({} as never)({}) as unknown as StyleMixNode;
		const textarea_style = node.props.mix[0].args[0];

		expect(textarea_style).toMatchObject({
			"&:invalid, &[aria-invalid='true'], &[data-invalid]": {
				borderColor: "red",
			},
			"&::placeholder": {
				color: "gray",
			},
		});
	});
});
