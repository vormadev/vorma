import { createElement, useState } from "react";
import { createRoot } from "react-dom/client";

export const variant = "react";
export const h = createElement;
export const class_prop = "className";
export const input_event = "onChange";
export const use_text_state = useState;

export function read_box(box: any) {
	return box;
}

export function read_text_state(value: any) {
	return value;
}

export function dynamic(read_value: () => any) {
	return read_value();
}

export function render_vorma(input: { App: any; el: HTMLElement }) {
	createRoot(input.el).render(createElement(input.App));
}
