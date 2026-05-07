// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createTextarea,
	type TextareaStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Textarea", () => {
	setup_remix_component_test_environment();

	it("renders textarea semantics and applies props to the textarea host", async () => {
		const textarea_recipe = {
			defaultVariants: { size: "md", variant: "default" },
			slots: {
				root: {
					base: {
						display: "block",
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
		const style_system = {
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
		} satisfies TextareaStyleSystem<"light", typeof textarea_recipe>;
		const Textarea = createTextarea(style_system);
		let current_target_tag: string | undefined;
		let current_target_value: string | undefined;
		const result = render(
			createElement(Textarea, {
				"data-testid": "textarea",
				mix: on<HTMLTextAreaElement, "input">("input", (event) => {
					current_target_tag = event.currentTarget.tagName;
					current_target_value = event.currentTarget.value;
				}),
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
		expect(textarea.getAttribute("placeholder")).toBe("Message");
		expect(textarea.getAttribute("rows")).toBe("4");
		expect(textarea.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"textarea",
		);
		expect(textarea.getAttribute(componentAnatomyAttrs.part)).toBe("root");

		await result.act(() => {
			textarea.value = "Updated";
			textarea.dispatchEvent(new Event("input", { bubbles: true }));
		});

		expect(current_target_tag).toBe("TEXTAREA");
		expect(current_target_value).toBe("Updated");

		result.cleanup();
	});
});
