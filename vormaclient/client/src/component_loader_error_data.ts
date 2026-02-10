import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function getEffectiveErrorData(): {
	index: number | undefined;
	error: string | undefined;
} {
	const serverErrorIdx = __vormaClientGlobal.get("outermostServerErrorIdx");
	const clientErrorIdx = __vormaClientGlobal.get("outermostClientErrorIdx");
	let errorIdx: number | undefined;
	if (serverErrorIdx != null && clientErrorIdx != null) {
		errorIdx = Math.min(serverErrorIdx, clientErrorIdx);
	} else {
		errorIdx = serverErrorIdx ?? clientErrorIdx;
	}
	return {
		index: errorIdx,
		error:
			errorIdx === serverErrorIdx
				? __vormaClientGlobal.get("outermostServerError")
				: errorIdx === clientErrorIdx
					? __vormaClientGlobal.get("outermostClientError")
					: undefined,
	};
}
