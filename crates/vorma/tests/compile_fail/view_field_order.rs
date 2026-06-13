/*
View declarations are field-order strict; the diagnostic must name the
expected field rather than emitting a generic parse error.
*/
fn main() {
	let _ = vorma::__vorma_view! {
		pattern: "/",
		client_file: "src/views/root.tsx",
		input: (),
		output: (),
		handler: |_ctx| async move { Ok(()) },
	};
}
