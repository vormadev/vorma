//! Go-style benchmark harness: one line per benchmark, a detected
//! machine header, and nothing else — so terminal output, recorded
//! bench.results.txt files, and what a human scans are the same text.
//!
//! Timing model follows Go's testing.B: calibrate the iteration count
//! until one batch fills a target window, then time several batches and
//! report the median per-operation time.

#![deny(missing_docs)]
#![forbid(unsafe_code)]

use std::future::Future;
use std::time::{Duration, Instant};

// One calibration batch must reach this before measurement begins.
const CALIBRATION_TARGET: Duration = Duration::from_millis(200);
// Each measured batch aims for roughly this window.
const BATCH_TARGET: Duration = Duration::from_millis(300);
const MEASURED_BATCHES: usize = 5;

/// Benchmark session for one crate: prints the machine header on
/// creation, then one line per benchmark.
pub struct Bench {
	name_width: usize,
}

impl Bench {
	/// Start a session. Pass `env!("CARGO_PKG_NAME")`.
	pub fn new(crate_name: &str) -> Self {
		println!("os: {}", std::env::consts::OS);
		println!("arch: {}", std::env::consts::ARCH);
		println!("crate: {crate_name}");
		println!("cpu: {}", cpu_model());
		Self { name_width: 48 }
	}

	/// Benchmark a synchronous operation.
	pub fn bench(&mut self, name: &str, mut op: impl FnMut()) {
		self.bench_batches(name, |iters| {
			let start = Instant::now();
			for _ in 0..iters {
				op();
			}
			start.elapsed()
		});
	}

	/// Benchmark an asynchronous operation on the provided runtime.
	pub fn bench_async<Fut>(
		&mut self,
		name: &str,
		runtime: &tokio::runtime::Runtime,
		mut op: impl FnMut() -> Fut,
	) where
		Fut: Future,
	{
		self.bench_batches(name, |iters| {
			runtime.block_on(async {
				let start = Instant::now();
				for _ in 0..iters {
					op().await;
				}
				start.elapsed()
			})
		});
	}

	/// Benchmark with a caller-supplied batch runner: given an iteration
	/// count, run that many operations and return the elapsed time.
	pub fn bench_batches(&mut self, name: &str, mut run_batch: impl FnMut(u64) -> Duration) {
		// Calibrate: grow the iteration count until one batch fills the
		// calibration window. The growth runs double as warmup.
		let mut iters: u64 = 1;
		loop {
			let elapsed = run_batch(iters);
			if elapsed >= CALIBRATION_TARGET {
				let per_op = elapsed.as_nanos() as f64 / iters as f64;
				iters = ((BATCH_TARGET.as_nanos() as f64 / per_op) as u64).max(1);
				break;
			}
			let grown = if elapsed.is_zero() {
				iters.saturating_mul(100)
			} else {
				let scale =
					CALIBRATION_TARGET.as_nanos() as f64 * 1.2 / elapsed.as_nanos().max(1) as f64;
				let scaled = (iters as f64 * scale) as u64;
				scaled.clamp(iters.saturating_mul(2), iters.saturating_mul(100))
			};
			iters = grown.max(iters + 1);
		}

		// Measure: several batches at the calibrated count; report the
		// median so stray system noise cannot skew the recorded number.
		let mut samples = Vec::with_capacity(MEASURED_BATCHES);
		for _ in 0..MEASURED_BATCHES {
			let elapsed = run_batch(iters);
			samples.push(elapsed.as_nanos() as f64 / iters as f64);
		}
		samples.sort_by(|a, b| a.partial_cmp(b).expect("benchmark samples are finite"));
		let median = samples[samples.len() / 2];
		let total_iters = iters * MEASURED_BATCHES as u64;

		println!(
			"{name:<name_width$} {total_iters:>12} {} ns/op",
			format_ns(median),
			name_width = self.name_width,
		);
	}
}

// Go-style significant figures: more decimals as values shrink.
fn format_ns(ns: f64) -> String {
	if ns >= 1000.0 {
		format!("{ns:>10.0}")
	} else if ns >= 100.0 {
		format!("{ns:>10.1}")
	} else if ns >= 10.0 {
		format!("{ns:>10.2}")
	} else {
		format!("{ns:>10.3}")
	}
}

fn cpu_model() -> String {
	#[cfg(target_os = "macos")]
	{
		if let Ok(output) = std::process::Command::new("sysctl")
			.args(["-n", "machdep.cpu.brand_string"])
			.output() && output.status.success()
		{
			let model = String::from_utf8_lossy(&output.stdout).trim().to_owned();
			if !model.is_empty() {
				return model;
			}
		}
	}
	#[cfg(target_os = "linux")]
	{
		if let Ok(cpuinfo) = std::fs::read_to_string("/proc/cpuinfo") {
			for line in cpuinfo.lines() {
				if let Some(model) = line.strip_prefix("model name")
					&& let Some((_, value)) = model.split_once(':')
				{
					return value.trim().to_owned();
				}
			}
		}
	}
	"unknown".to_owned()
}
