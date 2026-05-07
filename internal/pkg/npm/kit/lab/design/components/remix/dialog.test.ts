// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import {
	componentAnatomyAttrs,
	createDialog,
	type DialogOpenChangeDetails,
	type DialogOpenChangeReason,
	type DialogStyleSystem,
} from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

const test_dialog_recipe = {
	slots: {
		close: {},
		description: {},
		overlay: {
			base: {
				background: "rgb(0 0 0 / 0.4)",
			},
			conditions: {
				open: {
					opacity: 1,
				},
			},
		},
		popup: {
			base: {
				background: "white",
			},
		},
		title: {},
		trigger: {},
	},
} as const;

function create_test_dialog_style_system(): DialogStyleSystem<
	"light",
	typeof test_dialog_recipe
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
				dialog: test_dialog_recipe,
			},
		},
		variablePrefix: "test",
	};
}

describe("Remix Dialog", () => {
	setup_remix_component_test_environment();

	it("does not require dialog recipes to define an overlay slot", () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				popup: {
					base: {
						background: "white",
					},
				},
				title: {},
				trigger: {},
			},
		} as const;
		const style_system = {
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		expect(result.$("dialog")?.textContent).toBe("Panel");

		result.cleanup();
	});

	it("opens and closes Dialog with trigger and close button", async () => {
		const Dialog = createDialog(create_test_dialog_style_system());
		const open_values: boolean[] = [];
		const open_reasons: (DialogOpenChangeReason | undefined)[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					onOpenChange: (
						open: boolean,
						details: DialogOpenChangeDetails | undefined,
					) => {
						open_values.push(open);
						open_reasons.push(details?.reason);
					},
				},
				createElement(Dialog.Trigger, {}, "Open"),
				createElement(
					Dialog.Popup,
					{},
					createElement(Dialog.Close, {}, "Close"),
				),
			),
		);
		const trigger = result.$("button");
		const popup = result.$("dialog");
		if (!trigger || !(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog trigger and popup hosts");
		}

		expect(popup.open).toBe(false);

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true]);
		expect(popup.open).toBe(true);

		const close = result.$$("button")[1];
		if (!close) {
			throw new Error("Expected Dialog close host");
		}

		await result.act(() => {
			close.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true, false]);
		expect(open_reasons).toEqual(["trigger", "close"]);
		expect(popup.open).toBe(false);

		result.cleanup();
	});

	it("requests controlled Dialog open changes without mutating open state", async () => {
		const Dialog = createDialog(create_test_dialog_style_system());
		const open_values: boolean[] = [];
		const open_reasons: (DialogOpenChangeReason | undefined)[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					onOpenChange: (
						open: boolean,
						details: DialogOpenChangeDetails | undefined,
					) => {
						open_values.push(open);
						open_reasons.push(details?.reason);
					},
					open: false,
				},
				createElement(Dialog.Trigger, {}, "Open"),
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);
		const trigger = result.$("button");
		const popup = result.$("dialog");
		if (!trigger || !(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog trigger and popup hosts");
		}

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true]);
		expect(open_reasons).toEqual(["trigger"]);
		expect(popup.open).toBe(false);

		result.cleanup();
	});

	it("closes Dialog on native cancel", () => {
		const Dialog = createDialog(create_test_dialog_style_system());
		const open_values: boolean[] = [];
		const open_reasons: (DialogOpenChangeReason | undefined)[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
					onOpenChange: (
						open: boolean,
						details: DialogOpenChangeDetails | undefined,
					) => {
						open_values.push(open);
						open_reasons.push(details?.reason);
					},
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);
		const popup = result.$("dialog");
		if (!(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog popup host");
		}

		popup.dispatchEvent(new Event("cancel"));

		expect(open_values).toEqual([false]);
		expect(open_reasons).toEqual(["escape"]);

		result.cleanup();
	});

	it("uses non-modal native dialog show when modal is false", () => {
		const Dialog = createDialog(create_test_dialog_style_system());
		const calls: string[] = [];
		Object.defineProperty(HTMLDialogElement.prototype, "show", {
			configurable: true,
			value: function show(this: HTMLDialogElement): void {
				calls.push("show");
				this.open = true;
			},
		});
		Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
			configurable: true,
			value: function showModal(this: HTMLDialogElement): void {
				calls.push("showModal");
				this.open = true;
			},
		});
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
					modal: false,
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		expect(calls).toEqual(["show"]);

		result.cleanup();
	});

	it("creates a dialog overlay host and closes from overlay click", () => {
		const Dialog = createDialog(create_test_dialog_style_system());
		const open_values: boolean[] = [];
		const open_reasons: (DialogOpenChangeReason | undefined)[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
					onOpenChange: (
						open: boolean,
						details: DialogOpenChangeDetails | undefined,
					) => {
						open_values.push(open);
						open_reasons.push(details?.reason);
					},
				},
				createElement(Dialog.Overlay, {}, "Overlay"),
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		const overlay = result.$("div");
		const popup = result.$("dialog");
		expect(overlay?.getAttribute(componentAnatomyAttrs.part)).toBe(
			"overlay",
		);
		expect(overlay?.getAttribute("aria-hidden")).toBe("true");
		expect(overlay?.hidden).toBe(false);
		if (!(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog popup host");
		}

		expect(popup.open).toBe(true);
		overlay?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

		expect(open_values).toEqual([false]);
		expect(open_reasons).toEqual(["interactOutside"]);

		result.cleanup();
	});

	it("creates a dialog backdrop style target when no overlay host is rendered", () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				overlay: {
					base: {
						background: "rgb(0 0 0 / 0.4)",
					},
				},
				popup: {
					base: {
						background: "white",
					},
				},
				title: {},
				trigger: {},
			},
		} as const;
		const style_system = {
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const result = render(
			createElement(
				Dialog.Root,
				{
					defaultOpen: true,
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		expect(
			result.$("dialog")?.getAttribute(componentAnatomyAttrs.part),
		).toBe("popup");
		expect(result.$("div")).toBe(null);

		result.cleanup();
	});

	it("respects Dialog closeOnInteractOutside", () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				popup: {},
				title: {},
				trigger: {},
			},
		} as const;
		const style_system = {
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					closeOnInteractOutside: false,
					defaultOpen: true,
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);

		const popup = result.$("dialog");
		if (!(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected Dialog popup host");
		}
		Object.defineProperty(popup, "getBoundingClientRect", {
			configurable: true,
			value: () => {
				return {
					bottom: 100,
					height: 90,
					left: 10,
					right: 100,
					top: 10,
					width: 90,
					x: 10,
					y: 10,
					toJSON: () => {
						return {};
					},
				};
			},
		});

		popup.dispatchEvent(
			new MouseEvent("click", {
				bubbles: true,
				clientX: 0,
				clientY: 0,
			}),
		);

		expect(open_values).toEqual([]);
		expect(popup.open).toBe(true);

		result.cleanup();
	});

	it("emits Dialog open lifecycle completion after updates", async () => {
		const dialog_recipe = {
			slots: {
				close: {},
				description: {},
				popup: {},
				title: {},
				trigger: {},
			},
		} as const;
		const style_system = {
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					dialog: dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies DialogStyleSystem<"light", typeof dialog_recipe>;
		const Dialog = createDialog(style_system);
		const complete_values: boolean[] = [];
		const complete_reasons: (DialogOpenChangeReason | undefined)[] = [];
		const result = render(
			createElement(
				Dialog.Root,
				{
					onOpenChangeComplete: (
						open: boolean,
						details: DialogOpenChangeDetails | undefined,
					) => {
						complete_values.push(open);
						complete_reasons.push(details?.reason);
					},
				},
				createElement(Dialog.Trigger, {}, "Open"),
				createElement(Dialog.Popup, {}, "Panel"),
			),
		);
		const trigger = result.$("button");
		if (!trigger) {
			throw new Error("Expected Dialog trigger host");
		}

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(complete_values).toEqual([true]);
		expect(complete_reasons).toEqual(["trigger"]);

		result.cleanup();
	});
});
