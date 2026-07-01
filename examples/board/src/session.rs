//! Demo auth: passwordless sessions in SQLite, token in an HttpOnly cookie.

use vorma::HttpCookie;

use crate::app;
use crate::repo::{self, DbInput, User};
use crate::store::AppState;
use crate::{CSRF_ECHO_HEADER, CSRF_HEADER};

pub const SESSION_COOKIE: &str = "board_session";

/// Random 128-bit hex token.
pub fn mint_token() -> vorma::Result<String> {
	let mut bytes = [0u8; 16];
	getrandom::fill(&mut bytes).map_err(|source| {
		vorma::Error::new("session token generation failed").with_source(source)
	})?;
	let mut token = String::with_capacity(32);
	for byte in bytes {
		use std::fmt::Write as _;
		#[expect(
			clippy::unwrap_used,
			reason = "writing hex digits into a String cannot fail"
		)]
		write!(&mut token, "{byte:02x}").unwrap();
	}
	Ok(token)
}

/// Read the session token from request headers.
pub fn token_of(headers: &vorma::HttpHeaderMap) -> Option<String> {
	let header = headers.get("cookie")?.to_str().ok()?;
	for piece in HttpCookie::split_parse(header.to_owned()) {
		let Ok(cookie) = piece else { continue };
		if cookie.name() == SESSION_COOKIE {
			return Some(cookie.value().to_owned());
		}
	}
	None
}

pub fn session_cookie(token: String) -> HttpCookie<'static> {
	HttpCookie::build((SESSION_COOKIE, token))
		.path("/")
		.http_only(true)
		.build()
}

pub fn removal_cookie() -> HttpCookie<'static> {
	HttpCookie::build((SESSION_COOKIE, ""))
		.path("/")
		.removal()
		.build()
}

/// Session user via the shared task: callers anywhere in the request
/// (middleware, layout, resources) pay for one database read total.
pub async fn current_user(
	state: &AppState,
	headers: &vorma::HttpHeaderMap,
	exec_ctx: &vorma::tasks::ExecCtx<vorma::Error>,
) -> vorma::Result<Option<User>> {
	let Some(token) = token_of(headers) else {
		return Ok(None);
	};
	let user = repo::USER_FOR_SESSION
		.run(
			exec_ctx,
			DbInput {
				db: std::sync::Arc::clone(&state.db),
				input: token,
			},
		)
		.await
		.map_err(|error| vorma::Error::new(error.to_string()))?;
	Ok((*user).clone())
}

/// Global preload: starts the session lookup before any handler runs.
/// Handlers that ask again get the deduped result.
pub fn current_user_preload() -> app::Middleware {
	app::Middleware::new(|ctx| async move {
		let _ = current_user(ctx.state(), ctx.request().headers(), ctx.exec_ctx())
			.await
			.map_err(|error| vorma::HttpExit::err(error.to_string()))?;
		Ok(())
	})
}

/// Method-scoped middleware for app-owned mutation headers.
///
/// Vorma does not prescribe CSRF; applications can use middleware filters to add their
/// own request policy. Board keeps this as an echo instead of real enforcement so the
/// example stays runnable without a production auth setup.
pub fn csrf_header_echo() -> app::Middleware {
	app::Middleware::new(|ctx| async move {
		if let Some(value) = ctx.request().headers().get(CSRF_HEADER) {
			ctx.response().set_header(
				vorma::HttpHeaderName::from_static(CSRF_ECHO_HEADER),
				value.clone(),
			);
		}
		Ok(())
	})
	.with_methods([vorma::HttpMethod::POST, vorma::HttpMethod::DELETE])
}

/// Gate everything under /mod: anonymous users are redirected home;
/// banned sessions get a real 401 (the JSON envelope on resources).
/// Scope patterns are URL patterns: the mod resources use concrete
/// `/api/mod/...` URLs, so the middleware declares that prefix too.
pub fn mod_gate() -> crate::app::Middleware {
	crate::app::Middleware::new(|ctx| async move {
		match current_user(ctx.state(), ctx.request().headers(), ctx.exec_ctx()).await {
			Ok(Some(user)) if user.banned => Err(vorma::HttpExit::err(format!(
				"banned user {} hit the mod area",
				user.username
			))
			.with_status(vorma::HttpStatusCode::UNAUTHORIZED)
			.with_client_msg("This account cannot access moderation.")),
			Ok(Some(_)) => Ok(()),
			Ok(None) => ctx.redirect("/"),
			Err(error) => Err(vorma::HttpExit::err(error.to_string())),
		}
	})
	.with_patterns(["/mod", "/mod/*", "/api/mod/*"])
}
