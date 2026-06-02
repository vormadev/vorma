import { ui } from "../../route_factory.ts";
import { dynamic, h, klass, view_data_box, view_item_pattern } from "./support.ts";

export default ui.defineView({
	pattern: view_item_pattern,
	component: (props: any) => {
		const data = view_data_box(props);
		return h(
			"section",
			{
				...klass("panel"),
				"data-bmb-view": "item",
				"data-bmb-item-id": dynamic(() => {
					return data().ID;
				}),
			},
			h(
				"h2",
				null,
				dynamic(() => {
					return `Item ${data().ID}`;
				}),
			),
			h(
				"div",
				klass("row"),
				h(
					ui.Link,
					{ href: "/items/alpha", "data-bmb-action": "item-alpha" },
					"Alpha",
				),
				h(
					ui.Link,
					{ href: "/items/beta", "data-bmb-action": "item-beta" },
					"Beta",
				),
			),
		);
	},
});
