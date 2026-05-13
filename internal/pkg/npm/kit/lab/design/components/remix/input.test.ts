// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	componentDataAttribute,
	createInput,
	inputAnatomy,
	type InputStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const input_event = "input";

const input_recipe = {
	defaultVariants: { size: "md", variant: "default" },
	slots: {
		root: {
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
						minBlockSize: "2.5rem",
					},
				},
			},
		},
		variant: {
			default: {
				root: {},
			},
		},
	},
} as const;

function create_test_style_system(): InputStyleSystem<
	"light",
	typeof input_recipe,
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
				input: input_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Input", () => {
	setup_remix_component_test_environment();

	it("renders native input semantics and applies props to the input host", async () => {
		const Input = createInput(create_test_style_system());
		let current_target_tag: string | undefined;
		let current_target_value: string | undefined;
		const result = render(
			createElement(Input, {
				"data-testid": "input",
				htmlSize: 24,
				mix: on<HTMLInputElement, typeof input_event>(
					input_event,
					(event) => {
						current_target_tag = event.currentTarget.tagName;
						current_target_value = event.currentTarget.value;
					},
				),
				name: "email",
				placeholder: "Email",
				type: "email",
				value: "hello@example.com",
			}),
		);

		const input = result.$(
			"[data-testid='input']",
		) as HTMLInputElement | null;
		expect(input).not.toBeNull();
		if (!input) {
			throw new Error("Expected Input to render an input");
		}
		expect(input.tagName).toBe("INPUT");
		expect(input.type).toBe("email");
		expect(input.name).toBe("email");
		expect(input.value).toBe("hello@example.com");
		expect(input.size).toBe(24);
		expect(input.getAttribute("placeholder")).toBe("Email");
		expect(input.getAttribute(componentAnatomyAttrs.scope)).toBe("input");
		expect(input.getAttribute(componentAnatomyAttrs.part)).toBe(
			inputAnatomy.root,
		);

		await result.act(() => {
			input.value = "updated@example.com";
			input.dispatchEvent(new Event(input_event, { bubbles: true }));
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_value).toBe("updated@example.com");

		result.cleanup();
	});

	it("reflects native input state into component data attributes", () => {
		const Input = createInput(create_test_style_system());
		const result = render(
			createElement(Input, {
				"aria-invalid": "spelling",
				"data-testid": "input",
				disabled: true,
				readOnly: true,
				required: true,
			}),
		);

		const input = result.$("[data-testid='input']") as HTMLInputElement;
		expect(input.disabled).toBe(true);
		expect(input.readOnly).toBe(true);
		expect(input.required).toBe(true);
		expect(input.getAttribute("aria-invalid")).toBe("spelling");
		expect(input.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(input.getAttribute(componentDataAttribute.invalid)).toBe("");
		expect(input.getAttribute(componentDataAttribute.readOnly)).toBe("");
		expect(input.getAttribute(componentDataAttribute.required)).toBe("");

		result.cleanup();
	});

	it("maps input state conditions onto the input host mix", () => {
		type StyleMixNode = {
			props: {
				mix: readonly [
					{
						args: readonly [Record<string, unknown>];
					},
				];
			};
		};
		const Input = createInput(create_test_style_system());
		const node = Input({} as never)({}) as unknown as StyleMixNode;
		const input_style = node.props.mix[0].args[0];

		expect(input_style).toMatchObject({
			"&:invalid, &[aria-invalid='true'], &[data-invalid]": {
				borderColor: "red",
			},
			"&::placeholder": {
				color: "gray",
			},
		});
	});
});
