// @vitest-environment jsdom

import { createSignal, onCleanup } from "solid-js";
import h from "solid-js/h";
import { render } from "solid-js/web";
import {
	define_adapter_tests,
	type AdapterTestHarness,
} from "../../tests/adapter_test_core.ts";
import { createVormaClient } from "./solid.tsx";

define_adapter_tests({
	create_client: (config, options) => {
		return createVormaClient(config as any, options) as any;
	},

	h: h as AdapterTestHarness["h"],
	create_define_view_component: ((render: (props: object) => unknown) => {
		return render;
	}) as AdapterTestHarness["create_define_view_component"],
	dynamic: ((read_value: () => unknown) => {
		return read_value;
	}) as AdapterTestHarness["dynamic"],
	unwrap: ((v: unknown) => {
		return typeof v === "function" ? v() : v;
	}) as AdapterTestHarness["unwrap"],

	mount: () => {
		const container = document.createElement("div");
		document.body.appendChild(container);
		let dispose: (() => void) | undefined;
		return {
			container,
			render: (element: unknown) => {
				dispose?.();
				dispose = render(() => {
					return element as any;
				}, container);
			},
			cleanup: () => {
				dispose?.();
				container.remove();
			},
		};
	},

	use_ref: <T>(initial: T) => {
		return { current: initial };
	},

	create_identity_parent: () => {
		let mount_count = 0;
		let unmount_count = 0;

		const component = (props: any) => {
			mount_count++;
			onCleanup(() => {
				unmount_count++;
			});
			const [draft, set_draft] = createSignal("initial");
			return h(
				"section",
				{},
				h("div", { "data-draft": true }, () => {
					return draft();
				}),
				h("button", {
					"data-set": true,
					onClick: () => {
						return set_draft("modified");
					},
				}),
				h(props.Outlet, {}),
			);
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
