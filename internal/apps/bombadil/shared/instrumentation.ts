type ProbeRoute = {
	href: string;
	clientBuildID: string;
	matchPatterns: string[];
	errorSource: string | null;
	errorIndex: number | null;
	params: Record<string, string>;
};

type ProbeWork = {
	navigationHref: string | null;
	revalidationStatus: string | null;
	prefetchHref: string | null;
	submissionCount: number;
};

type Probe = {
	variant: string;
	route: ProbeRoute | null;
	work: ProbeWork | null;
	routeUpdates: number;
	workUpdates: number;
	lastReason: string | null;
	revalidate: () => Promise<unknown>;
};

type InitResult =
	| {
			ok: true;
	  }
	| {
			ok: false;
			err: string;
	  };

type RouteLike = {
	href: string;
	clientBuildID: string;
	matches: Array<{ pattern: string }>;
	error: null | { source: string; idx: number };
	params: Record<string, string>;
};

type WorkLike = {
	navigation: null | { href: string };
	revalidation: null | { status: string };
	prefetch: null | { href: string };
	submissions: unknown[];
};

declare global {
	interface Window {
		__vormaBombadil?: Probe;
	}
}

export async function install_vorma_probe(input: {
	variant: string;
	app: unknown;
	render: (args: { App: unknown; el: HTMLElement }) => void | Promise<void>;
}): Promise<void> {
	const app = input.app as {
		init: (options: {
			render: (args: {
				App: unknown;
				el: HTMLElement;
			}) => void | Promise<void>;
			onRouteUpdate: (
				route: RouteLike,
				previousRoute: RouteLike | null,
				reason: string,
			) => void;
			onWorkUpdate: (work: WorkLike) => void;
		}) => Promise<InitResult>;
		revalidate: () => Promise<unknown>;
		getRouteState: () => RouteLike;
		getWorkState: () => WorkLike;
	};
	const probe: Probe = {
		variant: input.variant,
		route: null,
		work: null,
		routeUpdates: 0,
		workUpdates: 0,
		lastReason: null,
		revalidate: app.revalidate,
	};
	window.__vormaBombadil = probe;

	const result = await app.init({
		render: input.render,
		onRouteUpdate: (route, _previous_route, reason) => {
			probe.route = serialize_route(route);
			probe.routeUpdates += 1;
			probe.lastReason = reason;
		},
		onWorkUpdate: (work) => {
			probe.work = serialize_work(work);
			probe.workUpdates += 1;
		},
	});
	if (!result.ok) {
		throw new Error(result.err);
	}
	probe.route = serialize_route(app.getRouteState());
	probe.work = serialize_work(app.getWorkState());
}

function serialize_route(route: RouteLike): ProbeRoute {
	return {
		href: route.href,
		clientBuildID: route.clientBuildID,
		matchPatterns: route.matches.map((match) => {
			return match.pattern;
		}),
		errorSource: route.error?.source ?? null,
		errorIndex: route.error?.idx ?? null,
		params: route.params,
	};
}

function serialize_work(work: WorkLike): ProbeWork {
	return {
		navigationHref: work.navigation?.href ?? null,
		revalidationStatus: work.revalidation?.status ?? null,
		prefetchHref: work.prefetch?.href ?? null,
		submissionCount: work.submissions.length,
	};
}
