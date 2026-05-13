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
	createSwitch,
	type SwitchCheckedChangeDetails,
	type SwitchStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const switch_recipe = {
	defaultVariants: { size: "md", variant: "default" },
	slots: {
		root: {
			conditions: {
				checked: {
					background: "green",
				},
				unchecked: {
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
						inlineSize: "2rem",
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

function create_test_style_system(): SwitchStyleSystem<
	"light",
	typeof switch_recipe
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
				switch: switch_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Switch", () => {
	setup_remix_component_test_environment();

	it("renders switch semantics on a native checkbox host", async () => {
		const Switch = createSwitch(create_test_style_system());
		let current_target_tag: string | undefined;
		let current_target_checked: boolean | undefined;
		const result = render(
			createElement(Switch, {
				"data-testid": "switch",
				defaultChecked: false,
				mix: on<HTMLInputElement, typeof checkableChangeEvent>(
					checkableChangeEvent,
					(event) => {
						current_target_tag = event.currentTarget.tagName;
						current_target_checked = event.currentTarget.checked;
					},
				),
				name: "notifications",
			}),
		);

		const switch_host = result.$(
			"[data-testid='switch']",
		) as HTMLInputElement | null;
		expect(switch_host).not.toBeNull();
		if (!switch_host) {
			throw new Error("Expected Switch to render an input");
		}
		expect(switch_host.type).toBe("checkbox");
		expect(switch_host.getAttribute("role")).toBe("switch");
		expect(switch_host.name).toBe("notifications");
		expect(switch_host.checked).toBe(false);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);
		expect(switch_host.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"switch",
		);
		expect(switch_host.getAttribute(componentAnatomyAttrs.part)).toBe(
			"root",
		);

		await result.act(() => {
			switch_host.checked = true;
			switch_host.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_checked).toBe(true);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		result.cleanup();
	});

	it("applies host state and form props to the native input", () => {
		const Switch = createSwitch(create_test_style_system());
		const result = render(
			createElement(Switch, {
				"data-testid": "switch",
				disabled: true,
				form: "settings",
				readOnly: true,
				required: true,
			}),
		);

		const switch_host = result.$(
			"[data-testid='switch']",
		) as HTMLInputElement;

		expect(switch_host.disabled).toBe(true);
		expect(switch_host.required).toBe(true);
		expect(switch_host.getAttribute("form")).toBe("settings");
		expect(switch_host.getAttribute(componentDataAttribute.disabled)).toBe(
			"",
		);
		expect(switch_host.getAttribute(componentDataAttribute.readOnly)).toBe(
			"",
		);
		expect(switch_host.getAttribute(componentDataAttribute.required)).toBe(
			"",
		);

		result.cleanup();
	});

	it("emits semantic checked changes and syncs controlled props", async () => {
		const Switch = createSwitch(create_test_style_system());
		let checked = false;
		const changes: boolean[] = [];
		const event_types: string[] = [];

		function view(): ReturnType<typeof createElement> {
			return createElement(Switch, {
				"data-testid": "switch",
				checked,
				onCheckedChange: (
					next_checked: boolean,
					details?: SwitchCheckedChangeDetails,
				) => {
					changes.push(next_checked);
					if (details?.event) {
						event_types.push(details.event.type);
					}
				},
			});
		}

		const result = render(view());
		const switch_host = result.$(
			"[data-testid='switch']",
		) as HTMLInputElement;

		expect(switch_host.checked).toBe(false);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);

		await result.act(() => {
			switch_host.checked = true;
			switch_host.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(changes).toEqual([true]);
		expect(event_types).toEqual([checkableChangeEvent]);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		await result.act(() => {
			result.root.render(view());
		});

		expect(switch_host.checked).toBe(false);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);

		checked = true;
		await result.act(() => {
			result.root.render(view());
		});

		expect(switch_host.checked).toBe(true);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		result.cleanup();
	});

	it("prevents read-only changes before emitting checked changes", async () => {
		const Switch = createSwitch(create_test_style_system());
		const changes: boolean[] = [];
		const result = render(
			createElement(Switch, {
				"data-testid": "switch",
				defaultChecked: false,
				onCheckedChange: (checked: boolean) => {
					changes.push(checked);
				},
				readOnly: true,
			}),
		);

		const switch_host = result.$(
			"[data-testid='switch']",
		) as HTMLInputElement;

		await result.act(() => {
			switch_host.checked = true;
			switch_host.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(changes).toEqual([]);
		expect(switch_host.checked).toBe(false);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);
		expect(switch_host.getAttribute(componentDataAttribute.readOnly)).toBe(
			"",
		);

		result.cleanup();
	});

	it("keeps data-state synced after uncontrolled form reset", async () => {
		const Switch = createSwitch(create_test_style_system());
		const result = render(
			createElement(
				"form",
				{},
				createElement(Switch, {
					"data-testid": "switch",
					defaultChecked: true,
				}),
			),
		);

		const form = result.$("form") as HTMLFormElement;
		const switch_host = result.$(
			"[data-testid='switch']",
		) as HTMLInputElement;
		expect(switch_host.checked).toBe(true);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		await result.act(() => {
			switch_host.checked = false;
			switch_host.dispatchEvent(
				new Event(checkableChangeEvent, {
					bubbles: true,
				}),
			);
		});

		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.unchecked,
		);

		await result.act(() => {
			form.dispatchEvent(
				new Event(formResetEvent, {
					bubbles: true,
					cancelable: true,
				}),
			);
			switch_host.checked = switch_host.defaultChecked;
			queueMicrotask(() => {});
		});

		expect(switch_host.checked).toBe(true);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			checkableState.checked,
		);

		result.cleanup();
	});
});
