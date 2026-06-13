import { useAtomValue } from "jotai";
import type { FormEvent } from "react";
import { useState } from "react";
import { useApiMutation } from "../api.ts";
import { Link, defineView, useViewData, useWorkState } from "../app.tsx";
import { themeAtom, toggleTheme } from "../theme.ts";

export default defineView({
	pattern: "/",
	component: (props) => {
		const data = useViewData(props);
		const busy = useWorkState((work) => {
			return work.navigation !== null || work.apiRequests.length > 0;
		});
		const [username, set_username] = useState("");
		const theme_state = useAtomValue(themeAtom);
		const sign_in = useApiMutation({ method: "POST", pattern: "/api/session" });
		const sign_out = useApiMutation({ method: "DELETE", pattern: "/api/session" });

		const submit_sign_in = (event: FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			sign_in.mutate(
				{ input: { username } },
				{
					onSuccess: () => {
						set_username("");
					},
				},
			);
		};

		return (
			<div className="shell" data-busy={busy ? "1" : undefined}>
				<header className="topbar">
					<Link href="/">
						<h1>{data.app_name}</h1>
					</Link>
					{data.current_user ? (
						<>
							<span>{data.current_user.username}</span>
							<button
								disabled={sign_out.isPending}
								onClick={() => {
									sign_out.mutate({});
								}}
								type="button"
							>
								Sign out
							</button>
						</>
					) : (
						<form onSubmit={submit_sign_in}>
							<input
								aria-label="Username"
								onChange={(event) => {
									set_username(event.currentTarget.value);
								}}
								placeholder="username"
								value={username}
							/>
							<button disabled={sign_in.isPending} type="submit">
								Sign in
							</button>
						</form>
					)}
					<button onClick={toggleTheme} type="button">
						Theme: {theme_state.theme}
						{theme_state.theme === "system"
							? ` (${theme_state.resolved})`
							: null}
					</button>
				</header>
				{sign_in.error ? <p className="error">{sign_in.error.message}</p> : null}
				<props.Outlet />
			</div>
		);
	},
});
