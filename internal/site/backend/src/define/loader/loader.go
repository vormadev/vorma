package loader

import (
	"site/backend/src/app"

	"github.com/vormadev/vorma"
)

type Ctx struct {
	*vorma.LoaderReqData
}

func Define[O any](
	pattern string,
	loader vorma.LoaderFunc[Ctx, O],
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(
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
