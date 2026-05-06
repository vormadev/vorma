// @vitest-environment jsdom

import React from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";
import {
	define_adapter_tests,
	type AdapterTestHarness,
} from "../../tests/adapter_test_core.ts";
import { createVormaClient } from "./react.tsx";

define_adapter_tests({
	create_client: (config, options) => {
		return createVormaClient(config as any, options) as any;
	},

	h: React.createElement as AdapterTestHarness["h"],
	create_define_view_component: ((render: (props: object) => unknown) => {
		return render;
	}) as AdapterTestHarness["create_define_view_component"],
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
				flushSync(() => {
					return root.render(element as React.ReactNode);
				});
			},
			cleanup: () => {
				flushSync(() => {
					return root.unmount();
				});
				container.remove();
			},
		};
	},

	use_ref: React.useRef,

	create_identity_parent: () => {
		let mount_count = 0;
		let unmount_count = 0;

		const component = (props: any) => {
			const ref = React.useRef<number | null>(null);
			if (ref.current === null) {
				mount_count++;
				ref.current = mount_count;
			}
			React.useEffect(() => {
				return () => {
					unmount_count++;
				};
			}, []);
			const [draft, set_draft] = React.useState("initial");
			return React.createElement(
				"section",
				{},
				React.createElement("div", { "data-draft": "true" }, draft),
				React.createElement("button", {
					"data-set": "true",
					onClick: () => {
						return set_draft("modified");
					},
				}),
				React.createElement(props.Outlet, {}),
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
