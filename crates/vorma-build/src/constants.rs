use std::time::Duration;

pub(crate) const LIVE_STATE_MODE_ENV_KEY: &str = "__VORMA_LIVE_STATE_MODE";
pub(crate) const VITE_PLUGIN_SERVER_PORT_ENV_KEY: &str = "__VORMA_VITE_PLUGIN_SERVER_PORT";
pub(crate) const VITE_PLUGIN_SERVER_TOKEN_ENV_KEY: &str = "__VORMA_VITE_PLUGIN_SERVER_TOKEN";
pub(crate) const VITE_PLUGIN_TOKEN_HEADER: &str = "x-vorma-vite-plugin-token";

pub(crate) const SUPERVISOR_SHUTDOWN_GRACE_PERIOD: Duration = Duration::from_secs(2);
pub(crate) const SUPERVISOR_READY_TIMEOUT: Duration = Duration::from_secs(10);

pub(crate) const GITIGNORE_CONTENT: &str = "*\n";

pub(crate) const DEV_LOOPBACK_HOST: &str = "127.0.0.1";
