import { createMixin, type ElementProps } from "remix/ui";
import { componentStateAttribute } from "./component-state.ts";

export const checkableStateAttribute = componentStateAttribute;
export const checkableState = {
	checked: "checked",
	indeterminate: "indeterminate",
	unchecked: "unchecked",
} as const;

export type CheckableStateName =
	(typeof checkableState)[keyof typeof checkableState];

export type CheckableChecked = boolean | typeof checkableState.indeterminate;

export type CheckableStateMixinInput = {
	allowIndeterminate?: boolean;
	checked: CheckableChecked | undefined;
	defaultChecked: CheckableChecked | undefined;
};

function is_checkable_indeterminate(
	value: CheckableChecked | undefined,
	allow_indeterminate: boolean,
): boolean {
	return allow_indeterminate && value === checkableState.indeterminate;
}

export function checkableInitialChecked(
	value: CheckableChecked | undefined,
): boolean {
	return value === true;
}

export function checkableStateFromValue(
	value: CheckableChecked | undefined,
	allow_indeterminate = true,
): CheckableStateName {
	if (is_checkable_indeterminate(value, allow_indeterminate)) {
		return checkableState.indeterminate;
	}
	return value === true ? checkableState.checked : checkableState.unchecked;
}

function checkable_dom_state(
	node: HTMLInputElement,
	allow_indeterminate: boolean,
): CheckableStateName {
	if (allow_indeterminate && node.indeterminate) {
		return checkableState.indeterminate;
	}
	return node.checked ? checkableState.checked : checkableState.unchecked;
}

function sync_checkable_state_attribute(
	node: HTMLInputElement,
	allow_indeterminate: boolean,
): void {
	node.setAttribute(
		checkableStateAttribute,
		checkable_dom_state(node, allow_indeterminate),
	);
}

function sync_checkable_state(
	node: HTMLInputElement,
	state: CheckableStateMixinInput,
	initial: boolean,
): void {
	const allow_indeterminate = state.allowIndeterminate !== false;
	if (state.checked !== undefined) {
		node.checked = state.checked === true;
		node.indeterminate = is_checkable_indeterminate(
			state.checked,
			allow_indeterminate,
		);
		sync_checkable_state_attribute(node, allow_indeterminate);
		return;
	}

	if (initial && state.defaultChecked !== undefined) {
		node.checked = state.defaultChecked === true;
		node.indeterminate = is_checkable_indeterminate(
			state.defaultChecked,
			allow_indeterminate,
		);
	}
	sync_checkable_state_attribute(node, allow_indeterminate);
}

export const checkableStateMixin = createMixin<
	HTMLInputElement,
	[state: CheckableStateMixinInput],
	ElementProps
>((handle) => {
	let current_node: HTMLInputElement | undefined;
	let current_state: CheckableStateMixinInput = {
		checked: undefined,
		defaultChecked: undefined,
	};

	function handle_change(event: Event): void {
		sync_checkable_state_attribute(
			event.currentTarget as HTMLInputElement,
			current_state.allowIndeterminate !== false,
		);
	}

	handle.addEventListener("insert", (event) => {
		current_node = event.node;
		current_node.addEventListener("change", handle_change);
		sync_checkable_state(current_node, current_state, true);
	});
	handle.addEventListener("commit", (event) => {
		sync_checkable_state(event.node, current_state, false);
	});
	handle.addEventListener("remove", () => {
		current_node?.removeEventListener("change", handle_change);
		current_node = undefined;
	});

	return (state) => {
		current_state = state;
		if (current_node) {
			sync_checkable_state(current_node, current_state, false);
		}
		return handle.element;
	};
});
