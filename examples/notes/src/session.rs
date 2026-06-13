//! Cookie-backed demo session + the middleware that gates `/admin`.

use vorma::{HttpCookie, HttpExit};

use crate::app;

pub(crate) const SESSION_COOKIE: &str = "notes_session";
pub(crate) const BANNED_SESSION: &str = "banned";

/// Read the demo session cookie from request headers.
pub(crate) fn session_of(headers: &vorma::HttpHeaderMap) -> Option<String> {
	let header = headers.get(http_cookie_header_name())?.to_str().ok()?;
	for piece in HttpCookie::split_parse(header.to_owned()) {
		let Ok(cookie) = piece else { continue };
		if cookie.name() == SESSION_COOKIE {
			return Some(cookie.value().to_owned());
		}
	}
	None
}

fn http_cookie_header_name() -> vorma::HttpHeaderName {
	vorma::HttpHeaderName::from_static("cookie")
}

/// Gate `/admin`: anonymous users are redirected home; banned sessions get
/// a real 401 (resources under /admin would receive the JSON envelope).
pub(crate) fn session_gate() -> app::Middleware {
	app::Middleware::new(|ctx| async move {
		match session_of(ctx.request().headers()).as_deref() {
			None => ctx.redirect("/"),
			Some(BANNED_SESSION) => Err(HttpExit::err("banned session hit /admin")
				.with_status(vorma::HttpStatusCode::UNAUTHORIZED)
				.with_client_msg("This session is not allowed here.")),
			Some(_) => Ok(()),
		}
	})
	.with_patterns(["/admin", "/admin/*"])
}
