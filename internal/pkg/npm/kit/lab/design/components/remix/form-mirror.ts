import { createElement, css, type RemixNode } from "remix/ui";
import { visuallyHiddenStyle } from "./visually-hidden.ts";

export type NativeSelectFormMirrorOption = {
	disabled?: boolean;
	text: string;
	value: string;
};

export type NativeSelectFormMirrorInput = {
	autoComplete?: string;
	disabled?: boolean;
	form?: string;
	name?: string;
	options: readonly NativeSelectFormMirrorOption[];
	placeholder?: RemixNode;
	required?: boolean;
	value: string | null;
};

const native_select_empty_value = "";
const native_select_aria_hidden = "true";
const native_select_tab_index = -1;
const native_select_mix = css<HTMLSelectElement>({
	...visuallyHiddenStyle,
	pointerEvents: "none",
});

function has_form_owner(input: NativeSelectFormMirrorInput): boolean {
	return input.name !== undefined || input.form !== undefined;
}

function has_current_value(input: NativeSelectFormMirrorInput): boolean {
	return (
		input.value === null ||
		input.options.some((option) => {
			return option.value === input.value;
		})
	);
}

export function createNativeSelectFormMirror(
	input: NativeSelectFormMirrorInput,
): RemixNode {
	if (!has_form_owner(input)) {
		return null;
	}

	const current_value = input.value ?? native_select_empty_value;

	return createElement(
		"select",
		{
			"aria-hidden": native_select_aria_hidden,
			autoComplete: input.autoComplete,
			disabled: input.disabled,
			form: input.form,
			mix: native_select_mix,
			name: input.name,
			required: input.required,
			tabIndex: native_select_tab_index,
			value: current_value,
		},
		createElement(
			"option",
			{
				selected: input.value === null,
				value: native_select_empty_value,
			},
			input.placeholder ?? "",
		),
		...input.options.map((option) => {
			return createElement(
				"option",
				{
					disabled: option.disabled,
					selected: option.value === input.value,
					value: option.value,
				},
				option.text,
			);
		}),
		has_current_value(input)
			? null
			: createElement(
					"option",
					{
						selected: true,
						value: current_value,
					},
					current_value,
				),
	);
}
