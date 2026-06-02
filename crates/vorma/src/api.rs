use std::any::TypeId;
use std::future::Future;
use std::pin::Pin;

use bytes::Bytes;
use futures_util::stream;
use http::Method;
use http::header::CONTENT_TYPE;
use serde::de::DeserializeOwned;
use url::form_urlencoded;

use crate::constants::QUERY_KEY_VORMA_JSON;
use crate::mux::{InputError, RawRequest};
use crate::tsgen::{Type, TypeRef};

/////////////////////////////////////////////////////////////////////
/////// Form Data
/////////////////////////////////////////////////////////////////////

/// Text field parsed from a form request.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FormField {
	name: String,
	value: String,
}

impl FormField {
	pub(crate) fn new(name: String, value: String) -> Self {
		Self { name, value }
	}

	/// Field name from the submitted form.
	pub fn name(&self) -> &str {
		&self.name
	}

	/// Field text value.
	pub fn value(&self) -> &str {
		&self.value
	}
}

/// File field parsed from a multipart form request.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FormFile {
	name: String,
	file_name: Option<String>,
	content_type: Option<String>,
	body: Bytes,
}

impl FormFile {
	pub(crate) fn new(
		name: String,
		file_name: Option<String>,
		content_type: Option<String>,
		body: Bytes,
	) -> Self {
		Self {
			name,
			file_name,
			content_type,
			body,
		}
	}

	/// Form field name for this uploaded file.
	pub fn name(&self) -> &str {
		&self.name
	}

	/// Submitted filename, when present.
	pub fn file_name(&self) -> Option<&str> {
		self.file_name.as_deref()
	}

	/// Submitted content type, when present.
	pub fn content_type(&self) -> Option<&str> {
		self.content_type.as_deref()
	}

	/// Uploaded file bytes.
	pub fn body(&self) -> &Bytes {
		&self.body
	}

	/// Consume this file and return its bytes.
	pub fn into_body(self) -> Bytes {
		self.body
	}
}

/// Parsed form request body or query-string form input.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct FormData {
	content_type: String,
	body: Bytes,
	fields: Vec<FormField>,
	files: Vec<FormFile>,
}

impl FormData {
	pub(crate) fn new(
		content_type: impl Into<String>,
		body: Bytes,
		fields: Vec<FormField>,
		files: Vec<FormFile>,
	) -> Self {
		Self {
			content_type: content_type.into(),
			body,
			fields,
			files,
		}
	}

	/// Original request content type, or an empty string for query-string form input.
	pub fn content_type(&self) -> &str {
		&self.content_type
	}

	/// Original body bytes.
	pub fn body(&self) -> &Bytes {
		&self.body
	}

	/// Consume the form data and return the original body bytes.
	pub fn into_body(self) -> Bytes {
		self.body
	}

	/// Parsed text fields in request order.
	pub fn fields(&self) -> &[FormField] {
		&self.fields
	}

	/// Parsed file fields in request order.
	pub fn files(&self) -> &[FormFile] {
		&self.files
	}

	/// First text field with the given name.
	pub fn field(&self, name: &str) -> Option<&FormField> {
		self.fields.iter().find(|field| field.name == name)
	}

	/// First text value with the given field name.
	pub fn text(&self, name: &str) -> Option<&str> {
		self.field(name).map(FormField::value)
	}

	/// All text fields with the given name, in request order.
	pub fn fields_named<'a>(&'a self, name: &'a str) -> impl Iterator<Item = &'a FormField> + 'a {
		self.fields.iter().filter(move |field| field.name == name)
	}

	/// All text values with the given field name, in request order.
	pub fn texts<'a>(&'a self, name: &'a str) -> impl Iterator<Item = &'a str> + 'a {
		self.fields_named(name).map(FormField::value)
	}

	/// First file field with the given name.
	pub fn file(&self, name: &str) -> Option<&FormFile> {
		self.files.iter().find(|file| file.name == name)
	}

	/// All file fields with the given name, in request order.
	pub fn files_named<'a>(&'a self, name: &'a str) -> impl Iterator<Item = &'a FormFile> + 'a {
		self.files.iter().filter(move |file| file.name == name)
	}
}

impl Type for FormData {
	fn type_ref() -> TypeRef {
		TypeRef::raw("FormData")
	}
}

#[doc(hidden)]
pub type ResourceInputFuture<'a, I> =
	Pin<Box<dyn Future<Output = Result<I, InputError>> + Send + 'a>>;

