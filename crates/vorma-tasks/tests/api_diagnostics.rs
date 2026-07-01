#[test]
fn task_api_rejects_removed_or_ambiguous_declaration_forms() {
	let cases = trybuild::TestCases::new();
	cases.compile_fail("tests/ui/*.rs");
}
