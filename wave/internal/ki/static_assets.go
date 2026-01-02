package ki

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

type fileVal struct {
	// The actual filename in dist directory
	DistName string
	// Hash for content change detection
	ContentHash string
	IsPrehashed bool
}

type FileMap map[string]fileVal

func (c *Config) GetServeStaticHandler(addImmutableCacheHeaders bool) (http.Handler, error) {
	publicFS, err := c.GetPublicFS()
	if err != nil {
		wrapped := fmt.Errorf("error getting public FS: %w", err)
		c.Logger.Error(wrapped.Error())
		return nil, wrapped
	}
	if addImmutableCacheHeaders {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix(c.GetPublicPathPrefix(), http.FileServer(http.FS(publicFS))).ServeHTTP(w, r)
		}), nil
	}
	return http.StripPrefix(c.GetPublicPathPrefix(), http.FileServer(http.FS(publicFS))), nil
}

func (c *Config) getInitialPublicFileMapFromGobBuildtime() (FileMap, error) {
	return c.loadMapFromGob(PublicFileMapGobName, true)
}

func (c *Config) getInitialPublicFileMapFromGobRuntime() (FileMap, error) {
	return c.loadMapFromGob(PublicFileMapGobName, false)
}

func (c *Config) MustGetPublicURLBuildtime(originalPublicURL string) string {
	fileMapFromGob, err := c.getInitialPublicFileMapFromGobBuildtime()
	if err != nil {
		c.Logger.Error(fmt.Sprintf(
			"error getting public file map from gob (buildtime) for originalPublicURL %s: %v", originalPublicURL, err,
		))
		panic(err)
	}

	url, err := c.getInitialPublicURLInner(originalPublicURL, fileMapFromGob)
	if err != nil {
		c.Logger.Error(fmt.Sprintf(
			"error getting initial public URL (buildtime) for originalPublicURL %s: %v", originalPublicURL, err,
		))
		panic(err)
	}

	return url
}

func (c *Config) getInitialPublicURL(originalPublicURL string) (string, error) {
	fileMapFromGob, err := c.runtime_cache.public_filemap_from_gob.Get()
	if err != nil {
		c.Logger.Error(fmt.Sprintf(
			"error getting public file map from gob for originalPublicURL %s: %v", originalPublicURL, err,
		))
		return matcher.EnsureLeadingSlash(
			path.Join(
				c._uc.Core.PublicPathPrefix,
				originalPublicURL,
			),
		), err
	}

	return c.getInitialPublicURLInner(originalPublicURL, fileMapFromGob)
}

func (c *Config) getInitialPublicURLInner(originalPublicURL string, fileMapFromGob FileMap) (string, error) {
	if strings.HasPrefix(originalPublicURL, "data:") {
		return originalPublicURL, nil
	}

	if hashedURL, existsInFileMap := fileMapFromGob[cleanURL(originalPublicURL)]; existsInFileMap {
		return matcher.EnsureLeadingSlash(
			path.Join(c._uc.Core.PublicPathPrefix, hashedURL.DistName),
		), nil
	}

	// If no hashed URL found, return the original URL
	c.Logger.Warn(fmt.Sprintf(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	))

	return matcher.EnsureLeadingSlash(
		path.Join(c._uc.Core.PublicPathPrefix, originalPublicURL),
	), nil
}

func publicURLsKeyMaker(x string) string { return x }

func (c *Config) GetPublicURL(originalPublicURL string) string {
	url, _ := c.runtime_cache.public_urls.Get(originalPublicURL)
	return url
}

func cleanURL(url string) string {
	return strings.TrimPrefix(path.Clean(url), "/")
}

func (c *Config) ServeStaticPublicAssets(addImmutableCacheHeaders bool) func(http.Handler) http.Handler {
	handler, err := c.GetServeStaticHandler(addImmutableCacheHeaders)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting serve static handler: %v", err))
		panic(err)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.shouldServeAsPublicAsset(r.URL.Path) {
				handler.ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func (c *Config) shouldServeAsPublicAsset(path string) bool {
	publicPathPrefix := c.GetPublicPathPrefix()
	if publicPathPrefix == "" || publicPathPrefix == "/" {
		return c.getIsPublicAsset(path)
	}
	return strings.HasPrefix(path, publicPathPrefix)
}

func (c *Config) getIsPublicAsset(hashedFileName string) bool {
	isAsset, _ := c.runtime_cache.is_public_asset.Get(hashedFileName)
	return isAsset
}

func (c *Config) getInitialIsPublicAsset(hashedFileName string) (bool, error) {
	publicFS, err := c.GetPublicFS()
	if err != nil {
		return false, err
	}
	clean := cleanURL(hashedFileName)
	_, err = fs.Stat(publicFS, clean)
	return err == nil, nil
}
