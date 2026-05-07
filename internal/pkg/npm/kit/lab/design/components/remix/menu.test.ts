// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render, type RenderResult } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentStateAttribute } from "./component-state.ts";
import {
	componentAnatomyAttrs,
	createMenu,
	menuOpenChangeReason,
	type MenuStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const test_menu_recipe = {
	defaultVariants: {
		layout: "default",
		size: "md",
		variant: "default",
	},
	slots: {
		group: {},
		groupLabel: {},
		item: {},
		popup: {},
		separator: {},
		trigger: {},
	},
	variants: {
		layout: {
			default: {
				popup: {},
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

function create_test_menu_style_system(): MenuStyleSystem<
	"light",
	typeof test_menu_recipe
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
				menu: test_menu_recipe,
			},
		},
		variablePrefix: "test",
	};
}

function require_element<TElement extends HTMLElement>(
	result: RenderResult,
	selector: string,
): TElement {
	const element = result.$(selector);
	if (!element) {
		throw new Error(`Expected element for selector ${selector}`);
	}
	return element as TElement;
}

async function flush_render(result: RenderResult): Promise<void> {
	await result.act(() => {});
}

async function dispatch_keydown(
	result: RenderResult,
	target: HTMLElement,
	key: string,
	init: KeyboardEventInit = {},
): Promise<KeyboardEvent> {
	const event = new KeyboardEvent("keydown", {
		bubbles: true,
		cancelable: true,
		key,
		...init,
	});
	await result.act(() => {
		target.dispatchEvent(event);
	});
	return event;
}

function get_menu_items(result: RenderResult): HTMLButtonElement[] {
	return Array.from(result.$$("[role='menuitem']")).map((item) => {
		return item as HTMLButtonElement;
	});
}

function require_menu_item(
	result: RenderResult,
	index: number,
): HTMLButtonElement {
	const item = get_menu_items(result)[index];
	if (!item) {
		throw new Error(`Expected menu item at index ${index}`);
	}
	return item;
}

describe("Remix Menu", () => {
	setup_remix_component_test_environment();

	it("opens from trigger, focuses the first item, and closes on item select", async () => {
		const Menu = createMenu(create_test_menu_style_system());
		const open_values: boolean[] = [];
		const open_reasons: (string | undefined)[] = [];
		const result = render(
			createElement(
				Menu.Root,
				{
					onOpenChange: (
						open: boolean,
						details: { reason: string } | undefined,
					) => {
						open_values.push(open);
						open_reasons.push(details?.reason);
					},
				},
				createElement(Menu.Trigger, {}, "Menu"),
				createElement(
					Menu.Popup,
					{},
					createElement(
						Menu.Group,
						{},
						createElement(Menu.Item, {}, "Edit"),
					),
					createElement(Menu.Separator, {}),
					createElement(Menu.Item, {}, "Delete"),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		const popup = require_element<HTMLElement>(result, "[role='menu']");
		expect(popup.hidden).toBe(true);
		expect(trigger.getAttribute("aria-haspopup")).toBe("menu");

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});
		await flush_render(result);

		const item = require_menu_item(result, 0);
		expect(open_values).toEqual([true]);
		expect(open_reasons).toEqual([menuOpenChangeReason.trigger]);
		expect(popup.hidden).toBe(false);
		expect(popup.getAttribute(componentStateAttribute)).toBe("open");
		expect(item.tabIndex).toBe(0);
		expect(document.activeElement).toBe(item);

		await result.act(() => {
			item.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true, false]);
		expect(open_reasons).toEqual([
			menuOpenChangeReason.trigger,
			menuOpenChangeReason.item,
		]);
		expect(popup.hidden).toBe(true);

		result.cleanup();
	});

	it("opens from keyboard and moves roving focus with arrow keys", async () => {
		const Menu = createMenu(create_test_menu_style_system());
		const result = render(
			createElement(
				Menu.Root,
				{},
				createElement(Menu.Trigger, {}, "Menu"),
				createElement(
					Menu.Popup,
					{},
					createElement(Menu.Item, {}, "Edit"),
					createElement(Menu.Item, {}, "Duplicate"),
					createElement(Menu.Item, {}, "Delete"),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		const popup = require_element<HTMLElement>(result, "[role='menu']");
		await dispatch_keydown(result, trigger, "ArrowDown");
		await flush_render(result);

		let items = get_menu_items(result);
		expect(popup.hidden).toBe(false);
		expect(document.activeElement).toBe(require_menu_item(result, 0));
		expect(items.map((item) => item.tabIndex)).toEqual([0, -1, -1]);

		await dispatch_keydown(
			result,
			require_menu_item(result, 0),
			"ArrowDown",
		);
		await flush_render(result);

		items = get_menu_items(result);
		expect(document.activeElement).toBe(require_menu_item(result, 1));
		expect(items.map((item) => item.tabIndex)).toEqual([-1, 0, -1]);

		await dispatch_keydown(result, require_menu_item(result, 1), "End");
		await flush_render(result);

		items = get_menu_items(result);
		expect(document.activeElement).toBe(require_menu_item(result, 2));
		expect(items.map((item) => item.tabIndex)).toEqual([-1, -1, 0]);

		result.cleanup();
	});

	it("keeps disabled menu items focusable but does not activate them", async () => {
		const Menu = createMenu(create_test_menu_style_system());
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Menu.Root,
				{
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Menu.Trigger, {}, "Menu"),
				createElement(
					Menu.Popup,
					{},
					createElement(Menu.Item, {}, "Edit"),
					createElement(Menu.Item, { disabled: true }, "Delete"),
				),
			),
		);
		await flush_render(result);

		const items = get_menu_items(result);
		await dispatch_keydown(
			result,
			require_menu_item(result, 0),
			"ArrowDown",
		);
		await flush_render(result);

		const disabled_item = require_menu_item(result, 1);
		expect(document.activeElement).toBe(disabled_item);
		expect(disabled_item.getAttribute("aria-disabled")).toBe("true");
		expect(disabled_item.disabled).toBe(false);

		await result.act(() => {
			disabled_item.dispatchEvent(
				new MouseEvent("click", { bubbles: true }),
			);
		});

		expect(open_values).toEqual([]);
		expect(
			require_element<HTMLElement>(result, "[role='menu']").hidden,
		).toBe(false);

		result.cleanup();
	});

	it("supports typeahead over menu item text", async () => {
		const Menu = createMenu(create_test_menu_style_system());
		const result = render(
			createElement(
				Menu.Root,
				{
					defaultOpen: true,
				},
				createElement(Menu.Trigger, {}, "Menu"),
				createElement(
					Menu.Popup,
					{},
					createElement(Menu.Item, {}, "Copy"),
					createElement(Menu.Item, {}, "Paste"),
					createElement(Menu.Item, {}, "Preferences"),
				),
			),
		);
		await flush_render(result);

		const popup = require_element<HTMLElement>(result, "[role='menu']");
		await dispatch_keydown(result, popup, "p");
		await flush_render(result);

		expect(document.activeElement?.textContent).toBe("Paste");
		await dispatch_keydown(
			result,
			document.activeElement as HTMLElement,
			"p",
		);
		await flush_render(result);
		expect(document.activeElement?.textContent).toBe("Preferences");

		result.cleanup();
	});

	it("closes with Escape and returns focus to the trigger", async () => {
		const Menu = createMenu(create_test_menu_style_system());
		const open_reasons: (string | undefined)[] = [];
		const result = render(
			createElement(
				Menu.Root,
				{
					onOpenChange: (
						_open: boolean,
						details: { reason: string } | undefined,
					) => {
						open_reasons.push(details?.reason);
					},
				},
				createElement(Menu.Trigger, {}, "Menu"),
				createElement(
					Menu.Popup,
					{},
					createElement(Menu.Item, {}, "Edit"),
					createElement(Menu.Item, {}, "Delete"),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "ArrowDown");
		await flush_render(result);
		expect(document.activeElement?.textContent).toBe("Edit");

		await dispatch_keydown(
			result,
			document.activeElement as HTMLElement,
			"Escape",
		);
		await flush_render(result);

		expect(open_reasons).toEqual([
			menuOpenChangeReason.trigger,
			menuOpenChangeReason.escape,
		]);
		expect(document.activeElement).toBe(trigger);
		expect(
			require_element<HTMLElement>(result, "[role='menu']").hidden,
		).toBe(true);

		result.cleanup();
	});

	it("preserves static menu anatomy", async () => {
		const Menu = createMenu(create_test_menu_style_system());
		const result = render(
			createElement(
				Menu.Root,
				{},
				createElement(Menu.Trigger, {}, "Menu"),
				createElement(
					Menu.Popup,
					{},
					createElement(
						Menu.Group,
						{},
						createElement(Menu.GroupLabel, {}, "Actions"),
						createElement(Menu.Item, {}, "Edit"),
					),
					createElement(Menu.Separator, {}),
				),
			),
		);
		await flush_render(result);

		expect(
			require_element<HTMLElement>(
				result,
				`[${componentAnatomyAttrs.part}='group']`,
			).getAttribute("role"),
		).toBe("group");
		expect(
			require_element<HTMLElement>(
				result,
				`[${componentAnatomyAttrs.part}='group']`,
			).getAttribute("aria-labelledby"),
		).toBe(
			require_element<HTMLElement>(
				result,
				`[${componentAnatomyAttrs.part}='groupLabel']`,
			).id,
		);
		expect(
			require_element<HTMLElement>(
				result,
				`[${componentAnatomyAttrs.part}='separator']`,
			).getAttribute("role"),
		).toBe("separator");

		result.cleanup();
	});
});
