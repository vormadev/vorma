// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createKbd,
	kbdDefaultElement,
	kbdScope,
	type KbdStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const kbd_test_id = "kbd";
const kbd_test_recipe = {
	slots: {
		root: {},
	},
	variants: {
		size: {
			sm: {
				root: {
					base: {
						fontSize: "12px",
					},
				},
			},
		},
	},
} as const;

function create_test_kbd(): ReturnType<typeof createKbd> {
	return createKbd({
		metadata: {},
		modes: { light: { variables: {} } },
		token: { recipe: { kbd: kbd_test_recipe } },
		variablePrefix: "test",
	} satisfies KbdStyleSystem<"light", typeof kbd_test_recipe>);
}

function style_text(): string {
	return document.adoptedStyleSheets
		.flatMap((sheet) => {
			return Array.from(sheet.cssRules).map((rule) => {
				return rule.cssText;
			});
		})
		.join("\n");
}

describe("Remix Kbd", () => {
	setup_remix_component_test_environment();

	it("renders semantic keyboard input text by default", () => {
		const Kbd = create_test_kbd();
		const result = render(
			createElement(Kbd, { "data-testid": kbd_test_id }, "Shift"),
		);
		const host = result.$(`[data-testid='${kbd_test_id}']`);

		expect(host?.tagName.toLowerCase()).toBe(kbdDefaultElement);
		expect(host?.textContent).toBe("Shift");
		expect(host?.getAttribute(componentAnatomyAttrs.scope)).toBe(kbdScope);
		expect(host?.getAttribute(componentAnatomyAttrs.part)).toBe("root");

		result.cleanup();
	});

	it("supports the size recipe variant", () => {
		const Kbd = create_test_kbd();
		const result = render(
			createElement(
				Kbd,
				{
					"data-testid": kbd_test_id,
					size: "sm",
				},
				"Ctrl",
			),
		);
		const host = result.$(`[data-testid='${kbd_test_id}']`);

		expect(host?.className).toContain("rmxc-");
		expect(style_text()).toContain("font-size: 12px");

		result.cleanup();
	});

	it("allows the host element to be changed", () => {
		const Kbd = create_test_kbd();
		const result = render(
			createElement(
				Kbd,
				{
					"data-testid": kbd_test_id,
					as: "span",
				},
				"Command",
			),
		);
		const host = result.$(`[data-testid='${kbd_test_id}']`);

		expect(host?.tagName).toBe("SPAN");
		expect(host?.textContent).toBe("Command");

		result.cleanup();
	});
});
