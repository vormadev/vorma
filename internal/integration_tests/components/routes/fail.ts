import { ui } from "../../route_factory.ts";
import { h, klass, route_fail_pattern } from "./support.ts";

export default ui.defineRoute({
	pattern: route_fail_pattern,
	component: () => {
		return h("section", { "data-bmb-route": "fail-unexpected" });
	},
	errorBoundary: (props: any) => {
		return h(
			"section",
			{ ...klass("panel"), "data-bmb-route": "fail" },
			h("h2", null, "Expected failure"),
			h("div", { "data-bmb-error": true }, String(props.error)),
		);
	},
});
