// @vitest-environment jsdom

import { h, render as preact_render } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import {
	define_adapter_tests,
	type AdapterTestHarness,
} from "../../tests/adapter_test_core.ts";
import { createVormaClient } from "./preact.tsx";

define_adapter_tests({
	create_client: (config, options) => {
		return createVormaClient(config as any, options) as any;
	},

	h: h as AdapterTestHarness["h"],
	create_define_view_component: ((render: (props: object) => unknown) => {
		return render;
	}) as AdapterTestHarness["create_define_view_component"],
	dynamic: ((read_value: () => unknown) => {
		return read_value();
	}) as AdapterTestHarness["dynamic"],
	unwrap: ((v: unknown) => {
		if (v && typeof v === "object" && "value" in v) {
			return (v as { value: unknown }).value;
		}
		return v;
	}) as AdapterTestHarness["unwrap"],

	mount: () => {
		const container = document.createElement("div");
		document.body.appendChild(container);
		return {
			container,
			render: (element: unknown) => {
				preact_render(element as any, container);
			},
			cleanup: () => {
				preact_render(null, container);
				container.remove();
			},
		};
	},

	use_ref: useRef,

	create_identity_parent: () => {
		let mount_count = 0;
		let unmount_count = 0;

		const component = (props: any) => {
			const ref = useRef<number | null>(null);
			if (ref.current === null) {
				mount_count++;
				ref.current = mount_count;
			}
			useEffect(() => {
				return () => {
					unmount_count++;
				};
			}, []);
			const [draft, set_draft] = useState("initial");
			return h(
				"section",
				{},
				h("div", { "data-draft": "true" }, draft),
				h("button", {
					"data-set": "true",
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