#[doc(hidden)]
pub trait ViewInput: Sized + Send + Sync + 'static {
	#[doc(hidden)]
	fn parse_view_input(request: &RawRequest) -> Result<Self, InputError>;
}

impl<I> ViewInput for I
where
	I: DeserializeOwned + Send + Sync + 'static,
{
	fn parse_view_input(request: &RawRequest) -> Result<Self, InputError> {
		let query = view_input_query(request.query().unwrap_or(""));
		search_params_into_struct_from_query(&query)
	}
}

#[doc(hidden)]
pub trait ResourceInput: Sized + Send + Sync + 'static {
	#[doc(hidden)]
	fn parse_resource_input(request: &RawRequest) -> ResourceInputFuture<'_, Self>;
}

impl<I> ResourceInput for I
where
	I: DeserializeOwned + Send + Sync + 'static,
{
	fn parse_resource_input(request: &RawRequest) -> ResourceInputFuture<'_, Self> {
		Box::pin(async move { parse_serde_resource_input(request).await })
	}
}

impl ResourceInput for FormData {
	fn parse_resource_input(request: &RawRequest) -> ResourceInputFuture<'_, Self> {
		Box::pin(async move { parse_form_resource_input(request).await })
	}
}

pub(crate) fn parse_view_input<I>(request: &RawRequest) -> Result<I, InputError>
where
	I: ViewInput,
{
	I::parse_view_input(request)
}

pub(crate) async fn parse_resource_input<I>(request: &RawRequest) -> Result<I, InputError>
where
	I: ResourceInput,
{
	I::parse_resource_input(request).await
}

async fn parse_serde_resource_input<I>(request: &RawRequest) -> Result<I, InputError>
where
	I: DeserializeOwned + Send + Sync + 'static,
{
	if request.method() == Method::GET || request.method() == Method::HEAD {
		return search_params_into_struct(request);
	}

	let raw_content_type = request
		.headers()
		.get(CONTENT_TYPE)
		.and_then(|value| value.to_str().ok());
	if let Some(form_content_type) = raw_content_type
		.map(parse_form_content_type)
		.transpose()?
		.flatten()
	{
		let _ = form_content_type;
		return Err(InputError::bad_request(
			"form content type required FormData input",
		));
	}

	if request.body().is_empty() {
		return serde_json::from_value(serde_json::Value::Null).map_err(|err| {
			InputError::bad_request(format!("error decoding empty JSON input: {err}"))
		});
	}

	serde_json::from_slice(request.body())
		.map_err(|err| InputError::bad_request(format!("error decoding JSON: {err}")))
}

async fn parse_form_resource_input(request: &RawRequest) -> Result<FormData, InputError> {
	if request.method() == Method::GET || request.method() == Method::HEAD {
		return Ok(parse_query_form_data(request.query().unwrap_or_default()));
	}

	let raw_content_type = request
		.headers()
		.get(CONTENT_TYPE)
		.and_then(|value| value.to_str().ok());
	if let Some(form_content_type) = raw_content_type
		.map(parse_form_content_type)
		.transpose()?
		.flatten()
	{
		let content_type = raw_content_type.expect("form content type parsed from raw header");
		return parse_form_data(form_content_type, content_type, request.body()).await;
	}

	Err(InputError::bad_request(
		"FormData input requires GET/HEAD query input or form content type",
	))
}

pub(crate) fn search_params_into_struct<I>(request: &RawRequest) -> Result<I, InputError>
where
	I: DeserializeOwned + Send + Sync + 'static,
{
	search_params_into_struct_from_query(request.query().unwrap_or(""))
}

fn search_params_into_struct_from_query<I>(query: &str) -> Result<I, InputError>
where
	I: DeserializeOwned + Send + Sync + 'static,
{
	if TypeId::of::<I>() == TypeId::of::<()>() {
		return serde_json::from_value(serde_json::Value::Null)
			.map_err(|err| InputError::internal(format!("error constructing unit input: {err}")));
	}

	serde_urlencoded::from_str(query)
		.map_err(|err| InputError::bad_request(format!("error parsing URL parameters: {err}")))
}

fn view_input_query(raw_query: &str) -> std::borrow::Cow<'_, str> {
	let mut removed_internal_param = false;
	let mut serializer = form_urlencoded::Serializer::new(String::new());

	for (key, value) in form_urlencoded::parse(raw_query.as_bytes()) {
		if key == QUERY_KEY_VORMA_JSON {
			removed_internal_param = true;
			continue;
		}
		serializer.append_pair(&key, &value);
	}

	if !removed_internal_param {
		return std::borrow::Cow::Borrowed(raw_query);
	}
	std::borrow::Cow::Owned(serializer.finish())
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum FormContentType {
	UrlEncoded,
	Multipart,
}

