package vormabuild

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/vormaruntime"
)

type stageTwoBuildIDDependencies struct {
	readHTMLTemplate  func(string) ([]byte, error)
	marshalPathsFile  func(any) ([]byte, error)
	summarizePublicFS func(fs.FS) ([]byte, error)
}

var stageTwoBuildIDDeps = stageTwoBuildIDDependencies{
	readHTMLTemplate:  os.ReadFile,
	marshalPathsFile:  json.Marshal,
	summarizePublicFS: getFSSummaryHash,
}

func computeStageTwoBuildID(v *vormaruntime.Vorma, pathsFile *vormaruntime.PathsFile) (string, error) {
	htmlTemplateContent, err := stageTwoBuildIDDeps.readHTMLTemplate(
		path.Join(v.Wave.GetPrivateStaticDir(), v.Config.HTMLTemplateLocation),
	)
	if err != nil {
		return "", fmt.Errorf("read HTML template: %w", err)
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	asJSON, err := stageTwoBuildIDDeps.marshalPathsFile(pathsFile)
	if err != nil {
		return "", fmt.Errorf("marshal paths file: %w", err)
	}
	pathsFileJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := stageTwoBuildIDDeps.summarizePublicFS(os.DirFS(v.Wave.GetStaticPublicOutDir()))
	if err != nil {
		return "", fmt.Errorf("get FS summary hash: %w", err)
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pathsFileJSONHash)
	fullHash.Write(publicFSSummaryHash)
	return base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16]), nil
}
