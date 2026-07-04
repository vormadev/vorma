//! Resource form-data input decoding.
//!
//! Use [`FormData`] as a `resource!`(app!) declaration's `input` type in place of a
//! typed struct when a resource needs to accept `application/x-www-form-urlencoded`,
//! `multipart/form-data` (including file uploads), or GET/HEAD query-string form input —
//! the framework parses the request into [`FormData`] before the handler runs, so a
//! [`ResourceCtx::input`](crate::ResourceCtx::input) call returns an already-decoded
//! [`FormData`] rather than the raw body. Reach for a typed `input` struct with
//! `#[derive(TsGen)]` instead whenever the shape is known ahead of time — `FormData`
//! exists for the specific cases a typed struct cannot express (mixed text/file
//! multipart submissions, or a form shape genuinely decided at runtime).

use bytes::Bytes;
use futures_util::stream;
use http::header::CONTENT_TYPE;
use http::{HeaderMap, Method};
use url::form_urlencoded;

impl crate::tsgen::Type for FormData {
	fn type_ref() -> crate::tsgen::TypeRef {
		crate::tsgen::TypeRef::FormData
	}
}

pub(crate) const DECODED_FORM_DATA_MISSING_MESSAGE: &str =
	"decoded route input did not contain FormData";

/// Text field parsed from a form request.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FormField {
	name: String,
	value: String,
}

