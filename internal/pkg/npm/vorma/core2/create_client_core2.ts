import { R, type Result } from "vorma/kit/result";
import type { AppConfig } from "../core/types.ts";
import { get_or_create_root_el } from "./browser_dom.ts";
import {
	create_browser_host,
	type BrowserHostOptions,
} from "./browser_host.ts";
import { create_client_callbacks } from "./client_callbacks.ts";
import type { ClientCommit, ClientCore } from "./client_contract.ts";
import { create_client_shell } from "./client_shell.ts";
import { create_core_controller } from "./controller.ts";
import { create_id_source } from "./ids.ts";
import { create_public_call_store } from "./public_calls.ts";

const browser_global_symbol = {
	data_revalidate_fn: "vorma-data-revalidate-fn",
} as const;

type DataRevalidationWindow = Window & {
	[key: symbol]: unknown;
};

export type Core2TestOptions = Pick<
	BrowserHostOptions,
	| "dev"
	| "fetch_fn"
	| "hard_redirect"
	| "load_module"
	| "now_ms"
	| "scroll_to"
>;

export function create_client_core2(
	app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: (commit: ClientCommit) => void,
	test_options?: Core2TestOptions,
): Result<ClientCore> {
	void app_config;
	const callbacks = create_client_callbacks(commit);
	const host = create_browser_host({
		commit: callbacks.commit,
		dev: test_options?.dev,
		fetch_fn: test_options?.fetch_fn,
		hard_redirect: test_options?.hard_redirect,
		load_module: test_options?.load_module,
		now_ms: test_options?.now_ms,
		render: callbacks.render,
		report_build_skew: callbacks.report_build_skew,
		scroll_to: test_options?.scroll_to,
	});
	const public_calls = create_public_call_store();
	const id_source = create_id_source();
	const controller = create_core_controller(host, public_calls, id_source);
	const client = create_client_shell({
		configure_client_options: callbacks.configure,
		controller,
		get_root_el: get_or_create_root_el,
		host,
		id_source,
		public_calls,
		work_indicator: callbacks.work_indicator,
	});
	(window as unknown as DataRevalidationWindow)[
		Symbol.for(browser_global_symbol.data_revalidate_fn)
	] = client.revalidate;
	return R.ok(client);
}
