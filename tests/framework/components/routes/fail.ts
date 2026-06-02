import { ui } from "../../route_factory.ts";
import { h, klass, view_fail_pattern } from "./support.ts";

export default ui.defineView({
	pattern: view_fail_pattern,
	component: () => {
		return h("section", { "data-bmb-view": "fail-unexpected" });
	},
	errorBoundary: (props: any) => {
		return h(
			"section",
			{ ...klass("panel"), "data-bmb-view": "fail" },
			h("h2", null, "Expected failure"),
			h("div", { "data-bmb-error": true }, String(props.error)),
		);
	},
});
