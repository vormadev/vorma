import { route } from "vorma/client";

route("/", import("./components/home.tsx"), "RootLayout");
route("/_index", import("./components/home.tsx"), "Home");
// route("/*", import("./components/md.tsx"), "MD", "ErrorBoundary");
route("/__/:dyn", import("./components/dyn.tsx"), "Dyn");
