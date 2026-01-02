package ki

import (
	"runtime"

	"github.com/vormadev/vorma/kit/dirs"
)

const (
	PUBLIC  = "public"
	PRIVATE = "private"
)

type Dist struct {
	Binary *dirs.File
	Static *dirs.Dir[DistStatic]
}

type DistStatic struct {
	Assets   *dirs.Dir[DistStaticAssets]
	Internal *dirs.Dir[DistWaveInternal]
	Keep     *dirs.File
}

type DistStaticAssets struct {
	Public  *dirs.DirEmpty
	Private *dirs.DirEmpty
}

type DistWaveInternal struct {
	CriticalDotCSS             *dirs.File
	NormalCSSFileRefDotTXT     *dirs.File
	PublicFileMapFileRefDotTXT *dirs.File
}

func toDistLayout(cleanDistDir string) *dirs.Dir[Dist] {
	mainOut := "main"
	if runtime.GOOS == "windows" {
		mainOut += ".exe"
	}
	x := dirs.Build(cleanDistDir, dirs.ToRoot(Dist{
		Binary: dirs.ToFile(mainOut),
		Static: dirs.ToDir("static", DistStatic{
			Assets: dirs.ToDir("assets", DistStaticAssets{
				Public:  dirs.ToDirEmpty(PUBLIC),
				Private: dirs.ToDirEmpty(PRIVATE),
			}),
			Internal: dirs.ToDir("internal", DistWaveInternal{
				CriticalDotCSS:             dirs.ToFile("critical.css"),
				NormalCSSFileRefDotTXT:     dirs.ToFile("normal_css_file_ref.txt"),
				PublicFileMapFileRefDotTXT: dirs.ToFile("public_file_map_file_ref.txt"),
			}),
			Keep: dirs.ToFile(".keep"),
		}),
	}))

	return x
}
