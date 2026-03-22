import { render } from "preact";
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/preact";
import { vormaAppConfig } from "../gen/vorma/index.ts";
import "./styles/tailwind.css";

await initClient({
	vormaAppConfig,
	renderFn: () => {
		render(<VormaRootOutlet />, getRootEl());
	},
});
