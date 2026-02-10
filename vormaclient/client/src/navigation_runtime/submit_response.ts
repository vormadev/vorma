import { getIsGETRequest } from "vorma/kit/url";
import {
	effectuateRedirectDataResult,
	type RedirectData,
} from "../redirects/redirects.ts";
import type { NavigateProps, SubmitOptions } from "./types.ts";

type SubmitResult<T> =
	| { success: true; data: T }
	| { success: false; error: string };

export async function finalizeSubmitResponse<T>(props: {
	response?: Response;
	redirectData: RedirectData | null;
	requestInit?: RequestInit;
	options?: SubmitOptions;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
}): Promise<SubmitResult<T>> {
	const { response, redirectData, requestInit, options, navigate } = props;

	if (!response || !response.ok) {
		return {
			success: false,
			error: String(response?.status || "unknown"),
		};
	}

	if (redirectData?.status === "should") {
		await effectuateRedirectDataResult(redirectData, 0);
		return { success: true, data: undefined as T };
	}

	const data = await response.json();

	// Auto-revalidate for mutations
	const isGET = getIsGETRequest(requestInit);
	const redirected = redirectData?.status === "did";
	if (!isGET && !redirected && options?.revalidate !== false) {
		await navigate({
			href: window.location.href,
			navigationType: "revalidation",
		});
	}

	return { success: true, data: data as T };
}
