// @vitest-environment jsdom

import { createElement, createRoot, on, type RemixNode } from "remix/ui";
import {
	define_adapter_tests,
	type AdapterTestHarness,
} from "../../tests/adapter_test_core.ts";
import {
	createVormaClient,
	type RemixComponent,
	type RemixViewComponent,
} from "./remix.tsx";

let stable_ref: { current: unknown } | undefined;
const component_wrapper_map = new WeakMap<
	(props: Record<string, unknown>) => RemixNode,
	RemixComponent<Record<string, unknown>>
>();

function create_test_element(
	type: unknown,
	props?: Record<string, unknown> | null,
	...children: unknown[]
): RemixNode {
	const next_props = { ...props };
	const child_nodes =
		children.length > 0
			? children
			: props?.children !== undefined
				? [props.children]
				: [];
	delete next_props.children;

	if (typeof type !== "function") {
		return createElement(type as string, next_props, ...child_nodes);
	}

	const component = type as (props: Record<string, unknown>) => RemixNode;
	let wrapper = component_wrapper_map.get(component);
	if (!wrapper) {
		wrapper = () => {
			return (render_props) => {
				return component(render_props);
			};
		};
		component_wrapper_map.set(component, wrapper);
	}
	return createElement(wrapper, next_props, ...child_nodes);
}

define_adapter_tests({
	create_client: (config, options) => {
		return createVormaClient(config as any, options) as any;
	},

	h: create_test_element as AdapterTestHarness["h"],
	create_define_view_component: ((render: (props: object) => unknown) => {
		const component: RemixViewComponent<any, any> = () => {
			return (props: object) => {
				return render(props) as RemixNode;
			};
		};
		return component;
	}) as AdapterTestHarness["create_define_view_component"],
	create_view: (({ client, pattern, render, clientLoader }) => {
		const remix_client = client as unknown as ReturnType<
			typeof createVormaClient<any>
		>;
		return remix_client.defineView({
			pattern: pattern as any,
			component: (_handle, v) => {
				return (props) => {
					return render({
						props: props as any,
						v: v as any,
					}) as RemixNode;
				};
			},
			clientLoader: clientLoader as any,
		}) as any;
	}) as AdapterTestHarness["create_view"],
	dynamic: ((read_value: () => unknown) => {
		return read_value();
	}) as AdapterTestHarness["dynamic"],
	unwrap: ((v: unknown) => {
		return v;
	}) as AdapterTestHarness["unwrap"],

	mount: () => {
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		return {
			container,
			render: (element: unknown) => {
				root.render(element as RemixNode);
				root.flush();
			},
			cleanup: () => {
				root.dispose();
				stable_ref = undefined;
				container.remove();
			},
		};
	},

	use_ref: <T>(initial: T) => {
		if (!stable_ref) {
			stable_ref = { current: initial };
		}
		return stable_ref as { current: T };
	},

	create_identity_parent: () => {
		let mount_count = 0;
		let unmount_count = 0;

		const identity_parent: RemixComponent<any> = (handle) => {
			mount_count++;
			let draft = "initial";
			handle.signal.addEventListener(
				"abort",
				() => {
					unmount_count++;
				},
				{ once: true },
			);
			return (props) => {
				return createElement(
					"section",
					{},
					createElement("div", { "data-draft": "true" }, draft),
					createElement("button", {
						"data-set": "true",
						mix: on<HTMLButtonElement, "click">("click", () => {
							draft = "modified";
							void handle.update();
						}),
					}),
					create_test_element(props.Outlet, {}),
				);
			};
		};

		const component = (props: any) => {
			return createElement(identity_parent, props);
		};

		return {
			component,
			get_mount_count: () => {
				return mount_count;
			},
			get_unmount_count: () => {
				return unmount_count;
			},
		};
	},
});
