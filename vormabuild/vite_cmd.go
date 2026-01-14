package vormabuild

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormaruntime"
)

func postViteProdBuild(v *vormaruntime.Vorma) error {
	pf, err := toPathsFile_StageTwo(v)
	if err != nil {
		return fmt.Errorf("convert paths to stage two: %w", err)
	}

	pathsAsJSON, err := json.MarshalIndent(pf, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal paths: %w", err)
	}

	pathsJSONOut := filepath.Join(
		v.Wave.GetStaticPrivateOutDir(),
		"vorma_out",
		vormaruntime.VormaPathsStageTwoJSONFileName,
	)
	if err := os.WriteFile(pathsJSONOut, pathsAsJSON, os.ModePerm); err != nil {
		return fmt.Errorf("write paths: %w", err)
	}

	return nil
}

func toPathsFile_StageTwo(v *vormaruntime.Vorma) (*vormaruntime.PathsFile, error) {
	vormaClientEntryOut := ""
	vormaClientEntryDeps := []string{}
	depToCSSBundleMap := make(map[string]string)

	viteManifest, err := viteutil.ReadManifest(v.Wave.GetViteManifestLocation())
	if err != nil {
		return nil, fmt.Errorf("read vite manifest: %w", err)
	}

	cleanClientEntry := filepath.Clean(v.Config.ClientEntry)
	paths := v.GetPathsSnapshot()

	for key, chunk := range viteManifest {
		cleanKey := filepath.Base(chunk.File)

		if len(chunk.CSS) > 0 {
			for _, cssFile := range chunk.CSS {
				depToCSSBundleMap[cleanKey] = filepath.Base(cssFile)
			}
		}

		deps := viteutil.FindAllDependencies(viteManifest, key)

		if chunk.IsEntry && cleanClientEntry == chunk.Src {
			vormaClientEntryOut = cleanKey
			depsWithoutEntry := make([]string, 0, len(deps)-1)
			for _, dep := range deps {
				if dep != vormaClientEntryOut {
					depsWithoutEntry = append(depsWithoutEntry, dep)
				}
			}
			vormaClientEntryDeps = depsWithoutEntry
		} else {
			for i, p := range paths {
				if p.SrcPath == chunk.Src {
					paths[i].OutPath = cleanKey
					paths[i].Deps = deps
				}
			}
		}
	}

	htmlTemplateContent, err := os.ReadFile(path.Join(v.Wave.GetPrivateStaticDir(), v.Config.HTMLTemplateLocation))
	if err != nil {
		return nil, fmt.Errorf("read HTML template: %w", err)
	}
	htmlContentHash := cryptoutil.Sha256Hash(htmlTemplateContent)

	pf := &vormaruntime.PathsFile{
		Stage:             "two",
		DepToCSSBundleMap: depToCSSBundleMap,
		Paths:             paths,
		ClientEntrySrc:    v.Config.ClientEntry,
		ClientEntryOut:    vormaClientEntryOut,
		ClientEntryDeps:   vormaClientEntryDeps,
		RouteManifestFile: v.GetRouteManifestFile(),
	}

	asJSON, err := json.Marshal(pf)
	if err != nil {
		return nil, fmt.Errorf("marshal paths file: %w", err)
	}
	pfJSONHash := cryptoutil.Sha256Hash(asJSON)

	publicFSSummaryHash, err := getFSSummaryHash(os.DirFS(v.Wave.GetStaticPublicOutDir()))
	if err != nil {
		return nil, fmt.Errorf("get FS summary hash: %w", err)
	}

	fullHash := sha256.New()
	fullHash.Write(htmlContentHash)
	fullHash.Write(pfJSONHash)
	fullHash.Write(publicFSSummaryHash)
	buildID := base64.RawURLEncoding.EncodeToString(fullHash.Sum(nil)[:16])

	v.Lock()
	v.UnsafeSetBuildID(buildID)
	v.Unlock()

	pf.BuildID = buildID
	return pf, nil
}
