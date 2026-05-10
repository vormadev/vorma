import type {
	BuildSkewDetectedEvent,
	ClientCommit,
	ClientOptions,
	WorkActivity,
	WorkIndicator,
	WorkIndicatorOptions,
} from "./client_contract.ts";
import { work_activity_kind } from "./client_contract.ts";

const work_indicator_delay = {
	start_ms: 12,
	stop_ms: 12,
} as const;

export type ClientCallbacks = {
	commit: (commit: ClientCommit) => void;
	configure: (options: ClientOptions) => void;
	render: () => void | Promise<void>;
	report_build_skew: (event: BuildSkewDetectedEvent) => void;
	work_indicator: WorkIndicator;
};

export function create_client_callbacks(
	commit: (commit: ClientCommit) => void,
): ClientCallbacks {
	let client_options: ClientOptions | undefined;
	let current_work_activity: WorkActivity = [];
	let work_indicator_options: WorkIndicatorOptions | undefined;
	let visible = false;
	let show_timer: ReturnType<typeof setTimeout> | undefined;
	let hide_timer: ReturnType<typeof setTimeout> | undefined;
	const active_tokens = new Set<symbol>();
	let release_vorma_work: (() => void) | undefined;

	function clear_show_timer(): void {
		if (show_timer === undefined) {
			return;
		}
		clearTimeout(show_timer);
		show_timer = undefined;
	}

	function clear_hide_timer(): void {
		if (hide_timer === undefined) {
			return;
		}
		clearTimeout(hide_timer);
		hide_timer = undefined;
	}

	function sync_indicator(): void {
		const options = work_indicator_options;
		if (!options) {
			clear_show_timer();
			clear_hide_timer();
			return;
		}
		if (active_tokens.size > 0) {
			clear_hide_timer();
			if (visible || show_timer !== undefined) {
				return;
			}
			show_timer = setTimeout(() => {
				show_timer = undefined;
				if (
					!work_indicator_options ||
					active_tokens.size === 0 ||
					visible
				) {
					return;
				}
				work_indicator_options.start();
				visible = true;
			}, options.startDelayMS ?? work_indicator_delay.start_ms);
			return;
		}
		clear_show_timer();
		if (!visible || hide_timer !== undefined) {
			return;
		}
		hide_timer = setTimeout(() => {
			hide_timer = undefined;
			if (!work_indicator_options || active_tokens.size > 0) {
				return;
			}
			work_indicator_options.stop();
			visible = false;
		}, options.stopDelayMS ?? work_indicator_delay.stop_ms);
	}

	function begin_indicator_work(): () => void {
		const token = Symbol("v-work-indicator");
		let released = false;
		active_tokens.add(token);
		sync_indicator();
		return () => {
			if (released) {
				return;
			}
			released = true;
			active_tokens.delete(token);
			sync_indicator();
		};
	}

	function configure_work_indicator(
		next_options: WorkIndicatorOptions | undefined,
	): void {
		const previous_options = work_indicator_options;
		clear_show_timer();
		clear_hide_timer();
		if (visible && previous_options && previous_options !== next_options) {
			previous_options.stop();
			visible = false;
		}
		work_indicator_options = next_options;
		sync_indicator();
	}

	function set_vorma_work_active(active: boolean): void {
		if (active) {
			if (!release_vorma_work) {
				release_vorma_work = begin_indicator_work();
			}
			return;
		}
		if (!release_vorma_work) {
			sync_indicator();
			return;
		}
		release_vorma_work();
		release_vorma_work = undefined;
	}

	function configure(options: ClientOptions): void {
		client_options = options;
		configure_work_indicator(options.workIndicator);
		set_vorma_work_active(work_is_active(options, current_work_activity));
	}

	function emit_commit(client_commit: ClientCommit): void {
		commit(client_commit);
		const options = client_options;
		if (client_commit.route_update) {
			options?.onRouteUpdate?.(
				client_commit.route_update.route,
				client_commit.route_update.previous_route,
				client_commit.route_update.reason,
			);
		}
		if (client_commit.work_activity !== undefined) {
			current_work_activity = client_commit.work_activity;
		}
		if (client_commit.work) {
			options?.onWorkUpdate?.(client_commit.work);
		}
		if (client_commit.work_activity !== undefined || client_commit.work) {
			set_vorma_work_active(
				work_is_active(options, current_work_activity),
			);
		}
	}

	return {
		commit: emit_commit,
		configure,
		render: () => {
			return client_options?.render?.();
		},
		report_build_skew: (event) => {
			client_options?.onBuildSkewDetected?.(event);
		},
		work_indicator: {
			isActive: () => {
				return active_tokens.size > 0;
			},
			track: <T>(promise: PromiseLike<T>): Promise<T> => {
				const release = begin_indicator_work();
				return Promise.resolve(promise).finally(() => {
					release();
				});
			},
		},
	};
}

function work_is_active(
	options: ClientOptions | undefined,
	work_activity: WorkActivity,
): boolean {
	if (!options?.workIndicator) {
		return false;
	}
	for (const activity of work_activity) {
		if (
			activity.kind === work_activity_kind.navigation &&
			options.workIndicator.skipNavigations !== true &&
			!activity.skip_work_indicator
		) {
			return true;
		}
		if (
			activity.kind === work_activity_kind.revalidation &&
			options.workIndicator.skipRevalidations !== true &&
			!activity.skip_work_indicator
		) {
			return true;
		}
		if (
			activity.kind === work_activity_kind.api_request &&
			options.workIndicator.skipAPIRequests !== true &&
			!activity.skip_work_indicator
		) {
			return true;
		}
	}
	return false;
}
