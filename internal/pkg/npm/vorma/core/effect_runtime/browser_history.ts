import { Effect, Ref } from "effect";
import { HISTORY_KEY_FIELD, HISTORY_USER_STATE_FIELD } from "../constants.ts";
import type { HistoryPosition } from "./route_publisher.ts";

export type BrowserHistory = {
	ensure_current: Effect.Effect<HistoryPosition>;
	current: Effect.Effect<HistoryPosition>;
	adopt_current: Effect.Effect<HistoryPosition>;
	commit: (
		href: string,
		replace: boolean,
		user_state: unknown,
	) => Effect.Effect<HistoryPosition>;
};

export function make_browser_history(): Effect.Effect<BrowserHistory, never> {
	return Effect.gen(function* () {
		const initial_position = read_position(true);
		const current_ref = yield* Ref.make(initial_position);

		const adopt_current = Effect.sync(() => {
			return read_position(false);
		}).pipe(
			Effect.flatMap((position) => {
				return Ref.set(current_ref, position).pipe(Effect.as(position));
			}),
		);

		const ensure_current = Effect.sync(() => {
			return read_position(true);
		}).pipe(
			Effect.flatMap((position) => {
				return Ref.set(current_ref, position).pipe(Effect.as(position));
			}),
		);

		return {
			ensure_current,
			current: Ref.get(current_ref),
			adopt_current,
			commit: (href, replace, user_state) => {
				return Effect.sync(() => {
					const key = make_history_key();
					const next_state = {
						[HISTORY_KEY_FIELD]: key,
						[HISTORY_USER_STATE_FIELD]: user_state,
					};
					if (replace) {
						const base = history_state_record();
						window.history.replaceState(
							{
								...base,
								...next_state,
							},
							"",
							href,
						);
					} else {
						window.history.pushState(next_state, "", href);
					}
					return { href, key, state: user_state };
				}).pipe(
					Effect.flatMap((position) => {
						return Ref.set(current_ref, position).pipe(
							Effect.as(position),
						);
					}),
				);
			},
		};
	});
}

function read_position(ensure_key: boolean): HistoryPosition {
	const state_record = history_state_record();
	const raw_key = state_record[HISTORY_KEY_FIELD];
	let key = typeof raw_key === "string" && raw_key.length > 0 ? raw_key : "";
	if (key.length === 0 && ensure_key) {
		key = make_history_key();
		window.history.replaceState(
			{
				...state_record,
				[HISTORY_KEY_FIELD]: key,
			},
			"",
			window.location.href,
		);
	}
	return {
		href: window.location.href,
		key,
		state: state_record[HISTORY_USER_STATE_FIELD],
	};
}

function history_state_record(): Record<string, unknown> {
	const state = window.history.state;
	if (state && typeof state === "object") {
		return state as Record<string, unknown>;
	}
	return {};
}

function make_history_key(): string {
	return Math.random().toString(36).slice(2, 10);
}
