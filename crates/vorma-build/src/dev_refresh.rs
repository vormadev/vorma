//! Development browser refresh messages and client subscriptions.

use std::collections::BTreeMap;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};

use axum::extract::ws::{Message, WebSocket};
use serde::Serialize;
use tokio::sync::mpsc;
use tokio::sync::mpsc::error::TrySendError;
use url::Url;

use crate::generation_epoch::BrowserRefreshEffects;

/// Dev browser refresh WebSocket path prefix.
pub const DEV_REFRESH_EVENTS_PATH_PREFIX: &str = "/vorma-dev-refresh-";
const DEV_REFRESH_CLIENT_QUEUE_SIZE: usize = 4;

/// Browser refresh payload consumed by the injected dev refresh script.
#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub struct RefreshPayload {
	change_type: ChangeType,
	critical_css: String,
	build_error: String,
}

impl RefreshPayload {
	/// Create a refresh payload.
	pub fn new(
		change_type: ChangeType,
		critical_css: impl Into<String>,
		build_error: impl Into<String>,
	) -> Self {
		Self {
			change_type,
			critical_css: critical_css.into(),
			build_error: build_error.into(),
		}
	}

	/// Refresh action.
	pub fn change_type(&self) -> ChangeType {
		self.change_type
	}

	/// Replacement critical CSS when applicable.
	#[cfg(test)]
	pub fn critical_css(&self) -> &str {
		&self.critical_css
	}

	/// Build error text when applicable.
	#[cfg(test)]
	pub fn build_error(&self) -> &str {
		&self.build_error
	}
}

/// Browser refresh action consumed by the injected dev refresh script.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize)]
pub enum ChangeType {
	/// Show the rebuilding overlay.
	#[serde(rename = "show_rebuilding_overlay")]
	ShowRebuildingOverlay,
	/// Hide the rebuilding overlay.
	#[serde(rename = "hide_rebuilding_overlay")]
	HideRebuildingOverlay,
	/// Show a build error overlay.
	#[serde(rename = "show_build_error")]
	ShowBuildError,
	/// Reload the browser document.
	#[serde(rename = "hard_reload")]
	HardReload,
	/// Replace the critical CSS style element.
	#[serde(rename = "update_critical_css")]
	UpdateCriticalCss,
	/// Revalidate active client route data.
	#[serde(rename = "revalidate_client")]
	ClientRevalidate,
}

/// Derive browser refresh payloads from committed generation effects.
pub fn refresh_payloads_from_effects(
	effects: &BrowserRefreshEffects,
	critical_css: &str,
) -> Vec<RefreshPayload> {
	if effects.public_assets_changed() {
		return vec![RefreshPayload::new(ChangeType::HardReload, "", "")];
	}
	let mut messages = Vec::new();
	if effects.critical_css_changed() {
		messages.push(RefreshPayload::new(
			ChangeType::UpdateCriticalCss,
			critical_css,
			"",
		));
	}
	if effects.client_revalidate_required() {
		messages.push(RefreshPayload::new(ChangeType::ClientRevalidate, "", ""));
	}
	if messages.is_empty()
		|| effects.critical_css_changed() && !effects.client_revalidate_required()
	{
		messages.push(RefreshPayload::new(
			ChangeType::HideRebuildingOverlay,
			"",
			"",
		));
	}
	messages
}

/// Dev refresh client manager.
#[derive(Clone, Debug)]
pub struct DevRefreshClients {
	next_client_id: Arc<AtomicUsize>,
	clients: Arc<Mutex<BTreeMap<usize, mpsc::Sender<RefreshPayload>>>>,
}

impl DevRefreshClients {
	/// Create an empty client manager.
	pub fn new() -> Self {
		Self {
			next_client_id: Arc::new(AtomicUsize::new(1)),
			clients: Arc::new(Mutex::new(BTreeMap::new())),
		}
	}

