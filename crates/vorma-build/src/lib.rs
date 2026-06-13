//! Vorma build entry: production builds, the dev server, Vite integration, and
//! generated-output publication.

#![deny(missing_docs)]
#![forbid(unsafe_code)]

mod app_server_build;
mod app_server_process;
mod build_output;
mod build_plan;
mod dev_build;
mod dev_generation;
mod dev_mux;
mod dev_refresh;
mod dev_signal;
mod dev_vite;
mod dev_watcher;
mod entrypoint;
mod generation_epoch;
mod generation_inputs;
mod live_state;
mod live_state_command;
mod output_lock;
mod process_ready;
mod process_runner;
mod production_build;
mod production_generation;
mod production_vite;
mod projection_compiler;
mod static_outputs;
#[cfg(test)]
mod test_support;
mod tokens;
mod typescript_contracts;
mod vite_command;
mod vite_manifest;
mod vite_plugin_contract;
mod vite_plugin_control;
mod vite_plugin_rpc;
#[cfg(test)]
mod vite_plugin_ts_contracts;
#[cfg(test)]
mod wire_ts_contracts;

pub use entrypoint::run;
