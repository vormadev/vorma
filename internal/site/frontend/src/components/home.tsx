import {
	addClientLoader,
	Link,
	type RouteProps,
	usePatternLoaderData,
} from "../vorma.bindings.ts";
import { useSplatClientLoaderData } from "./md.tsx";

export const useRootClientLoaderData = addClientLoader({
	pattern: "/",
	clientLoader: async (props) => {
		console.log(`Client loader '/' started at ${Date.now()}`);
		await new Promise((resolve, reject) => {
			const timer = setTimeout(resolve, 1_000);
			props.signal.addEventListener(
				"abort",
				() => {
					clearTimeout(timer);
					reject(props.signal.reason);
				},
				{ once: true },
			);
		});
		console.log("fake API call finished at ", Date.now());
		const { loaderData } = await props.serverDataPromise;
		console.log("Server data promise resolved at ", Date.now());
		return loaderData.LatestVersion;
	},
	reRunOnModuleChange: import.meta,
});

export function RootLayout(props: RouteProps<"/">) {
	return props.Outlet;
}

export function Home(_props: RouteProps<"/_index">) {
	const _x = usePatternLoaderData("/");
	const _y = useRootClientLoaderData();
	const _z = useSplatClientLoaderData();
	console.log("_x", _x());
	console.log("_y", _y());
	console.log("_z", _z()); // should be undefined on this page

	return (
		<>
			<div class="mt-4 flex flex-col gap-2 sm:gap-1">
				<h2 class="big-heading">
					The Golang metaframework, powered by Vite.
				</h2>
			</div>

			<div class="mb-6 flex flex-wrap gap-3">
				<a
					class="hover:bg-nice-blue cursor-pointer rounded-sm bg-(--fg) px-1.5 py-0.5 text-sm font-medium text-(--bg) hover:text-white"
					href="https://github.com/vormadev/vorma"
					target="_blank"
					rel="noreferrer"
				>
					⭐ github.com
				</a>

				<a
					class="hover:bg-nice-blue cursor-pointer rounded-sm bg-(--fg) px-1.5 py-0.5 text-sm font-medium text-(--bg) hover:text-white"
					href="https://pkg.go.dev/github.com/vormadev/vorma"
					target="_blank"
					rel="noreferrer"
				>
					🔷 pkg.go.dev
				</a>

				<a
					class="hover:bg-nice-blue cursor-pointer rounded-sm bg-(--fg) px-1.5 py-0.5 text-sm font-medium text-(--bg) hover:text-white"
					href="https://npmx.dev/package/vorma"
					target="_blank"
					rel="noreferrer"
				>
					📦 npmx.dev
				</a>

				<a
					class="hover:bg-nice-blue cursor-pointer rounded-sm bg-(--fg) px-1.5 py-0.5 text-sm font-medium text-(--bg) hover:text-white"
					href="https://x.com/vormadev"
					target="_blank"
					rel="noreferrer"
				>
					𝕏 x.com
				</a>
			</div>

			<div>
				<h2 class="scream-heading">Quick Start</h2>
				<code class="inline-code high-contrast self-start text-xl font-bold italic">
					npm create vorma@latest
				</code>
			</div>

			<div>
				<h2 class="scream-heading">What is Vorma?</h2>
				<div class="flex-col-wrapper">
					<p class="leading-[1.75]">
						Vorma is a lot like Next.js, Remix, or TanStack Start,
						but it uses{" "}
						<b>
							<i>Go</i>
						</b>{" "}
						on the backend, with your choice of{" "}
						<b>
							<i>React</i>
						</b>
						,{" "}
						<b>
							<i>Solid</i>
						</b>
						, or{" "}
						<b>
							<i>Preact</i>
						</b>{" "}
						on the frontend.
					</p>

					<p class="leading-[1.75]">
						It has{" "}
						<b>
							<i>nested routing</i>
						</b>
						, effortless
						<b>
							<i> end-to-end type safety</i>
						</b>{" "}
						(including Link components!),{" "}
						<b>
							<i>parallel-executed route loaders</i>
						</b>
						, and much, much more.
					</p>

					<p class="leading-[1.75]">
						It's deeply integrated with{" "}
						<b>
							<i>Vite</i>
						</b>{" "}
						to give you full{" "}
						<b>
							<i>hot module reloading</i>
						</b>{" "}
						at dev-time.
					</p>
				</div>
			</div>

			<div>
				<h2 class="scream-heading">Get started</h2>
				<div class="flex-col-wrapper">
					<p class="leading-[1.75]">
						If you want to dive right in, just open a terminal and
						run{" "}
						<code class="inline-code">npm create vorma@latest</code>{" "}
						and follow the prompts.
					</p>
					<p class="leading-[1.75]">
						If you'd prefer to read more first, take a peek at{" "}
						<Link
							pattern="/*"
							splatValues={["docs"]}
							class="underline"
						>
							our docs
						</Link>
						.
					</p>
				</div>
			</div>

			<div>
				<h2 class="scream-heading">Status</h2>
				<p class="leading-[1.75]">
					Vorma's underlying tech has reached a good degree of
					stability, but its APIs are still evolving. Sub-1.0 releases
					may contain breaking changes. If you ever need help
					upgrading to the latest version, feel free to{" "}
					<a
						href="https://github.com/vormadev/vorma"
						target="_blank"
						rel="noreferrer"
					>
						file an issue on GitHub
					</a>{" "}
					or{" "}
					<a
						href="https://x.com/vormadev"
						target="_blank"
						rel="noreferrer"
					>
						reach out on&nbsp;X
					</a>
					.
				</p>
			</div>
		</>
	);
}
