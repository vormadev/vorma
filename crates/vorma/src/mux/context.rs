use std::collections::BTreeMap;
#[cfg(test)]
use std::sync::MutexGuard;
use std::sync::{Arc, Mutex};

use serde::Serialize;
use vorma_matcher::Params;
use vorma_tasks::ExecCtx;

use crate::error::Error as VormaError;
use crate::manifest::public_src_key;
use crate::response::ResponseEffects;

use super::request::RawRequest;

#[derive(Clone, Copy, Debug, Default, Eq, Hash, PartialEq, Serialize)]
pub struct None;

pub struct RequestCtx<S, E, I = None> {
	pub(in crate::mux) matched_pattern: String,
	pub(in crate::mux) params: Params,
	pub(in crate::mux) splat_values: Vec<String>,
	pub(in crate::mux) state: Arc<S>,
	pub(in crate::mux) exec_ctx: ExecCtx<E>,
	pub(in crate::mux) public_filemap: Arc<BTreeMap<String, String>>,
	pub(in crate::mux) response_effects: Arc<Mutex<ResponseEffects>>,
	pub(in crate::mux) request: RawRequest,
	pub(in crate::mux) input: I,
}

impl<S, E, I> RequestCtx<S, E, I> {
	pub fn matched_pattern(&self) -> &str {
		&self.matched_pattern
	}

	pub fn params(&self) -> &Params {
		&self.params
	}

	pub fn param(&self, key: &str) -> Option<&str> {
		self.params.get(key).map(String::as_str)
	}

	pub fn splat_values(&self) -> &[String] {
		&self.splat_values
	}

	pub fn state(&self) -> &S {
		&self.state
	}

	pub fn exec_ctx(&self) -> &ExecCtx<E> {
		&self.exec_ctx
	}

	pub fn public_url(&self, src_path: &str) -> std::result::Result<String, VormaError> {
		let clean = public_src_key(src_path)
			.ok_or_else(|| VormaError::runtime("public URL source path is empty"))?;
		self.public_filemap.get(clean).cloned().ok_or_else(|| {
			VormaError::runtime(format!(
				"file {src_path} not found in manifest public filemap"
			))
		})
	}

	#[cfg(test)]
	pub(crate) fn response_effects_mut(&self) -> MutexGuard<'_, ResponseEffects> {
		self.response_effects
			.lock()
			.expect("response effects lock poisoned")
	}

	pub(crate) fn response_effects(&self) -> Arc<Mutex<ResponseEffects>> {
		self.response_effects.clone()
	}

	pub fn request(&self) -> &RawRequest {
		&self.request
	}

	pub fn input(&self) -> &I {
		&self.input
	}

	pub(crate) fn with_input<J>(self, input: J) -> RequestCtx<S, E, J> {
		RequestCtx {
			matched_pattern: self.matched_pattern,
			params: self.params,
			splat_values: self.splat_values,
			state: self.state,
			exec_ctx: self.exec_ctx,
			public_filemap: self.public_filemap,
			response_effects: self.response_effects,
			request: self.request,
			input,
		}
	}
}

impl<S, E, I> Clone for RequestCtx<S, E, I>
where
	I: Clone,
{
	fn clone(&self) -> Self {
		Self {
			matched_pattern: self.matched_pattern.clone(),
			params: self.params.clone(),
			splat_values: self.splat_values.clone(),
			state: self.state.clone(),
			exec_ctx: self.exec_ctx.clone(),
			public_filemap: self.public_filemap.clone(),
			response_effects: self.response_effects.clone(),
			request: self.request.clone(),
			input: self.input.clone(),
		}
	}
}

pub(crate) struct RequestBase<S, E> {
	pub(in crate::mux) matched_pattern: String,
	pub(in crate::mux) params: Params,
	pub(in crate::mux) splat_values: Vec<String>,
	pub(in crate::mux) state: Arc<S>,
	pub(in crate::mux) exec_ctx: ExecCtx<E>,
	pub(in crate::mux) public_filemap: Arc<BTreeMap<String, String>>,
	pub(in crate::mux) response_effects: Arc<Mutex<ResponseEffects>>,
}

impl<S, E, I> RequestCtx<S, E, I> {
	pub(in crate::mux) fn clone_for_task<J>(
		&self,
		exec_ctx: ExecCtx<E>,
		input: J,
	) -> RequestCtx<S, E, J>
	where
		S: Send + Sync + 'static,
		E: Send + Sync + 'static,
		I: Send + Sync + 'static,
		J: Send + Sync + 'static,
	{
		RequestCtx {
			matched_pattern: self.matched_pattern.clone(),
			params: self.params.clone(),
			splat_values: self.splat_values.clone(),
			state: self.state.clone(),
			exec_ctx,
			public_filemap: self.public_filemap.clone(),
			response_effects: self.response_effects.clone(),
			request: self.request.clone(),
			input,
		}
	}
}