	/// Broadcast one payload to connected clients.
	pub fn broadcast(&self, payload: RefreshPayload) {
		let mut clients = self
			.clients
			.lock()
			.expect("dev refresh clients lock poisoned");
		let mut closed_client_ids = Vec::new();
		for (client_id, tx) in clients.iter() {
			match tx.try_send(payload.clone()) {
				Ok(()) => {}
				Err(TrySendError::Full(_)) | Err(TrySendError::Closed(_)) => {
					closed_client_ids.push(*client_id);
				}
			}
		}
		for client_id in closed_client_ids {
			clients.remove(&client_id);
		}
	}

	/// Add one refresh client subscription.
	pub fn add_client(&self) -> DevRefreshClientSubscription {
		let (tx, rx) = mpsc::channel(DEV_REFRESH_CLIENT_QUEUE_SIZE);
		let client_id = self.next_client_id.fetch_add(1, Ordering::SeqCst);
		self.clients
			.lock()
			.expect("dev refresh clients lock poisoned")
			.insert(client_id, tx);
		DevRefreshClientSubscription {
			clients: self.clone(),
			client_id,
			rx,
		}
	}

	fn remove_client(&self, client_id: usize) {
		self.clients
			.lock()
			.expect("dev refresh clients lock poisoned")
			.remove(&client_id);
	}
}

impl Default for DevRefreshClients {
	fn default() -> Self {
		Self::new()
	}
}

/// One dev refresh client subscription.
pub struct DevRefreshClientSubscription {
	clients: DevRefreshClients,
	client_id: usize,
	rx: mpsc::Receiver<RefreshPayload>,
}

impl DevRefreshClientSubscription {
	/// Receive the next payload.
	pub async fn recv(&mut self) -> Option<RefreshPayload> {
		self.rx.recv().await
	}

	/// Try to receive one queued payload.
	#[cfg(test)]
	pub fn try_recv(&mut self) -> Result<RefreshPayload, mpsc::error::TryRecvError> {
		self.rx.try_recv()
	}
}

impl Drop for DevRefreshClientSubscription {
	fn drop(&mut self) {
		self.clients.remove_client(self.client_id);
	}
}

/// Allowed browser origin policy for the dev refresh WebSocket.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DevRefreshOriginPolicy {
	browser_origin_port: u16,
}

impl DevRefreshOriginPolicy {
	/// Create a browser-origin port policy.
	pub fn new(browser_origin_port: u16) -> Result<Self, DevRefreshError> {
		if browser_origin_port == 0 {
			return Err(DevRefreshError::InvalidBrowserOriginPort);
		}
		Ok(Self {
			browser_origin_port,
		})
	}

	/// Whether a browser Origin header is allowed to connect.
	pub fn allows_origin(&self, origin: &str) -> bool {
		let Ok(url) = Url::parse(origin) else {
			return false;
		};
		if !matches!(url.scheme(), "http" | "https") {
			return false;
		}
		if !matches!(url.host_str(), Some("127.0.0.1" | "localhost")) {
			return false;
		}
		url.port_or_known_default() == Some(self.browser_origin_port)
	}
}

/// Dev refresh server error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DevRefreshError {
	/// Browser origin port cannot be zero.
	InvalidBrowserOriginPort,
}

impl std::fmt::Display for DevRefreshError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidBrowserOriginPort => f.write_str("browser origin port cannot be zero"),
		}
	}
}

impl std::error::Error for DevRefreshError {}

pub(crate) fn dev_refresh_endpoint(dev_refresh_token: &str) -> String {
	format!("{DEV_REFRESH_EVENTS_PATH_PREFIX}{dev_refresh_token}")
}

