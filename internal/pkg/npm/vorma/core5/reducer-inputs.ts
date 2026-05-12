import type {
	APIFetchSettledInput,
	BeforeUnloadInput,
	BootInput,
	Effect,
	FocusInput,
	HMRRouteUpdateInput,
	InputEvent,
	PopstateInput,
	PublicNavigateInput,
	PublicPrefetchStartInput,
	PublicPrefetchStopInput,
	PublicRevalidateInput,
	PublicSubmitInput,
	PublicationCommittedInput,
	PublicationFailedInput,
	ReducerOutput,
	RefreshTimerFiredInput,
	RouteFetchSettledInput,
	RoutePreparationSettledInput,
} from "./events.ts";
import type { Model } from "./model.ts";
import type { AbortHandle } from "./platform.ts";
import {
	finalize,
	is_http_url,
	is_same_document,
	is_same_origin,
	normalize_url,
} from "./reducer-core.ts";
import {
	apply_hmr_route_update,
	apply_same_document_navigation,
	begin_api_submission,
	begin_boot,
	begin_cross_document_popstate,
	begin_navigation_fetch,
	begin_prefetch,
	cancel_prefetch_by_href,
	commit_publication,
	fail_publication,
	fire_refresh_timer,
	pickup_pending_boot_revalidations,
	schedule_refresh,
	settle_api_submission,
	settle_route_preparation,
	settle_route_response,
	start_pending_revalidation,
	supersede_active_route,
	take_deferred_api_redirect,
	try_merge_navigate_into_active,
	try_promote_prefetch,
} from "./transitions.ts";

export function reduce(
	model: Model,
	event: InputEvent,
	now_ms: number,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	switch (event.type) {
		case "boot":
			return reduce_boot(model, event, now_ms, abort_handle_factory);
		case "public_navigate":
			return reduce_public_navigate(
				model,
				event,
				now_ms,
				abort_handle_factory,
			);
		case "public_revalidate":
			return reduce_public_revalidate(model, event);
		case "public_submit":
			return reduce_public_submit(model, event, abort_handle_factory);
		case "public_prefetch_start":
			return reduce_prefetch_start(model, event, abort_handle_factory);
		case "public_prefetch_stop":
			return reduce_prefetch_stop(model, event);
		case "popstate":
			return reduce_popstate(model, event, abort_handle_factory);
		case "focus":
			return reduce_focus(model, event, abort_handle_factory);
		case "before_unload":
			return reduce_before_unload(model, event);
		case "route_fetch_settled":
			return reduce_route_fetch_settled(
				model,
				event,
				abort_handle_factory,
			);
		case "route_preparation_settled":
			return reduce_route_preparation_settled(model, event);
		case "api_fetch_settled":
			return reduce_api_fetch_settled(model, event, abort_handle_factory);
		case "refresh_timer_fired":
			return reduce_refresh_timer_fired(
				model,
				event,
				abort_handle_factory,
			);
		case "publication_committed":
			return reduce_publication_committed(
				model,
				event,
				now_ms,
				abort_handle_factory,
			);
		case "publication_failed":
			return reduce_publication_failed(model, event);
		case "hmr_route_update":
			return reduce_hmr_route_update(model, event);
	}
}

function finalize_or_skip(
	model: Model,
	effects: readonly Effect[],
): ReducerOutput<Model> {
	if (model.phase === "uninitialized") {
		return { model, effects };
	}
	return finalize(model, effects, model.work_indicator);
}

function no_op(model: Model): ReducerOutput<Model> {
	return finalize_or_skip(model, []);
}

function start_pending_refresh_if_idle(
	model: Model,
	effects: readonly Effect[],
	abort_handle_factory: () => AbortHandle,
): { model: Model; effects: Effect[] } {
	if (
		model.phase !== "ready" ||
		model.active_route ||
		model.refresh.kind !== "pending"
	) {
		return { model, effects: [...effects] };
	}
	const started = start_pending_revalidation(model, abort_handle_factory());
	if (!started) {
		return { model, effects: [...effects] };
	}
	return {
		model: started.model,
		effects: [...effects, ...started.effects],
	};
}

function reduce_boot(
	model: Model,
	event: BootInput,
	now_ms: number,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "uninitialized") {
		return no_op(model);
	}
	const t = begin_boot(
		{
			href: event.href,
			browser_key: event.browser_key,
			browser_state: event.browser_state,
			payload: event.payload,
			restored_scroll: event.restored_scroll,
			options: event.options,
			now_ms,
		},
		abort_handle_factory(),
	);
	return finalize(t.model, t.effects, t.model.work_indicator);
}

