package builder_test

import (
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
)

type builderContractForRuntimeBuild interface {
	ViteProdBuild() error
	Build(builder.BuildOpts) error
	Close() error
}

type builderContractForDocSync interface {
	ProcessPublicFilesOnly() error
	LoadPublicFileMap() (wavefilemap.FileMap, error)
	Close() error
}

var _ builderContractForRuntimeBuild = (*builder.Builder)(nil)
var _ builderContractForDocSync = (*builder.Builder)(nil)