pub(crate) async fn dev_refresh_socket(mut socket: WebSocket, clients: DevRefreshClients) {
	let mut subscription = clients.add_client();
	loop {
		tokio::select! {
			recv = socket.recv() => {
				if recv.is_none() {
					break;
				}
			}
			payload = subscription.recv() => {
				let Some(payload) = payload else {
					break;
				};
				let Ok(data) = serde_json::to_string(&payload) else {
					break;
				};
				if socket.send(Message::Text(data.into())).await.is_err() {
					break;
				}
			}
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::generation_epoch::GenerationEffects;

	/*
	The injected dev refresh script (crates/vorma/src/refresh_script.js) is
	hand-authored JS that switches on these wire strings; this pin fails if
	either side renames one without the other.
	*/
	#[test]
	fn refresh_script_handles_every_change_type_wire_string() {
		const REFRESH_SCRIPT: &str = include_str!(concat!(
			env!("CARGO_MANIFEST_DIR"),
			"/../vorma/src/refresh_script.js"
		));
		let change_types = [
			ChangeType::ShowRebuildingOverlay,
			ChangeType::HideRebuildingOverlay,
			ChangeType::ShowBuildError,
			ChangeType::HardReload,
			ChangeType::UpdateCriticalCss,
			ChangeType::ClientRevalidate,
		];
		for change_type in change_types {
			let wire = serde_json::to_value(change_type).unwrap();
			let literal = format!("\"{}\"", wire.as_str().unwrap());
			assert!(
				REFRESH_SCRIPT.contains(&literal),
				"refresh_script.js does not handle change type {literal}"
			);
		}
	}

	#[test]
	fn refresh_payloads_derive_from_committed_effects() {
		let reload = refresh_payloads_from_effects(
			&GenerationEffects {
				public_assets_changed: true,
				critical_css_changed: true,
				client_revalidate_required: true,
			}
			.browser_refresh_effects(1),
			"body{}",
		);
		let css_and_revalidate = refresh_payloads_from_effects(
			&GenerationEffects {
				public_assets_changed: false,
				critical_css_changed: true,
				client_revalidate_required: true,
			}
			.browser_refresh_effects(2),
			"body{}",
		);
		let css_only = refresh_payloads_from_effects(
			&GenerationEffects {
				public_assets_changed: false,
				critical_css_changed: true,
				client_revalidate_required: false,
			}
			.browser_refresh_effects(3),
			"body{}",
		);
		let quiet = refresh_payloads_from_effects(
			&GenerationEffects::default().browser_refresh_effects(4),
			"",
		);

		assert_eq!(reload[0].change_type(), ChangeType::HardReload);
		assert_eq!(
			css_and_revalidate[0].change_type(),
			ChangeType::UpdateCriticalCss
		);
		assert_eq!(css_and_revalidate[0].critical_css(), "body{}");
		assert_eq!(
			css_and_revalidate[1].change_type(),
			ChangeType::ClientRevalidate
		);
		assert_eq!(css_only[0].change_type(), ChangeType::UpdateCriticalCss);
		assert_eq!(css_only[0].critical_css(), "body{}");
		assert_eq!(css_only[1].change_type(), ChangeType::HideRebuildingOverlay);
		assert_eq!(quiet[0].change_type(), ChangeType::HideRebuildingOverlay);
	}

	#[test]
	fn refresh_clients_receive_broadcast_payloads() {
		let clients = DevRefreshClients::new();
		let mut first = clients.add_client();
		let mut second = clients.add_client();

		clients.broadcast(RefreshPayload::new(ChangeType::ClientRevalidate, "", ""));

		assert_eq!(
			first.try_recv().unwrap().change_type(),
			ChangeType::ClientRevalidate
		);
		assert_eq!(
			second.try_recv().unwrap().change_type(),
			ChangeType::ClientRevalidate
		);
	}

	#[test]
	fn refresh_origin_policy_accepts_only_app_loopback_origin() {
		let policy = DevRefreshOriginPolicy::new(3000).unwrap();

		assert!(policy.allows_origin("http://127.0.0.1:3000"));
		assert!(policy.allows_origin("http://localhost:3000"));
		assert!(!policy.allows_origin("http://127.0.0.1:3001"));
		assert!(!policy.allows_origin("https://example.com:3000"));
		assert!(!policy.allows_origin("not a url"));
	}
}
