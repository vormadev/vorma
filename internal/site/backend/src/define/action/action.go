package action

import (
	"site/backend/src/app"

	"github.com/vormadev/vorma"
)

type Ctx[I any] struct {
	*vorma.ActionReqData[I]
}

func Define[I any, O any](
	method string,
	pattern string,
	action vorma.ActionFunc[Ctx[I], I, O],
) *vorma.Action[I, O] {
	return vorma.RegisterAction(
		app.App,
		method,
		pattern,
		action,
		func(reqData *vorma.ActionReqData[I]) *Ctx[I] {
			return &Ctx[I]{
				ActionReqData: reqData,
			}
		},
	)
}