fn parse_form_content_type(raw_content_type: &str) -> Result<Option<FormContentType>, InputError> {
	match raw_content_type.parse::<mime::Mime>() {
		Ok(content_type)
			if content_type.type_() == mime::APPLICATION
				&& content_type.subtype() == mime::WWW_FORM_URLENCODED =>
		{
			Ok(Some(FormContentType::UrlEncoded))
		}
		Ok(content_type)
			if content_type.type_() == mime::MULTIPART
				&& content_type.subtype() == mime::FORM_DATA =>
		{
			Ok(Some(FormContentType::Multipart))
		}
		Ok(_) => Ok(None),
		Err(error) if raw_content_type_is_form_like(raw_content_type) => Err(
			InputError::bad_request(format!("error parsing form Content-Type: {error}")),
		),
		Err(_) => Ok(None),
	}
}

fn raw_content_type_is_form_like(raw_content_type: &str) -> bool {
	let media_type = raw_content_type
		.split(';')
		.next()
		.unwrap_or_default()
		.trim()
		.to_ascii_lowercase();
	media_type == "application/x-www-form-urlencoded" || media_type == "multipart/form-data"
}

async fn parse_form_data(
	form_content_type: FormContentType,
	content_type: &str,
	body: &Bytes,
) -> Result<FormData, InputError> {
	match form_content_type {
		FormContentType::UrlEncoded => Ok(parse_urlencoded_form_data(content_type, body)),
		FormContentType::Multipart => parse_multipart_form_data(content_type, body.clone()).await,
	}
}

fn parse_urlencoded_form_data(content_type: &str, body: &Bytes) -> FormData {
	let fields = form_urlencoded::parse(body)
		.map(|(name, value)| FormField::new(name.into_owned(), value.into_owned()))
		.collect();
	FormData::new(content_type, body.clone(), fields, Vec::new())
}

fn parse_query_form_data(query: &str) -> FormData {
	let fields = form_urlencoded::parse(query.as_bytes())
		.map(|(name, value)| FormField::new(name.into_owned(), value.into_owned()))
		.collect();
	FormData::new("", Bytes::new(), fields, Vec::new())
}

async fn parse_multipart_form_data(
	content_type: &str,
	body: Bytes,
) -> Result<FormData, InputError> {
	let boundary = multer::parse_boundary(content_type).map_err(|error| {
		InputError::bad_request(format!("error parsing multipart boundary: {error}"))
	})?;
	let body_for_record = body.clone();
	let stream = stream::once(async move { Ok::<Bytes, std::io::Error>(body) });
	let mut multipart = multer::Multipart::new(stream, boundary);
	let mut fields = Vec::new();
	let mut files = Vec::new();

	while let Some(field) = multipart.next_field().await.map_err(|error| {
		InputError::bad_request(format!("error parsing multipart form: {error}"))
	})? {
		let name = field
			.name()
			.ok_or_else(|| InputError::bad_request("multipart form field missing name"))?;
		if name.is_empty() {
			return Err(InputError::bad_request(
				"multipart form field name is empty",
			));
		}
		let name = name.to_owned();
		let file_name = field.file_name().map(ToOwned::to_owned);
		let content_type = field.content_type().map(ToString::to_string);
		if file_name.is_some() {
			let body = field.bytes().await.map_err(|error| {
				InputError::bad_request(format!("error reading multipart file: {error}"))
			})?;
			files.push(FormFile::new(name, file_name, content_type, body));
		} else {
			let value = field.text().await.map_err(|error| {
				InputError::bad_request(format!("error reading multipart field: {error}"))
			})?;
			fields.push(FormField::new(name, value));
		}
	}

	Ok(FormData::new(content_type, body_for_record, fields, files))
}

#[cfg(test)]
mod tests {
	use http::{HeaderMap, HeaderValue, Uri};
	use serde::Deserialize;

	use super::*;

	#[derive(Debug, Deserialize, Eq, PartialEq)]
	struct SearchInput {
		q: String,
		page: u32,
	}

	#[derive(Debug, Deserialize, Eq, PartialEq)]
	struct JsonInput {
		email: String,
		count: u32,
	}

