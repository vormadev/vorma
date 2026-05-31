use std::future;
use std::io;
use std::process::ExitStatus;
use std::sync::Arc;

use tokio::process::Child;
use tokio::sync::watch;

use crate::build_cancel::BuildCancel;

#[derive(Debug)]
pub(crate) enum ChildWaitError {
	Wait(io::Error),
	Cancelled,
}

pub(crate) fn current_thread_runtime() -> Result<tokio::runtime::Runtime, io::Error> {
	tokio::runtime::Builder::new_current_thread()
		.enable_all()
		.build()
}

pub(crate) async fn wait_child_or_cancel(
	child: &mut Child,
	build_cancel: &Arc<BuildCancel>,
) -> Result<ExitStatus, ChildWaitError> {
	let mut cancel_rx = build_cancel.subscribe();
	tokio::select! {
		status = child.wait() => status.map_err(ChildWaitError::Wait),
		_ = wait_for_cancel(&mut cancel_rx) => {
			if let Some(process_id) = child.id() {
				crate::supervisor::force_kill(process_id);
			}
			let _ = child.kill().await;
			Err(ChildWaitError::Cancelled)
		}
	}
}

async fn wait_for_cancel(cancel_rx: &mut watch::Receiver<bool>) {
	loop {
		if *cancel_rx.borrow_and_update() {
			return;
		}
		if cancel_rx.changed().await.is_err() {
			future::pending::<()>().await;
		}
	}
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::path::{Path, PathBuf};
	use std::process::Command;
	use std::sync::Arc;
	use std::sync::atomic::Ordering;
	use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

	use super::*;
	use crate::supervisor::prepare_child_process;

	#[cfg(unix)]
	#[test]
	fn wait_child_or_cancel_kills_process_group() {
		let runtime = current_thread_runtime().unwrap();
		runtime.block_on(async {
			let pid_file = temp_file("process-group-child.pid");
			let mut command = Command::new("/bin/sh");
			command.arg("-c").arg(format!(
				"sleep 30 &\necho $! > \"{}\"\nwait\n",
				pid_file.to_string_lossy()
			));
			prepare_child_process(&mut command);
			let mut child = tokio::process::Command::from(command).spawn().unwrap();
			let sleep_pid = read_pid_file(&pid_file).await;
			let build_cancel = Arc::new(BuildCancel::new(false));

			build_cancel.store(true, Ordering::SeqCst);
			let err = wait_child_or_cancel(&mut child, &build_cancel)
				.await
				.unwrap_err();

			assert!(matches!(err, ChildWaitError::Cancelled));
			assert_process_exits(sleep_pid).await;
			let _ = fs::remove_file(pid_file);
		});
	}

	#[cfg(unix)]
	async fn read_pid_file(pid_file: &Path) -> i32 {
		let deadline = Instant::now() + Duration::from_secs(2);
		loop {
			if let Ok(contents) = fs::read_to_string(pid_file)
				&& let Ok(pid) = contents.trim().parse::<i32>()
			{
				return pid;
			}
			assert!(Instant::now() < deadline, "child pid file was not written");
			tokio::time::sleep(Duration::from_millis(10)).await;
		}
	}

	#[cfg(unix)]
	async fn assert_process_exits(pid: i32) {
		let deadline = Instant::now() + Duration::from_secs(2);
		while process_exists(pid) {
			assert!(
				Instant::now() < deadline,
				"process {pid} survived cancellation"
			);
			tokio::time::sleep(Duration::from_millis(10)).await;
		}
	}

	#[cfg(unix)]
	fn process_exists(pid: i32) -> bool {
		unsafe {
			if libc::kill(pid, 0) == 0 {
				return true;
			}
		}
		std::io::Error::last_os_error().raw_os_error() == Some(libc::EPERM)
	}

	fn temp_file(label: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		std::env::temp_dir().join(format!("vorma-{label}-{}-{nonce}", std::process::id()))
	}
}
