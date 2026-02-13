package tooling

import (
	"runtime"

	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

var staticIgnoreList = map[string]struct{}{
	".DS_Store": {},
}

func (b *Builder) processPublicFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Public),
		distDir:    b.cfg.Dist.StaticPublic(),
		gobPath:    b.cfg.Dist.PublicFileMapGob(),
		granular:   granular,
		isPublic:   true,
		hashOutput: true,
	})
}

func (b *Builder) processPrivateFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Private),
		distDir:    b.cfg.Dist.StaticPrivate(),
		gobPath:    b.cfg.Dist.PrivateFileMapGob(),
		granular:   granular,
		isPublic:   false,
		hashOutput: false,
	})
}

func (b *Builder) processPublicFilesForChangedPaths(changedSourcePaths []string) error {
	return b.processStaticFilesForChangedPaths(
		staticOpts{
			srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Public),
			distDir:    b.cfg.Dist.StaticPublic(),
			gobPath:    b.cfg.Dist.PublicFileMapGob(),
			granular:   true,
			isPublic:   true,
			hashOutput: true,
		},
		changedSourcePaths,
	)
}

func (b *Builder) processPrivateFilesForChangedPaths(changedSourcePaths []string) error {
	return b.processStaticFilesForChangedPaths(
		staticOpts{
			srcDir:     pathnorm.Absolute(b.cfg.Core.StaticAssetDirs.Private),
			distDir:    b.cfg.Dist.StaticPrivate(),
			gobPath:    b.cfg.Dist.PrivateFileMapGob(),
			granular:   true,
			isPublic:   false,
			hashOutput: false,
		},
		changedSourcePaths,
	)
}

type staticOpts struct {
	srcDir     string
	distDir    string
	gobPath    string
	granular   bool
	isPublic   bool
	hashOutput bool
}

type fileInfo struct {
	srcPath string
	relPath string
	prehash bool
}

func determineStaticProcessingWorkerCount(gomaxprocs int) int {
	if gomaxprocs <= 0 {
		return 1
	}

	workerCount := gomaxprocs * 2
	if workerCount < 1 {
		return 1
	}
	if workerCount > 32 {
		return 32
	}

	return workerCount
}

func determineStaticProcessingWorkerCountFromRuntime() int {
	return determineStaticProcessingWorkerCount(runtime.GOMAXPROCS(0))
}
