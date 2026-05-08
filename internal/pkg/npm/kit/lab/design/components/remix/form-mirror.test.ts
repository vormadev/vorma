// @vitest-environment jsdom

import { createElement } from "remix/ui";
import { render } from "remix/ui/test";
import { describe, expect, it } from "vitest";
import { createNativeSelectFormMirror } from "./form-mirror.ts";
import { setup_remix_component_test_environment } from "./test-setup.ts";

describe("form mirror", () => {
	setup_remix_component_test_environment();

	it("does not render a native select without a form owner", () => {
		const result = render(
			createElement(
				"div",
				{},
				createNativeSelectFormMirror({
					options: [],
					value: null,
				}),
			),
		);

		expect(result.$("select")).toBeNull();

		result.cleanup();
	});

	it("mirrors single-value custom selects into native select controls", () => {
		const result = render(
			createElement(
				"div",
				{},
				createNativeSelectFormMirror({
					autoComplete: "off",
					form: "settings",
					name: "theme",
					options: [
						{
							text: "Light",
							value: "light",
						},
						{
							disabled: true,
							text: "Dark",
							value: "dark",
						},
					],
					placeholder: "Theme",
					required: true,
					value: "light",
				}),
			),
		);

		const select = result.$("select") as HTMLSelectElement | null;
		if (!select) {
			throw new Error("Expected native select mirror");
		}

		expect(select.getAttribute("aria-hidden")).toBe("true");
		expect(select.getAttribute("autocomplete")).toBe("off");
		expect(select.getAttribute("form")).toBe("settings");
		expect(select.name).toBe("theme");
		expect(select.required).toBe(true);
		expect(select.tabIndex).toBe(-1);
		expect(select.value).toBe("light");
		expect(
			Array.from(select.options).map((option) => {
				return {
					disabled: option.disabled,
					text: option.textContent,
					value: option.value,
				};
			}),
		).toEqual([
			{
				disabled: false,
				text: "Theme",
				value: "",
			},
			{
				disabled: false,
				text: "Light",
				value: "light",
			},
			{
				disabled: true,
				text: "Dark",
				value: "dark",
			},
		]);

		result.cleanup();
	});

	it("preserves controlled values that are not registered options", () => {
		const result = render(
			createElement(
				"div",
				{},
				createNativeSelectFormMirror({
					name: "theme",
					options: [
						{
							text: "Light",
							value: "light",
						},
					],
					value: "system",
				}),
			),
		);

		const select = result.$("select") as HTMLSelectElement | null;
		if (!select) {
			throw new Error("Expected native select mirror");
		}

		expect(select.value).toBe("system");
		expect(
			Array.from(select.options).map((option) => {
				return option.value;
			}),
		).toEqual(["", "light", "system"]);

		result.cleanup();
	});
});
