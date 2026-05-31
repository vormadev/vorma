// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render, type RenderResult } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createSelect,
	selectValueChangeReason,
	type SelectStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const test_select_recipe = {
	slots: {
		group: {},
		groupLabel: {},
		icon: {},
		list: {},
		option: {},
		optionIndicator: {},
		optionText: {},
		popup: {},
		separator: {},
		trigger: {},
		value: {},
	},
} as const;

function create_test_select_style_system(): SelectStyleSystem<
	"light",
	typeof test_select_recipe
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
				select: test_select_recipe,
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

describe("Remix Select", () => {
	setup_remix_component_test_environment();

	it("renders Vorma-owned select-only combobox anatomy", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const result = render(
			createElement(
				Select.Root,
				{
					invalid: true,
					placeholder: "Theme",
					required: true,
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		const popup = require_element<HTMLElement>(result, "div[popover='manual']");
		const list = require_element<HTMLElement>(result, "[role='listbox']");
		const option = require_element<HTMLElement>(result, "[role='option']");
		expect(trigger.getAttribute("role")).toBe("combobox");
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
		expect(trigger.getAttribute("aria-haspopup")).toBe("listbox");
		expect(trigger.getAttribute("aria-controls")).toBe(list.id);
		expect(trigger.getAttribute("aria-invalid")).toBe("true");
		expect(trigger.getAttribute("aria-required")).toBe("true");
		expect(popup.getAttribute(componentAnatomyAttrs.part)).toBe("popup");
		expect(list.getAttribute(componentAnatomyAttrs.part)).toBe("list");
		expect(list.getAttribute("tabindex")).toBe(null);
		expect(option.getAttribute(componentAnatomyAttrs.part)).toBe("option");
		expect(result.$("select")).toBe(null);

		result.cleanup();
	});

	it("mirrors controlled values into a native select for forms", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		let value: "dark" | "light" = "light";

		function view(): ReturnType<typeof createElement> {
			return createElement(
				Select.Root,
				{
					autoComplete: "off",
					form: "settings",
					name: "theme",
					placeholder: "Theme",
					required: true,
					value,
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			);
		}

		const result = render(view());
		await flush_render(result);
		const hidden_select = require_element<HTMLSelectElement>(
			result,
			"select[name='theme']",
		);
		expect(hidden_select.value).toBe("light");
		expect(hidden_select.required).toBe(true);
		expect(hidden_select.getAttribute("form")).toBe("settings");
		expect(hidden_select.getAttribute("autocomplete")).toBe("off");
		expect(
			Array.from(hidden_select.options).map((option) => {
				return option.value;
			}),
		).toEqual(["", "light", "dark"]);

		value = "dark";
		await result.act(() => {
			result.root.render(view());
		});

		expect(hidden_select.value).toBe("dark");

		result.cleanup();
	});

	it("emits immediate pointer value changes from options", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const changed_reasons: (string | undefined)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					name: "theme",
					onValueChange: (
						value: string | null,
						details: { reason: string } | undefined,
					) => {
						changed_values.push(value);
						changed_reasons.push(details?.reason);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
					),
				),
			),
		);
		await flush_render(result);

		const option = require_element<HTMLElement>(result, "[role='option']");
		await result.act(() => {
			option.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		const hidden_select = require_element<HTMLSelectElement>(
			result,
			"select[name='theme']",
		);
		expect(changed_values).toEqual(["light"]);
		expect(changed_reasons).toEqual([selectValueChangeReason.pointer]);
		expect(hidden_select.value).toBe("light");

		result.cleanup();
	});

	it("does not select disabled options", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(
							Select.Option,
							{
								disabled: true,
								value: "light",
							},
							"Light",
						),
					),
				),
			),
		);
		await flush_render(result);

		const option = require_element<HTMLElement>(result, "[role='option']");
		await result.act(() => {
			option.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(changed_values).toEqual([]);

		result.cleanup();
	});

	it("keeps DOM focus on the trigger while keyboard-selecting options", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		trigger.focus();
		await dispatch_keydown(result, trigger, "ArrowUp");
		expect(document.activeElement).toBe(trigger);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");
		expect(trigger.getAttribute("aria-activedescendant")).toBe(
			result.$$("[role='option']")[1]?.id,
		);

		await dispatch_keydown(result, trigger, "Enter");

		expect(changed_values).toEqual(["dark"]);
		expect(trigger.getAttribute("aria-expanded")).toBe("false");

		result.cleanup();
	});

	it("closes with Escape without committing the highlighted option", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					defaultValue: "system",
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "system" }, "System"),
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "ArrowUp");
		await dispatch_keydown(result, trigger, "Escape");

		expect(changed_values).toEqual([]);
		expect(open_values).toEqual([true, false]);
		expect(trigger.getAttribute("aria-expanded")).toBe("false");

		result.cleanup();
	});

	it("commits the highlighted option on Tab without preventing default", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "ArrowUp");
		const tab_event = await dispatch_keydown(result, trigger, "Tab");

		expect(changed_values).toEqual(["dark"]);
		expect(tab_event.defaultPrevented).toBe(false);

		result.cleanup();
	});

	it("does not wrap keyboard navigation by default", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const highlighted_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					defaultHighlightedValue: "light",
					defaultOpen: true,
					onHighlightChange: (value: string | null) => {
						highlighted_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "ArrowUp");

		expect(highlighted_values).toEqual([]);
		expect(trigger.getAttribute("aria-activedescendant")).toBe(
			result.$$("[role='option']")[0]?.id,
		);

		result.cleanup();
	});

	it("can opt into wrapping keyboard navigation", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const highlighted_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					defaultHighlightedValue: "light",
					defaultOpen: true,
					loopFocus: true,
					onHighlightChange: (value: string | null) => {
						highlighted_values.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "ArrowUp");

		expect(highlighted_values).toEqual(["dark"]);
		expect(trigger.getAttribute("aria-activedescendant")).toBe(
			result.$$("[role='option']")[1]?.id,
		);

		result.cleanup();
	});

	it("supports printable typeahead without committing until selection", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const highlighted_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onHighlightChange: (value: string | null) => {
						highlighted_values.push(value);
					},
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Fruit",
				},
				createElement(Select.Trigger, {}, "Fruit"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "apple" }, "Apple"),
						createElement(Select.Option, { value: "apricot" }, "Apricot"),
						createElement(Select.Option, { value: "banana" }, "Banana"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "a");
		await dispatch_keydown(result, trigger, "a");
		expect(changed_values).toEqual([]);
		expect(highlighted_values).toEqual(["apple", "apricot"]);

		await dispatch_keydown(result, trigger, "Enter");

		expect(changed_values).toEqual(["apricot"]);

		result.cleanup();
	});

	it("does not change value while read-only", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					defaultHighlightedValue: "dark",
					defaultOpen: true,
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					placeholder: "Theme",
					readOnly: true,
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await dispatch_keydown(result, trigger, "Enter");

		expect(changed_values).toEqual([]);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");
		expect(trigger.getAttribute("aria-readonly")).toBe("true");

		result.cleanup();
	});

	it("supports controlled highlighted option state", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		let highlighted_value: "dark" | "light" | null = "light";
		const highlight_requests: (string | null)[] = [];

		function view(): ReturnType<typeof createElement> {
			return createElement(
				Select.Root,
				{
					defaultOpen: true,
					highlightedValue: highlighted_value,
					onHighlightChange: (value: string | null) => {
						highlight_requests.push(value);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			);
		}

		const result = render(view());
		await flush_render(result);
		const trigger = require_element<HTMLButtonElement>(result, "button");

		await dispatch_keydown(result, trigger, "ArrowDown");

		expect(highlight_requests).toEqual(["dark"]);
		expect(trigger.getAttribute("aria-activedescendant")).toBe(
			result.$$("[role='option']")[0]?.id,
		);

		highlighted_value = "dark";
		await result.act(() => {
			result.root.render(view());
		});

		expect(trigger.getAttribute("aria-activedescendant")).toBe(
			result.$$("[role='option']")[1]?.id,
		);

		result.cleanup();
	});

	it("wires group labels to listbox groups", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const result = render(
			createElement(
				Select.Root,
				{ placeholder: "Theme" },
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(
							Select.Group,
							{},
							createElement(Select.GroupLabel, {}, "Appearance"),
							createElement(Select.Option, { value: "light" }, "Light"),
						),
					),
				),
			),
		);
		await flush_render(result);

		const group = require_element<HTMLElement>(result, "[role='group']");
		const group_label = require_element<HTMLElement>(
			result,
			`[${componentAnatomyAttrs.part}='groupLabel']`,
		);
		expect(group.getAttribute("aria-labelledby")).toBe(group_label.id);

		result.cleanup();
	});

	it("closes from outside interaction", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
					),
				),
			),
		);
		await flush_render(result);

		await result.act(() => {
			document.body.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true }));
		});

		expect(open_values).toEqual([false]);
		expect(result.$("button")?.getAttribute("aria-expanded")).toBe("false");

		result.cleanup();
	});

	it("emits open lifecycle completion after updates", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const complete_values: boolean[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					onOpenChangeComplete: (open: boolean) => {
						complete_values.push(open);
					},
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
					),
				),
			),
		);
		await flush_render(result);

		const trigger = require_element<HTMLButtonElement>(result, "button");
		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(complete_values).toEqual([true]);

		result.cleanup();
	});

	it("releases popover scroll lock when the popup closes", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const result = render(
			createElement(
				Select.Root,
				{
					placeholder: "Theme",
				},
				createElement(Select.Trigger, {}, "Theme"),
				createElement(
					Select.Popup,
					{},
					createElement(
						Select.List,
						{},
						createElement(Select.Option, { value: "light" }, "Light"),
					),
				),
			),
		);
		await flush_render(result);

		const popup = require_element<HTMLElement>(result, "div[popover='manual']");
		const open_event = new Event("beforetoggle");
		Object.defineProperty(open_event, "newState", {
			value: "open",
		});
		popup.dispatchEvent(open_event);
		document.documentElement.style.overflow = "hidden";
		document.documentElement.style.scrollbarGutter = "stable";

		const close_event = new Event("toggle");
		Object.defineProperty(close_event, "newState", {
			value: "closed",
		});
		popup.dispatchEvent(close_event);

		expect(document.documentElement.style.overflow).toBe("");
		expect(document.documentElement.style.scrollbarGutter).toBe("");

		result.cleanup();
	});
});
