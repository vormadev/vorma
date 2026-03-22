import { route } from "vorma/buildtime";

route("/", import("./components/home.tsx"), "Home");
