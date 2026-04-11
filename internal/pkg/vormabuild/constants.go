package vormabuild

import "time"

const live_state_mode_env_key = "__VORMA_LIVE_STATE_MODE"
const vite_plugin_go_port_env_key = "__VORMA_VITE_PLUGIN_GO_PORT"
const dev_loopback_host = "127.0.0.1"

const supervisor_shutdown_grace_period = 2 * time.Second
const supervisor_ready_timeout = 10 * time.Second

var base_watch_ignore_patterns = []string{"**/.git", "**/node_modules"}

const mailbox_key_go = "go"
const mailbox_key_static = "static"

const gitignore_content = `*
`
