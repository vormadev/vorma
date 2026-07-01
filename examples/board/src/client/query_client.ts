import { QueryClient } from "@tanstack/react-query";

/*
React Query expects one long-lived client for the app. Keeping it in its
own file lets `app.tsx` install the provider while `api.ts` defines
typed hooks, without either module importing the other through a cycle.
*/
export const query_client = new QueryClient();
