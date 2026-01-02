package ki

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
)

const (
	PublicFileMapJSName   = "vorma_internal_public_filemap.js"
	PublicFileMapGobName  = "public_filemap.gob"
	PrivateFileMapGobName = "private_filemap.gob"
)

func (c *Config) loadMapFromGob(gobFileName string, isBuildTime bool) (FileMap, error) {
	appropriateFS, err := c.getAppropriateFSMaybeBuildTime(isBuildTime)
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %w", err)
	}

	distWaveInternal := c._dist.S().Static.S().Internal

	// __LOCATION_ASSUMPTION: Inside "dist/static"
	file, err := appropriateFS.Open(path.Join(distWaveInternal.LastSegment(), gobFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %w", gobFileName, err)
	}

	defer file.Close()

	return fsutil.FromGob[FileMap](file)
}

func (c *Config) getAppropriateFSMaybeBuildTime(isBuildTime bool) (fs.FS, error) {
	if isBuildTime {
		return c.runtime_cache.base_dir_fs.Get()
	}
	return c.GetBaseFS()
}

func (c *Config) saveMapToGob(mapToSave FileMap, dest string) error {
	file, err := os.Create(filepath.Join(c._dist.S().Static.S().Internal.FullPath(), dest))
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	return encoder.Encode(mapToSave)
}

func (c *Config) savePublicFileMapJSToInternalPublicDir(mapToSave FileMap) error {
	simpleStrMap := make(map[string]string, len(mapToSave))
	for k, v := range mapToSave {
		simpleStrMap[k] = v.DistName
	}

	mapAsJSON, err := json.Marshal(simpleStrMap)
	if err != nil {
		return fmt.Errorf("error marshalling map to JSON: %w", err)
	}

	bytes := []byte(fmt.Sprintf("export const wavePublicFileMap = %s;", string(mapAsJSON)))

	hashedFilename := getHashedFilename(bytes, PublicFileMapJSName)

	// Clean up old public_filemap files before writing new one
	publicAssetsPath := c._dist.S().Static.S().Assets.S().Public.FullPath()
	oldFileMapPattern := filepath.Join(
		publicAssetsPath,
		"vorma_out_vorma_internal_public_filemap_*.js",
	)
	oldFileMapFiles, err := filepath.Glob(oldFileMapPattern)
	if err != nil {
		return fmt.Errorf("error finding old public filemap files: %w", err)
	}
	for _, oldFile := range oldFileMapFiles {
		if err := os.Remove(oldFile); err != nil {
			return fmt.Errorf("error removing old public filemap file: %w", err)
		}
	}

	// Write the reference file
	hashedFileRefPath := c._dist.S().Static.S().Internal.S().PublicFileMapFileRefDotTXT.FullPath()
	if err := os.WriteFile(hashedFileRefPath, []byte(hashedFilename), 0644); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	// Write the new hashed file
	return os.WriteFile(filepath.Join(
		publicAssetsPath,
		hashedFilename,
	), bytes, 0644)
}

type publicFileMapDetails struct {
	Elements   template.HTML
	Sha256Hash string
}

func (c *Config) getInitialPublicFileMapDetails() (*publicFileMapDetails, error) {
	innerHTMLFormatStr := `
		import { wavePublicFileMap } from "%s";
		if (!window.__wave) window.__wave = {};
		function getPublicURL(originalPublicURL) { 
			if (originalPublicURL.startsWith("/")) originalPublicURL = originalPublicURL.slice(1);
			return "%s" + (wavePublicFileMap[originalPublicURL] || originalPublicURL);
		}
		window.__wave.getPublicURL = getPublicURL;` + "\n"

	publicFileMapURL := c.GetPublicFileMapURL()

	linkEl := htmlutil.Element{
		Tag:        "link",
		Attributes: map[string]string{"rel": "modulepreload", "href": publicFileMapURL},
	}

	scriptEl := htmlutil.Element{
		Tag:                "script",
		Attributes:         map[string]string{"type": "module"},
		DangerousInnerHTML: fmt.Sprintf(innerHTMLFormatStr, publicFileMapURL, c._uc.Core.PublicPathPrefix),
	}

	sha256Hash, err := htmlutil.AddSha256HashInline(&scriptEl)
	if err != nil {
		return nil, fmt.Errorf("error handling CSP: %w", err)
	}

	var htmlBuilder strings.Builder

	err = htmlutil.RenderElementToBuilder(&linkEl, &htmlBuilder)
	if err != nil {
		return nil, fmt.Errorf("error rendering element to builder: %w", err)
	}
	err = htmlutil.RenderElementToBuilder(&scriptEl, &htmlBuilder)
	if err != nil {
		return nil, fmt.Errorf("error rendering element to builder: %w", err)
	}

	return &publicFileMapDetails{
		Elements:   template.HTML(htmlBuilder.String()),
		Sha256Hash: sha256Hash,
	}, nil
}

func (c *Config) getInitialPublicFileMapURL() (string, error) {
	baseFS, err := c.GetBaseFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return "", err
	}

	distWaveInternal := c._dist.S().Static.S().Internal

	// __LOCATION_ASSUMPTION: Inside "dist/static"
	content, err := fs.ReadFile(baseFS,
		path.Join(
			distWaveInternal.LastSegment(),
			distWaveInternal.S().PublicFileMapFileRefDotTXT.LastSegment(),
		))
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error reading publicFileMapFileRefFile: %v", err))
		return "", err
	}

	return matcher.EnsureLeadingSlash(
		path.Join(
			c._uc.Core.PublicPathPrefix,
			c._dist.S().Static.S().Assets.S().Public.LastSegment(),
			string(content),
		),
	), nil
}

func (c *Config) GetPublicFileMapURL() string {
	url, _ := c.runtime_cache.public_filemap_url.Get()
	return url
}
func (c *Config) GetPublicFileMap() (FileMap, error) {
	return c.runtime_cache.public_filemap_from_gob.Get()
}
func (c *Config) GetPublicFileMapElements() template.HTML {
	details, _ := c.runtime_cache.public_filemap_details.Get()
	return details.Elements
}
func (c *Config) GetPublicFileMapScriptSha256Hash() string {
	details, _ := c.runtime_cache.public_filemap_details.Get()
	return details.Sha256Hash
}

func (c *Config) GetPublicFileMapKeysBuildtime() ([]string, error) {
	filemap, err := c.getInitialPublicFileMapFromGobBuildtime()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(filemap))
	for k, v := range filemap {
		if !v.IsPrehashed {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (c *Config) GetSimplePublicFileMapBuildtime() (map[string]string, error) {
	filemap, err := c.getInitialPublicFileMapFromGobBuildtime()

	if err != nil && errors.Is(err, fs.ErrNotExist) {
		if err := c.do_build_time_file_processing(false); err != nil {
			return nil, fmt.Errorf("error processing build time files: %w", err)
		}

		filemap, err = c.getInitialPublicFileMapFromGobBuildtime()
		if err != nil {
			return nil, fmt.Errorf("error getting initial public file map: %w", err)
		}
	}

	if err != nil {
		return nil, err
	}

	simpleStrMap := make(map[string]string, len(filemap))
	for k, v := range filemap {
		if !v.IsPrehashed {
			simpleStrMap[k] = v.DistName
		}
	}

	return simpleStrMap, nil
}
