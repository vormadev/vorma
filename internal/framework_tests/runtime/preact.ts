import { h, render } from "preact";
import { useState } from "preact/hooks";

export const variant = "preact";
export { h };
export const class_prop = "className";
export const input_event = "onInput";
export const use_text_state = useState;

export function read_box(box: any) {
	return box.value;
}

export function read_text_state(value: any) {
	return value;
}

export function dynamic(read_value: () => any) {
	return read_value();
}

export function render_vorma(input: { App: any; el: HTMLElement }) {
	render(h(input.App, {}), input.el);
}
