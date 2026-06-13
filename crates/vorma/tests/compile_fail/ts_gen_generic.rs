use vorma::TsGen;

#[derive(TsGen)]
struct Wrapper<T> {
	inner: T,
}

fn main() {}
