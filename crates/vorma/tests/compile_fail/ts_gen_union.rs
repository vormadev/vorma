use vorma::TsGen;

#[derive(TsGen)]
union Raw {
	int: u32,
	float: f32,
}

fn main() {}
