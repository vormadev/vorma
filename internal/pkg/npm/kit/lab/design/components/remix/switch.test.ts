// @vitest-environment jsdom

import { createElement, on } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { checkableStateAttribute } from "./checkable-state.ts";
import {
	componentAnatomyAttrs,
	createSwitch,
	type SwitchStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Switch", () => {
	setup_remix_component_test_environment();

	it("renders switch semantics on a native checkbox host", async () => {
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
					switch: switch_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies SwitchStyleSystem<"light", typeof switch_recipe>;
		const Switch = createSwitch(style_system);
		let current_target_tag: string | undefined;
		let current_target_checked: boolean | undefined;
		const result = render(
			createElement(Switch, {
				"data-testid": "switch",
				defaultChecked: false,
				mix: on<HTMLInputElement, "change">("change", (event) => {
					current_target_tag = event.currentTarget.tagName;
					current_target_checked = event.currentTarget.checked;
				}),
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
			"unchecked",
		);
		expect(switch_host.getAttribute(componentAnatomyAttrs.scope)).toBe(
			"switch",
		);
		expect(switch_host.getAttribute(componentAnatomyAttrs.part)).toBe(
			"root",
		);

		await result.act(() => {
			switch_host.checked = true;
			switch_host.dispatchEvent(new Event("change", { bubbles: true }));
		});

		expect(current_target_tag).toBe("INPUT");
		expect(current_target_checked).toBe(true);
		expect(switch_host.getAttribute(checkableStateAttribute)).toBe(
			"checked",
		);

		result.cleanup();
	});
});
