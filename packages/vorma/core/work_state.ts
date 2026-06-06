export type WorkState = {
	navigation: null | {
		href: string;
		replace: boolean;
		source: "navigate" | "popstate" | "redirect";
	};
	revalidation: null | {
		status: "debouncing" | "running" | "retrying";
		attempt: number;
	};
	prefetch: null | {
		href: string;
	};
	apiRequests: Array<{
		key: string;
		method: string;
		href: string;
	}>;
};

export function create_empty_work_state(): WorkState {
	return {
		navigation: null,
		revalidation: null,
		prefetch: null,
		apiRequests: [],
	};
}