	fn request(
		method: Method,
		uri: &str,
		content_type: Option<&str>,
		body: &'static [u8],
	) -> RawRequest {
		let mut headers = HeaderMap::new();
		if let Some(content_type) = content_type {
			headers.insert(CONTENT_TYPE, HeaderValue::from_str(content_type).unwrap());
		}
		RawRequest::new(
			method,
			uri.parse::<Uri>().unwrap(),
			headers,
			Bytes::from_static(body),
		)
	}

	#[test]
	fn view_input_parses_url_search_params() {
		let request = RawRequest::get("/users?q=ada&page=2");
		let input: SearchInput = parse_view_input(&request).unwrap();

		assert_eq!(
			input,
			SearchInput {
				q: "ada".to_owned(),
				page: 2,
			}
		);
	}

	#[test]
	fn view_input_ignores_internal_json_request_param() {
		let request = RawRequest::get("/users?q=ada&vorma-json=current-build&page=2");
		let input: SearchInput = parse_view_input(&request).unwrap();

		assert_eq!(
			input,
			SearchInput {
				q: "ada".to_owned(),
				page: 2,
			}
		);

		let request = RawRequest::get("/users?vorma-json=current-build");
		parse_view_input::<()>(&request).unwrap();
	}

	#[test]
	fn view_unit_input_ignores_url_search_params() {
		let request = RawRequest::get("/slow?delay_ms=120");
		parse_view_input::<()>(&request).unwrap();
	}

	#[tokio::test]
	async fn resource_get_and_head_parse_url_search_params() {
		let get = RawRequest::get("/users?q=ada&page=2");
		let input: SearchInput = parse_resource_input(&get).await.unwrap();
		assert_eq!(input.page, 2);

		let head = request(Method::HEAD, "/users?q=grace&page=3", None, b"");
		let input: SearchInput = parse_resource_input(&head).await.unwrap();
		assert_eq!(
			input,
			SearchInput {
				q: "grace".to_owned(),
				page: 3,
			}
		);
	}

	#[tokio::test]
	async fn resource_get_unit_input_ignores_query_params() {
		let request = RawRequest::get("/count?delta=9");
		parse_resource_input::<()>(&request).await.unwrap();
	}

	#[tokio::test]
	async fn resource_non_get_decodes_json_body() {
		let request = request(
			Method::POST,
			"/users",
			Some("application/json"),
			br#"{"email":"jeff@example.com","count":4}"#,
		);
		let input: JsonInput = parse_resource_input(&request).await.unwrap();

		assert_eq!(
			input,
			JsonInput {
				email: "jeff@example.com".to_owned(),
				count: 4,
			}
		);
	}

	#[tokio::test]
	async fn resource_non_get_accepts_json_suffix_content_type() {
		let request = request(
			Method::PATCH,
			"/users",
			Some("application/vnd.api+json"),
			br#"{"email":"jeff@example.com","count":4}"#,
		);
		let input: JsonInput = parse_resource_input(&request).await.unwrap();

		assert_eq!(input.count, 4);
	}

	#[tokio::test]
	async fn resource_non_get_json_input_does_not_require_json_content_type() {
		for content_type in [None, Some("text/plain"), Some("application/json")] {
			let request = request(
				Method::POST,
				"/users",
				content_type,
				br#"{"email":"jeff@example.com","count":4}"#,
			);
			let input: JsonInput = parse_resource_input(&request).await.unwrap();

			assert_eq!(input.count, 4);
		}
	}

	#[tokio::test]
	async fn resource_non_get_json_input_ignores_malformed_non_form_content_type() {
		let request = request(
			Method::POST,
			"/users",
			Some("not a valid content type"),
			br#"{"email":"jeff@example.com","count":4}"#,
		);

		let input: JsonInput = parse_resource_input(&request).await.unwrap();

		assert_eq!(input.count, 4);
	}

	#[tokio::test]
	async fn api_form_like_malformed_content_type_is_bad_request() {
		let request = request(
			Method::POST,
			"/users",
			Some("multipart/form-data; boundary=\"unterminated"),
			br#"{"email":"jeff@example.com","count":4}"#,
		);

		let err = parse_resource_input::<JsonInput>(&request)
			.await
			.unwrap_err();

		assert!(err.is_bad_request());
		assert!(err.to_string().contains("error parsing form Content-Type"));
	}

	#[tokio::test]
	async fn api_empty_non_get_input_accepts_empty_body_without_content_type() {
		let request = request(Method::POST, "/empty", None, b"");
		parse_resource_input::<()>(&request).await.unwrap();
	}

