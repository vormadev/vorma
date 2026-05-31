import { createSignal } from "solid-js";
import h from "solid-js/h";
import { render } from "solid-js/web";

export const variant = "solid";
export { h };
export const class_prop = "class";
export const input_event = "onInput";
export const use_text_state = createSignal;

export function read_box(box: any): any {
	return box();
}

export function read_text_state(value: any): any {
	return value();
}

export function dynamic(read_value: () => any): any {
	return read_value;
}

export function prepare_view_definition(input: any): any {
	return input;
}

export function render_vorma(input: { RootOutlet: any; rootEl: HTMLElement }): void {
	render(() => {
		return input.RootOutlet({});
	}, input.rootEl);
}
