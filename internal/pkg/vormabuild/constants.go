package vormabuild

import "time"

const live_state_mode_env_key = "__VORMA_LIVE_STATE_MODE"
const vite_plugin_go_port_env_key = "__VORMA_VITE_PLUGIN_GO_PORT"

const supervisor_shutdown_grace_period = 2 * time.Second
const supervisor_ready_timeout = 10 * time.Second

var base_watch_ignore_patterns = []string{"**/.git", "**/node_modules"}

const mailbox_key_go = "go"
const mailbox_key_static = "static"

const gitignore_content = `*
`

const dev_app_server_preferred_port_env_key = "__VORMA_DEV_APP_SERVER_PREFERRED_PORT"
const dev_vite_server_preferred_port_env_key = "__VORMA_DEV_VITE_SERVER_PREFERRED_PORT"
const dev_loopback_host = "127.0.0.1"

const __test_panic_stage_env_key = "__VORMA_TEST_PANIC_STAGE"
const __test_panic_stage_after_initial_vite_start = "after-initial-vite-start"
const __test_panic_stage_during_go_refresh = "during-go-refresh"
const __test_panic_stage_before_vite_restart = "before-vite-restart"
const __test_panic_stage_after_vite_restart = "after-vite-restart"
const __test_panic_stage_async_after_initial_vite_start = "async-after-initial-vite-start"
