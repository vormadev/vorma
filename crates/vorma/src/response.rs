use bytes::Bytes;
use cookie::Cookie;
use http::header::{CONTENT_TYPE, HeaderName, HeaderValue, LOCATION, SET_COOKIE};
use http::{HeaderMap, Response, StatusCode, Uri};

use crate::constants::X_VORMA_CLIENT_BUILD_ID;
use crate::head;

const CLIENT_REDIRECT_HEADER: HeaderName = HeaderName::from_static("x-client-redirect");
pub(crate) const CLIENT_ACCEPTS_REDIRECT_HEADER: &str = "X-Accepts-Client-Redirect";

#[derive(Clone, Debug, Eq, PartialEq)]
struct HeaderOp {
	op: HeaderOpKind,
	value: HeaderValue,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum HeaderOpKind {
	Set,
	Add,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct ResponseEffects {
	status: Option<StatusCode>,
	status_text: String,
	header_ops: Vec<(HeaderName, HeaderOp)>,
	cookies: Vec<Cookie<'static>>,
	head: Option<head::HeadBuilder>,
	location: String,
}

impl ResponseEffects {
	pub(crate) fn new() -> Self {
		Self::default()
	}

	pub(crate) fn set_status(&mut self, status: StatusCode, error_text: impl Into<Option<String>>) {
		self.status = Some(status);
		self.status_text = error_text.into().unwrap_or_default();
	}

	pub(crate) fn status(&self) -> (Option<StatusCode>, &str) {
		(self.status, &self.status_text)
	}

	pub(crate) fn set_header(&mut self, key: HeaderName, value: HeaderValue) {
		self.header_ops.push((
			key,
			HeaderOp {
				op: HeaderOpKind::Set,
				value,
			},
		));
	}

	pub(crate) fn add_header(&mut self, key: HeaderName, value: HeaderValue) {
		self.header_ops.push((
			key,
			HeaderOp {
				op: HeaderOpKind::Add,
				value,
			},
		));
	}

	pub(crate) fn header(&self, key: &HeaderName) -> Option<&HeaderValue> {
		self.headers(key).first().copied()
	}

	pub(crate) fn headers(&self, key: &HeaderName) -> Vec<&HeaderValue> {
		let mut values = Vec::new();
		for (op_key, op) in &self.header_ops {
			if op_key != key {
				continue;
			}
			match op.op {
				HeaderOpKind::Set => values = vec![&op.value],
				HeaderOpKind::Add => values.push(&op.value),
			}
		}
		values
	}

	pub(crate) fn set_cookie(&mut self, cookie: Cookie<'static>) {
		self.cookies.push(cookie);
	}

	pub(crate) fn cookies(&self) -> &[Cookie<'static>] {
		&self.cookies
	}

	pub(crate) fn merge_head(&mut self, b: &head::HeadBuilder) {
		if self.head.is_none() {
			self.head = Some(head::HeadBuilder::new());
		}
		self.head
			.as_mut()
			.expect("effects head should exist")
			.append(b);
	}

	pub(crate) fn head_builder(&mut self) -> &mut head::HeadBuilder {
		if self.head.is_none() {
			self.head = Some(head::HeadBuilder::new());
		}
		self.head.as_mut().expect("effects head should exist")
	}

	pub(crate) fn head_builder_ref(&self) -> Option<&head::HeadBuilder> {
		self.head.as_ref()
	}

	pub(crate) fn redirect(
		&mut self,
		accepts_client_redirect: bool,
		location: &str,
		code: Option<StatusCode>,
	) -> Result<bool, String> {
		let code = resolve_redirect_code(code)?;
		if accepts_client_redirect {
			self.client_redirect(location)?;
			return Ok(true);
		}
		self.server_redirect(location, code)?;
		Ok(false)
	}

	pub(crate) fn location(&self) -> &str {
		&self.location
	}

	pub(crate) fn is_error(&self) -> bool {
		self.status.is_some_and(is_error)
	}

	pub(crate) fn is_redirect(&self) -> bool {
		self.is_server_redirect() || self.is_client_redirect()
	}

	pub(crate) fn is_terminal_response(&self) -> bool {
		self.is_error() || self.is_redirect()
	}

	pub(crate) fn apply_header_ops_to(&self, headers: &mut HeaderMap) {
		for (key, op) in &self.header_ops {
			match op.op {
				HeaderOpKind::Set => {
					headers.remove(key);
					headers.append(key, op.value.clone());
				}
				HeaderOpKind::Add => {
					headers.append(key, op.value.clone());
				}
			}
		}
	}

	fn server_redirect(&mut self, location: &str, code: StatusCode) -> Result<(), String> {
		if !validate_url(location) {
			return Err(format!("invalid URL: {location}"));
		}
		if self.is_error() {
			return Ok(());
		}
		self.status = Some(code);
		self.location = location.to_owned();
		Ok(())
	}

	fn client_redirect(&mut self, location: &str) -> Result<(), String> {
		if !validate_url(location) {
			return Err(format!("invalid URL: {location}"));
		}
		if self.is_error() {
			return Ok(());
		}
		if self.status.is_none() {
			self.set_status(StatusCode::OK, None);
		}
		self.set_header(
			CLIENT_REDIRECT_HEADER,
			HeaderValue::from_str(location).map_err(|err| err.to_string())?,
		);
		Ok(())
	}

	fn is_server_redirect(&self) -> bool {
		self.status.is_some_and(is_server_redirect) && !self.location.is_empty()
	}

	fn is_client_redirect(&self) -> bool {
		self.header(&CLIENT_REDIRECT_HEADER).is_some()
	}
}

pub(crate) fn merge_response_effects(effects_list: &[Option<&ResponseEffects>]) -> ResponseEffects {
	let mut merged = ResponseEffects::new();
	let effects_list =
		&effects_list[..response_effects_prefix_through_first_terminal(effects_list)];

	for effects in effects_list.iter().flatten() {
		if let Some(head) = &effects.head {
			merged.merge_head(head);
		}
	}

	for effects in effects_list.iter().flatten() {
		merged.header_ops.extend(effects.header_ops.clone());
	}

	let mut unique_cookies = Vec::<(usize, Cookie<'static>)>::new();
	for (idx, effects) in effects_list.iter().enumerate() {
		let Some(effects) = effects else {
			continue;
		};
		for cookie in &effects.cookies {
			if let Some((existing_idx, existing_cookie)) = unique_cookies
				.iter_mut()
				.find(|(_, existing_cookie)| existing_cookie.name() == cookie.name())
			{
				*existing_idx = idx;
				*existing_cookie = cookie.clone();
			} else {
				unique_cookies.push((idx, cookie.clone()));
			}
		}
	}
	unique_cookies.sort_by_key(|(idx, _)| *idx);
	merged.cookies = unique_cookies
		.into_iter()
		.map(|(_, cookie)| cookie)
		.collect();

	let mut short_circuited = false;
	for effects in effects_list.iter().flatten() {
		if let Some(status) = effects.status
			&& is_error(status)
		{
			merged.status = Some(status);
			merged.status_text = effects.status_text.clone();
			short_circuited = true;
			break;
		}
		if effects.is_redirect() {
			merged.status = effects.status;
			merged.location = effects.location.clone();
			if effects.is_client_redirect()
				&& let Some(value) = effects.header(&CLIENT_REDIRECT_HEADER)
			{
				merged.set_header(CLIENT_REDIRECT_HEADER, value.clone());
			}
			short_circuited = true;
			break;
		}
	}

	if !short_circuited {
		for effects in effects_list.iter().flatten() {
			let Some(status) = effects.status else {
				continue;
			};
			if merged.status.is_none_or(|status| status.as_u16() < 300) {
				merged.status = Some(status);
				merged.status_text = effects.status_text.clone();
			}
		}
	}

	merged
}

fn response_effects_prefix_through_first_terminal(
	effects_list: &[Option<&ResponseEffects>],
) -> usize {
	for (idx, effects) in effects_list.iter().enumerate() {
		if let Some(effects) = effects
			&& effects.is_terminal_response()
		{
			return idx + 1;
		}
	}
	effects_list.len()
}

pub(crate) enum ResponseStatusPolicy {
	Apply,
	Suppress,
}

pub(crate) enum ResponsePlan<'a> {
	ShortCircuit {
		effects: &'a ResponseEffects,
	},
	Respond {
		effects: &'a ResponseEffects,
		response: Response<Bytes>,
		status_policy: ResponseStatusPolicy,
	},
}

pub(crate) fn finalize_response_plan(
	plan: ResponsePlan<'_>,
	expected_client_build_id: &str,
) -> Result<Response<Bytes>, String> {
	match plan {
		ResponsePlan::ShortCircuit { effects } => finalize_response_with_effects(
			effects,
			response_effects_short_circuit_response(effects)?,
			expected_client_build_id,
			ResponseStatusPolicy::Apply,
		),
		ResponsePlan::Respond {
			effects,
			response,
			status_policy,
		} => finalize_response_with_effects(
			effects,
			response,
			expected_client_build_id,
			status_policy,
		),
	}
}

fn response_effects_short_circuit_response(
	effects: &ResponseEffects,
) -> Result<Response<Bytes>, String> {
	let (status, status_text) = effects.status();
	let status = status.unwrap_or(StatusCode::INTERNAL_SERVER_ERROR);
	let mut response = if effects.is_error() {
		plain_text_response(status, Bytes::from(format!("{status_text}\n")))
	} else {
		Response::new(Bytes::new())
	};
	*response.status_mut() = status;
	Ok(response)
}

pub(crate) fn response_with_client_build_id(
	mut response: Response<Bytes>,
	expected_client_build_id: &str,
) -> Result<Response<Bytes>, String> {
	insert_header(
		response.headers_mut(),
		X_VORMA_CLIENT_BUILD_ID,
		expected_client_build_id,
	)?;
	Ok(response)
}

pub(crate) fn finalize_response_with_effects(
	effects: &ResponseEffects,
	mut response: Response<Bytes>,
	expected_client_build_id: &str,
	status_policy: ResponseStatusPolicy,
) -> Result<Response<Bytes>, String> {
	apply_response_effects_to_response_with_options(effects, &mut response, status_policy)?;
	insert_header(
		response.headers_mut(),
		X_VORMA_CLIENT_BUILD_ID,
		expected_client_build_id,
	)?;
	Ok(response)
}

fn apply_response_effects_to_response_with_options(
	effects: &ResponseEffects,
	response: &mut Response<Bytes>,
	status_policy: ResponseStatusPolicy,
) -> Result<(), String> {
	let (status, _) = effects.status();
	if matches!(status_policy, ResponseStatusPolicy::Apply)
		&& let Some(status) = status
	{
		*response.status_mut() = status;
	}

	effects.apply_header_ops_to(response.headers_mut());

	for cookie in effects.cookies() {
		let value = HeaderValue::from_str(&cookie.to_string()).map_err(|err| err.to_string())?;
		response.headers_mut().append(SET_COOKIE, value);
	}

	if effects.is_redirect() && !effects.is_error() && !effects.location().is_empty() {
		insert_header(response.headers_mut(), LOCATION, effects.location())?;
	}

	Ok(())
}

pub(crate) fn json_response(status: StatusCode, body: Bytes) -> Response<Bytes> {
	let mut response = Response::new(body);
	*response.status_mut() = status;
	response.headers_mut().insert(
		CONTENT_TYPE,
		HeaderValue::from_static("application/json; charset=utf-8"),
	);
	response
}

pub(crate) fn plain_text_response(status: StatusCode, body: Bytes) -> Response<Bytes> {
	let mut response = Response::new(body);
	*response.status_mut() = status;
	response.headers_mut().insert(
		CONTENT_TYPE,
		HeaderValue::from_static("text/plain; charset=utf-8"),
	);
	response
}

pub(crate) fn internal_server_error_response() -> Response<Bytes> {
	plain_text_response(
		StatusCode::INTERNAL_SERVER_ERROR,
		Bytes::from_static(b"Internal Server Error\n"),
	)
}

pub(crate) fn internal_server_error_with_client_build_id(
	expected_client_build_id: &str,
) -> Result<Response<Bytes>, String> {
	response_with_client_build_id(internal_server_error_response(), expected_client_build_id)
}

pub(crate) fn insert_header<K>(headers: &mut HeaderMap, key: K, value: &str) -> Result<(), String>
where
	K: TryInto<HeaderName>,
	K::Error: std::fmt::Display,
{
	let key = key.try_into().map_err(|err| err.to_string())?;
	let value = HeaderValue::from_str(value).map_err(|err| err.to_string())?;
	headers.insert(key, value);
	Ok(())
}

fn is_error(status: StatusCode) -> bool {
	status.as_u16() >= 400
}

fn is_server_redirect(status: StatusCode) -> bool {
	let status = status.as_u16();
	(300..400).contains(&status)
}

fn resolve_redirect_code(code: Option<StatusCode>) -> Result<StatusCode, String> {
	let code = code.unwrap_or(StatusCode::SEE_OTHER);
	if !is_server_redirect(code) {
		return Err(format!("redirect status must be 3xx, got {code}"));
	}
	Ok(code)
}

fn validate_url(location: &str) -> bool {
	if location.is_empty() {
		return false;
	}
	if location.starts_with("//") {
		return false;
	}
	if let Ok(url) = url::Url::parse(location) {
		return matches!(url.scheme(), "http" | "https");
	}
	let Ok(uri) = location.parse::<Uri>() else {
		return false;
	};
	uri.scheme_str().is_none() && uri.authority().is_none()
}

#[cfg(test)]
mod tests {
	use super::*;
	use http::header::CACHE_CONTROL;

	#[test]
	fn headers_follow_set_and_add_semantics() {
		let mut effects = ResponseEffects::new();
		effects.add_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("one"),
		);
		effects.add_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("two"),
		);
		assert_eq!(effects.headers(&HeaderName::from_static("x-test")).len(), 2);

		effects.set_header(
			HeaderName::from_static("x-test"),
			HeaderValue::from_static("three"),
		);
		assert_eq!(
			effects.header(&HeaderName::from_static("x-test")).unwrap(),
			HeaderValue::from_static("three")
		);
		assert_eq!(effects.headers(&HeaderName::from_static("x-test")).len(), 1);
	}

