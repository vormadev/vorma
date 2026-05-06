import type { RemixNode } from "remix/ui";
import { defineView, Link } from "../../app.tsx";

/////////////////////////////////////////////////////////////////////
/////// PREACT / REACT / SOLID
/////////////////////////////////////////////////////////////////////

// export default defineView({
// 	pattern: "/",
// 	component: (props) => {
// 		return (
// 			<>
// 				<nav>
// 					<Link pattern="/">Home</Link>
// 				</nav>
// 				<props.Outlet />
// 			</>
// 		);
// 	},
// });

/////////////////////////////////////////////////////////////////////
/////// REMIX
/////////////////////////////////////////////////////////////////////

export default defineView({
	pattern: "/",
	component: () => {
		return (props): RemixNode => {
			return (
				<>
					<nav>
						<Link pattern="/">Home</Link>
					</nav>
					{props.Outlet()}
				</>
			);
		};
	},
});
