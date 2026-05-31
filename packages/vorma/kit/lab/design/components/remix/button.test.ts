// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	buttonAnatomy,
	componentAnatomyAttrs,
	componentDataAttribute,
	createButton,
	type ButtonStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const button_recipe = {
	defaultVariants: {
		size: "md",
		variant: "primary",
	},
	slots: {
		[buttonAnatomy.content]: {
			base: {
				display: "inline-flex",
			},
		},
		[buttonAnatomy.loadingIndicator]: {
			base: {
				blockSize: "1em",
				inlineSize: "1em",
			},
			conditions: {
				reducedMotion: {
					animation: "none",
				},
			},
		},
		[buttonAnatomy.loadingIndicatorFrame]: {
			base: {
				position: "absolute",
			},
		},
		[buttonAnatomy.root]: {
			base: {
				alignItems: "center",
				display: "inline-flex",
			},
			conditions: {
				hover: {
					background: "blue",
				},
			},
		},
	},
	variants: {
		fluid: {
			true: {
				root: {
					base: {
						inlineSize: "100%",
					},
				},
			},
		},
		layout: {
			control: {
				root: {
					base: {
						justifyContent: "center",
					},
				},
			},
		},
		loading: {
			true: {
				content: {
					base: {
						opacity: 0,
					},
				},
			},
		},
		size: {
			md: {
				root: {
					base: {
						minHeight: "2.5rem",
					},
				},
			},
		},
		variant: {
			primary: {
				root: {
					base: {
						color: "white",
					},
				},
			},
		},
	},
} as const;

function create_test_style_system(): ButtonStyleSystem<
	"light",
	typeof button_recipe,
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
				button: button_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Button", () => {
	setup_remix_component_test_environment();

	it("renders native button semantics from a Vorma recipe", () => {
		const Button = createButton(create_test_style_system());
		let current_target_tag: string | undefined;
		const result = render(
			createElement(
				Button,
				{
					at: {
						md: {
							size: "md",
						},
					},
					fluid: true,
					layout: "control",
					mix: on<HTMLButtonElement, "click">("click", (event) => {
						current_target_tag = event.currentTarget.tagName;
					}),
					variant: "primary",
				},
				"Save",
			),
		);

		const button = result.$("button") as HTMLButtonElement | null;
		expect(button).not.toBeNull();
		if (!button) {
			throw new Error("Expected Button to render a button host");
		}
		expect(button.type).toBe("button");
		expect(button.disabled).toBe(false);
		expect(button.getAttribute(componentAnatomyAttrs.scope)).toBe("button");
		expect(button.getAttribute(componentAnatomyAttrs.part)).toBe(buttonAnatomy.root);
		expect(
			button.querySelector(
				`[${componentAnatomyAttrs.part}='${buttonAnatomy.content}']`,
			)?.textContent,
		).toBe("Save");

		button.dispatchEvent(new Event("click", { bubbles: true }));
		expect(current_target_tag).toBe("BUTTON");

		result.cleanup();
	});

	it("preserves explicit form button type", () => {
		const Button = createButton(create_test_style_system());
		const result = render(
			createElement(
				Button,
				{
					type: "submit",
				},
				"Submit",
			),
		);

		const button = result.$("button") as HTMLButtonElement;
		expect(button.type).toBe("submit");

		result.cleanup();
	});

	it("renders loading semantics without inventing a generic accessible name", () => {
		const Button = createButton(create_test_style_system());
		const result = render(
			createElement(
				Button,
				{
					"aria-label": "Save changes",
					loading: true,
					loadingIndicator: createElement("span", {
						"data-testid": "spinner",
					}),
				},
				"Save",
			),
		);

		const button = result.$("button") as HTMLButtonElement;
		const indicator = result.$("[data-testid='spinner']");
		expect(button.disabled).toBe(true);
		expect(button.getAttribute("aria-busy")).toBe("true");
		expect(button.getAttribute("aria-label")).toBe("Save changes");
		expect(button.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(button.getAttribute(componentDataAttribute.loading)).toBe("");
		expect(indicator).not.toBeNull();
		expect(
			indicator?.closest(
				`[${componentAnatomyAttrs.part}='${buttonAnatomy.loadingIndicator}']`,
			),
		).not.toBeNull();

		result.cleanup();
	});

	it("uses loadingLabel as an explicit loading accessible name", () => {
		const Button = createButton(create_test_style_system());
		const result = render(
			createElement(
				Button,
				{
					loading: true,
					loadingLabel: "Saving",
				},
				"Save",
			),
		);

		const button = result.$("button") as HTMLButtonElement;
		expect(button.getAttribute("aria-label")).toBe("Saving");

		result.cleanup();
	});
});
