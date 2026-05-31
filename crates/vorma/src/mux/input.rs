use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use super::error::InputError;
use super::request::RawRequest;

type InputParseFuture<'a, I> = Pin<Box<dyn Future<Output = Result<I, InputError>> + Send + 'a>>;
type InputParseFn<I> = dyn for<'a> Fn(&'a RawRequest) -> InputParseFuture<'a, I> + Send + Sync;

pub struct InputParser<I> {
	parse: Arc<InputParseFn<I>>,
}

impl<I> Clone for InputParser<I> {
	fn clone(&self) -> Self {
		Self {
			parse: self.parse.clone(),
		}
	}
}

impl<I> InputParser<I>
where
	I: Send + 'static,
{
	#[cfg(test)]
	pub fn callback(
		parse: impl Fn(&RawRequest) -> Result<I, InputError> + Send + Sync + 'static,
	) -> Self {
		Self {
			parse: Arc::new(move |request| Box::pin(std::future::ready(parse(request)))),
		}
	}

	pub(crate) fn parse<'a>(&self, request: &'a RawRequest) -> InputParseFuture<'a, I> {
		(self.parse)(request)
	}
}

impl<I> InputParser<I>
where
	I: Default + Send + Sync + 'static,
{
	pub fn default_input() -> Self {
		Self {
			parse: Arc::new(|_| Box::pin(std::future::ready(Ok(I::default())))),
		}
	}
}
