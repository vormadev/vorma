package ki

import (
	"github.com/vormadev/vorma/kit/tsgen"
)

// If you pass nil to this function, it will return a pointer to a new Statements
// object. If you pass a pointer to an existing Statements object, it will mutate
// that object and return it.
func (c *Config) AddPublicAssetKeys(statements *tsgen.Statements) *tsgen.Statements {
	a := statements
	if a == nil {
		a = &tsgen.Statements{}
	}

	keys, err := c.GetPublicFileMapKeysBuildtime()
	if err != nil {
		panic(err)
	}

	a.Serialize("const WAVE_PUBLIC_ASSETS", keys)
	a.Raw("export type WavePublicAsset", "`${\"/\" | \"\"}${(typeof WAVE_PUBLIC_ASSETS)[number]}`")

	return a
}
