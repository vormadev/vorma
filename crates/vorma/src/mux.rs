//! Internal typed route matching and task execution machinery.

mod api;
mod context;
mod error;
#[cfg(test)]
mod input;
mod middleware;
mod nested;
mod request;
mod task;

pub use api::{Options, Router, TaskRouteResult};
pub use context::{None, RequestCtx};
pub use error::{Error, InputError, RouteExecutionError};
#[cfg(test)]
pub use input::InputParser;
pub(crate) use middleware::Middleware;
pub use nested::{NestedOptions, NestedRouter, NestedTasksResults};
pub use request::RawRequest;
pub(crate) use task::erased_handler;
pub use vorma_matcher::Params;
