use crate::contracts::{RouteTypeContract, TypeRefContract};

pub(crate) fn route_type_contract() -> RouteTypeContract {
	RouteTypeContract::new(TypeRefContract::Unit, TypeRefContract::Unknown)
}