function reduce_public_navigate(
	model: Model,
	event: PublicNavigateInput,
	now_ms: number,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return finalize_or_skip(model, [
			{
				type: "settle_navigation_call",
				call_id: event.call_id,
				result: { didNavigate: false },
			},
		]);
	}

	const target = normalize_url(event.href, model.browser.href);
	if (!target) {
		return finalize(
			model,
			[
				{
					type: "settle_navigation_call",
					call_id: event.call_id,
					result: { didNavigate: false },
				},
			],
			model.work_indicator,
		);
	}

	if (!is_http_url(target) || !is_same_origin(target, model.browser.href)) {
		return finalize(
			model,
			[
				{ type: "hard_redirect", url: target },
				{
					type: "settle_navigation_call",
					call_id: event.call_id,
					result: { didNavigate: false },
				},
			],
			model.work_indicator,
		);
	}

	if (is_same_document(target, model.current.position.href)) {
		const sd = apply_same_document_navigation(model, {
			target_href: target,
			replace: event.replace,
			state: event.state,
			source: "navigate",
			popstate_browser_key: null,
			popstate_restored_scroll: undefined,
		});
		return finalize(
			sd.model,
			[
				...sd.effects,
				{
					type: "settle_navigation_call",
					call_id: event.call_id,
					result: { didNavigate: sd.did_navigate },
				},
			],
			model.work_indicator,
		);
	}

	const merged = try_merge_navigate_into_active(model, {
		target_href: target,
		replace: event.replace,
		scroll_to_top: event.scroll_to_top,
		state: event.state,
		skip_work_indicator: event.skip_work_indicator,
		call_id: event.call_id,
	});
	if (merged) {
		return finalize(merged.model, merged.effects, model.work_indicator);
	}

	const promoted = try_promote_prefetch(model, {
		target_href: target,
		replace: event.replace,
		scroll_to_top: event.scroll_to_top,
		state: event.state,
		skip_work_indicator: event.skip_work_indicator,
		call_id: event.call_id,
		source: "navigate",
	});
	if (promoted) {
		return finalize(promoted.model, promoted.effects, model.work_indicator);
	}

	const cleared = supersede_active_route(model);
	const fetched = begin_navigation_fetch(
		cleared.model,
		{
			target_href: target,
			replace: event.replace,
			scroll_to_top: event.scroll_to_top,
			state: event.state,
			skip_work_indicator: event.skip_work_indicator,
			source: "navigate",
			is_popstate: false,
			popstate_restored_scroll: undefined,
			public_calls: [event.call_id],
			redirect_count: 0,
		},
		abort_handle_factory(),
	);
	return finalize(
		fetched.model,
		[...cleared.effects, ...fetched.effects],
		model.work_indicator,
	);
}

function reduce_public_revalidate(
	model: Model,
	event: PublicRevalidateInput,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return finalize_or_skip(model, [
			{
				type: "settle_revalidate_call",
				call_id: event.call_id,
				result: { ok: true },
			},
		]);
	}
	const t = schedule_refresh(model, {
		reason: "manual",
		skip_work_indicator: false,
		waiter_call_id: event.call_id,
		debounce: true,
	});
	return finalize(t.model, t.effects, model.work_indicator);
}

function reduce_public_submit(
	model: Model,
	event: PublicSubmitInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase === "uninitialized") {
		return finalize_or_skip(model, [
			{
				type: "settle_submit_call",
				call_id: event.call_id,
				result: {
					success: false,
					error: "Vorma not booted",
					revalidation_call_id: null,
				},
			},
		]);
	}
	const should_revalidate =
		event.should_revalidate !== undefined
			? event.should_revalidate
			: event.route_kind === "mutation";
	const t = begin_api_submission(
		model,
		{
			href: event.href,
			method: event.method,
			route_kind: event.route_kind,
			dedupe_key: event.dedupe_key,
			should_revalidate,
			skip_work_indicator: event.skip_work_indicator,
			call_id: event.call_id,
			request_init: event.request_init,
		},
		abort_handle_factory(),
	);
	return finalize(t.model, t.effects, t.model.work_indicator);
}

function reduce_prefetch_start(
	model: Model,
	event: PublicPrefetchStartInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	const t = begin_prefetch(
		model,
		{ href: event.href },
		abort_handle_factory(),
	);
	return finalize(t.model, t.effects, model.work_indicator);
}

function reduce_prefetch_stop(
	model: Model,
	event: PublicPrefetchStopInput,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	const t = cancel_prefetch_by_href(model, event.href);
	return finalize(t.model, t.effects, model.work_indicator);
}

function reduce_popstate(
	model: Model,
	event: PopstateInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	if (
		event.browser_key === model.browser.key &&
		event.href === model.browser.href
	) {
		return no_op(model);
	}

	const save_effects: Effect[] = event.previous_scroll_save
		? [
				{
					type: "save_scroll",
					browser_key: event.previous_scroll_save.key,
					position: event.previous_scroll_save.position,
				},
			]
		: [];

	if (is_same_document(event.href, model.current.position.href)) {
		const sd = apply_same_document_navigation(
			{
				...model,
				browser: {
					href: event.href,
					key: event.browser_key,
					state: event.state,
				},
			},
			{
				target_href: event.href,
				replace: true,
				state: event.state,
				source: "popstate",
				popstate_browser_key: event.browser_key,
				popstate_restored_scroll: event.restored_scroll,
			},
		);
		return finalize(
			sd.model,
			[...save_effects, ...sd.effects],
			model.work_indicator,
		);
	}

	const t = begin_cross_document_popstate(
		{
			...model,
			browser: {
				href: event.href,
				key: event.browser_key,
				state: event.state,
			},
		},
		{
			target_href: event.href,
			state: event.state,
			browser_key: event.browser_key,
			restored_scroll: event.restored_scroll,
		},
		abort_handle_factory(),
	);
	return finalize(
		t.model,
		[...save_effects, ...t.effects],
		model.work_indicator,
	);
}

