// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { checkableStateAttribute } from "./checkable-state.ts";
import {
	componentAnatomyAttrs,
	createCheckbox,
	type CheckboxStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Checkbox", () => {
	setup_remix_component_test_environment();

	it("renders a native checkbox host with checked state semantics", async () => {
		const checkbox_recipe = {
			defaultVariants: { size: "md", variant: "default" },
			slots: {
				root: {
					base: {
						appearance: "none",
					},
					conditions: {
						checked: {
							background: "blue",
						},
						indeterminate: {
							background: "gray",
						},
					},
				},
			},
			variants: {
				size: {
					md: {
						root: {
							base: {
								inlineSize: "1rem",
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
					checkbox: checkbox_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies CheckboxStyleSystem<"light", typeof checkbox_recipe>;
		const Checkbox = createCheckbox(style_system);
		let current_target_tag: string | undefined;
		let current_target_checked: boolean | undefined;
		const result = render(
			createElement(Checkbox, {
				"data-testid": "checkbox",
				checked: "indeterminate",
				mix: on<HTMLInputElement, "change">("change", (event) => {
					current_target_tag = event.currentTarget.tagName;
					current_target_checked = event.currentTarget.checked;
				}),
				name: "terms",
			}),
		);

		const checkbox = result.$(
			"[data-testid='checkbox']",
		) as HTMLInputElement | null;
		expect(checkbox).not.toBeNull();
		if (!checkbox) {
			throw new Error("Expected Checkbox to render an input");
		}
		expect(checkbox.type).toBe("checkbox");
		expect(checkbox.name).toBe("terms");
		expect(checkbox.checked).toBe(false);
		expect(checkbox.indeterminate).toBe(true);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			"indeterminate",
		);
		expect(checkbox.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"checkbox",
		);
		expect(checkbox.getAttribute(componentAnatomyAttrs.part)).toBe("root");

		await result.act(() => {
			checkbox.indeterminate = false;
			checkbox.checked = true;
			checkbox.dispatchEvent(new Event("change", { bubbles: true }));
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_checked).toBe(true);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe("checked");

		result.cleanup();
	});

	it("maps checkbox state conditions onto the input host mix", () => {
		type StyleMixNode = {
			props: {
				mix: readonly [
					unknown,
					{
						args: readonly [Record<string, unknown>];
					},
				];
			};
		};
		const checkbox_recipe = {
			slots: {
				root: {
					conditions: {
						checked: {
							background: "blue",
						},
						unchecked: {
							background: "white",
						},
					},
				},
			},
		} as const;
		const Checkbox = createCheckbox({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					checkbox: checkbox_recipe,
				},
			},
			variablePrefix: "test",
		});
		const node = Checkbox({} as never)({}) as unknown as StyleMixNode;
		const input_style = node.props.mix[1].args[0];

		expect(input_style).toMatchObject({
			"&[data-state='checked']": {
				background: "blue",
			},
			"&[data-state='unchecked']": {
				background: "white",
			},
		});
	});
});
