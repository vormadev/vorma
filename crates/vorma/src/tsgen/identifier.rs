use super::{Error, Result};

pub(super) fn validate_ts_identifier(name: &str, label: &str) -> Result<()> {
	if name.is_empty() {
		return Err(Error::new(format!("{label} cannot be empty")));
	}
	for (index, ch) in name.chars().enumerate() {
		let valid = if index == 0 {
			ch.is_ascii_alphabetic() || ch == '_' || ch == '$'
		} else {
			ch.is_ascii_alphanumeric() || ch == '_' || ch == '$'
		};
		if !valid {
			return Err(Error::new(format!("invalid {label} {name:?}")));
		}
	}
	if ts_identifier_is_reserved(name) {
		return Err(Error::new(format!(
			"invalid {label} {name:?}: reserved word"
		)));
	}
	Ok(())
}

fn ts_identifier_is_reserved(name: &str) -> bool {
	matches!(
		name,
		"abstract"
			| "any" | "as"
			| "async" | "await"
			| "boolean"
			| "break" | "case"
			| "catch" | "class"
			| "const" | "constructor"
			| "continue"
			| "debugger"
			| "declare"
			| "default"
			| "delete"
			| "do" | "else"
			| "enum" | "export"
			| "extends"
			| "false" | "finally"
			| "for" | "from"
			| "function"
			| "get" | "if"
			| "implements"
			| "import"
			| "in" | "infer"
			| "instanceof"
			| "interface"
			| "is" | "keyof"
			| "let" | "module"
			| "namespace"
			| "never" | "new"
			| "null" | "number"
			| "object"
			| "of" | "package"
			| "private"
			| "protected"
			| "public"
			| "readonly"
			| "require"
			| "return"
			| "set" | "static"
			| "string"
			| "super" | "switch"
			| "symbol"
			| "this" | "throw"
			| "true" | "try"
			| "type" | "typeof"
			| "undefined"
			| "unique"
			| "unknown"
			| "var" | "void"
			| "while" | "with"
			| "yield"
	)
}
