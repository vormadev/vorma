import { ui } from "../../route_factory.ts";
import {
	dynamic,
	h,
	klass,
	view_data_box,
	view_nested_detail_alpha_href,
	view_nested_detail_beta_href,
	view_nested_detail_pattern,
} from "./support.ts";

export default ui.defineView({
	pattern: view_nested_detail_pattern,
	component: (props: any) => {
		const data = view_data_box(props);
		return h(
			"section",
			{
				...klass("panel"),
				"data-bmb-view": "nested-detail",
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
						href: view_nested_detail_alpha_href,
						"data-bmb-action": "nested-detail-alpha",
					},
					"Alpha",
				),
				h(
					ui.Link,
					{
						href: view_nested_detail_beta_href,
						"data-bmb-action": "nested-detail-beta",
					},
					"Beta",
				),
			),
		);
	},
});
