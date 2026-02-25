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
	"/explode",
	import("../components/error_boundary.tsx"),
	"Explode",
	"ErrorBoundary",
);
