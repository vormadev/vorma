import { done, isStarted, start } from "nprogress";
import { setupGlobalLoadingIndicator } from "vorma/client";
import { setTheme, THEMES } from "vorma/kit/theme";
import { VormaRootOutlet } from "vorma/solid";
import "../styles/tailwind.css";
import { Link, theme, useRouterData } from "../vorma.utils.tsx";

setupGlobalLoadingIndicator({ start, stop: done, isRunning: isStarted });

const theme_to_label_map = {
	[THEMES.Light]: hashedURL("sun.svg"),
	[THEMES.Dark]: hashedURL("moon.svg"),
	[THEMES.System]: hashedURL("desktop.svg"),
};

for (const url of Object.values(theme_to_label_map)) {
	preload_img(url);
}

function preload_img(url: string) {
	const img = new Image();
	img.src = url;
	return new Promise((resolve, reject) => {
		img.onload = resolve;
		img.onerror = reject;
	});
}

export function App() {
	const routerData = useRouterData();

	return (
		<>
			<nav class="w-full flex items-center flex-wrap">
				<div class="flex items-baseline">
					<Link pattern="/">
						<h1 class="logo">
							<img
								src={hashedURL("favicon.svg")}
								alt="Vorma logo"
								class="w-5 h-5 brightness-85 dark:brightness-[unset]"
							/>
							<span>Vorma</span>
						</h1>
					</Link>
					<div class="text-xs opacity-70 hidden sm:flex">
						({routerData().rootData?.LatestVersion})
					</div>
				</div>

				<div class="flex nav-right gap-1 flex-wrap">
					<Link pattern="/*" splatValues={["docs"]} class="nav-item">
						Docs
					</Link>

					<Link pattern="/*" splatValues={["blog"]} class="nav-item">
						Blog
					</Link>

					<a
						href="https://github.com/vormadev/vorma"
						class="nav-item"
						target="_blank"
						rel="noreferrer"
						title="GitHub repository"
					>
						⭐ GitHub
					</a>

					<button
						type="button"
						title="Change theme"
						class="cursor-pointer nav-item"
						onClick={() => {
							if (theme() === "dark") {
								setTheme(THEMES.Light);
								return;
							}
							if (theme() === "light") {
								setTheme(THEMES.System);
								return;
							}
							if (theme() === "system") {
								setTheme(THEMES.Dark);
								return;
							}
						}}
					>
						<img
							src={theme_to_label_map[theme()]}
							alt="Theme icon"
							class="w-5 h-5"
						/>
					</button>
				</div>
			</nav>

			<main class="root-outlet-wrapper">
				<VormaRootOutlet />
			</main>

			<footer>
				<span>
					BSD-3-Clause license. Copyright (c) 2023–
					{new Date().getFullYear()} Samuel J. Cook.
				</span>
			</footer>
		</>
	);
}
