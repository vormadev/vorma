import { getEffectiveErrorData } from "./component_loader_error_data.ts";
import { type ClientLoadersResult } from "./client_loader_execution.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function setClientLoadersState(
	clr: ClientLoadersResult | undefined,
): void {
	if (clr) {
		__vormaClientGlobal.set("clientLoadersData", clr.data ?? []);
		__vormaClientGlobal.set(
			"outermostClientErrorIdx",
			clr.errorMessage ? clr.data.length - 1 : undefined,
		);
		__vormaClientGlobal.set("outermostClientError", clr.errorMessage);
	}
}

export function deriveAndSetErrorState(): void {
	const effectiveErrData = getEffectiveErrorData();
	__vormaClientGlobal.set("outermostErrorIdx", effectiveErrData.index);
	__vormaClientGlobal.set("outermostError", effectiveErrData.error);
}
