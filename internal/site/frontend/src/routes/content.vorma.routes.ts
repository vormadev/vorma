import { route } from "vorma/buildtime";

route("/*", import("../components/md.tsx"), "MD", "ErrorBoundary");
