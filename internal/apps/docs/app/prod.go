//go:build prod

package app

import "embed"

//go:embed all:.vorma/static
var static_fs embed.FS

var _ = Vorma.MustSetStaticFS(static_fs, ".vorma/static")
