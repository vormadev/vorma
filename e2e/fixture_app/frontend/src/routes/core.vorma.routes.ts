import { route } from "vorma/buildtime";

route("/", import("../components/root.tsx"), "Root");
route("/_index", import("../components/home.tsx"), "Home");
route("/users/:id", import("../components/user.tsx"), "User");
route("/slow/:bucket", import("../components/slow.tsx"), "Slow");
route("/mutation-lab", import("../components/mutation_lab.tsx"), "MutationLab");
route(
	"/navigation-race",
	import("../components/navigation_race.tsx"),
	"NavigationRace",
);
route("/hmr-probe", import("../components/hmr_probe.tsx"), "HMRProbe");
route(
	"/redirect-chain/start",
	import("../components/redirect_chain.tsx"),
	"RedirectChainStart",
);
route(
	"/redirect-chain/middle",
	import("../components/redirect_chain.tsx"),
	"RedirectChainMiddle",
);
route(
	"/redirect-chain/end",
	import("../components/redirect_chain.tsx"),
	"RedirectChainEnd",
);
route("/odd-shapes", import("../components/odd_shapes.tsx"), "OddShapes");
route(
	"/explode",
	import("../components/error_boundary.tsx"),
	"Explode",
	"ErrorBoundary",
);
