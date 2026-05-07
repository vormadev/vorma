// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentStateAttribute } from "./component-state.ts";
import {
	componentAnatomyAttrs,
	createAccordion,
	type AccordionStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const accordion_recipe = {
	defaultVariants: {
		layout: "stacked",
		size: "md",
		variant: "default",
	},
	slots: {
		item: {},
		panel: {},
		root: {},
		trigger: {},
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

function create_test_style_system(): AccordionStyleSystem<
	"light",
	typeof accordion_recipe
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
				accordion: accordion_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Accordion", () => {
	setup_remix_component_test_environment();

	it("toggles item panels through trigger buttons", async () => {
		const Accordion = createAccordion(create_test_style_system());
		const value_changes: (readonly string[] | string | null)[] = [];
		const result = render(
			createElement(
				Accordion.Root,
				{
					defaultValue: "shipping",
					onValueChange: (
						value: readonly string[] | string | null,
					) => {
						value_changes.push(value);
					},
				},
				createElement(
					Accordion.Item,
					{
						value: "shipping",
					},
					createElement(Accordion.Trigger, {}, "Shipping"),
					createElement(Accordion.Panel, {}, "Ships tomorrow"),
				),
				createElement(
					Accordion.Item,
					{
						value: "billing",
					},
					createElement(Accordion.Trigger, {}, "Billing"),
					createElement(Accordion.Panel, {}, "Visa"),
				),
			),
		);

		const root = result.$("[data-vorma-scope='accordion']");
		const triggers = result.$$("button");
		const panels = result.$$("[data-vorma-part='panel']");
		expect(root?.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(triggers[0]?.getAttribute("aria-expanded")).toBe("true");
		expect(panels[0]?.getAttribute(componentStateAttribute)).toBe("open");
		expect((panels[0] as HTMLElement | undefined)?.hidden).toBe(false);
		expect(triggers[1]?.getAttribute("aria-expanded")).toBe("false");
		expect((panels[1] as HTMLElement | undefined)?.hidden).toBe(true);

		await result.act(() => {
			triggers[1]?.dispatchEvent(
				new MouseEvent("click", { bubbles: true }),
			);
		});

		expect(value_changes).toEqual(["billing"]);
		expect(triggers[0]?.getAttribute("aria-expanded")).toBe("false");
		expect((panels[0] as HTMLElement | undefined)?.hidden).toBe(true);
		expect(triggers[1]?.getAttribute("aria-expanded")).toBe("true");
		expect((panels[1] as HTMLElement | undefined)?.hidden).toBe(false);

		result.cleanup();
	});
});
