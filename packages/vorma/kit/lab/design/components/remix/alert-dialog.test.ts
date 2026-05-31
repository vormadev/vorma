// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createAlertDialog, type AlertDialogStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix AlertDialog", () => {
	setup_remix_component_test_environment();

	it("opens from trigger and closes from cancel", async () => {
		const alert_dialog_recipe = {
			defaultVariants: {
				layout: "default",
			},
			slots: {
				action: {},
				cancel: {},
				description: {},
				overlay: {},
				popup: {},
				title: {},
				trigger: {},
			},
			variants: {
				layout: {
					default: {
						popup: {},
					},
				},
			},
		} as const;
		const AlertDialog = createAlertDialog({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					alertDialog: alert_dialog_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies AlertDialogStyleSystem<"light", typeof alert_dialog_recipe>);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				AlertDialog.Root,
				{
					onOpenChange: (open: boolean) => {
						open_values.push(open);
					},
				},
				createElement(AlertDialog.Trigger, {}, "Delete"),
				createElement(
					AlertDialog.Popup,
					{},
					createElement(AlertDialog.Title, {}, "Delete item?"),
					createElement(AlertDialog.Description, {}, "This cannot be undone."),
					createElement(AlertDialog.Cancel, {}, "Cancel"),
					createElement(AlertDialog.Action, {}, "Delete"),
				),
			),
		);
		const trigger = result.$("button");
		const popup = result.$("dialog");
		if (!trigger || !(popup instanceof HTMLDialogElement)) {
			throw new Error("Expected AlertDialog trigger and popup hosts");
		}
		expect(popup.open).toBe(false);
		expect(popup.getAttribute("role")).toBe("alertdialog");

		await result.act(() => {
			trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true]);
		expect(popup.open).toBe(true);
		expect(popup.getAttribute("aria-labelledby")).toBe(
			result.$("h2")?.getAttribute("id"),
		);
		expect(popup.getAttribute("aria-describedby")).toBe(
			result.$("p")?.getAttribute("id"),
		);

		const cancel = result.$("[data-vorma-part='cancel']");
		await result.act(() => {
			cancel?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([true, false]);
		expect(popup.open).toBe(false);

		result.cleanup();
	});
});
