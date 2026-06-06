use bytes::Bytes;
use http::{Extensions, HeaderMap, Method, Uri};
use percent_encoding::percent_decode_str;
use vorma_matcher::{Match, Params};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RouteMatch {
	method: Method,
	original_pattern: String,
	params: Params,
	splat_values: Vec<String>,
	head_fallback: bool,
}

impl RouteMatch {
	pub fn method(&self) -> &Method {
		&self.method
	}

	pub fn original_pattern(&self) -> &str {
		&self.original_pattern
	}

	pub fn params(&self) -> &Params {
		&self.params
	}

	pub fn splat_values(&self) -> &[String] {
		&self.splat_values
	}

	#[cfg(test)]
	pub(crate) fn is_head_fallback(&self) -> bool {
		self.head_fallback
	}
}

#[derive(Clone, Debug)]
pub struct RawRequest {
	method: Method,
	uri: Uri,
	decoded_path: String,
	headers: HeaderMap,
	body: Bytes,
	extensions: Extensions,
}

impl RawRequest {
	pub fn new(method: Method, uri: Uri, headers: HeaderMap, body: Bytes) -> Self {
		Self::try_new(method, uri, headers, body)
			.expect("request path should be valid UTF-8 after percent decoding")
	}

	pub fn try_new(
		method: Method,
		uri: Uri,
		headers: HeaderMap,
		body: Bytes,
	) -> Result<Self, String> {
		Self::try_with_extensions(method, uri, headers, body, Extensions::new())
	}

	pub fn with_extensions(
		method: Method,
		uri: Uri,
		headers: HeaderMap,
		body: Bytes,
		extensions: Extensions,
	) -> Self {
		Self::try_with_extensions(method, uri, headers, body, extensions)
			.expect("request path should be valid UTF-8 after percent decoding")
	}

	pub fn try_with_extensions(
		method: Method,
		uri: Uri,
		headers: HeaderMap,
		body: Bytes,
		extensions: Extensions,
	) -> Result<Self, String> {
		let decoded_path = percent_decode_str(uri.path())
			.decode_utf8()
			.map_err(|_| "request path must be valid UTF-8 after percent decoding".to_owned())?
			.into_owned();
		Ok(Self {
			method,
			uri,
			decoded_path,
			headers,
			body,
			extensions,
		})
	}

	pub fn get(uri: impl AsRef<str>) -> Self {
		let uri = uri.as_ref().parse::<Uri>().expect("valid request URI");
		Self::new(Method::GET, uri, HeaderMap::new(), Bytes::new())
	}

	pub fn method(&self) -> &Method {
		&self.method
	}

	pub fn uri(&self) -> &Uri {
		&self.uri
	}

	pub fn path(&self) -> &str {
		&self.decoded_path
	}

	pub fn query(&self) -> Option<&str> {
		self.uri.query()
	}

	pub fn headers(&self) -> &HeaderMap {
		&self.headers
	}

	pub fn body(&self) -> &Bytes {
		&self.body
	}

	pub fn extensions(&self) -> &Extensions {
		&self.extensions
	}

	pub fn extension<T>(&self) -> Option<&T>
	where
		T: Send + Sync + 'static,
	{
		self.extensions.get::<T>()
	}
}

pub(in crate::mux) fn route_match(method: Method, found: Match, head_fallback: bool) -> RouteMatch {
	RouteMatch {
		method,
		original_pattern: found.pattern.original_pattern().to_owned(),
		params: found.params,
		splat_values: found.splat_values,
		head_fallback,
	}
}
