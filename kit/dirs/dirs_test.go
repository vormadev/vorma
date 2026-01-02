package dirs_test

import (
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/kit/dirs"
)

type Dist struct {
	Bin *dirs.Dir[DistBin]
	Arb *dirs.Dir[DistArb]
}

type DistBin struct {
	Main *dirs.File
}

type DistArb struct {
	Static   *dirs.Dir[DistArbStatic]
	Internal *dirs.Dir[DistArbInternal]
	X        *dirs.File
}

type DistArbStatic struct {
	Public  *dirs.Dir[DistArbStaticPublic]
	Private *dirs.DirEmpty
}

type DistArbStaticPublic struct {
	PublicInternal *dirs.DirEmpty
}

type DistArbInternal struct {
	FileA *dirs.File
	FileB *dirs.File
	FileC *dirs.File
}

func TestArbLayout(t *testing.T) {
	base := "/base/dist"

	dist := dirs.Build(base, dirs.ToRoot(Dist{
		Bin: dirs.ToDir("bin", DistBin{
			Main: dirs.ToFile("main"),
		}),
		Arb: dirs.ToDir("arb", DistArb{
			Static: dirs.ToDir("static", DistArbStatic{
				Public: dirs.ToDir("public", DistArbStaticPublic{
					PublicInternal: dirs.ToDirEmpty("arb_internalSh_()"),
				}),
				Private: dirs.ToDirEmpty("private"),
			}),
			Internal: dirs.ToDir("internal", DistArbInternal{
				FileA: dirs.ToFile("file-a.txt"),
				FileB: dirs.ToFile("file-b.txt"),
				FileC: dirs.ToFile("file-c.txt"),
			}),
			X: dirs.ToFile("x"),
		}),
	}))

	// For each node, we check both LastSegment() and FullPath().
	tests := []struct {
		name     string
		gotName  string
		wantName string
		gotPath  string
		wantPath string
	}{
		{
			"root path",
			dist.LastSegment(),
			"",
			dist.FullPath(),
			base,
		},
		{
			"bin path",
			dist.S().Bin.LastSegment(),
			"bin",
			dist.S().Bin.FullPath(),
			filepath.Join(base, "bin"),
		},
		{
			"bin/main path",
			dist.S().Bin.S().Main.LastSegment(),
			"main",
			dist.S().Bin.S().Main.FullPath(),
			filepath.Join(base, "bin", "main"),
		},

		{
			"arb path",
			dist.S().Arb.LastSegment(),
			"arb",
			dist.S().Arb.FullPath(),
			filepath.Join(base, "arb"),
		},
		{
			"arb/static path",
			dist.S().Arb.S().Static.LastSegment(),
			"static",
			dist.S().Arb.S().Static.FullPath(),
			filepath.Join(base, "arb", "static"),
		},
		{
			"arb/static/public path",
			dist.S().Arb.S().Static.S().Public.LastSegment(),
			"public",
			dist.S().Arb.S().Static.S().Public.FullPath(),
			filepath.Join(base, "arb", "static", "public"),
		},
		{
			"arb/static/public/arb_internalSh_() path",
			dist.S().Arb.S().Static.S().Public.S().PublicInternal.LastSegment(),
			"arb_internalSh_()",
			dist.S().Arb.S().Static.S().Public.S().PublicInternal.FullPath(),
			filepath.Join(base, "arb", "static", "public", "arb_internalSh_()"),
		},
		{
			"arb/static/private path",
			dist.S().Arb.S().Static.S().Private.LastSegment(),
			"private",
			dist.S().Arb.S().Static.S().Private.FullPath(),
			filepath.Join(base, "arb", "static", "private"),
		},
		{
			"arb/internal path",
			dist.S().Arb.S().Internal.LastSegment(),
			"internal",
			dist.S().Arb.S().Internal.FullPath(),
			filepath.Join(base, "arb", "internal"),
		},
		{
			"arb/internal/file-a path",
			dist.S().Arb.S().Internal.S().FileA.LastSegment(),
			"file-a.txt",
			dist.S().Arb.S().Internal.S().FileA.FullPath(),
			filepath.Join(base, "arb", "internal", "file-a.txt"),
		},
		{
			"arb/internal/file-b path",
			dist.S().Arb.S().Internal.S().FileB.LastSegment(),
			"file-b.txt",
			dist.S().Arb.S().Internal.S().FileB.FullPath(),
			filepath.Join(base, "arb", "internal", "file-b.txt"),
		},
		{
			"arb/internal/file-c path",
			dist.S().Arb.S().Internal.S().FileC.LastSegment(),
			"file-c.txt",
			dist.S().Arb.S().Internal.S().FileC.FullPath(),
			filepath.Join(base, "arb", "internal", "file-c.txt"),
		},
		{
			"arb/x path",
			dist.S().Arb.S().X.LastSegment(),
			"x",
			dist.S().Arb.S().X.FullPath(),
			filepath.Join(base, "arb", "x"),
		},
	}

	for _, tt := range tests {
		if tt.gotName != tt.wantName {
			t.Errorf("%s LastSegment() = %q; want %q", tt.name, tt.gotName, tt.wantName)
		}
		if tt.gotPath != tt.wantPath {
			t.Errorf("%s FullPath() = %q; want %q", tt.name, tt.gotPath, tt.wantPath)
		}
	}
}
