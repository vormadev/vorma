export type WorkIndicator = {
	track: <T>(promise: PromiseLike<T>) => Promise<T>;
	isActive: () => boolean;
};

export type WorkIndicatorOptions = {
	start: () => void;
	stop: () => void;
	startDelayMs?: number;
	stopDelayMs?: number;
	skipNavigations?: boolean;
	skipApiRequests?: boolean;
	skipRevalidations?: boolean;
};

export type WorkIndicatorController = {
	indicator: WorkIndicator;
	configure: (options: WorkIndicatorOptions | undefined) => void;
	set_vorma_active: (active: boolean) => void;
};

export function create_work_indicator(): WorkIndicatorController {
	let options: WorkIndicatorOptions | undefined;
	let visible = false;
	let show_timer: number | undefined;
	let hide_timer: number | undefined;
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

	function sync(): void {
		const current_options = options;
		if (!current_options) {
			clear_show_timer();
			clear_hide_timer();
			return;
		}

		if (active_tokens.size > 0) {
			clear_hide_timer();
			if (visible || show_timer !== undefined) {
				return;
			}
			show_timer = window.setTimeout(() => {
				show_timer = undefined;
				const latest_options = options;
				if (!latest_options || active_tokens.size === 0 || visible) {
					return;
				}
				latest_options.start();
				visible = true;
			}, current_options.startDelayMs ?? 12);
			return;
		}

		clear_show_timer();
		if (!visible) {
			return;
		}
		if (hide_timer !== undefined) {
			return;
		}
		hide_timer = window.setTimeout(() => {
			hide_timer = undefined;
			const latest_options = options;
			if (!latest_options || active_tokens.size > 0) {
				return;
			}
			latest_options.stop();
			visible = false;
		}, current_options.stopDelayMs ?? 12);
	}

	function begin(): () => void {
		const token = Symbol("v-work-indicator");
		let released = false;
		active_tokens.add(token);
		sync();
		return () => {
			if (released) {
				return;
			}
			released = true;
			active_tokens.delete(token);
			sync();
		};
	}

	function configure(next_options: WorkIndicatorOptions | undefined): void {
		const previous_options = options;
		clear_show_timer();
		clear_hide_timer();
		if (visible && previous_options && previous_options !== next_options) {
			previous_options.stop();
			visible = false;
		}
		options = next_options;
		sync();
	}

	function set_vorma_active(active: boolean): void {
		if (active) {
			if (!release_vorma_work) {
				release_vorma_work = begin();
			}
			return;
		}
		if (!release_vorma_work) {
			sync();
			return;
		}
		release_vorma_work();
		release_vorma_work = undefined;
	}

	return {
		indicator: {
			track: async <T>(promise: PromiseLike<T>): Promise<T> => {
				const release = begin();
				return Promise.resolve(promise).finally(() => {
					release();
				});
			},
			isActive: (): boolean => {
				return active_tokens.size > 0;
			},
		},
		configure,
		set_vorma_active,
	};
}
