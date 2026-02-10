import { executeSubmit } from "./submit.ts";
import type { NavigateProps, SubmitOptions, SubmissionEntry } from "./types.ts";

export type RuntimeSubmit = <T = any>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
) => Promise<{ success: true; data: T } | { success: false; error: string }>;

export type CreateRuntimeSubmitOptions = {
	submissions: Map<string | symbol, SubmissionEntry>;
	scheduleStatusUpdate: () => void;
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

export function createRuntimeSubmit(
	options: CreateRuntimeSubmitOptions,
): RuntimeSubmit {
	const { submissions, scheduleStatusUpdate, navigate } = options;

	return async function submit<T = any>(
		url: string | URL,
		requestInit?: RequestInit,
		submitOptions?: SubmitOptions,
	): Promise<{ success: true; data: T } | { success: false; error: string }> {
		return executeSubmit<T>(
			{
				submissions,
				scheduleStatusUpdate,
				navigate,
			},
			url,
			requestInit,
			submitOptions,
		);
	};
}
