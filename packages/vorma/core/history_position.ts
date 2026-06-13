import { HISTORY_KEY_FIELD, HISTORY_USER_STATE_FIELD } from "./constants.ts";
import type { HistoryPosition } from "./route_state_projection.ts";
import { get_scroll_pos, save_scroll_for_key } from "./scroll.ts";

/*
Owns the `browser` base fact — the router's view of the browser's current
history entry — and every window.history write that moves it: key-stamped
push/replace commits, adoption of popstate positions, and the boot-time
guarantee that the current entry carries a Vorma history key.
*/

export function create_history_position() {
	let position: HistoryPosition = { href: "", key: "", state: undefined };

	function make_history_key(): string {
		return Math.random().toString(36).slice(2, 10);
	}

	function current(): HistoryPosition {
		return position;
	}

	function read_from_window(): HistoryPosition {
		const state = window.history.state;
		const key =
			state && typeof state === "object" && HISTORY_KEY_FIELD in state
				? (state as Record<string, string>)[HISTORY_KEY_FIELD]!
				: "";
		return {
			href: window.location.href,
			key,
			state: state?.[HISTORY_USER_STATE_FIELD],
		};
	}

	function adopt(next: HistoryPosition): void {
		position = next;
	}

	function commit(
		url: string,
		replace: boolean | undefined,
		user_state: unknown,
	): HistoryPosition {
		const key = make_history_key();
		const next_state: Record<string, unknown> = {
			[HISTORY_KEY_FIELD]: key,
			[HISTORY_USER_STATE_FIELD]: user_state,
		};

		if (replace) {
			const existing = window.history.state;
			const base = existing && typeof existing === "object" ? existing : {};
			window.history.replaceState({ ...(base as object), ...next_state }, "", url);
		} else {
			window.history.pushState(next_state, "", url);
		}

		position = { href: url, key, state: user_state };
		return position;
	}

	// Guarantee the current entry carries a Vorma history key (preserving any
	// foreign state object), then adopt it as the current position.
	function ensure_window_key(): void {
		const history_state = window.history.state;
		const has_history_key =
			history_state &&
			typeof history_state === "object" &&
			HISTORY_KEY_FIELD in (history_state as Record<string, unknown>);
		if (!has_history_key) {
			const base =
				history_state && typeof history_state === "object" ? history_state : {};
			window.history.replaceState(
				{
					...(base as object),
					[HISTORY_KEY_FIELD]: make_history_key(),
				},
				"",
				window.location.href,
			);
		}
		position = read_from_window();
	}

	function save_current_scroll(): void {
		if (position.key) {
			save_scroll_for_key(position.key, get_scroll_pos());
		}
	}

	return {
		position: current,
		read_from_window,
		adopt,
		commit,
		ensure_window_key,
		save_current_scroll,
	};
}

export type HistoryPositionStore = ReturnType<typeof create_history_position>;
