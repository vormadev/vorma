package loader

import (
	"web/pkg/app"

	"github.com/vormadev/vorma"
)

type Ctx struct {
	*vorma.LoaderReqData
}

func New[O any](
	pattern string,
	loader vorma.LoaderFunc[Ctx, O],
) *vorma.Loader[O] {
	return vorma.RegisterLoader(
		app.App,
		pattern,
		loader,
		func(reqData *vorma.LoaderReqData) *Ctx {
			return &Ctx{
				LoaderReqData: reqData,
			}
		},
	)
}
