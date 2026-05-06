import { vorma } from "./app.tsx";
import "./styles/main.css";

await vorma.boot();

void import("./setup.ts");
