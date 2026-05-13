import { createMixin, type ElementProps } from "remix/ui";

export const formResetEvent = "reset";

export type FormResetMixinInput = {
	form?: string;
	onReset: (event: Event) => void;
};

function is_form_element(element: Element | null): element is HTMLFormElement {
	if (!element) {
		return false;
	}
	const view = element.ownerDocument.defaultView;
	return view !== null && element instanceof view.HTMLFormElement;
}

function find_form_owner(
	node: HTMLElement,
	form: string | undefined,
): HTMLFormElement | null {
	if (form !== undefined) {
		const element = node.ownerDocument.getElementById(form);
		if (is_form_element(element)) {
			return element;
		}
		return null;
	}

	const closest_form = node.closest("form");
	if (is_form_element(closest_form)) {
		return closest_form;
	}
	return null;
}

export const formResetMixin = createMixin<
	HTMLElement,
	[input: FormResetMixinInput],
	ElementProps
>((handle) => {
	let current_form: HTMLFormElement | null = null;
	let current_input: FormResetMixinInput = {
		onReset: () => {},
	};
	let current_node: HTMLElement | null = null;

	function handle_reset(event: Event): void {
		current_input.onReset(event);
	}

	function set_form(form: HTMLFormElement | null): void {
		if (current_form === form) {
			return;
		}
		current_form?.removeEventListener(formResetEvent, handle_reset);
		current_form = form;
		current_form?.addEventListener(formResetEvent, handle_reset);
	}

	function sync_form_owner(node: HTMLElement): void {
		set_form(find_form_owner(node, current_input.form));
	}

	handle.addEventListener("insert", (event) => {
		current_node = event.node;
		sync_form_owner(event.node);
	});
	handle.addEventListener("commit", (event) => {
		current_node = event.node;
		sync_form_owner(event.node);
	});
	handle.addEventListener("remove", () => {
		current_node = null;
		set_form(null);
	});

	return (input) => {
		current_input = input;
		if (current_node) {
			sync_form_owner(current_node);
		}
		return handle.element;
	};
});