	#[test]
	fn redirects_validate_client_urls_and_resolve_codes() {
		let mut effects = ResponseEffects::new();

		assert!(effects.redirect(true, "", None).is_err());
		assert!(effects.redirect(true, "javascript:alert(1)", None).is_err());
		assert!(
			effects
				.redirect(true, "//example.com/target", None)
				.is_err()
		);
		assert!(effects.redirect(true, "/target", None).unwrap());
		assert!(
			effects
				.redirect(true, "https://example.com/target", None)
				.unwrap()
		);
		assert_eq!(effects.status().0, Some(StatusCode::OK));
		assert_eq!(
			effects.header(&CLIENT_REDIRECT_HEADER).unwrap(),
			HeaderValue::from_static("https://example.com/target")
		);

		let mut effects = ResponseEffects::new();
		assert!(
			effects
				.redirect(false, "javascript:alert(1)", None)
				.is_err()
		);
		assert!(
			effects
				.redirect(false, "//example.com/target", None)
				.is_err()
		);
		assert!(
			effects
				.redirect(false, "/target", Some(StatusCode::OK))
				.is_err()
		);
		assert!(
			effects
				.redirect(true, "/target", Some(StatusCode::OK))
				.is_err()
		);
		assert!(
			!effects
				.redirect(false, "/target", Some(StatusCode::FOUND))
				.unwrap()
		);
		assert_eq!(effects.status().0, Some(StatusCode::FOUND));
		assert_eq!(effects.location(), "/target");
	}

