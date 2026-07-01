import { useEffect, useState } from "react";
import { debounce, type Debounced } from "vorma/kit/debounce";
import { addOnWindowFocusListener } from "vorma/kit/listeners";
import { MutationError } from "vorma/react";
import { apiClient, defineView } from "../app.tsx";
import { build_kit_diagnostics } from "../kit_diagnostics.ts";

export default defineView({
	pattern: "/mod/diagnostics",
	component: () => {
		const [trigger_route_error, set_trigger_route_error] = useState(false);
		const [envelope_message, set_envelope_message] = useState<string | null>(null);
		const [typed_error_message, set_typed_error_message] = useState<string | null>(
			null,
		);
		const [kit_input, set_kit_input] = useState("Vorma Board");
		const [kit_report, set_kit_report] = useState<string | null>(null);
		const [kit_error, set_kit_error] = useState<string | null>(null);
		const [focus_count, set_focus_count] = useState(0);

		useEffect(() => {
			/*
			`addOnWindowFocusListener` is useful for app-owned browser work
			that should happen when a user returns to the tab. Vorma owns
			route-data revalidation; this local counter is just diagnostics UI.
			*/
			return addOnWindowFocusListener(() => {
				set_focus_count((count) => {
					return count + 1;
				});
			});
		}, []);

		useEffect(() => {
			/*
			The standalone debounce helper is for non-route local work. Route
			search params should still use `useRouteSync({ debounceMs })`.
			*/
			const run_kit_diagnostics: Debounced<typeof build_kit_diagnostics> = debounce(
				build_kit_diagnostics,
				120,
			);
			void run_kit_diagnostics(kit_input).then(
				(result) => {
					if (result.ok) {
						set_kit_report(result.val.pretty_json);
						set_kit_error(null);
					} else {
						set_kit_report(null);
						set_kit_error(result.err);
					}
				},
				(error: unknown) => {
					if (
						error instanceof Error &&
						error.message === "Debounced call cancelled"
					) {
						return;
					}
					set_kit_report(null);
					set_kit_error(error instanceof Error ? error.message : String(error));
				},
			);
			return () => {
				run_kit_diagnostics.cancel();
			};
		}, [kit_input]);

		if (trigger_route_error) {
			/*
			Throwing from a view component exercises the route-local error
			boundary below. App-level `defaultErrorBoundary` covers views that
			do not provide their own boundary.
			*/
			throw new Error("diagnostics route boundary probe");
		}

		const probe_mutation_envelope = async () => {
			/*
			`mutate` returns the envelope instead of throwing. That is useful for
			diagnostics or screens that want to display success/error payloads
			in the same control flow.
			*/
			const result = await apiClient.mutate({
				method: "POST",
				pattern: "/api/mod/stories/:story_id/restore",
				params: { story_id: "0" },
				revalidate: false,
				skipWorkIndicator: true,
			});
			set_envelope_message(
				result.success ? "unexpected restore success" : result.error,
			);
		};

		const probe_typed_mutation_error = async () => {
			try {
				/*
				`mutateOrThrow` is the usual UI path: success returns data,
				failure throws `MutationError` with the original envelope.
				*/
				await apiClient.mutateOrThrow({
					method: "POST",
					pattern: "/api/mod/stories/:story_id/restore",
					params: { story_id: "0" },
					revalidate: false,
					skipWorkIndicator: true,
				});
				set_typed_error_message("unexpected restore success");
			} catch (error) {
				set_typed_error_message(
					error instanceof MutationError
						? error.result.error
						: error instanceof Error
							? error.message
							: "unknown mutation error",
				);
			}
		};

		return (
			<section className="stack">
				<h3>Diagnostics</h3>
				<div className="actions">
					<button
						onClick={() => {
							set_trigger_route_error(true);
						}}
						type="button"
					>
						Trigger route boundary
					</button>
					<button
						onClick={() => {
							void probe_mutation_envelope();
						}}
						type="button"
					>
						Check mutation envelope
					</button>
					<button
						onClick={() => {
							void probe_typed_mutation_error();
						}}
						type="button"
					>
						Check typed mutation error
					</button>
				</div>
				{envelope_message ? <p className="meta">{envelope_message}</p> : null}
				{typed_error_message ? (
					<p className="error">{typed_error_message}</p>
				) : null}
				<label className="stack">
					Kit diagnostics seed
					<input
						onChange={(event) => {
							set_kit_input(event.currentTarget.value);
						}}
						value={kit_input}
					/>
				</label>
				<p className="meta">Window focus diagnostics: {focus_count}</p>
				{kit_error ? <p className="error">{kit_error}</p> : null}
				{kit_report ? <pre>{kit_report}</pre> : null}
			</section>
		);
	},
	errorBoundary: ({ error }) => {
		return (
			<section>
				<h3>Diagnostics</h3>
				<p className="error">
					{error instanceof Error ? error.message : "Diagnostics unavailable."}
				</p>
			</section>
		);
	},
});
