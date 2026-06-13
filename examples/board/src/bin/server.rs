use axum::Router;
use axum::http::StatusCode;
use axum::http::header::CONTENT_TYPE;
use axum::response::IntoResponse;
use axum::routing::get;
use tower::ServiceBuilder;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

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
	let app = vorma::App::from_config(vorma_board_example::app_config()?)?;
	let listener = tokio::net::TcpListener::bind(addr)
		.await
		.map_err(|error| vorma::Error::new(format!("bind board example server: {error}")))?;
	let router = Router::new()
		.route("/healthz", get(healthz))
		.route("/robots.txt", get(robots_txt))
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
				.layer(vorma::middleware::etag())
				.layer(vorma::middleware::response_body_timeout(60))
				.layer(vorma::middleware::compression())
				.layer(vorma::middleware::request_body_timeout(60))
				.layer(vorma::middleware::handler_timeout(60)),
		);

	tracing::info!("Vorma board example listening on http://{addr}");
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
