//! Compile-fail pins for the diagnostics `vorma-macros` emits.
/*
These pin the ERROR QUALITY, not just the rejection: each unsupported shape
must keep telling the user exactly what to do instead. Regenerate expected
output with TRYBUILD=overwrite after intentional diagnostic changes.
*/

#[test]
fn macro_diagnostics() {
	let cases = trybuild::TestCases::new();
	cases.compile_fail("tests/compile_fail/*.rs");
}
