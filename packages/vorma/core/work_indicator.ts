/**
 * Imperative handle for wiring non-Vorma async work into the global work
 * indicator (see {@link WorkIndicatorOptions}).
 *
 * Vorma's own work — navigations, revalidations, and `apiClient` requests —
 * is already reflected in the indicator automatically; nothing needs to call
 * `track` for those. `track` exists for everything else the app wants the
 * same progress bar to cover: a client-side file parse before upload, a
 * one-off `fetch` that bypasses `apiClient`, a `@tanstack/react-query`
 * mutation. Wrap the promise and the indicator counts it exactly like an
 * in-flight Vorma navigation for as long as it is pending:
 *
 * ```
 * await client.workIndicator.track(parseAttachmentBeforeUpload(file));
 * ```
 *
 * `isActive` reports whether the indicator is currently showing (Vorma work
 * or tracked work, after the configured start/stop delays), useful for a
 * custom UI that reacts to indicator state directly instead of rendering an
 * nprogress-style bar.
 */
export type WorkIndicator = {
	track: <T>(promise: PromiseLike<T>) => Promise<T>;
	isActive: () => boolean;
};

/**
 * `ClientOptions.workIndicator` — an nprogress-shaped contract for driving a
 * global loading bar from Vorma's navigation, revalidation, and API-request
 * activity.
 *
 * `start`/`stop` are the only required fields — call whatever library or
 * custom code shows/hides your bar (nprogress's own `start`/`done` slot in
 * directly). Everything else tunes when start/stop actually fire:
 *
 * - `startDelayMs`/`stopDelayMs` (default 12ms each) debounce fast work so
 *   the bar does not flash on-screen for near-instant requests.
 * - `skipNavigations`/`skipApiRequests`/`skipRevalidations` exclude entire
 *   categories of Vorma work from driving the indicator at all — useful when
 *   only some kinds of activity should show a global bar. Per-call opt-outs
 *   (`skipWorkIndicator` on `navigate`/`Link`/`apiClient` calls) layer on
 *   top of these category-level switches.
 *
 * Work tracked through {@link WorkIndicator.track} always counts, regardless
 * of these category switches — they only gate Vorma's own work.
 */
export type WorkIndicatorOptions = {
	start: () => void;
	stop: () => void;
	startDelayMs?: number;
	stopDelayMs?: number;
	skipNavigations?: boolean;
	skipApiRequests?: boolean;
	skipRevalidations?: boolean;
};

// Internal: the client core's owned instance behind the public `WorkIndicator`
// handle. `configure` applies user options at boot; `set_vorma_active` is the
// one input the core drives from its own navigation/revalidation/apiRequest
// state, kept separate from `indicator.track`'s app-driven tokens so both
// sources can be active independently without stepping on each other.
export type WorkIndicatorController = {
	indicator: WorkIndicator;
	configure: (options: WorkIndicatorOptions | undefined) => void;
	set_vorma_active: (active: boolean) => void;
};

// Debounced show/hide state machine: `start`/`stop` fire only after
// `startDelayMs`/`stopDelayMs` of continuous activity, so a burst of very
// fast operations never flashes the bar. Vorma's own activity
// (`set_vorma_active`) and app-tracked promises (`indicator.track`) share one
// token set, so either source alone — or both together — keeps it visible.
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
