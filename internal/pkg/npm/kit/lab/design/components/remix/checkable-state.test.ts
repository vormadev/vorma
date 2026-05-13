// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	checkableChangeEvent,
	checkableStateAttribute,
	checkableStateMixin,
} from "./checkable-state.ts";
import { formResetEvent } from "./form-reset.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("checkable state mixin", () => {
	setup_remix_component_test_environment();

	it("syncs uncontrolled checkable state after native form resets", async () => {
		const result = render(
			createElement(
				"form",
				{},
				createElement("input", {
					defaultChecked: true,
					mix: checkableStateMixin({
						checked: undefined,
						defaultChecked: true,
					}),
					type: "checkbox",
				}),
			),
		);

		const form = result.$("form") as HTMLFormElement;
		const input = result.$("input") as HTMLInputElement;
		expect(input.checked).toBe(true);
		expect(input.getAttribute(checkableStateAttribute)).toBe("checked");

		await result.act(() => {
			input.checked = false;
			input.dispatchEvent(
				new Event(checkableChangeEvent, { bubbles: true }),
			);
		});

		expect(input.getAttribute(checkableStateAttribute)).toBe("unchecked");

		await result.act(() => {
			form.dispatchEvent(
				new Event(formResetEvent, {
					bubbles: true,
					cancelable: true,
				}),
			);
			input.checked = input.defaultChecked;
			queueMicrotask(() => {});
		});

		expect(input.checked).toBe(true);
		expect(input.getAttribute(checkableStateAttribute)).toBe("checked");

		result.cleanup();
	});
});
