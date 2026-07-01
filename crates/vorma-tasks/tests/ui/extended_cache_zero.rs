use std::time::Duration;

type TestError = &'static str;

vorma_tasks::task! {
	static ZERO_TTL_TASK: vorma_tasks::Task<(), (), TestError> =
		extended_cache(Duration::ZERO, |_ctx, _input: ()| async move {
			Ok(())
		});
}

fn main() {}