	#[tokio::test]
	async fn api_invalid_json_is_bad_request() {
		let request = request(Method::POST, "/users", Some("application/json"), b"{");
		let err = parse_resource_input::<JsonInput>(&request)
			.await
			.unwrap_err();

		assert!(err.is_bad_request());
		assert!(err.to_string().contains("error decoding JSON"));
	}

	#[tokio::test]
	async fn api_form_content_type_requires_form_data() {
		let request = request(
			Method::POST,
			"/upload",
			Some("application/x-www-form-urlencoded"),
			b"",
		);
		let form: FormData = parse_resource_input(&request).await.unwrap();
		assert!(form.fields().is_empty());

		let err = parse_resource_input::<JsonInput>(&request)
			.await
			.unwrap_err();
		assert!(err.is_bad_request());
		assert_eq!(err.to_string(), "form content type required FormData input");
	}

	#[tokio::test]
	async fn form_data_parses_urlencoded_repeated_fields() {
		let request = request(
			Method::POST,
			"/form",
			Some("application/x-www-form-urlencoded"),
			b"tag=a&tag=b&name=jeff",
		);
		let form: FormData = parse_resource_input(&request).await.unwrap();

		assert_eq!(form.content_type(), "application/x-www-form-urlencoded");
		assert_eq!(form.body(), &Bytes::from_static(b"tag=a&tag=b&name=jeff"));
		assert_eq!(form.field("name").unwrap().value(), "jeff");
		assert_eq!(form.text("name"), Some("jeff"));
		assert_eq!(
			form.fields_named("tag")
				.map(FormField::value)
				.collect::<Vec<_>>(),
			vec!["a", "b"]
		);
		assert_eq!(form.texts("tag").collect::<Vec<_>>(), vec!["a", "b"]);
	}

	#[tokio::test]
	async fn form_data_parses_multipart_fields_and_files() {
		let body = concat!(
			"--vorma\r\n",
			"Content-Disposition: form-data; name=\"title\"\r\n",
			"\r\n",
			"Hello\r\n",
			"--vorma\r\n",
			"Content-Disposition: form-data; name=\"upload\"; filename=\"a.txt\"\r\n",
			"Content-Type: text/plain\r\n",
			"\r\n",
			"file body\r\n",
			"--vorma--\r\n",
		);
		let request = request(
			Method::POST,
			"/form",
			Some("multipart/form-data; boundary=vorma"),
			body.as_bytes(),
		);
		let form: FormData = parse_resource_input(&request).await.unwrap();
		let file = form.file("upload").unwrap();

		assert_eq!(form.field("title").unwrap().value(), "Hello");
		assert_eq!(file.file_name(), Some("a.txt"));
		assert_eq!(file.content_type(), Some("text/plain"));
		assert_eq!(file.body(), &Bytes::from_static(b"file body"));
	}

	#[tokio::test]
	async fn form_data_rejects_multipart_part_without_name() {
		let body = concat!(
			"--vorma\r\n",
			"Content-Disposition: form-data\r\n",
			"\r\n",
			"Hello\r\n",
			"--vorma--\r\n",
		);
		let request = request(
			Method::POST,
			"/form",
			Some("multipart/form-data; boundary=vorma"),
			body.as_bytes(),
		);

		let error = parse_resource_input::<FormData>(&request)
			.await
			.unwrap_err();

		assert!(error.is_bad_request());
		assert_eq!(error.to_string(), "multipart form field missing name");
	}

	#[tokio::test]
	async fn form_data_rejects_non_form_body_content_type() {
		let request = request(Method::POST, "/form", Some("application/json"), b"{}");
		let error = parse_resource_input::<FormData>(&request)
			.await
			.unwrap_err();

		assert!(
			error
				.to_string()
				.contains("FormData input requires GET/HEAD query input or form content type")
		);
	}

	#[tokio::test]
	async fn form_data_parses_query_for_get_and_head() {
		for method in [Method::GET, Method::HEAD] {
			let request = request(method, "/form?tag=a&tag=b", None, b"");
			let form: FormData = parse_resource_input(&request).await.unwrap();

			assert_eq!(form.content_type(), "");
			assert!(form.body().is_empty());
			assert_eq!(
				form.fields_named("tag")
					.map(FormField::value)
					.collect::<Vec<_>>(),
				vec!["a", "b"]
			);
		}
	}
}
