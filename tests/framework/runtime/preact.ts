import { h, render } from "preact";
import { useState } from "preact/hooks";

export const variant = "preact";
export { h };
export const class_prop = "className";
export const input_event = "onInput";
export const use_text_state = useState;

export function read_box(box: any): any {
	return box.value;
}

export function read_text_state(value: any): any {
	return value;
}

export function dynamic(read_value: () => any): any {
	return read_value();
}

export const use_view_data = undefined;
export const use_client_loader_data = undefined;
export const use_route_state = undefined;
export const use_work_state = undefined;

export function prepare_view_definition(input: any): any {
	return input;
}

export function render_vorma(input: { RootOutlet: any; rootEl: HTMLElement }): void {
	render(h(input.RootOutlet, {}), input.rootEl);
}
