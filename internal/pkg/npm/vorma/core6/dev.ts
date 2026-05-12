/// <reference types="vite/client" />

import type {
	Core6RouteHMRConfig,
	Core6RouteHMROwner,
	Core6RouteHMRUpdateInput,
} from "./route_hmr.ts";

export const core6_dev_hmr_route_update_global =
	"__vorma_hmr_route_update" as const;

export const core6_dev_hmr_install_result_kind = {
	disabled: "disabled",
	installed: "installed",
} as const;

export type Core6DevHMRHandler = (
	raw_module_url: string,
	module: Core6RouteHMRUpdateInput["module"],
) => Promise<void>;

export type Core6DevHMRWindow = Window & {
	[core6_dev_hmr_route_update_global]?: Core6DevHMRHandler;
};

export type Core6DevHMRInstallResult =
	| {
			kind: typeof core6_dev_hmr_install_result_kind.disabled;
	  }
	| {
			kind: typeof core6_dev_hmr_install_result_kind.installed;
			owner: Core6RouteHMROwner;
	  };

export type Core6DevHMRInstallConfig = Core6RouteHMRConfig & {
	window?: Core6DevHMRWindow;
};

const core6_dev_hmr_view_configs = new Map<string, boolean>();
let core6_dev_hmr_owner: Core6RouteHMROwner | null = null;

export function configure_core6_dev_hmr_view(
	pattern: string,
	run_client_loader: boolean,
): void {
	core6_dev_hmr_view_configs.set(pattern, run_client_loader);
	core6_dev_hmr_owner?.configure_view({
		pattern,
		run_client_loader,
	});
}

export async function install_core6_dev_hmr(
	config: Core6DevHMRInstallConfig,
): Promise<Core6DevHMRInstallResult> {
	if (!import.meta.env.DEV || !import.meta.hot) {
		return { kind: core6_dev_hmr_install_result_kind.disabled };
	}
	const hot = import.meta.hot;
	const { create_core6_route_hmr_owner } = await import("./route_hmr.ts");
	const owner = create_core6_route_hmr_owner(config);
	core6_dev_hmr_owner = owner;
	for (const [pattern, run_client_loader] of core6_dev_hmr_view_configs) {
		owner.configure_view({
			pattern,
			run_client_loader,
		});
	}
	const target_window = (config.window ?? window) as Core6DevHMRWindow;
	const on_route_update: Core6DevHMRHandler = async (
		raw_module_url,
		module,
	) => {
		await owner.update({ module, raw_module_url });
	};
	target_window[core6_dev_hmr_route_update_global] = on_route_update;
	hot.dispose(() => {
		if (
			target_window[core6_dev_hmr_route_update_global] === on_route_update
		) {
			target_window[core6_dev_hmr_route_update_global] = undefined;
		}
	});
	return {
		kind: core6_dev_hmr_install_result_kind.installed,
		owner,
	};
}
