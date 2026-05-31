// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createPopover,
	popoverAnatomy,
	type PopoverStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const test_popover_recipe = {
	defaultVariants: {
		size: "md",
	},
	slots: {
		arrow: {},
		close: {},
		description: {},
		popup: {
			base: {
				background: "white",
				padding: "0.75rem",
			},
			conditions: {
				closed: {
					pointerEvents: "none",
				},
				open: {
					opacity: 1,
				},
			},
		},
		title: {},
		trigger: {
			base: {
				display: "inline-flex",
			},
			conditions: {
				focusVisible: {
					outline: "2px solid blue",
				},
			},
		},
	},
	variants: {
		size: {
			md: {
				popup: {
					base: {
						minWidth: "12rem",
					},
				},
			},
		},
	},
} as const;

function create_test_popover_style_system(): PopoverStyleSystem<
	"light",
	typeof test_popover_recipe
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
				popover: test_popover_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Popover", () => {
	setup_remix_component_test_environment();

	it("creates Vorma-owned Remix popover parts", () => {
		const style_system = create_test_popover_style_system();
		const Popover = createPopover(style_system);
		const result = render(
			createElement(
				Popover.Root,
				{},
				createElement(Popover.Trigger, {}, "Open"),
				createElement(
					Popover.Popup,
					{},
					createElement(Popover.Title, {}, "Title"),
					createElement(Popover.Description, {}, "Description"),
					createElement(Popover.Arrow, {}),
					createElement(Popover.Close, {}, "Close"),
				),
			),
		);

		const trigger = result.$("button");
		const popup = result.$("div[popover='manual']");
		expect(trigger?.getAttribute("aria-expanded")).toBe("false");
		expect(trigger?.getAttribute(componentAnatomyAttrs.part)).toBe(
			popoverAnatomy.parts.trigger,
		);
		expect(trigger?.getAttribute(componentAnatomyAttrs.scope)).toBe(
			popoverAnatomy.scope,
		);
		expect(popup?.getAttribute(componentAnatomyAttrs.part)).toBe(
			popoverAnatomy.parts.popup,
		);
		expect(popup?.textContent).toBe("TitleDescriptionClose");

		result.cleanup();
	});

	it("toggles Popover from the trigger", async () => {
		const style_system = create_test_popover_style_system();
		const Popover = createPopover(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Popover.Root,
				{
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Popover.Trigger, {}, "Open"),
				createElement(Popover.Popup, {}, "Panel"),
			),
		);
		const trigger = result.$("button");
		const popup = result.$("div[popover='manual']");
		if (!trigger || !popup) {
			throw new Error("Expected Popover trigger and popup hosts");
		}

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});
		expect(trigger.getAttribute("aria-expanded")).toBe("true");
		expect(popup.hasAttribute("hidden")).toBe(false);

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
		expect(popup.hasAttribute("hidden")).toBe(true);
		expect(open_values).toEqual([true, false]);

		result.cleanup();
	});

	it("closes Popover from outside interaction and Escape", async () => {
		const style_system = create_test_popover_style_system();
		const Popover = createPopover(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Popover.Root,
				{
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Popover.Trigger, {}, "Open"),
				createElement(Popover.Popup, {}, "Panel"),
			),
		);
		const trigger = result.$("button");
		const popup = result.$("div[popover='manual']");
		if (!trigger || !popup) {
			throw new Error("Expected Popover trigger and popup hosts");
		}

		await result.act(() => {
			document.body.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true }));
		});
		expect(open_values).toEqual([false]);

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});
		await result.act(() => {
			popup.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
		});

		expect(open_values).toEqual([false, true, false]);

		result.cleanup();
	});

	it("closes Popover from the close button", async () => {
		const style_system = create_test_popover_style_system();
		const Popover = createPopover(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Popover.Root,
				{
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Popover.Trigger, {}, "Open"),
				createElement(
					Popover.Popup,
					{},
					createElement(Popover.Close, {}, "Close"),
				),
			),
		);
		const close = result.$("[data-vorma-part='close']");
		if (!close) {
			throw new Error("Expected Popover close host");
		}

		await result.act(() => {
			close.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([false]);

		result.cleanup();
	});

	it("emits Popover open lifecycle completion after updates", async () => {
		const style_system = create_test_popover_style_system();
		const Popover = createPopover(style_system);
		const complete_values: boolean[] = [];
		const result = render(
			createElement(
				Popover.Root,
				{
					onOpenChangeComplete: (open: boolean) => {
						complete_values.push(open);
					},
				},
				createElement(Popover.Trigger, {}, "Open"),
				createElement(Popover.Popup, {}, "Panel"),
			),
		);
		const trigger = result.$("button");
		if (!trigger) {
			throw new Error("Expected Popover trigger host");
		}

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(complete_values).toEqual([true]);

		result.cleanup();
	});
});
