/// <reference types="vite/client" />

import "./styles/main.css";
import { app } from "./vorma.app.ts";

await app.init();

void import("./setup.ts");
