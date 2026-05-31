use url::form_urlencoded;

use crate::mux::RawRequest;

/// Read-only HTTP request view exposed to Vorma handlers.
#[derive(Clone, Copy, Debug)]
pub struct HttpRequest<'a> {
	inner: &'a RawRequest,
}

impl<'a> HttpRequest<'a> {
	pub(crate) fn new(inner: &'a RawRequest) -> Self {
		Self { inner }
	}

	/// HTTP method.
	pub fn method(&self) -> &http::Method {
		self.inner.method()
	}

	/// Full request URI.
	pub fn uri(&self) -> &http::Uri {
		self.inner.uri()
	}

	/// Request path.
	pub fn path(&self) -> &str {
		self.inner.path()
	}

	/// Raw query string without the leading `?`.
	pub fn query(&self) -> Option<&str> {
		self.inner.query()
	}

	/// Parsed query/search params.
	pub fn search_params(&self) -> HttpSearchParams<'_> {
		HttpSearchParams {
			query: self.inner.query().unwrap_or_default(),
		}
	}

	/// Request headers.
	pub fn headers(&self) -> &http::HeaderMap {
		self.inner.headers()
	}

	/// Collected request body bytes.
	pub fn body(&self) -> &bytes::Bytes {
		self.inner.body()
	}

	/// Request extensions.
	pub fn extensions(&self) -> &http::Extensions {
		self.inner.extensions()
	}

	/// Typed request extension lookup.
	pub fn extension<T>(&self) -> Option<&T>
	where
		T: Send + Sync + 'static,
	{
		self.inner.extension::<T>()
	}
}

/// Iterator-style view over URL query/search params.
#[derive(Clone, Copy, Debug)]
pub struct HttpSearchParams<'a> {
	query: &'a str,
}

impl<'a> HttpSearchParams<'a> {
	/// First decoded value for `name`.
	pub fn get(&self, name: &str) -> Option<String> {
		self.get_all(name).next()
	}

	/// All decoded values for `name`.
	pub fn get_all<'b>(&'b self, name: &'b str) -> impl Iterator<Item = String> + 'b {
		form_urlencoded::parse(self.query.as_bytes()).filter_map(move |(key, value)| {
			if key == name {
				return Some(value.into_owned());
			}
			None
		})
	}

	/// All decoded query params in source order.
	pub fn iter(&self) -> impl Iterator<Item = (String, String)> + '_ {
		form_urlencoded::parse(self.query.as_bytes())
			.map(|(key, value)| (key.into_owned(), value.into_owned()))
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn search_params_parse_repeated_and_encoded_query_values() {
		let raw = RawRequest::get("/items?tag=rust&tag=vorma&empty=&q=a%20b");
		let request = HttpRequest::new(&raw);
		let params = request.search_params();

		assert_eq!(params.get("tag"), Some("rust".to_owned()));
		assert_eq!(
			params.get_all("tag").collect::<Vec<_>>(),
			["rust".to_owned(), "vorma".to_owned()]
		);
		assert_eq!(params.get("empty"), Some(String::new()));
		assert_eq!(params.get("q"), Some("a b".to_owned()));
		assert_eq!(params.get("missing"), None);
	}
}
