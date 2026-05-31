import { createElement, createRoot, on, type Handle, type RemixNode } from "remix/ui";

type TextBox = {
	value: string;
};

type ViewScope = {
	clientLoaderData: (props: any) => any;
	loaderData: (props: any) => any;
	routeState: () => any;
	workState: () => any;
};

const on_click_prop = "onClick";
const on_change_prop = "onChange";
const on_input_prop = "onInput";
const click_event = "click";
const change_event = "change";
const input_event_name = "input";

const event_mappings = [
	{ prop: on_click_prop, event: click_event },
	{ prop: on_change_prop, event: change_event },
	{ prop: on_input_prop, event: input_event_name },
] as const;

const state_slots_by_handle = new WeakMap<Handle<any>, TextBox[]>();

let current_handle: Handle<any> | undefined;
let current_view_scope: ViewScope | undefined;
let current_state_index = 0;

export const variant = "remix";
export const class_prop = "className";
export const input_event = on_input_prop;

export function h(type: any, props: any, ...children: any[]): RemixNode {
	return createElement(type, normalize_props(props), ...children);
}

export function use_text_state(initial: string): [TextBox, (next: string) => void] {
	const handle = require_current_handle();
	let slots = state_slots_by_handle.get(handle);
	if (!slots) {
		slots = [];
		state_slots_by_handle.set(handle, slots);
	}

	const state_index = current_state_index;
	current_state_index += 1;
	if (!slots[state_index]) {
		slots[state_index] = { value: initial };
	}

	const box = slots[state_index]!;
	return [
		box,
		(next: string) => {
			box.value = next;
			void handle.update();
		},
	];
}

export function read_box(box: any): any {
	return box;
}

export function read_text_state(value: TextBox): string {
	return value.value;
}

export function dynamic(read_value: () => any): any {
	return read_value();
}

export function use_loader_data(props: any): any {
	return require_current_view_scope().loaderData(props);
}

export function use_client_loader_data(props: any): any {
	return require_current_view_scope().clientLoaderData(props);
}

export function use_route_state(): any {
	return require_current_view_scope().routeState();
}

export function use_work_state(): any {
	return require_current_view_scope().workState();
}

export function prepare_view_definition(input: any): any {
	return {
		...input,
		component: (handle: Handle<any>, v: ViewScope) => {
			return (props: any) => {
				return render_with_handle(handle, v, () => {
					return input.component(props);
				});
			};
		},
		errorBoundary: input.errorBoundary
			? (handle: Handle<any>, v: ViewScope) => {
					return (props: any) => {
						return render_with_handle(handle, v, () => {
							return input.errorBoundary(props);
						});
					};
				}
			: undefined,
	};
}

export function render_vorma(input: { RootOutlet: any; rootEl: HTMLElement }): void {
	const root = createRoot(input.rootEl);
	root.render(createElement(input.RootOutlet));
	root.flush();
}

function normalize_props(props: any): any {
	if (!props) {
		return props;
	}

	const next_props = { ...props };
	const mixins: unknown[] = [];
	if (next_props.mix !== undefined) {
		mixins.push(next_props.mix);
	}

	for (const mapping of event_mappings) {
		const handler = next_props[mapping.prop];
		delete next_props[mapping.prop];
		if (handler !== undefined) {
			mixins.push(
				on(
					mapping.event as any,
					((event: Event) => {
						return handler(event);
					}) as any,
				),
			);
		}
	}

	if (mixins.length > 0) {
		next_props.mix = mixins;
	}
	return next_props;
}

function require_current_handle(): Handle<any> {
	if (!current_handle) {
		throw new Error("Remix framework test state must render inside a view");
	}
	return current_handle;
}

function require_current_view_scope(): ViewScope {
	if (!current_view_scope) {
		throw new Error("Remix framework test data must render inside a view");
	}
	return current_view_scope;
}

function render_with_handle(
	handle: Handle<any>,
	view_scope: ViewScope,
	render: () => RemixNode,
): RemixNode {
	const previous_handle = current_handle;
	const previous_view_scope = current_view_scope;
	const previous_state_index = current_state_index;
	current_handle = handle;
	current_view_scope = view_scope;
	current_state_index = 0;
	try {
		return render();
	} finally {
		current_handle = previous_handle;
		current_view_scope = previous_view_scope;
		current_state_index = previous_state_index;
	}
}