impl FormField {
	/// Create a parsed form text field.
	pub(crate) fn new(name: impl Into<String>, value: impl Into<String>) -> Self {
		Self {
			name: name.into(),
			value: value.into(),
		}
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
	/// Create a parsed multipart file field.
	pub(crate) fn new(
		name: impl Into<String>,
		file_name: Option<String>,
		content_type: Option<String>,
		body: Bytes,
	) -> Self {
		Self {
			name: name.into(),
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
	/// Create parsed form data.
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

/// Form-data decoding error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FormDataDecodeError {
	/// Form-like content type header could not be parsed.
	InvalidFormContentType {
		/// Parser error message.
		message: String,
	},
	/// Non-GET/HEAD FormData input did not have a form content type.
	MissingFormContentType,
	/// Multipart boundary could not be parsed.
	InvalidMultipartBoundary {
		/// Parser error message.
		message: String,
	},
	/// Multipart body could not be parsed.
	InvalidMultipartBody {
		/// Parser error message.
		message: String,
	},
	/// Multipart field was missing a name.
	MultipartFieldMissingName,
	/// Multipart field name was empty.
	MultipartFieldEmptyName,
}

impl std::fmt::Display for FormDataDecodeError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidFormContentType { message } => {
				write!(f, "error parsing form Content-Type: {message}")
			}
			Self::MissingFormContentType => write!(
				f,
				"FormData input requires GET/HEAD query input or form content type"
			),
			Self::InvalidMultipartBoundary { message } => {
				write!(f, "error parsing multipart boundary: {message}")
			}
			Self::InvalidMultipartBody { message } => {
				write!(f, "error parsing multipart form: {message}")
			}
			Self::MultipartFieldMissingName => write!(f, "multipart form field missing name"),
			Self::MultipartFieldEmptyName => write!(f, "multipart form field name is empty"),
		}
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum FormContentType {
	UrlEncoded,
	Multipart,
}

/// Decode `FormData` from a request method/query/header/body tuple.
pub async fn decode_form_data(
	method: &Method,
	query: Option<&str>,
	headers: &HeaderMap,
	body: &Bytes,
) -> Result<FormData, FormDataDecodeError> {
	if *method == Method::GET || *method == Method::HEAD {
		return Ok(parse_query_form_data(query.unwrap_or_default()));
	}
	let raw_content_type = headers
		.get(CONTENT_TYPE)
		.and_then(|value| value.to_str().ok());
	if let Some(form_content_type) = raw_content_type
		.map(parse_form_content_type)
		.transpose()?
		.flatten()
	{
		let content_type = raw_content_type.expect("form content type parsed from raw header");
		return parse_body_form_data(form_content_type, content_type, body).await;
	}
	Err(FormDataDecodeError::MissingFormContentType)
}

fn parse_form_content_type(
	raw_content_type: &str,
) -> Result<Option<FormContentType>, FormDataDecodeError> {
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
		Err(error) if raw_content_type_is_form_like(raw_content_type) => {
			Err(FormDataDecodeError::InvalidFormContentType {
				message: error.to_string(),
			})
		}
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

async fn parse_body_form_data(
	form_content_type: FormContentType,
	content_type: &str,
	body: &Bytes,
) -> Result<FormData, FormDataDecodeError> {
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
) -> Result<FormData, FormDataDecodeError> {
	let boundary = multer::parse_boundary(content_type).map_err(|error| {
		FormDataDecodeError::InvalidMultipartBoundary {
			message: error.to_string(),
		}
	})?;
	let body_for_record = body.clone();
	let stream = stream::once(async move { Ok::<Bytes, std::io::Error>(body) });
	let mut multipart = multer::Multipart::new(stream, boundary);
	let mut fields = Vec::new();
	let mut files = Vec::new();
	while let Some(field) =
		multipart
			.next_field()
			.await
			.map_err(|error| FormDataDecodeError::InvalidMultipartBody {
				message: error.to_string(),
			})? {
		let name = field
			.name()
			.ok_or(FormDataDecodeError::MultipartFieldMissingName)?;
		if name.is_empty() {
			return Err(FormDataDecodeError::MultipartFieldEmptyName);
		}
		let name = name.to_owned();
		let file_name = field.file_name().map(ToOwned::to_owned);
		let content_type = field.content_type().map(ToString::to_string);
		if file_name.is_some() {
			let body =
				field
					.bytes()
					.await
					.map_err(|error| FormDataDecodeError::InvalidMultipartBody {
						message: error.to_string(),
					})?;
			files.push(FormFile::new(name, file_name, content_type, body));
		} else {
			let value =
				field
					.text()
					.await
					.map_err(|error| FormDataDecodeError::InvalidMultipartBody {
						message: error.to_string(),
					})?;
			fields.push(FormField::new(name, value));
		}
	}
	Ok(FormData::new(content_type, body_for_record, fields, files))
}

#[cfg(test)]
mod tests {
	use http::HeaderValue;

	use super::*;

	fn headers(content_type: &str) -> HeaderMap {
		let mut headers = HeaderMap::new();
		headers.insert(CONTENT_TYPE, HeaderValue::from_str(content_type).unwrap());
		headers
	}

	#[tokio::test]
	async fn form_data_decodes_get_query_fields() {
		let form = decode_form_data(
			&Method::GET,
			Some("tag=a&tag=b&name=ada"),
			&HeaderMap::new(),
			&Bytes::new(),
		)
		.await
		.unwrap();

		assert_eq!(form.content_type(), "");
		assert_eq!(form.text("name"), Some("ada"));
		assert_eq!(form.texts("tag").collect::<Vec<_>>(), ["a", "b"]);
	}

	#[tokio::test]
	async fn form_data_decodes_urlencoded_body() {
		let form = decode_form_data(
			&Method::POST,
			None,
			&headers("application/x-www-form-urlencoded"),
			&Bytes::from_static(b"tag=a&tag=b&name=ada"),
		)
		.await
		.unwrap();

		assert_eq!(form.content_type(), "application/x-www-form-urlencoded");
		assert_eq!(form.body(), &Bytes::from_static(b"tag=a&tag=b&name=ada"));
		assert_eq!(form.text("name"), Some("ada"));
		assert_eq!(form.texts("tag").collect::<Vec<_>>(), ["a", "b"]);
	}

	#[tokio::test]
	async fn form_data_decodes_multipart_fields_and_files() {
		let body = Bytes::from_static(
			b"--vorma\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\nHello\r\n--vorma\r\nContent-Disposition: form-data; name=\"upload\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nfile body\r\n--vorma--\r\n",
		);

		let form = decode_form_data(
			&Method::POST,
			None,
			&headers("multipart/form-data; boundary=vorma"),
			&body,
		)
		.await
		.unwrap();

		let file = form.file("upload").unwrap();
		assert_eq!(form.text("title"), Some("Hello"));
		assert_eq!(file.file_name(), Some("a.txt"));
		assert_eq!(file.content_type(), Some("text/plain"));
		assert_eq!(file.body(), &Bytes::from_static(b"file body"));
	}

	#[tokio::test]
	async fn form_data_rejects_missing_form_content_type_for_non_get() {
		let error = decode_form_data(&Method::POST, None, &HeaderMap::new(), &Bytes::new())
			.await
			.unwrap_err();

		assert!(matches!(error, FormDataDecodeError::MissingFormContentType));
	}

	#[tokio::test]
	async fn form_data_rejects_form_like_malformed_content_type() {
		let error = decode_form_data(
			&Method::POST,
			None,
			&headers("multipart/form-data; boundary=\"unterminated"),
			&Bytes::new(),
		)
		.await
		.unwrap_err();

		assert!(matches!(
			error,
			FormDataDecodeError::InvalidFormContentType { .. }
		));
	}
}
