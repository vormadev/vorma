use std::net::IpAddr;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use axum::extract::State;
use axum::extract::ws::{Message, WebSocket, WebSocketUpgrade};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use serde::Serialize;
use tokio::sync::mpsc;
use tokio::sync::mpsc::error::TrySendError;

use crate::dev_mux::DevMuxState;

#[derive(Clone, Debug)]
pub(crate) struct ClientManager {
	next_client_id: Arc<AtomicUsize>,
	clients: Arc<Mutex<std::collections::BTreeMap<usize, mpsc::Sender<RefreshPayload>>>>,
}

pub(crate) struct ClientSubscription {
	client_manager: ClientManager,
	client_id: usize,
	rx: mpsc::Receiver<RefreshPayload>,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub(crate) struct RefreshPayload {
	pub(crate) change_type: ChangeType,
	pub(crate) critical_css: String,
	pub(crate) build_error: String,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub(crate) enum ChangeType {
	#[serde(rename = "show_rebuilding_overlay")]
	ShowRebuildingOverlay,
	#[serde(rename = "hide_rebuilding_overlay")]
	HideRebuildingOverlay,
	#[serde(rename = "show_build_error")]
	ShowBuildError,
	#[serde(rename = "hard_reload")]
	HardReload,
	#[serde(rename = "update_critical_css")]
	UpdateCriticalCss,
	#[serde(rename = "revalidate_client")]
	ClientRevalidate,
}

impl ClientManager {
	pub(crate) fn new() -> Self {
		Self {
			next_client_id: Arc::new(AtomicUsize::new(1)),
			clients: Arc::new(Mutex::new(std::collections::BTreeMap::new())),
		}
	}

	pub(crate) fn broadcast(&self, msg: RefreshPayload) {
		let mut clients = self.clients.lock().expect("client_manager lock poisoned");
		let mut closed_client_ids = Vec::new();
		for (client_id, tx) in clients.iter() {
			match tx.try_send(msg.clone()) {
				Ok(()) => {}
				Err(TrySendError::Full(_)) => {
					closed_client_ids.push(*client_id);
				}
				Err(TrySendError::Closed(_)) => {
					closed_client_ids.push(*client_id);
				}
			}
		}
		for client_id in closed_client_ids {
			clients.remove(&client_id);
		}
	}

	pub(crate) fn add(&self) -> ClientSubscription {
		let (tx, rx) = mpsc::channel(4);
		let client_id = self.next_client_id.fetch_add(1, Ordering::SeqCst);
		self.clients
			.lock()
			.expect("client_manager lock poisoned")
			.insert(client_id, tx);
		ClientSubscription {
			client_manager: self.clone(),
			client_id,
			rx,
		}
	}

	fn remove(&self, client_id: usize) {
		self.clients
			.lock()
			.expect("client_manager lock poisoned")
			.remove(&client_id);
	}
}

impl Default for ClientManager {
	fn default() -> Self {
		Self::new()
	}
}

impl ClientSubscription {
	async fn recv(&mut self) -> Option<RefreshPayload> {
		self.rx.recv().await
	}

	#[cfg(test)]
	pub(crate) fn try_recv(&mut self) -> Result<RefreshPayload, mpsc::error::TryRecvError> {
		self.rx.try_recv()
	}
}

impl Drop for ClientSubscription {
	fn drop(&mut self) {
		self.client_manager.remove(self.client_id);
	}
}

pub(crate) fn dev_refresh_endpoint(dev_refresh_token: &str) -> String {
	format!("/vorma-dev-refresh-{dev_refresh_token}")
}

pub(crate) async fn dev_refresh_handler(
	ws: WebSocketUpgrade,
	headers: HeaderMap,
	State(dev_mux_state): State<DevMuxState>,
) -> Response {
	if !allow_dev_ws_origin(
		header_str(&headers, axum::http::header::HOST),
		header_str(&headers, axum::http::header::ORIGIN),
	) {
		return StatusCode::FORBIDDEN.into_response();
	}

	ws.on_upgrade(move |socket| dev_refresh_socket(socket, dev_mux_state))
}

async fn dev_refresh_socket(mut socket: WebSocket, dev_mux_state: DevMuxState) {
	let mut subscription = dev_mux_state.add_client();
	loop {
		tokio::select! {
			recv = socket.recv() => {
				if recv.is_none() {
					break;
				}
			}
			msg = subscription.recv() => {
				let Some(msg) = msg else {
					break;
				};
				let Ok(data) = serde_json::to_string(&msg) else {
					break;
				};
				let send = tokio::time::timeout(
					Duration::from_secs(5),
					socket.send(Message::Text(data.into())),
				)
				.await;
				if !matches!(send, Ok(Ok(()))) {
					break;
				}
			}
		}
	}
}

pub(crate) fn allow_dev_ws_origin(host: Option<&str>, origin: Option<&str>) -> bool {
	if !host.is_some_and(is_localhost) {
		return false;
	}
	let Some(origin) = origin else {
		return true;
	};
	let Ok(parsed) = url::Url::parse(origin) else {
		return false;
	};
	parsed.host_str().is_some_and(is_localhost)
}

fn header_str(headers: &HeaderMap, key: axum::http::header::HeaderName) -> Option<&str> {
	headers.get(key).and_then(|value| value.to_str().ok())
}

fn is_localhost(host: &str) -> bool {
	let host = split_host(host);
	if host.eq_ignore_ascii_case("localhost") {
		return true;
	}
	host.parse::<IpAddr>().is_ok_and(is_loopback_ip)
}

fn is_loopback_ip(ip: IpAddr) -> bool {
	match ip {
		IpAddr::V4(ip) => ip.is_loopback(),
		IpAddr::V6(ip) => {
			ip.is_loopback()
				|| ip
					.to_ipv4_mapped()
					.is_some_and(|mapped| mapped.is_loopback())
		}
	}
}

fn split_host(host: &str) -> &str {
	if let Some(rest) = host.strip_prefix('[')
		&& let Some((addr, _)) = rest.split_once(']')
	{
		return addr;
	}
	if let Some((addr, port)) = host.rsplit_once(':')
		&& host.matches(':').count() == 1
		&& port.chars().all(|c| c.is_ascii_digit())
	{
		return addr;
	}
	host
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn refresh_payload_serializes_with_snake_case_fields() {
		let json = serde_json::to_value(RefreshPayload {
			change_type: ChangeType::UpdateCriticalCss,
			critical_css: "body{}".to_owned(),
			build_error: String::new(),
		})
		.unwrap();

		assert_eq!(json["change_type"], "update_critical_css");
		assert_eq!(json["critical_css"], "body{}");
		assert_eq!(json["build_error"], "");
	}

	#[test]
	fn client_manager_disconnects_clients_that_cannot_keep_up() {
		let client_manager = ClientManager::new();
		let mut subscription = client_manager.add();
		for i in 0..8 {
			client_manager.broadcast(RefreshPayload {
				change_type: ChangeType::ShowBuildError,
				critical_css: String::new(),
				build_error: i.to_string(),
			});
		}

		let runtime = tokio::runtime::Builder::new_current_thread()
			.enable_all()
			.build()
			.unwrap();
		runtime.block_on(async move {
			for i in 0..4 {
				let next = subscription.recv().await.unwrap();
				assert_eq!(next.build_error, i.to_string());
			}
			assert!(subscription.recv().await.is_none());
		});
	}

	#[test]
	fn allow_dev_ws_origin_matches_localhost_rules() {
		assert!(allow_dev_ws_origin(Some("localhost:3000"), None));
		assert!(allow_dev_ws_origin(
			Some("127.0.0.1:3000"),
			Some("http://localhost:5173"),
		));
		assert!(!allow_dev_ws_origin(
			Some("example.com"),
			Some("http://localhost:5173"),
		));
		assert!(!allow_dev_ws_origin(
			Some("localhost:3000"),
			Some("https://example.com"),
		));
	}

	#[test]
	fn is_localhost_matches_ipv6_loopback_cases() {
		assert!(is_localhost("::1"));
		assert!(is_localhost("[::1]"));
		assert!(is_localhost("[::1]:8080"));
		assert!(is_localhost("0:0:0:0:0:0:0:1"));
		assert!(is_localhost("::ffff:127.0.0.1"));
		assert!(is_localhost("[::ffff:127.0.0.1]:8080"));
		assert!(!is_localhost("fe80::1"));
		assert!(!is_localhost("2001:db8::1"));
	}
}
