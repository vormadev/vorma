import { createMixin, type ElementProps } from "remix/ui";
import { componentStateAttribute } from "./component-state.ts";
import { formResetEvent } from "./form-reset.ts";

export const checkableStateAttribute = componentStateAttribute;
export const checkableState = {
	checked: "checked",
	indeterminate: "indeterminate",
	unchecked: "unchecked",
} as const;
export const checkableChangeEvent = "change";

export type CheckableStateName =
	(typeof checkableState)[keyof typeof checkableState];

export type CheckableChecked = boolean | typeof checkableState.indeterminate;

export type CheckableCheckedChangeDetails = {
	event?: Event;
};

export type CheckableCheckedChangeHandler = (
	checked: CheckableChecked,
	details?: CheckableCheckedChangeDetails,
) => void;

export type CheckableStateMixinInput = {
	allowIndeterminate?: boolean;
	checked: CheckableChecked | undefined;
	defaultChecked: CheckableChecked | undefined;
	disabled?: boolean;
	onCheckedChange?: CheckableCheckedChangeHandler;
	readOnly?: boolean;
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

function checkable_value_from_state(
	state: CheckableStateName,
): CheckableChecked {
	if (state === checkableState.indeterminate) {
		return checkableState.indeterminate;
	}
	return state === checkableState.checked;
}

function set_checkable_value(
	node: HTMLInputElement,
	value: CheckableChecked,
	allow_indeterminate: boolean,
): void {
	node.checked = value === true;
	node.indeterminate = is_checkable_indeterminate(value, allow_indeterminate);
}

function sync_checkable_state_attribute(
	node: HTMLInputElement,
	allow_indeterminate: boolean,
): CheckableChecked {
	const state = checkable_dom_state(node, allow_indeterminate);
	node.setAttribute(checkableStateAttribute, state);
	return checkable_value_from_state(state);
}

function sync_checkable_state(
	node: HTMLInputElement,
	state: CheckableStateMixinInput,
	initial: boolean,
): CheckableChecked {
	const allow_indeterminate = state.allowIndeterminate !== false;
	if (state.checked !== undefined) {
		set_checkable_value(node, state.checked, allow_indeterminate);
		return sync_checkable_state_attribute(node, allow_indeterminate);
	}

	if (initial && state.defaultChecked !== undefined) {
		set_checkable_value(node, state.defaultChecked, allow_indeterminate);
	}
	return sync_checkable_state_attribute(node, allow_indeterminate);
}

export const checkableStateMixin = createMixin<
	HTMLInputElement,
	[state: CheckableStateMixinInput],
	ElementProps
>((handle) => {
	let current_form: HTMLFormElement | null = null;
	let current_node: HTMLInputElement | undefined;
	let current_state: CheckableStateMixinInput = {
		checked: undefined,
		defaultChecked: undefined,
	};
	let current_value: CheckableChecked = false;

	function handle_change(event: Event): void {
		const node = event.currentTarget as HTMLInputElement;
		const allow_indeterminate = current_state.allowIndeterminate !== false;
		if (
			current_state.disabled === true ||
			current_state.readOnly === true
		) {
			set_checkable_value(node, current_value, allow_indeterminate);
			sync_checkable_state_attribute(node, allow_indeterminate);
			return;
		}

		const next_value = sync_checkable_state_attribute(
			node,
			allow_indeterminate,
		);
		if (Object.is(current_value, next_value)) {
			return;
		}
		current_value = next_value;
		current_state.onCheckedChange?.(next_value, { event });
	}

	function handle_reset(): void {
		const node = current_node;
		if (!node) {
			return;
		}
		queueMicrotask(() => {
			if (node !== current_node) {
				return;
			}
			current_value = sync_checkable_state(node, current_state, true);
		});
	}

	function set_form(form: HTMLFormElement | null): void {
		if (current_form === form) {
			return;
		}
		current_form?.removeEventListener(formResetEvent, handle_reset);
		current_form = form;
		current_form?.addEventListener(formResetEvent, handle_reset);
	}

	function sync_form_owner(node: HTMLInputElement): void {
		set_form(node.form);
	}

	handle.addEventListener("insert", (event) => {
		current_node = event.node;
		current_node.addEventListener(checkableChangeEvent, handle_change);
		sync_form_owner(current_node);
		current_value = sync_checkable_state(current_node, current_state, true);
	});
	handle.addEventListener("commit", (event) => {
		sync_form_owner(event.node);
		current_value = sync_checkable_state(event.node, current_state, false);
	});
	handle.addEventListener("remove", () => {
		current_node?.removeEventListener(checkableChangeEvent, handle_change);
		current_node = undefined;
		set_form(null);
	});

	return (state) => {
		current_state = state;
		if (current_node) {
			sync_checkable_state(current_node, current_state, false);
		}
		return handle.element;
	};
});
