import { ui } from "../../route_factory.ts";
import {
	dynamic,
	h,
	klass,
	nav_items,
	read_box,
	route_root_pattern,
} from "./support.ts";

export default ui.defineRoute({
	pattern: route_root_pattern,
	component: (props: any) => {
		const route_state = ui.useRouteState();
		const route = () => {
			return read_box(route_state);
		};
		const pathname = () => {
			return new URL(route().href).pathname;
		};
		const work_state = ui.useWorkState();
		const work = () => {
			return read_box(work_state);
		};
		return h(
			"main",
			{
				...klass("shell"),
				"data-bmb-shell": ui.variant,
				"data-bmb-href": dynamic(() => {
					return route().href;
				}),
				"data-bmb-pending-href": dynamic(() => {
					return work().navigation?.href ?? "";
				}),
			},
			h(
				"nav",
				{ ...klass("nav"), "data-bmb-nav": true },
				nav_items.map((item) => {
					return h(
						ui.Link,
						{ href: item.href, "data-bmb-link": item.key },
						item.label,
					);
				}),
			),
			h(
				"section",
				{ ...klass("panel"), "data-bmb-route-summary": true },
				h(
					"div",
					{ "data-bmb-current-href": true },
					dynamic(() => {
						return route().href;
					}),
				),
				h(
					"div",
					{ "data-bmb-work-submissions": true },
					dynamic(() => {
						return work().submissions.length;
					}),
				),
				h(
					"div",
					klass("row"),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "prefetch-slow",
							type: "button",
							onClick: () => {
								ui.prefetch({ href: "/slow?delay_ms=220" });
							},
						},
						"Prefetch slow",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "prefetch-client-alpha",
							type: "button",
							onClick: () => {
								ui.prefetch({ href: "/client/alpha" });
							},
						},
						"Prefetch client",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "cancel-prefetch-slow",
							type: "button",
							onClick: () => {
								ui.cancelPrefetch({
									href: "/slow?delay_ms=220",
								});
							},
						},
						"Cancel prefetch",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "cancel-prefetch-client-alpha",
							type: "button",
							onClick: () => {
								ui.cancelPrefetch({
									href: "/client/alpha",
								});
							},
						},
						"Cancel client",
					),
					h(
						"button",
						{
							...klass("button"),
							"data-bmb-action": "prefetch-fail",
							type: "button",
							onClick: () => {
								ui.prefetch({ href: "/fail" });
							},
						},
						"Prefetch fail",
					),
				),
			),
			dynamic(() => {
				if (pathname() !== route_root_pattern) {
					return null;
				}
				return h(
					"section",
					{ ...klass("panel"), "data-bmb-route": "home" },
					h("h1", null, "Vorma Bombadil Fixture"),
					h("p", null, `${ui.variant} variant`),
				);
			}),
			props.Outlet(),
		);
	},
});
