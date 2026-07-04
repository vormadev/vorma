//! Vorma build entry: production builds, the dev server, Vite integration, and
//! generated-output publication.
//!
//! # One door in
//!
//! This crate has exactly one thing an application ever calls: [`run`]. Every
//! other type and function compiled into this crate lives in a private
//! module and exists only so the build pipeline's own internals — production
//! generation, the dev loop, Vite process management, TypeScript contract
//! rendering — can share types across files; none of it is reachable from
//! outside the crate, and `cargo doc` for a downstream consumer would show
//! nothing but `run`. If you are building a Vorma app, this whole crate is a
//! single function call in a small `build.rs`-style binary:
//!
//! ```no_run
//! fn app_config() -> vorma::Result<vorma::AppConfig<()>> {
//!     Ok(vorma::AppConfig::default())
//! }
//!
//! fn main() {
//!     if let Err(error) = vorma_build::run(app_config) {
//!         eprintln!("{error}");
//!         std::process::exit(1);
//!     }
//! }
//! ```
//!
//! Run that binary with no arguments for a production build, or with a
//! single `dev` argument to start the dev server. See [`run`] for exactly
//! what each mode does. (This doctest is `no_run`: [`run`] reads real
//! process arguments and, depending on the app's actual project layout,
//! spawns `cargo` and binds real loopback ports — none of which belong in a
//! doc build.)

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
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
