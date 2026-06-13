fn push_u32(out: &mut Vec<u8>, value: u32) {
	out.extend_from_slice(&value.to_le_bytes());
}

pub(crate) fn push_len(out: &mut Vec<u8>, value: usize) -> Result<(), ()> {
	let value = u32::try_from(value).map_err(|_| ())?;
	push_u32(out, value);
	Ok(())
}

fn push_string(out: &mut Vec<u8>, value: &str) -> Result<(), ()> {
	push_len(out, value.len())?;
	out.extend_from_slice(value.as_bytes());
	Ok(())
}

pub(crate) fn encode_nested_match(matched: &vorma_matcher::NestedMatches) -> Result<Vec<u8>, ()> {
	let mut out = Vec::new();
	let mut params = matched.params.iter().collect::<Vec<_>>();
	params.sort_by(|left, right| left.0.cmp(right.0));

	push_len(&mut out, params.len())?;
	for (key, value) in params {
		push_string(&mut out, key)?;
		push_string(&mut out, value)?;
	}

	push_len(&mut out, matched.splat_values.len())?;
	for value in matched.splat_values.iter() {
		push_string(&mut out, value)?;
	}

	push_len(&mut out, matched.matches.len())?;
	for item in &matched.matches {
		push_string(&mut out, item.pattern.original_pattern())?;
	}

	Ok(out)
}
