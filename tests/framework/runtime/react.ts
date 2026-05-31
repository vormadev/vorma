import { createElement, useState } from "react";
import { createRoot } from "react-dom/client";

export const variant = "react";
export const h = createElement;
export const class_prop = "className";
export const input_event = "onChange";
export const use_text_state = useState;

export function read_box(box: any): any {
	return box;
}

export function read_text_state(value: any): any {
	return value;
}

export function dynamic(read_value: () => any): any {
	return read_value();
}

export function prepare_view_definition(input: any): any {
	return input;
}

export function render_vorma(input: { RootOutlet: any; rootEl: HTMLElement }): void {
	createRoot(input.rootEl).render(createElement(input.RootOutlet));
}
