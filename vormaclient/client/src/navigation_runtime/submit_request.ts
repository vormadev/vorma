import { handleRedirects } from "../redirects/redirects.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";

export function buildSubmitRequestInit(props: {
	requestInit?: RequestInit;
	signal: AbortSignal;
}): RequestInit {
	const { requestInit, signal } = props;
	const headers = new Headers(requestInit?.headers);
	const deploymentID = __vormaClientGlobal.get("deploymentID");
	if (deploymentID) {
		headers.set("x-deployment-id", deploymentID);
	}

	return {
		...requestInit,
		headers,
		signal,
	};
}

export async function executeSubmitRequest(props: {
	abortController: AbortController;
	url: URL;
	requestInit: RequestInit;
}) {
	return handleRedirects({
		abortController: props.abortController,
		url: props.url,
		isPrefetch: false,
		redirectCount: 0,
		requestInit: props.requestInit,
	});
}
