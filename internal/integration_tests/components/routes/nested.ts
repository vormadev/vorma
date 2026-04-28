import { ui } from "../../route_factory.ts";
import {
	dynamic,
	h,
	klass,
	loader_box,
	read_box,
	route_nested_detail_alpha_href,
	route_nested_detail_beta_href,
	route_nested_pattern,
} from "./support.ts";

export default ui.defineRoute({
	pattern: route_nested_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		const route_state = ui.useRouteState();
		const route = () => {
			return read_box(route_state);
		};
		const pathname = () => {
			return new URL(route().href).pathname;
		};
		return h(
			"section",
			{
				...klass("panel"),
				"data-bmb-nested-layout": true,
				"data-bmb-nested-section": dynamic(() => {
					return data().Section;
				}),
			},
			h("h2", null, "Nested"),
			h(
				"div",
				klass("row"),
				h(
					ui.Link,
					{
						href: route_nested_detail_alpha_href,
						"data-bmb-action": "nested-alpha",
					},
					"Nested alpha",
				),
				h(
					ui.Link,
					{
						href: route_nested_detail_beta_href,
						"data-bmb-action": "nested-beta",
					},
					"Nested beta",
				),
			),
			dynamic(() => {
				if (pathname() !== route_nested_pattern) {
					return props.Outlet();
				}
				return h(
					"div",
					{ "data-bmb-route": "nested-index" },
					dynamic(() => {
						return data().Section;
					}),
				);
			}),
		);
	},
});
