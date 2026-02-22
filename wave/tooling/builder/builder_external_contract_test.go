package builder_test

import (
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder"
)

type builderContractForRuntimeBuild interface {
	ViteProdBuild() error
	Build(builder.BuildOpts) error
	Close() error
}

type builderContractForPublicFileMapWriter interface {
	WritePublicFileMapTS(string) error
	Close() error
}

type builderContractForDocSync interface {
	ProcessPublicFilesOnly() error
	LoadPublicFileMap() (wave.FileMap, error)
	Close() error
}

var _ builderContractForRuntimeBuild = (*builder.Builder)(nil)
var _ builderContractForPublicFileMapWriter = (*builder.Builder)(nil)
var _ builderContractForDocSync = (*builder.Builder)(nil)
