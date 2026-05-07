// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { checkableStateAttribute } from "./checkable-state.ts";
import {
	componentAnatomyAttrs,
	createRadioGroup,
	type RadioGroupStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const radio_group_recipe = {
	defaultVariants: {
		layout: "stacked",
		size: "md",
		variant: "default",
	},
	slots: {
		item: {
			conditions: {
				checked: {
					background: "blue",
				},
				unchecked: {
					background: "white",
				},
			},
		},
		root: {
			base: {
				display: "grid",
			},
		},
	},
	variants: {
		layout: {
			stacked: {
				root: {
					base: {
						gap: "0.5rem",
					},
				},
			},
		},
		size: {
			md: {
				item: {
					base: {
						inlineSize: "1rem",
					},
				},
			},
		},
		variant: {
			default: {
				item: {},
			},
		},
	},
} as const;

function create_test_style_system(): RadioGroupStyleSystem<
	"light",
	typeof radio_group_recipe
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
				radioGroup: radio_group_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix RadioGroup", () => {
	setup_remix_component_test_environment();

	it("coordinates native radio items through group value state", async () => {
		const RadioGroup = createRadioGroup(create_test_style_system());
		const value_changes: string[] = [];
		let item_mix_target_tag: string | undefined;
		const result = render(
			createElement(
				RadioGroup.Root,
				{
					defaultValue: "email",
					name: "contact",
					onValueChange: (value: string) => {
						value_changes.push(value);
					},
				},
				createElement(RadioGroup.Item, {
					"data-testid": "email",
					value: "email",
				}),
				createElement(RadioGroup.Item, {
					"data-testid": "sms",
					mix: on<HTMLInputElement, "change">("change", (event) => {
						item_mix_target_tag = event.currentTarget.tagName;
					}),
					value: "sms",
				}),
			),
		);

		const root = result.$("[role='radiogroup']");
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;
		expect(root?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"radioGroup",
		);
		expect(email.type).toBe("radio");
		expect(email.name).toBe("contact");
		expect(sms.name).toBe("contact");
		expect(email.checked).toBe(true);
		expect(email.getAttribute(checkableStateAttribute)).toBe("checked");
		expect(sms.checked).toBe(false);
		expect(sms.getAttribute(checkableStateAttribute)).toBe("unchecked");

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event("change", { bubbles: true }));
		});

		expect(value_changes).toEqual(["sms"]);
		expect(item_mix_target_tag).toBe("INPUT");
		expect(email.checked).toBe(false);
		expect(email.getAttribute(checkableStateAttribute)).toBe("unchecked");
		expect(sms.checked).toBe(true);
		expect(sms.getAttribute(checkableStateAttribute)).toBe("checked");

		result.cleanup();
	});
});
