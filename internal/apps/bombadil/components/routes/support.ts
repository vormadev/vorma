import { ui } from "../../route_factory.ts";

type TextState = {
	value: () => string;
	set: (next: string) => void;
};

export const route_root_pattern = "/";
export const route_counter_pattern = "/counter";
export const route_slow_pattern = "/slow";
export const route_echo_pattern = "/echo";
export const route_item_pattern = "/items/:id";
export const route_client_pattern = "/client/:id";
export const route_nested_pattern = "/nested";
export const route_nested_detail_pattern = "/nested/:id/details";
export const route_fail_pattern = "/fail";
export const action_echo_pattern = "/echo";
export const route_nested_detail_alpha_href = "/nested/alpha/details";
export const route_nested_detail_beta_href = "/nested/beta/details";

export const nav_items = [
	{ href: "/", key: "home", label: "Home" },
	{ href: "/counter?n=0", key: "counter", label: "Counter" },
	{ href: "/slow?delay_ms=120", key: "slow", label: "Slow" },
	{ href: "/echo", key: "echo", label: "Echo" },
	{ href: "/items/alpha", key: "item-alpha", label: "Item Alpha" },
	{ href: "/client/alpha", key: "client-alpha", label: "Client Alpha" },
	{
		href: route_nested_detail_alpha_href,
		key: "nested-alpha",
		label: "Nested Alpha",
	},
	{ href: "/fail", key: "fail", label: "Fail" },
];

export function h(tag: any, props: any, ...children: any[]) {
	return ui.h(tag, props, ...children);
}

export function klass(class_name: string) {
	return { [ui.class_prop]: class_name };
}

export function read_box(box: any) {
	return ui.read_box(box);
}

export function dynamic(read_value: () => any) {
	return ui.dynamic(read_value);
}

export function loader_box(props: any) {
	const data = ui.useLoaderData(props);
	return () => {
		return read_box(data);
	};
}

export function client_loader_box(props: any) {
	const data = ui.useClientLoaderData(props);
	return () => {
		return read_box(data);
	};
}

export function text_state(initial: string): TextState {
	const [value, set_value] = ui.use_text_state(initial);
	return {
		value: () => {
			return ui.read_text_state(value);
		},
		set: set_value,
	};
}

export function counter_href(value: number) {
	return `/counter?n=${value}`;
}

export function counter_previous(value: number) {
	return Math.max(value - 1, -5);
}

export function counter_next(value: number) {
	return Math.min(value + 1, 5);
}