function reduce_focus(
	model: Model,
	event: FocusInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	const cfg = model.config.revalidate_on_focus;
	if (!cfg) {
		return no_op(model);
	}
	if (model.active_route || model.refresh.kind !== "idle") {
		return no_op(model);
	}
	if (Object.values(model.submissions).some((s) => !!s)) {
		return no_op(model);
	}
	if (event.now_ms - model.activity.last_activity_ms < cfg.stale_time_ms) {
		return no_op(model);
	}
	const t = schedule_refresh(model, {
		reason: "windowFocus",
		skip_work_indicator: cfg.skip_work_indicator,
		waiter_call_id: undefined,
		debounce: false,
	});
	const started = start_pending_refresh_if_idle(
		t.model,
		t.effects,
		abort_handle_factory,
	);
	return finalize(started.model, started.effects, model.work_indicator);
}

function reduce_before_unload(
	model: Model,
	event: BeforeUnloadInput,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	return finalize_or_skip(model, [
		{
			type: "write_reload_scroll",
			href: event.href,
			position: event.scroll,
			now_ms: event.now_ms,
		},
	]);
}

function reduce_route_fetch_settled(
	model: Model,
	event: RouteFetchSettledInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	const t = settle_route_response(
		model,
		{ token: event.token, outcome: event.outcome },
		abort_handle_factory,
	);
	return finalize(t.model, t.effects, model.work_indicator);
}

function reduce_route_preparation_settled(
	model: Model,
	event: RoutePreparationSettledInput,
): ReducerOutput<Model> {
	if (model.phase === "uninitialized") {
		return no_op(model);
	}
	const t = settle_route_preparation(model, {
		token: event.token,
		outcome: event.outcome,
	});
	return finalize(t.model, t.effects, model.work_indicator);
}

function reduce_api_fetch_settled(
	model: Model,
	event: APIFetchSettledInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase === "uninitialized") {
		return no_op(model);
	}
	const t = settle_api_submission(model, {
		token: event.token,
		outcome: event.outcome,
	});
	const started = start_pending_refresh_if_idle(
		t.model,
		t.effects,
		abort_handle_factory,
	);
	return finalize(started.model, started.effects, model.work_indicator);
}

function reduce_refresh_timer_fired(
	model: Model,
	event: RefreshTimerFiredInput,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	const fired = fire_refresh_timer(model, event.id);
	let next_model: Model = fired.model;
	let effects = [...fired.effects];
	const started = start_pending_refresh_if_idle(
		next_model,
		effects,
		abort_handle_factory,
	);
	next_model = started.model;
	effects = started.effects;
	return finalize(next_model, effects, model.work_indicator);
}

function reduce_publication_committed(
	model: Model,
	event: PublicationCommittedInput,
	now_ms: number,
	abort_handle_factory: () => AbortHandle,
): ReducerOutput<Model> {
	if (model.phase === "uninitialized") {
		return no_op(model);
	}
	const previous_indicator = model.work_indicator;
	const was_booting = model.phase === "booting";
	const pending_boot_revalidations = was_booting
		? model.pending_boot_revalidations
		: null;

	const t = commit_publication(model, { token: event.token, now_ms });
	let effects = [...t.effects];
	let next_model: Model = t.model;

	if (was_booting && next_model.phase === "ready") {
		const pickup = pickup_pending_boot_revalidations(
			next_model,
			pending_boot_revalidations,
		);
		next_model = pickup.model;
		effects = [...effects, ...pickup.effects];
	}

	if (next_model.phase === "ready") {
		const taken = take_deferred_api_redirect(next_model);
		if (taken.redirect) {
			next_model = taken.model;
			effects.push({
				type: "dispatch_navigation",
				href: taken.redirect.href,
				replace: true,
				state: undefined,
			});
		}
	}

	const started = start_pending_refresh_if_idle(
		next_model,
		effects,
		abort_handle_factory,
	);
	next_model = started.model;
	effects = started.effects;

	return finalize(next_model, effects, previous_indicator);
}

function reduce_publication_failed(
	model: Model,
	event: PublicationFailedInput,
): ReducerOutput<Model> {
	if (model.phase === "uninitialized") {
		return no_op(model);
	}
	const t = fail_publication(model, event.token, event.error);
	return finalize(t.model, t.effects, model.work_indicator);
}

function reduce_hmr_route_update(
	model: Model,
	event: HMRRouteUpdateInput,
): ReducerOutput<Model> {
	if (model.phase !== "ready") {
		return no_op(model);
	}
	const t = apply_hmr_route_update(model, event);
	return finalize(t.model, t.effects, model.work_indicator);
}
