import { ui } from "../../route_factory.ts";
import {
	client_loader_box,
	dynamic,
	h,
	klass,
	view_client_pattern,
	view_data_box,
} from "./support.ts";

export default ui.defineView({
	pattern: view_client_pattern,
	clientLoader: async (args: any) => {
		await new Promise((resolve) => {
			setTimeout(resolve, 20);
		});
		return {
			ID: args.params.id,
			Trigger: args.trigger,
			ClientStamp: `${args.trigger}:${args.href}`,
		};
	},
	component: (props: any) => {
		const data = view_data_box(props);
		const client_data = client_loader_box(props);
		return h(
			"section",
			{
				...klass("panel"),
				"data-bmb-view": "client",
				"data-bmb-client-id": dynamic(() => {
					return data().ID;
				}),
			},
			h("h2", null, "Client"),
			h(
				"div",
				{ "data-bmb-client-server-id": true },
				dynamic(() => {
					return data().ID;
				}),
			),
			h(
				"div",
				{ "data-bmb-client-server-stamp": true },
				dynamic(() => {
					return data().ServerStamp;
				}),
			),
			h(
				"div",
				{ "data-bmb-client-loader-id": true },
				dynamic(() => {
					return client_data().ID;
				}),
			),
			h(
				"div",
				{ "data-bmb-client-loader-trigger": true },
				dynamic(() => {
					return client_data().Trigger;
				}),
			),
			h(
				"div",
				{ "data-bmb-client-loader-stamp": true },
				dynamic(() => {
					return client_data().ClientStamp;
				}),
			),
			h(
				"div",
				klass("row"),
				h(
					ui.Link,
					{
						href: "/client/alpha",
						"data-bmb-action": "client-alpha",
					},
					"Alpha",
				),
				h(
					ui.Link,
					{ href: "/client/beta", "data-bmb-action": "client-beta" },
					"Beta",
				),
			),
		);
	},
});
