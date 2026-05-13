// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	checkableChangeEvent,
	checkableState,
	checkableStateAttribute,
} from "./checkable-state.ts";
import { formResetEvent } from "./form-reset.ts";
import {
	componentAnatomyAttrs,
	componentDataAttribute,
	createCheckbox,
	type CheckboxChecked,
	type CheckboxCheckedChangeDetails,
	type CheckboxStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

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
				unchecked: {
					background: "white",
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

function create_test_style_system(): CheckboxStyleSystem<
	"light",
	typeof checkbox_recipe
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
				checkbox: checkbox_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Checkbox", () => {
	setup_remix_component_test_environment();

	it("renders a native checkbox host with checked state semantics", async () => {
		const Checkbox = createCheckbox(create_test_style_system());
		let current_target_tag: string | undefined;
		let current_target_checked: boolean | undefined;
		const result = render(
			createElement(Checkbox, {
				"data-testid": "checkbox",
				checked: checkableState.indeterminate,
				mix: on<HTMLInputElement, typeof checkableChangeEvent>(
					checkableChangeEvent,
					(event) => {
						current_target_tag = event.currentTarget.tagName;
						current_target_checked = event.currentTarget.checked;
					},
				),
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
			checkableState.indeterminate,
		);
		expect(checkbox.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"checkbox",
		);
		expect(checkbox.getAttribute(componentAnatomyAttrs.part)).toBe("root");

		await result.act(() => {
			checkbox.indeterminate = false;
			checkbox.checked = true;
			checkbox.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_checked).toBe(true);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		result.cleanup();
	});

	it("applies host state and form props to the native input", () => {
		const Checkbox = createCheckbox(create_test_style_system());
		const result = render(
			createElement(Checkbox, {
				"data-testid": "checkbox",
				disabled: true,
				form: "settings",
				readOnly: true,
				required: true,
			}),
		);

		const checkbox = result.$(
			"[data-testid='checkbox']",
		) as HTMLInputElement;

		expect(checkbox.disabled).toBe(true);
		expect(checkbox.required).toBe(true);
		expect(checkbox.getAttribute("form")).toBe("settings");
		expect(checkbox.getAttribute(componentDataAttribute.disabled)).toBe("");
		expect(checkbox.getAttribute(componentDataAttribute.readOnly)).toBe("");
		expect(checkbox.getAttribute(componentDataAttribute.required)).toBe("");

		result.cleanup();
	});

	it("emits semantic checked changes and syncs controlled props", async () => {
		const Checkbox = createCheckbox(create_test_style_system());
		let checked: CheckboxChecked = false;
		const changes: CheckboxChecked[] = [];
		const event_types: string[] = [];

		function view(): ReturnType<typeof createElement> {
			return createElement(Checkbox, {
				"data-testid": "checkbox",
				checked,
				onCheckedChange: (
					next_checked: CheckboxChecked,
					details?: CheckboxCheckedChangeDetails,
				) => {
					changes.push(next_checked);
					if (details?.event) {
						event_types.push(details.event.type);
					}
				},
			});
		}

		const result = render(view());
		const checkbox = result.$(
			"[data-testid='checkbox']",
		) as HTMLInputElement;

		expect(checkbox.checked).toBe(false);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);

		await result.act(() => {
			checkbox.checked = true;
			checkbox.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(changes).toEqual([true]);
		expect(event_types).toEqual([checkableChangeEvent]);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		await result.act(() => {
			result.root.render(view());
		});

		expect(checkbox.checked).toBe(false);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);

		checked = true;
		await result.act(() => {
			result.root.render(view());
		});

		expect(checkbox.checked).toBe(true);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		result.cleanup();
	});

	it("prevents read-only changes before emitting checked changes", async () => {
		const Checkbox = createCheckbox(create_test_style_system());
		const changes: CheckboxChecked[] = [];
		const result = render(
			createElement(Checkbox, {
				"data-testid": "checkbox",
				defaultChecked: false,
				onCheckedChange: (checked: CheckboxChecked) => {
					changes.push(checked);
				},
				readOnly: true,
			}),
		);

		const checkbox = result.$(
			"[data-testid='checkbox']",
		) as HTMLInputElement;

		await result.act(() => {
			checkbox.checked = true;
			checkbox.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(changes).toEqual([]);
		expect(checkbox.checked).toBe(false);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);
		expect(checkbox.getAttribute(componentDataAttribute.readOnly)).toBe("");

		result.cleanup();
	});

	it("resets uncontrolled indeterminate state from the form owner", async () => {
		const Checkbox = createCheckbox(create_test_style_system());
		const result = render(
			createElement(
				"form",
				{},
				createElement(Checkbox, {
					"data-testid": "checkbox",
					defaultChecked: checkableState.indeterminate,
				}),
			),
		);

		const form = result.$("form") as HTMLFormElement;
		const checkbox = result.$(
			"[data-testid='checkbox']",
		) as HTMLInputElement;
		expect(checkbox.checked).toBe(false);
		expect(checkbox.indeterminate).toBe(true);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.indeterminate,
		);

		await result.act(() => {
			checkbox.indeterminate = false;
			checkbox.checked = true;
			checkbox.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		await result.act(() => {
			form.dispatchEvent(
				new Event(formResetEvent, {
					bubbles: true,
					cancelable: true,
				}),
			);
			checkbox.checked = checkbox.defaultChecked;
			checkbox.indeterminate = false;
			queueMicrotask(() => {});
		});

		expect(checkbox.checked).toBe(false);
		expect(checkbox.indeterminate).toBe(true);
		expect(checkbox.getAttribute(checkableStateAttribute)).toBe(
			checkableState.indeterminate,
		);

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
		const Checkbox = createCheckbox(create_test_style_system());
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
