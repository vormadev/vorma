export const componentStateAttribute = "data-state";

export const componentDataAttribute = {
	disabled: "data-disabled",
	highlighted: "data-highlighted",
	invalid: "data-invalid",
	loading: "data-loading",
	open: "data-open",
	placeholder: "data-placeholder",
	readOnly: "data-readonly",
	required: "data-required",
	selected: "data-selected",
} as const;

export type ARIABoolean = "false" | "true";

export function ariaBoolean(value: boolean): ARIABoolean {
	return value ? "true" : "false";
}

export function ariaTrue(value: boolean): "true" | undefined {
	return value ? "true" : undefined;
}

export function dataFlag(value: boolean): "" | undefined {
	return value ? "" : undefined;
}

export function is_aria_invalid(value: unknown): boolean {
	return (
		value === true ||
		value === "true" ||
		value === "grammar" ||
		value === "spelling"
	);
}

export const openState = {
	closed: "closed",
	open: "open",
} as const;

export type OpenStateName = (typeof openState)[keyof typeof openState];

export const selectionState = {
	selected: "selected",
	unselected: "unselected",
} as const;

export type SelectionStateName =
	(typeof selectionState)[keyof typeof selectionState];

export function openStateFromBoolean(open: boolean): OpenStateName {
	return open ? openState.open : openState.closed;
}

export function selectionStateFromBoolean(
	selected: boolean,
): SelectionStateName {
	return selected ? selectionState.selected : selectionState.unselected;
}
