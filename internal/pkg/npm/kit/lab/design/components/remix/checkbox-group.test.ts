// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { checkableStateAttribute } from "./checkable-state.ts";
import {
	componentAnatomyAttrs,
	createCheckboxGroup,
	type CheckboxGroupStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const checkbox_group_recipe = {
	defaultVariants: {
		layout: "stacked",
		size: "md",
		variant: "default",
	},
	slots: {
		item: {},
		root: {},
	},
	variants: {
		layout: {
			stacked: {
				root: {},
			},
		},
		size: {
			md: {
				item: {},
			},
		},
		variant: {
			default: {
				item: {},
			},
		},
	},
} as const;

function create_test_style_system(): CheckboxGroupStyleSystem<
	"light",
	typeof checkbox_group_recipe
> {
	return {
		metadata: {},
		modes: {
			light: {
				variables: {},
			},
		},
		token: {
			recipe: {
				checkboxGroup: checkbox_group_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix CheckboxGroup", () => {
	setup_remix_component_test_environment();

	it("coordinates native checkbox items through group value state", async () => {
		const CheckboxGroup = createCheckboxGroup(create_test_style_system());
		const value_changes: string[][] = [];
		let item_mix_target_tag: string | undefined;
		const result = render(
			createElement(
				CheckboxGroup.Root,
				{
					defaultValue: ["email"],
					name: "channels",
					onValueChange: (value: readonly string[]) => {
						value_changes.push([...value]);
					},
				},
				createElement(CheckboxGroup.Item, {
					"data-testid": "email",
					value: "email",
				}),
				createElement(CheckboxGroup.Item, {
					"data-testid": "sms",
					mix: on<HTMLInputElement, "change">(
						"change",
						(event) => {
							item_mix_target_tag =
								event.currentTarget.tagName;
						},
					),
					value: "sms",
				}),
			),
		);

		const root = result.$("[role='group']");
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;
		expect(root?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"checkboxGroup",
		);
		expect(email.type).toBe("checkbox");
		expect(email.name).toBe("channels");
		expect(sms.name).toBe("channels");
		expect(email.checked).toBe(true);
		expect(email.getAttribute(checkableStateAttribute)).toBe("checked");
		expect(sms.checked).toBe(false);
		expect(sms.getAttribute(checkableStateAttribute)).toBe("unchecked");

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event("change", { bubbles: true }));
		});

		expect(value_changes).toEqual([["email", "sms"]]);
		expect(item_mix_target_tag).toBe("INPUT");
		expect(sms.checked).toBe(true);
		expect(sms.getAttribute(checkableStateAttribute)).toBe("checked");

		result.cleanup();
	});
});
