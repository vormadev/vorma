import { createVormaClient } from "vorma/solid";
import { vormaAppConfig } from "./vorma.gen.ts";

export const app = createVormaClient(vormaAppConfig, {
	linkDefaultProps: { prefetch: "intent", visitOnPointerDown: true },
});

export const { defineRoute, Link, useLoaderData, useClientLoaderData } = app;
