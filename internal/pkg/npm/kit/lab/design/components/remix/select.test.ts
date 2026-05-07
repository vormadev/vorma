// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createSelect,
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

describe("Remix Select", () => {
	setup_remix_component_test_environment();

	it("creates a Vorma-owned Remix select popup", () => {
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
						createElement(
							Select.Option,
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);

		const trigger = result.$("button");
		const popup = result.$("div[popover='manual']");
		const list = result.$("[role='listbox']");
		const option = result.$("[role='option']");
		expect(trigger?.getAttribute("aria-expanded")).toBe("false");
		expect(popup?.getAttribute(componentAnatomyAttrs.part)).toBe("popup");
		expect(list?.getAttribute(componentAnatomyAttrs.part)).toBe("list");
		expect(option?.getAttribute(componentAnatomyAttrs.part)).toBe("option");

		result.cleanup();
	});

	it("updates Select internals from controlled value props", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		let value: "dark" | "light" = "light";

		function view(): ReturnType<typeof createElement> {
			return createElement(
				Select.Root,
				{
					name: "theme",
					placeholder: "Theme",
					value,
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
							{ value: "light" },
							"Light",
						),
						createElement(Select.Option, { value: "dark" }, "Dark"),
					),
				),
			);
		}

		const result = render(view());
		expect(result.$("input[type='hidden']")?.getAttribute("value")).toBe(
			"light",
		);

		value = "dark";
		await result.act(() => {
			result.root.render(view());
		});

		expect(result.$("input[type='hidden']")?.getAttribute("value")).toBe(
			"dark",
		);

		result.cleanup();
	});

	it("keeps Select internals mounted for locally emitted controlled values", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		let value: "light" | "system" = "system";

		function view(): ReturnType<typeof createElement> {
			return createElement(
				Select.Root,
				{
					onValueChange: (next_value: string | null) => {
						value = next_value as "light" | "system";
					},
					placeholder: "Theme",
					value,
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
							{ value: "light" },
							"Light",
						),
					),
				),
			);
		}

		const result = render(view());
		const popup = result.$("div[popover='manual']");
		const option = result.$("[role='option']");
		if (!popup || !option) {
			throw new Error("Expected Select popup and option hosts");
		}

		option.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		await result.act(() => {
			result.root.render(view());
		});

		expect(value).toBe("light");
		expect(result.$("div[popover='manual']")).toBe(popup);

		result.cleanup();
	});

	it("releases Select popover scroll lock when the popup closes", () => {
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
						createElement(
							Select.Option,
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);
		const popup = result.$("div[popover='manual']");
		if (!popup) {
			throw new Error("Expected Select popup host");
		}

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

	it("emits Select onValueChange immediately from option click", async () => {
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
					name: "theme",
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
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);

		const option = result.$("[role='option']");
		if (!option) {
			throw new Error("Expected Select option host");
		}
		await result.act(() => {
			option.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(changed_values).toEqual(["light"]);
		expect(result.$("input[type='hidden']")?.getAttribute("value")).toBe(
			"light",
		);

		result.cleanup();
	});

	it("does not emit Select immediate value changes for disabled options", () => {
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

		const option = result.$("[role='option']");
		if (!option) {
			throw new Error("Expected Select option host");
		}
		option.dispatchEvent(new MouseEvent("click", { bubbles: true }));

		expect(changed_values).toEqual([]);

		result.cleanup();
	});

	it("emits Select onValueChange immediately from keyboard selection", async () => {
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
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);

		const list = result.$("[role='listbox']");
		if (!list) {
			throw new Error("Expected Select list host");
		}
		await result.act(() => {
			list.dispatchEvent(
				new KeyboardEvent("keydown", { key: "ArrowDown" }),
			);
		});
		await result.act(() => {
			list.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
		});

		expect(changed_values).toEqual(["light"]);

		result.cleanup();
	});

	it("closes Select from outside interaction", async () => {
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
						createElement(
							Select.Option,
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);

		await result.act(() => {
			document.body.dispatchEvent(
				new MouseEvent("pointerdown", { bubbles: true }),
			);
		});

		expect(open_values).toEqual([false]);
		expect(result.$("button")?.getAttribute("aria-expanded")).toBe("false");

		result.cleanup();
	});

	it("emits Select open lifecycle completion after updates", async () => {
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
						createElement(
							Select.Option,
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);
		const trigger = result.$("button");
		if (!trigger) {
			throw new Error("Expected Select trigger host");
		}

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(complete_values).toEqual([true]);

		result.cleanup();
	});

	it("closes Select with Escape without committing highlighted options", async () => {
		const style_system = create_test_select_style_system();
		const Select = createSelect(style_system);
		const changed_values: (string | null)[] = [];
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Select.Root,
				{
					defaultOpen: true,
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
						createElement(
							Select.Option,
							{ value: "light" },
							"Light",
						),
					),
				),
			),
		);

		const list = result.$("[role='listbox']");
		if (!list) {
			throw new Error("Expected Select list host");
		}
		await result.act(() => {
			list.dispatchEvent(
				new KeyboardEvent("keydown", { key: "ArrowDown" }),
			);
		});
		await result.act(() => {
			list.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
		});

		expect(changed_values).toEqual([]);
		expect(open_values).toEqual([false]);

		result.cleanup();
	});
});
