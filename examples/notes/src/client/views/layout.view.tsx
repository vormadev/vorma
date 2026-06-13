import { initTheme } from "vorma/kit/theme";
import {
	Link,
	cancelPrefetch,
	defineView,
	prefetch,
	useViewData,
	useWorkState,
} from "../app.tsx";

const theme = initTheme();

export default defineView({
	pattern: "/",
	component: (props) => {
		const data = useViewData(props);
		const busy = useWorkState((work) => {
			return work.navigation !== null || work.apiRequests.length > 0;
		});

		return (
			<div className="shell" data-busy={busy ? "1" : undefined}>
				<header className="topbar">
					<Link href="/">
						<img alt="" className="mark" src={data.mark_url} />
						<h1>{data.app_name}</h1>
					</Link>
					<nav aria-label="Sections">
						<Link href="/tags">Tags</Link>
						<Link
							href="/stats"
							onMouseEnter={() => {
								prefetch({ pattern: "/stats" });
							}}
							onMouseLeave={() => {
								cancelPrefetch({ pattern: "/stats" });
							}}
							prefetch="none"
						>
							Stats
						</Link>
						{data.signed_in ? <Link href="/admin">Admin</Link> : null}
					</nav>
					<button
						onClick={() => {
							theme.setTheme(theme.getNextToggleValue(theme.getTheme()));
						}}
						type="button"
					>
						Theme: {theme.getResolvedTheme()}
					</button>
					{busy ? <span aria-live="polite">Working…</span> : null}
				</header>
				<props.Outlet />
			</div>
		);
	},
});
