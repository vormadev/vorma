use axum::Router;
use axum::http::StatusCode;
use axum::http::header::CONTENT_TYPE;
use axum::response::IntoResponse;
use axum::routing::get;
use tower::ServiceBuilder;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;
use vorma::tasks::CancelToken;
use vorma_board_example::maintenance;

const HEALTHZ_PATH: &str = "/healthz";
const ROBOTS_TXT_PATH: &str = "/robots.txt";
const ETAG_MAX_BODY_SIZE: usize = 512 * 1024;
const SKIP_ETAG_HEADER: &str = "x-board-skip-etag";

#[tokio::main]
async fn main() {
	init_tracing();

	if let Err(error) = serve().await {
		tracing::error!("{error}");
		std::process::exit(1);
	}
}

/////////////////////////////////////////////////////////////////////
/////// Server
/////////////////////////////////////////////////////////////////////

async fn serve() -> vorma::Result<()> {
	let addr = vorma::bind_addr()?;
	let running_in_dev = vorma::is_dev();
	let running_in_build = vorma::is_build();
	let db = vorma_board_example::Db::open(&vorma_board_example::db_path())?;
	/*
	The slow-task observer instance is shared between the framework's own
	task runtime (wired below through `tasks_options`) and the background
	worker's independent runtime (spawned further down): one `TaskObserver`
	sees every task run in the process, whether it was asked for by a
	request handler or by a job that runs whether or not anyone is
	browsing the site.
	*/
	let observer = maintenance::slow_task_observer();
	let app_config = vorma_board_example::app_config_with(
		std::sync::Arc::clone(&db),
		vorma::tasks::TasksOptions {
			observer: Some(std::sync::Arc::clone(&observer)),
			..vorma::tasks::TasksOptions::default()
		},
	)?;
	let app = vorma::App::from_config(app_config)?;
	let listener = tokio::net::TcpListener::bind(addr)
		.await
		.map_err(|error| vorma::Error::new(format!("bind board example server: {error}")))?;

	/*
	The worker's shutdown token is this app's own cancellation wiring, built
	on the same `vorma_tasks::CancelToken` a request handler would see
	through its `ExecCtx` — but constructed and owned entirely by server
	code, because nothing here is a request. Cancelling it races the
	worker's current iteration to a stop at the same moment axum stops
	accepting connections.
	*/
	let worker_shutdown = CancelToken::new();
	let worker = tokio::spawn(maintenance::run_session_pruner(
		std::sync::Arc::clone(&db),
		worker_shutdown.clone(),
		observer,
	));
	let router = Router::new()
		.route(HEALTHZ_PATH, get(healthz))
		.route(ROBOTS_TXT_PATH, get(robots_txt))
		.fallback_service(app)
		.layer(
			ServiceBuilder::new()
				.layer(vorma::middleware::sensitive_headers())
				.layer(vorma::middleware::request_id())
				.layer(TraceLayer::new_for_http())
				.layer(vorma::middleware::secure_headers())
				.layer(vorma::middleware::panic_recovery())
				.layer(vorma::middleware::request_body_limit(
					vorma_board_example::REQUEST_BODY_LIMIT,
				))
				/*
				The response body timeout and etag are declared together
				here through `response_body_timeout_with_etag`, not as two
				separate `.layer(...)` calls, because their relative order
				is load-bearing and easy to get backwards. `etag` only tags
				a response when it can read an exact `size_hint` up front;
				the timeout layer's body wrapper (`tower_http`'s
				`TimeoutBody`) never forwards the wrapped body's
				`size_hint` — it always reports "unknown", no matter how
				small the real body is. A `ServiceBuilder` layer declared
				earlier is OUTER (sees each request first, each response
				last), so if `etag` were declared before (outer to) the
				timeout layer — the order this reads most naturally, and
				the order this file used before the footgun was found —
				then on the response path the timeout layer would run
				FIRST, permanently erasing the handler's real size hint,
				and `etag` would run SECOND, now blind to a body it could
				have tagged: every response would silently lose its ETag,
				with no warning of any kind.
				`response_body_timeout_with_etag` returns one pre-ordered
				layer with the timeout wrapper permanently outer and etag
				permanently inner, so that ordering mistake cannot happen
				through it. Reach for the two raw layers separately instead
				only when something else needs to sit between them in the
				chain — and if you do, declare the timeout layer first
				(outer) and `etag` second (inner), exactly as this helper
				composes them internally.
				*/
				/*
				ETags are useful for ordinary successful GET/HEAD responses.
				Board emits strong tags, caps buffering, and skips
				operational probes or caller-marked requests through the
				request metadata passed to `skip`.
				*/
				.layer(
					vorma::middleware::response_body_timeout_with_etag(60)
						.strong()
						.max_body_size(ETAG_MAX_BODY_SIZE)
						.skip(|request| {
							let health_check = request.method() == vorma::HttpMethod::GET
								&& request.uri().path() == HEALTHZ_PATH;
							let explicit_skip = request.headers().contains_key(SKIP_ETAG_HEADER);
							health_check || explicit_skip
						}),
				)
				.layer(vorma::middleware::compression())
				.layer(vorma::middleware::request_body_timeout(60))
				.layer(vorma::middleware::handler_timeout(60)),
		);

	tracing::info!(
		is_dev = running_in_dev,
		is_build = running_in_build,
		"Vorma board example listening on http://{addr}"
	);
	let serve_result = axum::serve(listener, router)
		.with_graceful_shutdown(async move {
			shutdown_signal().await;
			// Cancel the worker at the same moment axum stops accepting new
			// connections and starts draining in-flight ones, rather than
			// waiting for the drain to finish first.
			worker_shutdown.cancel();
		})
		.await
		.map_err(|error| vorma::Error::new(format!("serve board example server: {error}")));

	// The signal above already cancelled the worker; joining here confirms
	// its loop actually exited instead of leaving it detached at process
	// exit.
	if let Err(error) = worker.await {
		tracing::error!("session pruner task join failed: {error}");
	}

	serve_result
}

/////////////////////////////////////////////////////////////////////
/////// Platform Handlers
/////////////////////////////////////////////////////////////////////

async fn healthz() -> impl IntoResponse {
	(
		StatusCode::OK,
		[(CONTENT_TYPE, "text/plain; charset=utf-8")],
		"OK\n",
	)
}

async fn robots_txt() -> impl IntoResponse {
	(
		StatusCode::OK,
		[(CONTENT_TYPE, "text/plain; charset=utf-8")],
		"User-agent: *\nAllow: /\n",
	)
}

fn init_tracing() {
	let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));

	tracing_subscriber::fmt().with_env_filter(filter).init();
}

async fn shutdown_signal() {
	let ctrl_c = async {
		if let Err(error) = tokio::signal::ctrl_c().await {
			tracing::error!("failed to install Ctrl+C handler: {error}");
		}
	};

	#[cfg(unix)]
	let terminate = async {
		match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
			Ok(mut signal) => {
				signal.recv().await;
			}
			Err(error) => {
				tracing::error!("failed to install SIGTERM handler: {error}");
				std::future::pending::<()>().await;
			}
		}
	};

	#[cfg(not(unix))]
	let terminate = std::future::pending::<()>();

	tokio::select! {
		_ = ctrl_c => {},
		_ = terminate => {},
	}

	tracing::info!("shutdown signal received");
}
