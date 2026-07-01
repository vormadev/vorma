type TestError = &'static str;

vorma_tasks::task! {
	static WRONG_NAMESPACE: not_a_crate::Task<(), (), TestError> =
		memoized(|_ctx, ()| async move {
			Ok(())
		});
}

fn main() {}
