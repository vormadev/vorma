// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentStateAttribute } from "./component-state.ts";
import { createTooltip, type TooltipStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Tooltip", () => {
	setup_remix_component_test_environment();

	it("opens on trigger focus and closes on blur", async () => {
		const tooltip_recipe = {
			defaultVariants: {
				size: "md",
				variant: "default",
			},
			slots: {
				popup: {},
				trigger: {},
			},
			variants: {
				size: {
					md: {
						popup: {},
					},
				},
				variant: {
					default: {
						popup: {},
					},
				},
			},
		} as const;
		const Tooltip = createTooltip({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					tooltip: tooltip_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies TooltipStyleSystem<"light", typeof tooltip_recipe>);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Tooltip.Root,
				{
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Tooltip.Trigger, {}, "Info"),
				createElement(Tooltip.Popup, {}, "More information"),
			),
		);
		const trigger = result.$("button");
		const popup = result.$("[role='tooltip']") as HTMLElement | null;
		if (!trigger || !popup) {
			throw new Error("Expected Tooltip trigger and popup hosts");
		}

		expect(popup.hidden).toBe(true);
		expect(popup.getAttribute(componentStateAttribute)).toBe("closed");

		await result.act(() => {
			trigger.dispatchEvent(new FocusEvent("focus", { bubbles: true }));
		});

		expect(open_values).toEqual([true]);
		expect(trigger.getAttribute("aria-describedby")).toBe(
			popup.getAttribute("id"),
		);
		expect(popup.hidden).toBe(false);
		expect(popup.getAttribute(componentStateAttribute)).toBe("open");

		await result.act(() => {
			trigger.dispatchEvent(new FocusEvent("blur", { bubbles: true }));
		});

		expect(open_values).toEqual([true, false]);
		expect(popup.hidden).toBe(true);

		result.cleanup();
	});
});
