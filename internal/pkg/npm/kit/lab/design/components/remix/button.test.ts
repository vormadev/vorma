// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	createRecipeStyleSystem,
	type GeneratedSystem,
} from "../../core/core.ts";
import { createButton } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Button", () => {
	setup_remix_component_test_environment();

	it("renders button semantics from a Vorma recipe", () => {
		const button_recipe = {
			defaultVariants: {
				size: "md",
				variant: "primary",
			},
			slots: {
				content: {
					base: {
						display: "inline-flex",
					},
				},
				loadingIndicator: {
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
				loadingIndicatorFrame: {
					base: {
						position: "absolute",
					},
				},
				root: {
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
		const base_system = {
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
			token: {},
			variablePrefix: "test",
		} satisfies GeneratedSystem<
			"light",
			{},
			{ breakpoint: { md: string } }
		>;
		const style_system = createRecipeStyleSystem(base_system, {
			button: button_recipe,
		});
		const Button = createButton(style_system);
		const result = render(
			createElement(
				Button,
				{
					at: {
						md: {
							size: "md",
						},
					},
					loading: true,
					loadingLabel: "Saving",
				},
				"Save",
			),
		);

		const button = result.$("button");
		expect(button?.getAttribute("aria-busy")).toBe("true");
		expect(button?.getAttribute("aria-label")).toBe("Saving");
		expect(button?.getAttribute("type")).toBe("button");
		expect(button?.hasAttribute("disabled")).toBe(true);
		expect(button?.textContent).toBe("Save");

		result.cleanup();
	});
});
