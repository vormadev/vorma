package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p5_effects struct {
	broadcast_css_hot_reload            *tasks.Task[p5_batch_input, struct{}]
	notify_vite_public_file_map_changed *tasks.Task[p5_batch_input, struct{}]
	broadcast_revalidate                *tasks.Task[p5_batch_input, struct{}]
	broadcast_hard_reload               *tasks.Task[p5_batch_input, struct{}]
	publish_no_reload_needed_notice     *tasks.Task[p5_batch_input, struct{}]
	execute_terminal_browser_action     *tasks.Task[p5_batch_input, p5_completion_summary]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p5_effects_def = p5_effects{
	broadcast_css_hot_reload:            p5_broadcast_css_hot_reload_task,
	notify_vite_public_file_map_changed: p5_notify_vite_public_file_map_changed_task,
	broadcast_revalidate:                p5_broadcast_revalidate_task,
	broadcast_hard_reload:               p5_broadcast_hard_reload_task,
	publish_no_reload_needed_notice:     p5_publish_no_reload_needed_notice_task,
	execute_terminal_browser_action:     p5_execute_terminal_browser_action_task,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p5_broadcast_css_hot_reload_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p5_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P5_BROADCAST_CSS_HOT_RELOAD) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_notify_vite_public_file_map_changed_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p5_batch_input,
	) (struct{}, error) {
		if record_test_effect(
			tasks_ctx,
			_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_broadcast_revalidate_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p5_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P5_BROADCAST_REVALIDATE) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_broadcast_hard_reload_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p5_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P5_BROADCAST_HARD_RELOAD) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_publish_no_reload_needed_notice_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p5_batch_input,
	) (struct{}, error) {
		if record_test_effect(
			tasks_ctx,
			_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_execute_terminal_browser_action_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p5_batch_input,
	) (p5_completion_summary, error) {
		terminal_action := input.p5_requested_effects.terminal_browser_action
		switch terminal_action {
		case frontend_terminal_browser_action_hard_reload:
			if _, err := p5_broadcast_hard_reload_task.Run(
				tasks_ctx,
				input,
			); err != nil {
				return p5_completion_summary{}, err
			}
		case frontend_terminal_browser_action_notify_vite_public_file_map_changed:
			if _, err := p5_notify_vite_public_file_map_changed_task.Run(
				tasks_ctx,
				input,
			); err != nil {
				return p5_completion_summary{
					terminal_action:               frontend_terminal_browser_action_none,
					requires_backend_vite_healing: true,
				}, nil
			}
		case frontend_terminal_browser_action_revalidate:
			if _, err := p5_broadcast_revalidate_task.Run(
				tasks_ctx,
				input,
			); err != nil {
				return p5_completion_summary{}, err
			}
		case frontend_terminal_browser_action_css_hot_reload:
			if _, err := p5_broadcast_css_hot_reload_task.Run(
				tasks_ctx,
				input,
			); err != nil {
				return p5_completion_summary{}, err
			}
		default:
			if _, err := p5_publish_no_reload_needed_notice_task.Run(
				tasks_ctx,
				input,
			); err != nil {
				return p5_completion_summary{}, err
			}
		}

		return p5_completion_summary{terminal_action: terminal_action}, nil
	},
)
