import { ui } from "../../route_factory.ts";
import {
	dynamic,
	h,
	klass,
	loader_box,
	route_nested_detail_alpha_href,
	route_nested_detail_beta_href,
	route_nested_detail_pattern,
} from "./support.ts";

export default ui.defineView({
	pattern: route_nested_detail_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		return h(
			"section",
			{
				...klass("panel"),
				"data-bmb-route": "nested-detail",
				"data-bmb-nested-detail-id": dynamic(() => {
					return data().ID;
				}),
			},
			h(
				"h2",
				null,
				dynamic(() => {
					return `Nested ${data().ID}`;
				}),
			),
			h(
				"div",
				{ "data-bmb-nested-detail-section": true },
				dynamic(() => {
					return data().Section;
				}),
			),
			h(
				"div",
				klass("row"),
				h(
					ui.Link,
					{
						href: route_nested_detail_alpha_href,
						"data-bmb-action": "nested-detail-alpha",
					},
					"Alpha",
				),
				h(
					ui.Link,
					{
						href: route_nested_detail_beta_href,
						"data-bmb-action": "nested-detail-beta",
					},
					"Beta",
				),
			),
		);
	},
});
