use std::time::Duration;

use vorma_tasks::Task;

type TestError = &'static str;

fn main() {
	let _task: Task<(), (), TestError> = Task::new(Duration::ZERO, |_ctx, _input| async move {
		Ok(())
	});
}
