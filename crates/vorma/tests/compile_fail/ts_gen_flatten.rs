use serde::{Deserialize, Serialize};
use vorma::TsGen;

#[derive(Deserialize, Serialize, TsGen)]
struct Inner {
	value: String,
}

#[derive(Deserialize, Serialize, TsGen)]
struct Outer {
	#[serde(flatten)]
	inner: Inner,
}

fn main() {}
