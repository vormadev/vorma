package vormabuild

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/cryptoutil"
)

type stageTwoBuildIDDependencies struct {
	readHTMLTemplate  func(string) ([]byte, error)
	marshalPathsFile  func(any) ([]byte, error)
	summarizePublicFS func(fs.FS) ([]byte, error)
}

type stageTwoBuildIDExecutor struct {
	dependencies stageTwoBuildIDDependencies
}

var defaultStageTwoBuildIDExecutor = newStageTwoBuildIDExecutor(
	stageTwoBuildIDDependencies{},
)

func defaultStageTwoBuildIDDependencies() stageTwoBuildIDDependencies {
	return stageTwoBuildIDDependencies{
		readHTMLTemplate:  os.ReadFile,
		marshalPathsFile:  json.Marshal,
		summarizePublicFS: getFSSummaryHash,
	}
}

func normalizeStageTwoBuildIDDependencies(
	dependencies stageTwoBuildIDDependencies,
) stageTwoBuildIDDependencies {
	defaultDependencies := defaultStageTwoBuildIDDependencies()

	if dependencies.readHTMLTemplate == nil {
		dependencies.readHTMLTemplate = defaultDependencies.readHTMLTemplate
	}
	if dependencies.marshalPathsFile == nil {
		dependencies.marshalPathsFile = defaultDependencies.marshalPathsFile
	}
	if dependencies.summarizePublicFS == nil {
		dependencies.summarizePublicFS = defaultDependencies.summarizePublicFS
	}

	return dependencies
}

func newStageTwoBuildIDExecutor(
	dependencies stageTwoBuildIDDependencies,
) stageTwoBuildIDExecutor {
	return stageTwoBuildIDExecutor{
		dependencies: normalizeStageTwoBuildIDDependencies(dependencies),
	}
}

func computeStageTwoBuildID(v *vormaruntime.Vorma, pathsFile *vormaruntime.PathsFile) (string, error) {
	return defaultStageTwoBuildIDExecutor.computeStageTwoBuildID(v, pathsFile)
}

func (executor stageTwoBuildIDExecutor) computeStageTwoBuildID(
	v *vormaruntime.Vorma,
	pathsFile *vormaruntime.PathsFile,
) (string, error) {
	htmlTemplateContent, err := executor.dependencies.readHTMLTemplate(
		path.Join(v.Wave.GetPrivateStaticDir(), v.Config.HTMLTemplateLocation),
	)
	if err != nil {
		return "", fmt.Errorf("read HTML template: %w", err)
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	asJSON, err := executor.dependencies.marshalPathsFile(pathsFile)
	if err != nil {
		return "", fmt.Errorf("marshal paths file: %w", err)
	}
	pathsFileJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := executor.dependencies.summarizePublicFS(
		os.DirFS(v.Wave.GetStaticPublicOutDir()),
	)
	if err != nil {
		return "", fmt.Errorf("get FS summary hash: %w", err)
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pathsFileJSONHash)
	fullHash.Write(publicFSSummaryHash)
	return base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16]), nil
}
