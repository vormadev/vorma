// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { checkableChangeEvent, checkableStateAttribute } from "./checkable-state.ts";
import { formResetEvent } from "./form-reset.ts";
import {
	componentAnatomyAttrs,
	componentDataAttribute,
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
					form: "settings",
					name: "contact",
					onValueChange: (value: string) => {
						value_changes.push(value);
					},
					required: true,
				},
				createElement(RadioGroup.Item, {
					"data-testid": "email",
					value: "email",
				}),
				createElement(RadioGroup.Item, {
					"data-testid": "sms",
					mix: on<HTMLInputElement, typeof checkableChangeEvent>(
						checkableChangeEvent,
						(event) => {
							item_mix_target_tag = event.currentTarget.tagName;
						},
					),
					value: "sms",
				}),
			),
		);

		const root = result.$("[role='radiogroup']");
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;
		expect(root?.getAttribute(componentAnatomyAttrs.scope)).toBe("radioGroup");
		expect(email.type).toBe("radio");
		expect(root?.getAttribute("aria-required")).toBe("true");
		expect(root?.getAttribute(componentDataAttribute.required)).toBe("");
		expect(email.getAttribute("form")).toBe("settings");
		expect(email.required).toBe(true);
		expect(email.name).toBe("contact");
		expect(sms.name).toBe("contact");
		expect(email.checked).toBe(true);
		expect(email.getAttribute(checkableStateAttribute)).toBe("checked");
		expect(sms.checked).toBe(false);
		expect(sms.getAttribute(checkableStateAttribute)).toBe("unchecked");

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event(checkableChangeEvent, { bubbles: true }));
		});

		expect(value_changes).toEqual(["sms"]);
		expect(item_mix_target_tag).toBe("INPUT");
		expect(email.checked).toBe(false);
		expect(email.getAttribute(checkableStateAttribute)).toBe("unchecked");
		expect(sms.checked).toBe(true);
		expect(sms.getAttribute(checkableStateAttribute)).toBe("checked");

		result.cleanup();
	});

	it("preserves item-level form and required ownership", () => {
		const RadioGroup = createRadioGroup(create_test_style_system());
		const result = render(
			createElement(
				RadioGroup.Root,
				{
					form: "root-form",
					name: "contact",
				},
				createElement(RadioGroup.Item, {
					"data-testid": "email",
					value: "email",
				}),
				createElement(RadioGroup.Item, {
					"data-testid": "sms",
					form: "item-form",
					required: true,
					value: "sms",
				}),
			),
		);

		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;

		expect(email.getAttribute("form")).toBe("root-form");
		expect(email.required).toBe(false);
		expect(sms.getAttribute("form")).toBe("item-form");
		expect(sms.required).toBe(true);

		result.cleanup();
	});

	it("keeps controlled value state external until the owner updates it", async () => {
		const RadioGroup = createRadioGroup(create_test_style_system());
		let value: "email" | "sms" = "email";
		const value_changes: string[] = [];

		function view(): ReturnType<typeof createElement> {
			return createElement(
				RadioGroup.Root,
				{
					name: "contact",
					onValueChange: (next_value: string) => {
						value_changes.push(next_value);
					},
					value,
				},
				createElement(RadioGroup.Item, {
					"data-testid": "email",
					value: "email",
				}),
				createElement(RadioGroup.Item, {
					"data-testid": "sms",
					value: "sms",
				}),
			);
		}

		const result = render(view());
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event(checkableChangeEvent, { bubbles: true }));
		});

		expect(value_changes).toEqual(["sms"]);
		expect(email.checked).toBe(true);
		expect(sms.checked).toBe(false);

		value = "sms";
		await result.act(() => {
			result.root.render(view());
		});

		expect(email.checked).toBe(false);
		expect(sms.checked).toBe(true);

		result.cleanup();
	});

	it("does not change value from disabled root or item events", async () => {
		const RadioGroup = createRadioGroup(create_test_style_system());
		const value_changes: string[] = [];

		function disabled_root_view(): ReturnType<typeof createElement> {
			return createElement(
				RadioGroup.Root,
				{
					defaultValue: "email",
					disabled: true,
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
					value: "sms",
				}),
			);
		}

		const result = render(disabled_root_view());
		const root = result.$("[role='radiogroup']");
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;

		expect(root?.getAttribute("aria-disabled")).toBe("true");
		expect(root?.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(email.disabled).toBe(true);
		expect(sms.disabled).toBe(true);
		expect(sms.getAttribute(componentDataAttribute.disabled)).toBe("");

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event(checkableChangeEvent, { bubbles: true }));
		});
		await result.act(() => {
			result.root.render(disabled_root_view());
		});

		expect(value_changes).toEqual([]);
		expect(email.checked).toBe(true);
		expect(sms.checked).toBe(false);

		result.cleanup();

		const item_disabled_result = render(
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
					disabled: true,
					value: "sms",
				}),
			),
		);
		const item_disabled_email = item_disabled_result.$(
			"[data-testid='email']",
		) as HTMLInputElement;
		const item_disabled_sms = item_disabled_result.$(
			"[data-testid='sms']",
		) as HTMLInputElement;

		expect(item_disabled_sms.getAttribute(componentDataAttribute.disabled)).toBe("");

		await item_disabled_result.act(() => {
			item_disabled_sms.checked = true;
			item_disabled_sms.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});
		await item_disabled_result.act(() => {
			item_disabled_result.root.render(
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
						disabled: true,
						value: "sms",
					}),
				),
			);
		});

		expect(value_changes).toEqual([]);
		expect(item_disabled_email.checked).toBe(true);
		expect(item_disabled_sms.checked).toBe(false);

		item_disabled_result.cleanup();
	});

	it("does not change value from read-only root or item events", async () => {
		const RadioGroup = createRadioGroup(create_test_style_system());
		const value_changes: string[] = [];

		const result = render(
			createElement(
				RadioGroup.Root,
				{
					defaultValue: "email",
					name: "contact",
					onValueChange: (value: string) => {
						value_changes.push(value);
					},
					readOnly: true,
				},
				createElement(RadioGroup.Item, {
					"data-testid": "email",
					value: "email",
				}),
				createElement(RadioGroup.Item, {
					"data-testid": "sms",
					value: "sms",
				}),
			),
		);

		const root = result.$("[role='radiogroup']");
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;

		expect(root?.getAttribute(componentDataAttribute.readOnly)).toBe("");
		expect(sms.readOnly).toBe(true);
		expect(sms.getAttribute(componentDataAttribute.readOnly)).toBe("");

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event(checkableChangeEvent, { bubbles: true }));
		});

		expect(value_changes).toEqual([]);
		expect(email.checked).toBe(true);
		expect(sms.checked).toBe(false);

		result.cleanup();

		const item_read_only_result = render(
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
					readOnly: true,
					value: "sms",
				}),
			),
		);
		const item_read_only_email = item_read_only_result.$(
			"[data-testid='email']",
		) as HTMLInputElement;
		const item_read_only_sms = item_read_only_result.$(
			"[data-testid='sms']",
		) as HTMLInputElement;

		expect(item_read_only_sms.getAttribute(componentDataAttribute.readOnly)).toBe("");

		await item_read_only_result.act(() => {
			item_read_only_sms.checked = true;
			item_read_only_sms.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(value_changes).toEqual([]);
		expect(item_read_only_email.checked).toBe(true);
		expect(item_read_only_sms.checked).toBe(false);

		item_read_only_result.cleanup();
	});

	it("resets uncontrolled value state from the form owner", async () => {
		const RadioGroup = createRadioGroup(create_test_style_system());
		const value_changes: string[] = [];
		const result = render(
			createElement(
				"form",
				{ id: "contact-form" },
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
						value: "sms",
					}),
				),
			),
		);

		const form = result.$("form") as HTMLFormElement;
		const email = result.$("[data-testid='email']") as HTMLInputElement;
		const sms = result.$("[data-testid='sms']") as HTMLInputElement;

		await result.act(() => {
			sms.checked = true;
			sms.dispatchEvent(new Event(checkableChangeEvent, { bubbles: true }));
		});

		expect(value_changes).toEqual(["sms"]);
		expect(email.checked).toBe(false);
		expect(sms.checked).toBe(true);

		await result.act(() => {
			form.dispatchEvent(
				new Event(formResetEvent, {
					bubbles: true,
					cancelable: true,
				}),
			);
		});

		expect(value_changes).toEqual(["sms"]);
		expect(email.checked).toBe(true);
		expect(sms.checked).toBe(false);

		result.cleanup();
	});
});