	#[test]
	fn client_redirect_does_not_override_existing_error_status() {
		let mut effects = ResponseEffects::new();
		effects.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));

		assert!(effects.redirect(true, "/login", None).unwrap());

		assert_eq!(effects.status().0, Some(StatusCode::FORBIDDEN));
		assert!(effects.header(&CLIENT_REDIRECT_HEADER).is_none());
	}

	#[test]
	fn merge_response_effects_uses_first_error_or_last_success() {
		let mut first = ResponseEffects::new();
		first.set_status(StatusCode::ACCEPTED, None);
		let mut second = ResponseEffects::new();
		second.set_status(StatusCode::CREATED, None);

		let merged = merge_response_effects(&[Some(&first), Some(&second)]);
		assert_eq!(merged.status().0, Some(StatusCode::CREATED));

		let mut error = ResponseEffects::new();
		error.set_status(StatusCode::NOT_FOUND, Some("missing".to_owned()));
		let mut later_error = ResponseEffects::new();
		later_error.set_status(StatusCode::INTERNAL_SERVER_ERROR, None);

		let merged = merge_response_effects(&[Some(&first), Some(&error), Some(&later_error)]);
		assert_eq!(merged.status().0, Some(StatusCode::NOT_FOUND));
		assert_eq!(merged.status().1, "missing");
	}

	#[test]
	fn merge_response_effects_no_status_effects_does_not_clear_success() {
		let empty = ResponseEffects::new();
		let mut success = ResponseEffects::new();
		success.set_status(StatusCode::CREATED, None);

		let merged = merge_response_effects(&[Some(&success), Some(&empty)]);
		assert_eq!(merged.status().0, Some(StatusCode::CREATED));

		let merged = merge_response_effects(&[Some(&empty), Some(&success)]);
		assert_eq!(merged.status().0, Some(StatusCode::CREATED));
	}

	#[test]
	fn merge_response_effects_uses_first_redirect_when_no_error() {
		let mut first = ResponseEffects::new();
		first
			.redirect(false, "/first", Some(StatusCode::FOUND))
			.unwrap();
		let mut second = ResponseEffects::new();
		second
			.redirect(false, "/second", Some(StatusCode::FOUND))
			.unwrap();

		let merged = merge_response_effects(&[Some(&first), Some(&second)]);

		assert_eq!(merged.status().0, Some(StatusCode::FOUND));
		assert_eq!(merged.location(), "/first");
	}

	#[test]
	fn merge_response_effects_uses_first_error_or_redirect_short_circuit() {
		let mut parent_redirect = ResponseEffects::new();
		parent_redirect
			.redirect(false, "/login", Some(StatusCode::FOUND))
			.unwrap();
		parent_redirect.set_header(
			HeaderName::from_static("x-parent"),
			HeaderValue::from_static("kept"),
		);
		let mut child_error = ResponseEffects::new();
		child_error.set_status(
			StatusCode::INTERNAL_SERVER_ERROR,
			Some("child failed".to_owned()),
		);
		child_error.set_header(
			HeaderName::from_static("x-child"),
			HeaderValue::from_static("suppressed"),
		);

		let merged = merge_response_effects(&[Some(&parent_redirect), Some(&child_error)]);

		assert_eq!(merged.status().0, Some(StatusCode::FOUND));
		assert_eq!(merged.location(), "/login");
		assert_eq!(
			merged.header(&HeaderName::from_static("x-parent")).unwrap(),
			HeaderValue::from_static("kept")
		);
		assert!(merged.header(&HeaderName::from_static("x-child")).is_none());

		let mut parent_error = ResponseEffects::new();
		parent_error.set_status(StatusCode::FORBIDDEN, Some("denied".to_owned()));
		parent_error.set_cookie(Cookie::build(("session", "parent")).build());
		let mut child_redirect = ResponseEffects::new();
		child_redirect
			.redirect(false, "/child", Some(StatusCode::FOUND))
			.unwrap();
		child_redirect.set_cookie(Cookie::build(("session", "child")).build());

		let merged = merge_response_effects(&[Some(&parent_error), Some(&child_redirect)]);

		assert_eq!(merged.status().0, Some(StatusCode::FORBIDDEN));
		assert_eq!(merged.status().1, "denied");
		assert_eq!(merged.location(), "");
		assert_eq!(merged.cookies().len(), 1);
		assert_eq!(merged.cookies()[0].value(), "parent");
	}

	#[test]
	fn merge_response_effects_suppresses_all_effects_after_first_terminal() {
		let mut prior_success = ResponseEffects::new();
		prior_success.set_header(
			HeaderName::from_static("x-prior"),
			HeaderValue::from_static("kept"),
		);
		prior_success.set_cookie(Cookie::build(("prior", "kept")).build());
		let mut terminal = ResponseEffects::new();
		terminal.redirect(true, "/client", Option::None).unwrap();
		terminal.set_header(
			HeaderName::from_static("x-terminal"),
			HeaderValue::from_static("kept"),
		);
		terminal.set_cookie(Cookie::build(("terminal", "kept")).build());
		let mut later_success = ResponseEffects::new();
		later_success.set_status(StatusCode::CREATED, Option::None);
		later_success.set_header(
			HeaderName::from_static("x-later"),
			HeaderValue::from_static("suppressed"),
		);
		later_success.set_cookie(Cookie::build(("later", "suppressed")).build());

		let merged =
			merge_response_effects(&[Some(&prior_success), Some(&terminal), Some(&later_success)]);

		assert_eq!(merged.status().0, Some(StatusCode::OK));
		assert_eq!(
			merged.header(&CLIENT_REDIRECT_HEADER).unwrap(),
			HeaderValue::from_static("/client")
		);
		assert_eq!(
			merged.header(&HeaderName::from_static("x-prior")).unwrap(),
			HeaderValue::from_static("kept")
		);
		assert_eq!(
			merged
				.header(&HeaderName::from_static("x-terminal"))
				.unwrap(),
			HeaderValue::from_static("kept")
		);
		assert!(merged.header(&HeaderName::from_static("x-later")).is_none());
		assert_eq!(merged.cookies().len(), 2);
		assert_eq!(merged.cookies()[0].name(), "prior");
		assert_eq!(merged.cookies()[1].name(), "terminal");
	}

	#[test]
	fn finalize_response_plan_short_circuit_applies_effects_once() {
		let mut effects = ResponseEffects::new();
		effects.set_status(StatusCode::CONFLICT, Some("drifted".to_owned()));
		effects.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));

		let response =
			finalize_response_plan(ResponsePlan::ShortCircuit { effects: &effects }, "build-id")
				.unwrap();

		assert_eq!(response.status(), StatusCode::CONFLICT);
		assert_eq!(
			response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
			HeaderValue::from_static("build-id")
		);
		assert_eq!(response.headers().get_all(CACHE_CONTROL).iter().count(), 1);
		assert_eq!(
			response.headers().get(CACHE_CONTROL).unwrap(),
			HeaderValue::from_static("no-store")
		);
		assert_eq!(response.body(), "drifted\n");
	}

	#[test]
	fn finalize_response_with_effects_protects_framework_client_build_id_header() {
		let mut effects = ResponseEffects::new();
		effects.set_header(
			HeaderName::from_static("x-vorma-client-build-id"),
			HeaderValue::from_static("handler-build-id"),
		);
		effects.add_header(
			HeaderName::from_static("x-vorma-client-build-id"),
			HeaderValue::from_static("extra-build-id"),
		);
		effects.set_header(CACHE_CONTROL, HeaderValue::from_static("no-store"));

		let response = finalize_response_with_effects(
			&effects,
			plain_text_response(StatusCode::OK, Bytes::new()),
			"framework-build-id",
			ResponseStatusPolicy::Apply,
		)
		.unwrap();

		assert_eq!(
			response.headers().get(X_VORMA_CLIENT_BUILD_ID).unwrap(),
			HeaderValue::from_static("framework-build-id")
		);
		assert_eq!(
			response
				.headers()
				.get_all(X_VORMA_CLIENT_BUILD_ID)
				.iter()
				.count(),
			1
		);
		assert_eq!(
			response.headers().get(CACHE_CONTROL).unwrap(),
			HeaderValue::from_static("no-store")
		);
	}

	#[test]
	fn merge_response_effects_dedupes_cookies_by_name_with_later_values() {
		let mut first = ResponseEffects::new();
		first.set_cookie(Cookie::build(("session", "old")).build());
		first.set_cookie(Cookie::build(("theme", "dark")).build());
		let mut second = ResponseEffects::new();
		second.set_cookie(Cookie::build(("session", "new")).build());

		let merged = merge_response_effects(&[Some(&first), Some(&second)]);

		assert_eq!(merged.cookies().len(), 2);
		assert_eq!(merged.cookies()[0].name(), "theme");
		assert_eq!(merged.cookies()[1].name(), "session");
		assert_eq!(merged.cookies()[1].value(), "new");
	}
}
