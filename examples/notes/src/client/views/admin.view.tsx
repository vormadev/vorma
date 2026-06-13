import { useState } from "react";
import { getClientCookie } from "vorma/kit/cookies";
import { apiClient, defineView, navigate, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/admin",
	component: (props) => {
		const data = useViewData(props);
		const cookie_session = getClientCookie("notes_session");
		const [api_ok, set_api_ok] = useState<boolean | null>(null);

		const check_api = async () => {
			const ping = await apiClient.queryOrThrow({
				method: "GET",
				pattern: "/api/ping",
				input: undefined,
			});
			set_api_ok(ping.ok);
		};

		const sign_out = async () => {
			await apiClient.mutateOrThrow({
				method: "DELETE",
				pattern: "/api/session",
				input: undefined,
			});
			await navigate({ pattern: "/_index" });
		};

		return (
			<main>
				<h2>Admin</h2>
				<p>
					Server sees session <strong>{data.session}</strong>
					{cookie_session ? ` (cookie: ${cookie_session})` : null}
				</p>
				<button
					onClick={() => {
						void check_api();
					}}
					type="button"
				>
					Check API{api_ok === null ? "" : api_ok ? ": ok" : ": down"}
				</button>
				<button
					onClick={() => {
						void sign_out();
					}}
					type="button"
				>
					Sign out
				</button>
			</main>
		);
	},
});
