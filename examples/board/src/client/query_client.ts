import { QueryClient } from "@tanstack/react-query";

/*
One client for the whole app; lives alone so app.tsx (provider) and
api.ts (hooks) can both import it without a module cycle.
*/
export const queryClient = new QueryClient();
