import { ui } from "../../route_factory.ts";
import {
	dynamic,
	h,
	klass,
	loader_box,
	route_item_pattern,
} from "./support.ts";

export default ui.defineView({
	pattern: route_item_pattern,
	component: (props: any) => {
		const data = loader_box(props);
		return h(
			"section",
			{
				...klass("panel"),
				"data-bmb-route": "item",
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
