export const componentStateAttribute = "data-state";

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
