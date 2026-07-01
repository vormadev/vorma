use axum::Router;
use axum::http::StatusCode;
use axum::http::header::CONTENT_TYPE;
use axum::response::IntoResponse;
use axum::routing::get;
use tower::ServiceBuilder;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

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
	let app = vorma::App::from_config(vorma_board_example::app_config()?)?;
	let listener = tokio::net::TcpListener::bind(addr)
		.await
		.map_err(|error| vorma::Error::new(format!("bind board example server: {error}")))?;
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
				ETags are useful for ordinary successful GET/HEAD responses. Board
				emits strong tags, caps buffering, and skips operational probes or
				caller-marked requests through the request metadata passed to `skip`.
				*/
				.layer(
					vorma::middleware::etag()
						.strong()
						.max_body_size(ETAG_MAX_BODY_SIZE)
						.skip(|request| {
							let health_check = request.method() == vorma::HttpMethod::GET
								&& request.uri().path() == HEALTHZ_PATH;
							let explicit_skip = request.headers().contains_key(SKIP_ETAG_HEADER);
							health_check || explicit_skip
						}),
				)
				.layer(vorma::middleware::response_body_timeout(60))
				.layer(vorma::middleware::compression())
				.layer(vorma::middleware::request_body_timeout(60))
				.layer(vorma::middleware::handler_timeout(60)),
		);

	tracing::info!(
		is_dev = running_in_dev,
		is_build = running_in_build,
		"Vorma board example listening on http://{addr}"
	);
	axum::serve(listener, router)
		.with_graceful_shutdown(shutdown_signal())
		.await
		.map_err(|error| vorma::Error::new(format!("serve board example server: {error}")))
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
