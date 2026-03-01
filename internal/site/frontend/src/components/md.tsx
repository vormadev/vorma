import { For, Show } from "solid-js";
import { VormaLink } from "vorma/solid";
import { htmlToMarkdown } from "../html_to_md.ts";
import {
	addClientLoader,
	useLoaderData,
	type RouteProps,
} from "../vorma.bindings.ts";
import { useRootClientLoaderData } from "./home.tsx";
import { RenderedMarkdown } from "./rendered-markdown.tsx";

export const useSplatClientLoaderData = addClientLoader({
	pattern: "/*",
	clientLoader: async (props) => {
		// This is pointless -- just an example of how to use a client loader
		// await new Promise((r) => setTimeout(r, 1_000));
		// console.log(`Client loader '/*' started at ${Date.now()}`);
		const { loaderData } = await props.serverDataPromise;
		// console.log("Server data promise resolved at ", Date.now(), loaderData);

		// This is how you pass an abort signal to your API calls,
		// so that if the navigation aborts, the downstream requests
		// also abort:
		// const res = await api.mutate({
		// 	pattern: "/example",
		// 	requestInit: { signal: props.signal },
		// });

		return loaderData.Title as string;
	},
	reRunOnModuleChange: import.meta,
});

export function MD(props: RouteProps<"/*">) {
	const loaderData = useLoaderData(props);

	const splatClientLoaderData = useSplatClientLoaderData(props);
	const _y = useRootClientLoaderData();
	// console.log("_y", _y());

	return (
		<div class="flex flex-col gap-6" id="md-route">
			<div class="flex flex-wrap items-center gap-6">
				<Show when={loaderData()?.BackItem}>
					{(backUrl) => (
						<VormaLink
							prefetch="intent"
							href={backUrl()}
							class="back-link my-2 self-start"
						>
							↑ Go to parent folder
						</VormaLink>
					)}
				</Show>

				<Show when={loaderData().Content && !loaderData().IsFolder}>
					<button
						class="bg-dark text-light hover:outline-nice-blue rounded-sm border border-[#7777] px-2 py-1 text-xs font-normal tracking-wide uppercase hover:cursor-pointer hover:outline-3 hover:outline-offset-1 sm:ml-auto"
						onClick={async () => {
							const ld = loaderData();
							const markdown = `# ${ld.Title}\n\n${htmlToMarkdown(ld.Content ?? "")}\n`;
							navigator.clipboard.writeText(markdown);
						}}
					>
						✨ Copy as Markdown
					</button>
				</Show>
			</div>
			<Show when={splatClientLoaderData()}>{(n) => <h1>{n()}</h1>}</Show>
			<Show when={loaderData()?.Date}>{(n) => <i>{n()}</i>}</Show>
			<Show when={loaderData()?.Content}>
				{(n) => (
					<RenderedMarkdown
						markdown={n()}
						stripLeadingH1={Boolean(loaderData()?.Title)}
					/>
				)}
			</Show>
			<Show when={loaderData()?.IndexSitemap}>
				{(n) => (
					<ul class="index-grid">
						<For each={n()}>
							{(item) => (
								<li>
									<VormaLink
										prefetch="intent"
										href={item.url}
										class="index-card"
									>
										<h2>{`${item.isFolder ? "📁 " : ""}${item.title}`}</h2>
										<Show when={item.date}>
											<time>{item.date}</time>
										</Show>
										<Show when={item.description}>
											<p>{item.description}</p>
										</Show>
									</VormaLink>
								</li>
							)}
						</For>
					</ul>
				)}
			</Show>
		</div>
	);
}

export function ErrorBoundary(props: { error: string }) {
	return <div>Error: {props.error}</div>;
}
