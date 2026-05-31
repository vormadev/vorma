// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render, type RenderResult } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createListbox,
	listboxHighlightChangeReason,
	listboxValueChangeReason,
	type ListboxStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const test_listbox_recipe = {
	slots: {
		group: {},
		groupLabel: {},
		option: {},
		optionIndicator: {},
		optionText: {},
		root: {},
	},
} as const;

function create_test_listbox_style_system(): ListboxStyleSystem<
	"light",
	typeof test_listbox_recipe
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
				listbox: test_listbox_recipe,
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

async function dispatch_focus(result: RenderResult, target: HTMLElement): Promise<void> {
	await result.act(() => {
		target.dispatchEvent(new FocusEvent("focus"));
	});
}

describe("Remix Listbox", () => {
	setup_remix_component_test_environment();

	it("renders a focusable single-select listbox with option anatomy", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const result = render(
			createElement(
				Listbox.Root,
				{
					invalid: true,
					required: true,
				},
				createElement(Listbox.Option, { value: "light" }, "Light"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		const option = require_element<HTMLElement>(result, "[role='option']");
		expect(root.getAttribute(componentAnatomyAttrs.scope)).toBe("listbox");
		expect(root.getAttribute(componentAnatomyAttrs.part)).toBe("root");
		expect(root.getAttribute("tabindex")).toBe("0");
		expect(root.getAttribute("aria-invalid")).toBe("true");
		expect(root.getAttribute("aria-orientation")).toBe("vertical");
		expect(root.getAttribute("aria-required")).toBe("true");
		expect(option.getAttribute(componentAnatomyAttrs.part)).toBe("option");
		expect(option.getAttribute("aria-selected")).toBe("false");

		result.cleanup();
	});

	it("focuses the selected option with aria-activedescendant", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const highlighted_values: (string | null)[] = [];
		const highlighted_reasons: (string | undefined)[] = [];
		const result = render(
			createElement(
				Listbox.Root,
				{
					defaultValue: "dark",
					onHighlightChange: (
						value: string | null,
						details: { reason: string } | undefined,
					) => {
						highlighted_values.push(value);
						highlighted_reasons.push(details?.reason);
					},
				},
				createElement(Listbox.Option, { value: "light" }, "Light"),
				createElement(Listbox.Option, { value: "dark" }, "Dark"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		const selected_option = require_element<HTMLElement>(
			result,
			"[role='option'][aria-selected='true']",
		);
		await dispatch_focus(result, root);

		expect(root.getAttribute("aria-activedescendant")).toBe(selected_option.id);
		expect(selected_option.getAttribute("data-highlighted")).toBe("");
		expect(highlighted_values).toEqual(["dark"]);
		expect(highlighted_reasons).toEqual([listboxHighlightChangeReason.focus]);

		result.cleanup();
	});

	it("moves highlight with arrows and selects with Enter", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const changed_values: (string | null)[] = [];
		const changed_reasons: (string | undefined)[] = [];
		const result = render(
			createElement(
				Listbox.Root,
				{
					onValueChange: (
						value: string | null,
						details: { reason: string } | undefined,
					) => {
						changed_values.push(value);
						changed_reasons.push(details?.reason);
					},
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
				createElement(Listbox.Option, { value: "gamma" }, "Gamma"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		await dispatch_focus(result, root);
		await dispatch_keydown(result, root, "ArrowDown");
		expect(changed_values).toEqual([]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Beta");

		await dispatch_keydown(result, root, "Enter");

		expect(changed_values).toEqual(["beta"]);
		expect(changed_reasons).toEqual([listboxValueChangeReason.keyboard]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][aria-selected='true']")
				.textContent,
		).toBe("Beta");

		result.cleanup();
	});

	it("honors controlled value and highlighted value", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		let value: "alpha" | "beta" = "alpha";
		let highlighted_value: "alpha" | "beta" = "alpha";
		const value_requests: (string | null)[] = [];
		const highlight_requests: (string | null)[] = [];

		function view(): ReturnType<typeof createElement> {
			return createElement(
				Listbox.Root,
				{
					highlightedValue: highlighted_value,
					onHighlightChange: (next_value: string | null) => {
						highlight_requests.push(next_value);
					},
					onValueChange: (next_value: string | null) => {
						value_requests.push(next_value);
					},
					value,
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
			);
		}

		const result = render(view());
		await flush_render(result);
		const root = require_element<HTMLElement>(result, "[role='listbox']");

		await dispatch_keydown(result, root, "ArrowDown");
		expect(highlight_requests).toEqual(["beta"]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Alpha");

		highlighted_value = "beta";
		await result.act(() => {
			result.root.render(view());
		});
		await dispatch_keydown(result, root, "Enter");
		expect(value_requests).toEqual(["beta"]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][aria-selected='true']")
				.textContent,
		).toBe("Alpha");

		value = "beta";
		await result.act(() => {
			result.root.render(view());
		});
		expect(
			require_element<HTMLElement>(result, "[role='option'][aria-selected='true']")
				.textContent,
		).toBe("Beta");

		result.cleanup();
	});

	it("supports loop focus and clamped page jumps", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const result = render(
			createElement(
				Listbox.Root,
				{
					loopFocus: true,
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
				createElement(Listbox.Option, { value: "gamma" }, "Gamma"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		await dispatch_focus(result, root);
		await dispatch_keydown(result, root, "ArrowUp");
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Gamma");

		await dispatch_keydown(result, root, "PageUp");
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Alpha");

		await dispatch_keydown(result, root, "PageDown");
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Gamma");

		result.cleanup();
	});

	it("can opt into selection following focus", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Listbox.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
					selectionFollowsFocus: true,
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		await dispatch_focus(result, root);
		await dispatch_keydown(result, root, "ArrowDown");

		expect(changed_values).toEqual(["alpha", "beta"]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][aria-selected='true']")
				.textContent,
		).toBe("Beta");

		result.cleanup();
	});

	it("supports multiple selection with the recommended toggle model", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const changed_values: string[][] = [];
		const result = render(
			createElement(
				Listbox.Root,
				{
					defaultValues: ["alpha"],
					onValuesChange: (values: readonly string[]) => {
						changed_values.push([...values]);
					},
					selectionMode: "multiple",
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
				createElement(Listbox.Option, { value: "gamma" }, "Gamma"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		expect(root.getAttribute("aria-multiselectable")).toBe("true");
		expect(
			require_element<HTMLElement>(result, "[role='option'][aria-selected='true']")
				.textContent,
		).toBe("Alpha");

		await dispatch_focus(result, root);
		await dispatch_keydown(result, root, "ArrowDown");
		expect(changed_values).toEqual([]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Beta");

		await dispatch_keydown(result, root, " ");
		expect(changed_values).toEqual([["alpha", "beta"]]);
		expect(result.$$("[role='option'][aria-selected='true']").length).toBe(2);

		await dispatch_keydown(result, root, " ");
		expect(changed_values).toEqual([["alpha", "beta"], ["alpha"]]);
		expect(result.$$("[role='option'][aria-selected='true']").length).toBe(1);

		result.cleanup();
	});

	it("honors controlled multiple values", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		let values: readonly ("alpha" | "beta")[] = ["alpha"];
		const value_requests: string[][] = [];

		function view(): ReturnType<typeof createElement> {
			return createElement(
				Listbox.Root,
				{
					onValuesChange: (next_values: readonly string[]) => {
						value_requests.push([...next_values]);
					},
					selectionMode: "multiple",
					values,
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
			);
		}

		const result = render(view());
		await flush_render(result);
		const beta = require_element<HTMLElement>(result, "[role='option']:last-child");
		await result.act(() => {
			beta.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(value_requests).toEqual([["alpha", "beta"]]);
		expect(result.$$("[role='option'][aria-selected='true']").length).toBe(1);

		values = ["alpha", "beta"];
		await result.act(() => {
			result.root.render(view());
		});
		expect(result.$$("[role='option'][aria-selected='true']").length).toBe(2);

		result.cleanup();
	});

	it("skips disabled options during typeahead and pointer selection", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const changed_values: (string | null)[] = [];
		const result = render(
			createElement(
				Listbox.Root,
				{
					onValueChange: (value: string | null) => {
						changed_values.push(value);
					},
				},
				createElement(
					Listbox.Option,
					{
						disabled: true,
						value: "apple",
					},
					"Apple",
				),
				createElement(Listbox.Option, { value: "apricot" }, "Apricot"),
				createElement(Listbox.Option, { value: "banana" }, "Banana"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		const disabled_option = require_element<HTMLElement>(
			result,
			"[role='option'][aria-disabled='true']",
		);
		await result.act(() => {
			disabled_option.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});
		await dispatch_keydown(result, root, "a");

		expect(changed_values).toEqual([]);
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Apricot");

		result.cleanup();
	});

	it("supports horizontal arrow navigation", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const result = render(
			createElement(
				Listbox.Root,
				{
					orientation: "horizontal",
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
				createElement(Listbox.Option, { value: "beta" }, "Beta"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		await dispatch_focus(result, root);
		await dispatch_keydown(result, root, "ArrowRight");

		expect(root.getAttribute("aria-orientation")).toBe("horizontal");
		expect(root.getAttribute("data-orientation")).toBe("horizontal");
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Beta");

		await dispatch_keydown(result, root, "ArrowLeft");
		expect(
			require_element<HTMLElement>(result, "[role='option'][data-highlighted]")
				.textContent,
		).toBe("Alpha");

		result.cleanup();
	});

	it("does not focus or highlight while disabled", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const result = render(
			createElement(
				Listbox.Root,
				{
					disabled: true,
				},
				createElement(Listbox.Option, { value: "alpha" }, "Alpha"),
			),
		);
		await flush_render(result);

		const root = require_element<HTMLElement>(result, "[role='listbox']");
		const option = require_element<HTMLElement>(result, "[role='option']");
		await dispatch_focus(result, root);
		await result.act(() => {
			option.dispatchEvent(new MouseEvent("pointermove", { bubbles: true }));
		});

		expect(root.getAttribute("tabindex")).toBe(null);
		expect(root.getAttribute("aria-activedescendant")).toBe(null);
		expect(result.$("[role='option'][data-highlighted]")).toBe(null);

		result.cleanup();
	});

	it("wires grouped options to their label", async () => {
		const style_system = create_test_listbox_style_system();
		const Listbox = createListbox(style_system);
		const result = render(
			createElement(
				Listbox.Root,
				{},
				createElement(
					Listbox.Group,
					{},
					createElement(Listbox.GroupLabel, {}, "Theme"),
					createElement(Listbox.Option, { value: "light" }, "Light"),
				),
			),
		);
		await flush_render(result);

		const group = require_element<HTMLElement>(result, "[role='group']");
		const label = require_element<HTMLElement>(
			result,
			`[${componentAnatomyAttrs.part}='groupLabel']`,
		);
		expect(group.getAttribute("aria-labelledby")).toBe(label.id);

		result.cleanup();
	});
});
