use http_body_util::{BodyExt, Empty, StreamBody};
use hyper::body::{Bytes, Frame, Incoming};
use hyper_util::client::legacy::Client;
use hyper_util::rt::TokioExecutor;
use serde::Deserialize;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};
use tokio::sync::OnceCell;
use tokio::time::sleep;
use tokio_stream::StreamExt;
use vercel_runtime::{Error, Request, Response, ResponseBody, run, service_fn};

const TIMEOUT: Duration = Duration::from_secs(10);
const POLL: Duration = Duration::from_millis(25);
const PORT: u16 = 8080;

#[derive(Deserialize)]
struct Config {
    #[serde(rename = "Core")]
    core: CoreConfig,
    #[serde(rename = "Watch")]
    watch: WatchConfig,
}

#[derive(Deserialize)]
struct CoreConfig {
    #[serde(rename = "DistDir")]
    dist_dir: String,
}

#[derive(Deserialize)]
struct WatchConfig {
    #[serde(rename = "HealthcheckEndpoint")]
    healthcheck_endpoint: String,
}

type HttpConnector = hyper_util::client::legacy::connect::HttpConnector;

static CONFIG: OnceCell<Config> = OnceCell::const_new();
static GO: Mutex<Option<Child>> = Mutex::new(None);
static PROXY_CLIENT: OnceLock<Client<HttpConnector, Incoming>> = OnceLock::new();
static HEALTH_CLIENT: OnceLock<Client<HttpConnector, Empty<Bytes>>> = OnceLock::new();
static READY: AtomicBool = AtomicBool::new(false);
static INIT_LOCK: tokio::sync::Mutex<()> = tokio::sync::Mutex::const_new(());

async fn config() -> &'static Config {
    CONFIG
        .get_or_init(|| async {
            let data = std::fs::read_to_string("./backend/wave.config.json")
                .expect("failed to read wave.config.json");
            serde_json::from_str(&data).expect("failed to parse wave.config.json")
        })
        .await
}

fn proxy_client() -> &'static Client<HttpConnector, Incoming> {
    PROXY_CLIENT.get_or_init(|| Client::builder(TokioExecutor::new()).build_http())
}

fn health_client() -> &'static Client<HttpConnector, Empty<Bytes>> {
    HEALTH_CLIENT.get_or_init(|| Client::builder(TokioExecutor::new()).build_http())
}

fn kill_child() {
    let mut guard = GO.lock().unwrap_or_else(|e| e.into_inner());
    if let Some(mut child) = guard.take() {
        let _ = child.kill();
        let _ = child.wait();
    }
}

async fn ensure_ready() -> Result<(), String> {
    if READY.load(Ordering::Acquire) {
        return Ok(());
    }

    let _lock = INIT_LOCK.lock().await;

    // Double-check after acquiring lock
    if READY.load(Ordering::Acquire) {
        return Ok(());
    }

    kill_child();

    let cfg = config().await;
    let go_path = format!("./{}/main", cfg.core.dist_dir);
    let health = &cfg.watch.healthcheck_endpoint;
    let start = Instant::now();

    if std::fs::metadata(&go_path).is_err() {
        return Err(format!("go binary not found at {go_path}"));
    }

    let child = Command::new(&go_path)
        .env("PORT", PORT.to_string())
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit())
        .spawn()
        .map_err(|e| format!("spawn failed: {e}"))?;

    {
        let mut guard = GO.lock().unwrap_or_else(|e| e.into_inner());
        *guard = Some(child);
    }

    let uri: hyper::Uri = format!("http://127.0.0.1:{PORT}{health}").parse().unwrap();
    let deadline = Instant::now() + TIMEOUT;

    while Instant::now() < deadline {
        let req = hyper::Request::builder()
            .uri(uri.clone())
            .body(Empty::new())
            .unwrap();

        let is_healthy = health_client()
            .request(req)
            .await
            .map(|res| res.status().is_success())
            .unwrap_or(false);

        if is_healthy {
            READY.store(true, Ordering::Release);
            println!("[proxy] go ready in {:?}", start.elapsed());
            return Ok(());
        }
        sleep(POLL).await;
    }

    kill_child();
    Err("health check timed out".into())
}

fn is_hop_by_hop_header(name: &str) -> bool {
    matches!(
        name.to_lowercase().as_str(),
        "connection"
            | "keep-alive"
            | "proxy-authenticate"
            | "proxy-authorization"
            | "te"
            | "trailers"
            | "transfer-encoding"
            | "upgrade"
            | "host"
    )
}

async fn handler(req: Request) -> Result<Response<ResponseBody>, Error> {
    if let Err(e) = ensure_ready().await {
        return Ok(Response::builder()
            .status(503)
            .body(ResponseBody::from(e))?);
    }

    let path = req
        .uri()
        .path_and_query()
        .map(|pq| pq.as_str())
        .unwrap_or("/");
    let uri: hyper::Uri = format!("http://127.0.0.1:{PORT}{path}").parse().unwrap();

    let (parts, body) = req.into_parts();

    let mut builder = hyper::Request::builder().method(parts.method).uri(uri);
    for (k, v) in &parts.headers {
        if !is_hop_by_hop_header(k.as_str()) {
            builder = builder.header(k, v);
        }
    }

    match proxy_client().request(builder.body(body)?).await {
        Ok(res) => {
            let (parts, incoming) = res.into_parts();
            let mut response = Response::builder().status(parts.status);
            for (k, v) in &parts.headers {
                if !is_hop_by_hop_header(k.as_str()) {
                    response = response.header(k, v);
                }
            }

            let stream = incoming.into_data_stream().map(|result| {
                result
                    .map(Frame::data)
                    .map_err(|e| Error::from(e.to_string()))
            });

            Ok(response.body(ResponseBody::from(StreamBody::new(stream)))?)
        }
        Err(e) => {
            eprintln!("[proxy] backend unreachable: {e}");
            READY.store(false, Ordering::Release);
            panic!("backend connection failed: {e}");
        }
    }
}

fn shutdown() {
    kill_child();
    println!("[proxy] shutdown");
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    tokio::spawn(async {
        tokio::signal::ctrl_c().await.ok();
        shutdown();
        std::process::exit(0);
    });

    #[cfg(unix)]
    tokio::spawn(async {
        use tokio::signal::unix::{SignalKind, signal};
        if let Ok(mut sig) = signal(SignalKind::terminate()) {
            sig.recv().await;
            shutdown();
            std::process::exit(0);
        }
    });

    run(service_fn(handler)).await
}
