use std::future::Future;
use std::sync::Arc;

use vorma_tasks::{CancelToken, ExecCtx};

use super::error::Error;

#[derive(Clone)]
pub(in crate::mux) struct OrderedCancellation {
	later_tokens_by_index: Arc<Vec<Vec<CancelToken>>>,
}

impl OrderedCancellation {
	fn new(later_tokens_by_index: Vec<Vec<CancelToken>>) -> Self {
		Self {
			later_tokens_by_index: Arc::new(later_tokens_by_index),
		}
	}

	pub(in crate::mux) fn cancel_later(&self, index: usize) {
		if let Some(tokens) = self.later_tokens_by_index.get(index) {
			for token in tokens {
				token.cancel();
			}
		}
	}
}

pub(in crate::mux) struct OrderedTaskContexts<E> {
	contexts: Vec<OrderedTaskCtx<E>>,
	cancellation: OrderedCancellation,
}

impl<E> OrderedTaskContexts<E>
where
	E: Send + Sync + 'static,
{
	pub(in crate::mux) fn sibling_children(parent: &ExecCtx<E>, count: usize) -> Self {
		let exec_ctxs = (0..count).map(|_| parent.child()).collect::<Vec<_>>();
		let tokens = exec_ctxs
			.iter()
			.map(|exec_ctx| exec_ctx.cancel_token().clone())
			.collect::<Vec<_>>();
		let later_tokens_by_index = tokens
			.iter()
			.enumerate()
			.map(|(index, _)| tokens.iter().skip(index + 1).cloned().collect())
			.collect::<Vec<Vec<_>>>();
		Self::from_parts(exec_ctxs, later_tokens_by_index)
	}

	pub(in crate::mux) fn descendant_chain(parent: ExecCtx<E>, count: usize) -> Self {
		let mut current_exec_ctx = parent;
		let mut exec_ctxs = Vec::with_capacity(count);
		let mut later_tokens_by_index = Vec::with_capacity(count);
		for index in 0..count {
			let task_exec_ctx = current_exec_ctx.clone();
			let later_tokens = if index < count - 1 {
				let descendant_exec_ctx = current_exec_ctx.child();
				let token = descendant_exec_ctx.cancel_token().clone();
				current_exec_ctx = descendant_exec_ctx;
				vec![token]
			} else {
				Vec::new()
			};
			exec_ctxs.push(task_exec_ctx);
			later_tokens_by_index.push(later_tokens);
		}
		Self::from_parts(exec_ctxs, later_tokens_by_index)
	}

	pub(in crate::mux) fn cancellation(&self) -> OrderedCancellation {
		self.cancellation.clone()
	}

	pub(in crate::mux) fn into_contexts(self) -> Vec<OrderedTaskCtx<E>> {
		self.contexts
	}

	fn from_parts(
		exec_ctxs: Vec<ExecCtx<E>>,
		later_tokens_by_index: Vec<Vec<CancelToken>>,
	) -> Self {
		let cancellation = OrderedCancellation::new(later_tokens_by_index);
		let contexts = exec_ctxs
			.into_iter()
			.enumerate()
			.map(|(index, exec_ctx)| OrderedTaskCtx {
				index,
				exec_ctx,
				cancellation: cancellation.clone(),
			})
			.collect();
		Self {
			contexts,
			cancellation,
		}
	}
}

#[derive(Clone)]
pub(in crate::mux) struct OrderedTaskCtx<E> {
	index: usize,
	exec_ctx: ExecCtx<E>,
	cancellation: OrderedCancellation,
}

impl<E> OrderedTaskCtx<E> {
	pub(in crate::mux) fn index(&self) -> usize {
		self.index
	}

	pub(in crate::mux) fn exec_ctx(&self) -> ExecCtx<E> {
		self.exec_ctx.clone()
	}

	pub(in crate::mux) fn cancel_later(&self) {
		self.cancellation.cancel_later(self.index);
	}
}

pub(in crate::mux) async fn run_ordered_parallel<I, O, E, F, Fut>(
	items: Vec<I>,
	contexts: OrderedTaskContexts<E>,
	run_task: F,
) -> Result<OrderedParallelOutputs<O>, Error>
where
	I: Send + 'static,
	O: Send + 'static,
	E: Send + Sync + 'static,
	F: Fn(I, OrderedTaskCtx<E>) -> Fut + Clone + Send + Sync + 'static,
	Fut: Future<Output = O> + Send + 'static,
{
	let task_contexts = contexts.into_contexts();
	if items.len() != task_contexts.len() {
		return Err(Error::Invariant(format!(
			"ordered parallel item count {} does not match task context count {}",
			items.len(),
			task_contexts.len()
		)));
	}

	let mut handles = Vec::with_capacity(task_contexts.len());
	for (item, task_ctx) in items.into_iter().zip(task_contexts) {
		let run_task = run_task.clone();
		let index = task_ctx.index();
		handles.push(tokio::spawn(async move {
			let output = run_task(item, task_ctx).await;
			(index, output)
		}));
	}

	let mut outputs = Vec::with_capacity(handles.len());
	for handle in handles {
		outputs.push(
			handle
				.await
				.map_err(|error| Error::TaskJoin(error.to_string()))?,
		);
	}
	outputs.sort_by_key(|(index, _)| *index);
	Ok(OrderedParallelOutputs { outputs })
}

pub(in crate::mux) struct OrderedParallelOutputs<O> {
	outputs: Vec<(usize, O)>,
}

impl<O> OrderedParallelOutputs<O> {
	pub(in crate::mux) fn into_vec(self) -> Vec<(usize, O)> {
		self.outputs
	}
}
