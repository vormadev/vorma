// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentStateAttribute } from "./component-state.ts";
import {
	componentAnatomyAttrs,
	createCollapsible,
	type CollapsibleStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Collapsible", () => {
	setup_remix_component_test_environment();

	it("toggles content visibility from the trigger", async () => {
		const collapsible_recipe = {
			defaultVariants: {
				layout: "stacked",
			},
			slots: {
				content: {},
				root: {},
				trigger: {},
			},
			variants: {
				layout: {
					stacked: {
						root: {},
					},
				},
			},
		} as const;
		const Collapsible = createCollapsible({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					collapsible: collapsible_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies CollapsibleStyleSystem<"light", typeof collapsible_recipe>);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Collapsible.Root,
				{
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Collapsible.Trigger, {}, "Toggle"),
				createElement(Collapsible.Content, {}, "Panel"),
			),
		);
		const root = result.$("[data-vorma-scope='collapsible']");
		const trigger = result.$("button");
		const content = result.$("[data-vorma-part='content']");
		if (!trigger || !(content instanceof HTMLElement)) {
			throw new Error("Expected Collapsible trigger and content hosts");
		}

		expect(root?.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
		expect(content.hidden).toBe(true);
		expect(content.getAttribute(componentStateAttribute)).toBe("closed");

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true]);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");
		expect(content.hidden).toBe(false);
		expect(content.getAttribute(componentStateAttribute)).toBe("open");

		result.cleanup();
	});
});
