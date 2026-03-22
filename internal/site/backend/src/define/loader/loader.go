package loader

import (
	"site/backend/src/app"

	"github.com/vormadev/vorma/vorma2"
)

type Ctx struct {
	*vorma2.LoaderReqData
}

func Define[O any](
	pattern string,
	loader vorma2.LoaderFunc[Ctx, O],
) *vorma2.Loader[O] {
	return vorma2.RegisterLoader(
		app.App,
		pattern,
		loader,
		func(reqData *vorma2.LoaderReqData) *Ctx {
			return &Ctx{
				LoaderReqData: reqData,
			}
		},
	)
}
