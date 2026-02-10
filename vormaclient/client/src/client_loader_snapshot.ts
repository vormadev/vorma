import type { PartialWaitFnJSON } from "./client_loader_execution.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function buildClientLoaderSnapshotFromGlobal(): PartialWaitFnJSON {
	return {
		hasRootData: __vormaClientGlobal.get("hasRootData"),
		importURLs: __vormaClientGlobal.get("importURLs"),
		loadersData: __vormaClientGlobal.get("loadersData"),
		matchedPatterns: __vormaClientGlobal.get("matchedPatterns"),
		params: __vormaClientGlobal.get("params"),
		splatValues: __vormaClientGlobal.get("splatValues"),
	};
}
