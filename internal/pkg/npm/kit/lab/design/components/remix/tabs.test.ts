// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentStateAttribute } from "./component-state.ts";
import {
	componentAnatomyAttrs,
	createTabs,
	type TabsStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const tabs_recipe = {
	defaultVariants: {
		layout: "default",
		size: "md",
		variant: "default",
	},
	slots: {
		list: {},
		panel: {},
		root: {},
		trigger: {},
	},
	variants: {
		layout: {
			default: {
				root: {},
			},
		},
		size: {
			md: {
				trigger: {},
			},
		},
		variant: {
			default: {
				trigger: {},
			},
		},
	},
} as const;

function create_test_style_system(): TabsStyleSystem<
	"light",
	typeof tabs_recipe
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
				tabs: tabs_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Tabs", () => {
	setup_remix_component_test_environment();

	it("selects panels through tab triggers", async () => {
		const Tabs = createTabs(create_test_style_system());
		const value_changes: string[] = [];
		const result = render(
			createElement(
				Tabs.Root,
				{
					defaultValue: "account",
					onValueChange: (value: string) => {
						value_changes.push(value);
					},
				},
				createElement(
					Tabs.List,
					{},
					createElement(
						Tabs.Trigger,
						{ value: "account" },
						"Account",
					),
					createElement(
						Tabs.Trigger,
						{ value: "billing" },
						"Billing",
					),
				),
				createElement(
					Tabs.Panel,
					{ value: "account" },
					"Account panel",
				),
				createElement(
					Tabs.Panel,
					{ value: "billing" },
					"Billing panel",
				),
			),
		);

		const root = result.$("[data-vorma-scope='tabs']");
		const triggers = result.$$("[role='tab']");
		const panels = result.$$("[role='tabpanel']");
		expect(root?.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(result.$("[role='tablist']")).not.toBeNull();
		expect(triggers[0]?.getAttribute("aria-selected")).toBe("true");
		expect(triggers[0]?.getAttribute(componentStateAttribute)).toBe(
			"selected",
		);
		expect((panels[0] as HTMLElement | undefined)?.hidden).toBe(false);
		expect((panels[1] as HTMLElement | undefined)?.hidden).toBe(true);

		await result.act(() => {
			triggers[1]?.dispatchEvent(
				new MouseEvent("click", { bubbles: true }),
			);
		});

		expect(value_changes).toEqual(["billing"]);
		expect(triggers[0]?.getAttribute("aria-selected")).toBe("false");
		expect(triggers[1]?.getAttribute("aria-selected")).toBe("true");
		expect((panels[0] as HTMLElement | undefined)?.hidden).toBe(true);
		expect((panels[1] as HTMLElement | undefined)?.hidden).toBe(false);

		result.cleanup();
	});
});
