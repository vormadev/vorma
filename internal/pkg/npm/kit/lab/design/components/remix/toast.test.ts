// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { componentStateAttribute } from "./component-state.ts";
import { createToast, type ToastStyleSystem } from "./remix.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("Remix Toast", () => {
	setup_remix_component_test_environment();

	it("renders toast anatomy and closes from the close control", async () => {
		const toast_recipe = {
			defaultVariants: {
				tone: "info",
				variant: "default",
			},
			slots: {
				close: {},
				description: {},
				root: {},
				title: {},
				viewport: {},
			},
			variants: {
				tone: {
					info: {
						root: {},
					},
				},
				variant: {
					default: {
						root: {},
					},
				},
			},
		} as const;
		const Toast = createToast({
			metadata: {},
			modes: {
				light: {
					variables: {},
				},
			},
			token: {
				recipe: {
					toast: toast_recipe,
				},
			},
			variablePrefix: "test",
		} satisfies ToastStyleSystem<"light", typeof toast_recipe>);
		const open_values: boolean[] = [];
		const result = render(
			createElement(
				Toast.Viewport,
				{},
				createElement(
					Toast.Root,
					{
						onOpenChange: (open: boolean) => {
							open_values.push(open);
						},
					},
					createElement(Toast.Title, {}, "Saved"),
					createElement(Toast.Description, {}, "Changes saved."),
					createElement(Toast.Close, {}, "Dismiss"),
				),
			),
		);
		const toast = result.$("[role='status']") as HTMLElement | null;
		const close = result.$("button");
		if (!toast || !close) {
			throw new Error("Expected Toast root and close hosts");
		}

		expect(toast.hidden).toBe(false);
		expect(toast.getAttribute(componentStateAttribute)).toBe("open");

		await result.act(() => {
			close.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});

		expect(open_values).toEqual([false]);
		expect(toast.hidden).toBe(true);
		expect(toast.getAttribute(componentStateAttribute)).toBe("closed");

		result.cleanup();
	});
});
