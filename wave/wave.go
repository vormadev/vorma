// Package wave is the end-user runtime API for Wave applications.
//
// Application code should only need this package to construct runtime behavior
// and attach middleware/templating helpers.
package wave

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/wave/internal/waveruntimecore"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

var defaultPortResolver = waveenv.NewResolver()

// GetIsDev reports whether Wave is running in development mode.
func GetIsDev() bool {
	return waveenv.GetIsDev()
}

// SetModeToDev marks the current process as development mode.
func SetModeToDev() {
	waveenv.SetModeToDev()
}

// MustGetPort returns the application runtime port.
// In dev mode it resolves and caches a framework-controlled free port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int {
	if defaultPortResolver == nil {
		defaultPortResolver = waveenv.NewResolver()
	}
	return defaultPortResolver.MustGetPort()
}

// Config configures Wave initialization.
type Config struct {
	// Required -- filesystem root used to discover/read the raw config file
	// addressed by ConfigPath.
	FS fs.FS

	// Required -- config-path selector relative to Config.FS root. This can be
	// a simple basename (for example "wave.config.json") or a slash path.
	ConfigPath string

	// Optional logger.
	Logger *slog.Logger
}

// Wave provides the app-facing runtime API surface.
type Wave struct {
	cfg        waveconfig.ParsedConfig
	configPath string
	runtime    *waveruntimecore.Runtime
}

// New constructs a Wave runtime instance from one config file path rooted in
// one filesystem.
func New(c Config) *Wave {
	if c.FS == nil {
		panic("wave.New: FS is required")
	}

	normalizedConfigPath, normalizeErr := normalizeWaveConfigPathForFS(
		c.ConfigPath,
	)
	if normalizeErr != nil {
		panic("wave.New: " + normalizeErr.Error())
	}

	configPathRelativeToConfigFS, rawConfigJSON, resolveConfigErr := resolveWaveConfigPathAndRawConfigJSON(
		c.FS,
		normalizedConfigPath,
	)
	if resolveConfigErr != nil {
		panic("wave.New: " + resolveConfigErr.Error())
	}
	projectID, projectIDError := parseProjectIDFromWaveConfigJSONHeader(
		rawConfigJSON,
		configPathRelativeToConfigFS,
	)
	if projectIDError != nil {
		panic("wave.New: " + projectIDError.Error())
	}
	currentWorkingDirectoryRelativeConfigPath, resolveConfigPathRelativeToCurrentWorkingDirectoryError := resolveWaveConfigPathRelativeToCurrentWorkingDirectory(
		normalizedConfigPath,
		projectID,
	)
	if resolveConfigPathRelativeToCurrentWorkingDirectoryError != nil {
		panic(
			"wave.New: " +
				resolveConfigPathRelativeToCurrentWorkingDirectoryError.Error(),
		)
	}
	if strings.TrimSpace(currentWorkingDirectoryRelativeConfigPath) == "" {
		panic(
			fmt.Sprintf(
				"wave.New: resolve config path %q with Core.ProjectID %q relative to current working directory: contract violation (no matching file discovered)",
				normalizedConfigPath,
				projectID,
			),
		)
	}
	// Parsed config paths are anchored to the deterministic CWD discovery match,
	// not to Config.FS abstraction roots.
	configPathForParsingAndMachinePaths := currentWorkingDirectoryRelativeConfigPath
	distStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll := deriveDistStaticPathRelativeToConfigFS(
		configPathForParsingAndMachinePaths,
	)

	parsedConfig, parseError := waveconfig.ParseConfigJSONWithConfigPath(
		rawConfigJSON,
		configPathForParsingAndMachinePaths,
	)
	if parseError != nil {
		panic("wave.New: " + parseError.Error())
	}
	if parsedConfig.Core() == nil {
		panic("wave.New: parsed config core section is required")
	}

	distStaticPathRelativeToConfigFS := deriveDistStaticPathRelativeToConfigFS(
		configPathRelativeToConfigFS,
	)
	resolvedDistStaticPath := path.Clean(
		filepath.ToSlash(distStaticPathRelativeToConfigFS),
	)
	if !fs.ValidPath(resolvedDistStaticPath) {
		panic(
			fmt.Sprintf(
				"wave.New: resolved dist static path %q is invalid",
				resolvedDistStaticPath,
			),
		)
	}
	if ensureDistStaticDirectoryError := ensureDistStaticDirectoryExistsForWaveConfigFS(
		c.FS,
		resolvedDistStaticPath,
		distStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll,
	); ensureDistStaticDirectoryError != nil {
		panic(
			"wave.New: " +
				ensureDistStaticDirectoryError.Error(),
		)
	}

	distStaticFS, distStaticFSErr := fs.Sub(c.FS, resolvedDistStaticPath)
	if distStaticFSErr != nil {
		panic(
			fmt.Sprintf(
				"wave.New: resolve dist static fs %q: %v",
				resolvedDistStaticPath,
				distStaticFSErr,
			),
		)
	}

	clonedRawConfigJSON := append([]byte(nil), rawConfigJSON...)

	waveRuntime := &Wave{
		cfg:        parsedConfig,
		configPath: configPathForParsingAndMachinePaths,
	}
	waveRuntime.runtime = waveruntimecore.New(
		waveruntimecore.Config{
			ParsedConfig:  parsedConfig,
			RawConfigJSON: clonedRawConfigJSON,
			DistStaticFS:  distStaticFS,
			Logger:        c.Logger,
			IsDevMode:     GetIsDev(),
			PortResolver:  waveenv.NewResolverForMode(GetIsDev()),
		},
	)

	return waveRuntime
}

func normalizeWaveConfigPathForFS(configPath string) (string, error) {
	trimmedConfigPath := strings.TrimSpace(configPath)
	if trimmedConfigPath == "" {
		return "", fmt.Errorf("ConfigPath is required")
	}
	normalizedConfigPath := path.Clean(filepath.ToSlash(trimmedConfigPath))
	if normalizedConfigPath == "." {
		return "", fmt.Errorf("ConfigPath is required")
	}
	if strings.HasPrefix(normalizedConfigPath, "/") {
		return "", fmt.Errorf(
			"ConfigPath must be relative to FS root: %q",
			configPath,
		)
	}
	if normalizedConfigPath == ".." ||
		strings.HasPrefix(normalizedConfigPath, "../") {
		return "", fmt.Errorf(
			"ConfigPath must not escape FS root: %q",
			configPath,
		)
	}
	return normalizedConfigPath, nil
}

func deriveDistStaticPathRelativeToConfigFS(
	configPathRelativeToConfigFS string,
) string {
	normalizedConfigPathRelativeToConfigFS := path.Clean(
		filepath.ToSlash(configPathRelativeToConfigFS),
	)
	normalizedConfigDirectoryRelativeToConfigFS := path.Dir(
		normalizedConfigPathRelativeToConfigFS,
	)
	if normalizedConfigDirectoryRelativeToConfigFS == "." {
		return path.Clean(path.Join(".wavedist", "static"))
	}
	return path.Clean(
		path.Join(
			normalizedConfigDirectoryRelativeToConfigFS,
			".wavedist",
			"static",
		),
	)
}

func ensureDistStaticDirectoryExistsForWaveConfigFS(
	configFS fs.FS,
	distStaticPathRelativeToConfigFS string,
	distStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll string,
) error {
	if configFS == nil {
		return fmt.Errorf("config FS is required")
	}
	distStaticPathInfo, distStaticPathStatError := fs.Stat(
		configFS,
		distStaticPathRelativeToConfigFS,
	)
	if distStaticPathStatError == nil {
		if !distStaticPathInfo.IsDir() {
			return fmt.Errorf(
				"dist static path %q is not a directory",
				distStaticPathRelativeToConfigFS,
			)
		}
		return nil
	}
	if !os.IsNotExist(distStaticPathStatError) {
		return fmt.Errorf(
			"resolve dist static fs %q: %w",
			distStaticPathRelativeToConfigFS,
			distStaticPathStatError,
		)
	}

	trimmedDistStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll := strings.TrimSpace(
		distStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll,
	)
	if trimmedDistStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll == "" {
		return fmt.Errorf(
			"resolve dist static fs %q: auto-create requires config path discovery relative to current working directory; contract violation: %w",
			distStaticPathRelativeToConfigFS,
			distStaticPathStatError,
		)
	}
	distStaticDirectoryPathForMkdirAll := filepath.Clean(filepath.FromSlash(
		trimmedDistStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll,
	))
	if strings.TrimSpace(distStaticDirectoryPathForMkdirAll) == "" {
		return fmt.Errorf(
			"resolve dist static fs %q: %w",
			distStaticPathRelativeToConfigFS,
			distStaticPathStatError,
		)
	}
	if distStaticDirectoryPathForMkdirAll == "." {
		return fmt.Errorf(
			"create dist static directory %q: invalid directory path",
			trimmedDistStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll,
		)
	}
	if strings.HasPrefix(distStaticDirectoryPathForMkdirAll, ".."+string(filepath.Separator)) ||
		distStaticDirectoryPathForMkdirAll == ".." {
		return fmt.Errorf(
			"create dist static directory %q: path escapes current working directory",
			trimmedDistStaticPathRelativeToCurrentWorkingDirectoryForMkdirAll,
		)
	}
	if mkdirAllError := os.MkdirAll(
		distStaticDirectoryPathForMkdirAll,
		0o755,
	); mkdirAllError != nil {
		return fmt.Errorf(
			"create dist static directory %q: %w",
			distStaticDirectoryPathForMkdirAll,
			mkdirAllError,
		)
	}
	verifiedDistStaticPathInfo, verifyDistStaticPathError := fs.Stat(
		configFS,
		distStaticPathRelativeToConfigFS,
	)
	if verifyDistStaticPathError != nil {
		return fmt.Errorf(
			"verify dist static fs %q after create: %w",
			distStaticPathRelativeToConfigFS,
			verifyDistStaticPathError,
		)
	}
	if !verifiedDistStaticPathInfo.IsDir() {
		return fmt.Errorf(
			"dist static path %q is not a directory",
			distStaticPathRelativeToConfigFS,
		)
	}
	return nil
}

type waveConfigPathDiscoveryCandidate struct {
	configPathRelativeToFS string
	projectID              string
	rawConfigJSON          []byte
}

type waveProjectIDHeader struct {
	Core struct {
		ProjectID string `json:"ProjectID"`
	} `json:"Core"`
}

func resolveWaveConfigPathRelativeToCurrentWorkingDirectory(
	normalizedConfigPath string,
	targetProjectID string,
) (string, error) {
	currentWorkingDirectoryConfigFS := os.DirFS(".")
	discoveryCandidates, discoveryError := discoverWaveConfigPathCandidates(
		currentWorkingDirectoryConfigFS,
		normalizedConfigPath,
	)
	if discoveryError != nil {
		return "", discoveryError
	}
	if len(discoveryCandidates) == 0 {
		return "", nil
	}
	trimmedTargetProjectID := strings.TrimSpace(targetProjectID)
	if trimmedTargetProjectID == "" {
		return "", fmt.Errorf(
			"Core.ProjectID is required for current-working-directory config discovery",
		)
	}
	projectMatchedCandidates := make(
		[]waveConfigPathDiscoveryCandidate,
		0,
		len(discoveryCandidates),
	)
	for _, currentCandidate := range discoveryCandidates {
		if strings.TrimSpace(currentCandidate.projectID) == trimmedTargetProjectID {
			projectMatchedCandidates = append(
				projectMatchedCandidates,
				currentCandidate,
			)
		}
	}
	if len(projectMatchedCandidates) == 0 {
		return "", nil
	}
	if len(projectMatchedCandidates) == 1 {
		return projectMatchedCandidates[0].configPathRelativeToFS, nil
	}
	directCandidate := findDirectConfigPathDiscoveryCandidate(
		projectMatchedCandidates,
		normalizedConfigPath,
	)
	if directCandidate != nil {
		return directCandidate.configPathRelativeToFS, nil
	}
	return "", formatWaveConfigPathDiscoveryAmbiguousError(
		normalizedConfigPath,
		projectMatchedCandidates,
		trimmedTargetProjectID,
	)
}

func resolveWaveConfigPathAndRawConfigJSON(
	configFS fs.FS,
	normalizedConfigPath string,
) (string, []byte, error) {
	if configFS == nil || strings.TrimSpace(normalizedConfigPath) == "" {
		return "", nil, fmt.Errorf("config FS and ConfigPath are required")
	}
	discoveryCandidates, discoveryError := discoverWaveConfigPathCandidates(
		configFS,
		normalizedConfigPath,
	)
	if discoveryError != nil {
		return "", nil, discoveryError
	}
	if len(discoveryCandidates) == 0 {
		return "", nil, fmt.Errorf(
			"read config %q: file not found",
			normalizedConfigPath,
		)
	}

	if len(discoveryCandidates) == 1 {
		return discoveryCandidates[0].configPathRelativeToFS, append(
			[]byte(nil),
			discoveryCandidates[0].rawConfigJSON...,
		), nil
	}

	directCandidate := findDirectConfigPathDiscoveryCandidate(
		discoveryCandidates,
		normalizedConfigPath,
	)
	if directCandidate == nil {
		return "", nil, formatWaveConfigPathDiscoveryAmbiguousError(
			normalizedConfigPath,
			discoveryCandidates,
			"",
		)
	}
	targetProjectID := strings.TrimSpace(directCandidate.projectID)
	if targetProjectID == "" {
		return "", nil, fmt.Errorf(
			"config %q is missing Core.ProjectID; cannot resolve config path deterministically",
			normalizedConfigPath,
		)
	}
	projectMatchedCandidates := make(
		[]waveConfigPathDiscoveryCandidate,
		0,
		len(discoveryCandidates),
	)
	for _, currentCandidate := range discoveryCandidates {
		if strings.TrimSpace(currentCandidate.projectID) == targetProjectID {
			projectMatchedCandidates = append(
				projectMatchedCandidates,
				currentCandidate,
			)
		}
	}
	if len(projectMatchedCandidates) != 1 {
		return "", nil, formatWaveConfigPathDiscoveryAmbiguousError(
			normalizedConfigPath,
			discoveryCandidates,
			targetProjectID,
		)
	}
	selectedCandidate := projectMatchedCandidates[0]
	return selectedCandidate.configPathRelativeToFS, append(
		[]byte(nil),
		selectedCandidate.rawConfigJSON...,
	), nil
}

func discoverWaveConfigPathCandidates(
	configFS fs.FS,
	normalizedConfigPath string,
) ([]waveConfigPathDiscoveryCandidate, error) {
	shouldMatchBySuffix := strings.Contains(normalizedConfigPath, "/")
	expectedConfigBasename := path.Base(normalizedConfigPath)
	discoveryCandidates := make([]waveConfigPathDiscoveryCandidate, 0, 8)

	walkError := fs.WalkDir(configFS, ".", func(
		entryPath string,
		entry fs.DirEntry,
		walkError error,
	) error {
		if walkError != nil {
			return walkError
		}
		if entry == nil || entry.IsDir() {
			return nil
		}

		normalizedEntryPath := path.Clean(filepath.ToSlash(entryPath))
		if normalizedEntryPath == "." {
			return nil
		}
		if shouldMatchBySuffix {
			if normalizedEntryPath != normalizedConfigPath &&
				!strings.HasSuffix(normalizedEntryPath, "/"+normalizedConfigPath) {
				return nil
			}
		} else if path.Base(normalizedEntryPath) != expectedConfigBasename {
			return nil
		}

		rawConfigJSON, readError := fs.ReadFile(configFS, normalizedEntryPath)
		if readError != nil {
			return fmt.Errorf(
				"read discovered config candidate %q: %w",
				normalizedEntryPath,
				readError,
			)
		}
		projectID, projectIDError := parseProjectIDFromWaveConfigJSONHeader(
			rawConfigJSON,
			normalizedEntryPath,
		)
		if projectIDError != nil {
			return projectIDError
		}
		discoveryCandidates = append(
			discoveryCandidates,
			waveConfigPathDiscoveryCandidate{
				configPathRelativeToFS: normalizedEntryPath,
				projectID:              projectID,
				rawConfigJSON:          append([]byte(nil), rawConfigJSON...),
			},
		)
		return nil
	})
	if walkError != nil {
		return nil, fmt.Errorf("discover config path candidates: %w", walkError)
	}
	sort.SliceStable(discoveryCandidates, func(leftIndex, rightIndex int) bool {
		return discoveryCandidates[leftIndex].configPathRelativeToFS <
			discoveryCandidates[rightIndex].configPathRelativeToFS
	})
	return discoveryCandidates, nil
}

func parseProjectIDFromWaveConfigJSONHeader(
	rawConfigJSON []byte,
	configPathRelativeToFS string,
) (string, error) {
	var projectIDHeader waveProjectIDHeader
	if unmarshalError := json.Unmarshal(rawConfigJSON, &projectIDHeader); unmarshalError != nil {
		return "", fmt.Errorf(
			"parse config %q while resolving Core.ProjectID for discovery: %w",
			configPathRelativeToFS,
			unmarshalError,
		)
	}
	projectID := strings.TrimSpace(projectIDHeader.Core.ProjectID)
	if projectID == "" {
		return "", fmt.Errorf(
			"config %q is missing Core.ProjectID required for deterministic discovery",
			configPathRelativeToFS,
		)
	}
	return projectID, nil
}

func findDirectConfigPathDiscoveryCandidate(
	discoveryCandidates []waveConfigPathDiscoveryCandidate,
	normalizedConfigPath string,
) *waveConfigPathDiscoveryCandidate {
	for candidateIndex := range discoveryCandidates {
		if discoveryCandidates[candidateIndex].configPathRelativeToFS ==
			normalizedConfigPath {
			return &discoveryCandidates[candidateIndex]
		}
	}
	return nil
}

func formatWaveConfigPathDiscoveryAmbiguousError(
	normalizedConfigPath string,
	discoveryCandidates []waveConfigPathDiscoveryCandidate,
	projectID string,
) error {
	if len(discoveryCandidates) == 0 {
		return fmt.Errorf(
			"no configs discovered matching ConfigPath semantics for %q",
			normalizedConfigPath,
		)
	}
	candidateDescriptions := make([]string, 0, len(discoveryCandidates))
	for _, currentCandidate := range discoveryCandidates {
		candidateDescriptions = append(
			candidateDescriptions,
			fmt.Sprintf(
				"%s (Core.ProjectID=%q)",
				currentCandidate.configPathRelativeToFS,
				currentCandidate.projectID,
			),
		)
	}
	if strings.TrimSpace(projectID) == "" {
		return fmt.Errorf(
			"ambiguous config discovery for ConfigPath %q; multiple candidates found: %s",
			normalizedConfigPath,
			strings.Join(candidateDescriptions, ", "),
		)
	}
	return fmt.Errorf(
		"ambiguous config discovery for ConfigPath %q and Core.ProjectID %q; candidates: %s",
		normalizedConfigPath,
		projectID,
		strings.Join(candidateDescriptions, ", "),
	)
}

// Logger returns the Wave logger instance.
func (w *Wave) Logger() *slog.Logger {
	return w.runtime.Logger()
}

// RawConfigJSON returns the raw bytes of the configuration file.
func (w *Wave) RawConfigJSON() []byte {
	if w == nil {
		return nil
	}
	return w.runtime.RawConfigJSON()
}

// ParsedConfig returns the parsed Wave config used by this runtime.
func (w *Wave) ParsedConfig() waveconfig.ParsedConfig {
	if w == nil {
		return nil
	}
	return w.cfg
}

// IsDev returns true if running in development mode.
func (w *Wave) IsDev() bool {
	if w == nil {
		return GetIsDev()
	}
	return w.runtime.IsDev()
}

// MustGetPort returns the application runtime port.
func (w *Wave) MustGetPort() int {
	if w == nil {
		return MustGetPort()
	}
	return w.runtime.MustGetPort()
}

// SetModeToDev sets the environment to development mode.
func (w *Wave) SetModeToDev() {
	if w != nil {
		w.runtime.SetDevMode()
	}
	SetModeToDev()
}

// PublicPathPrefix returns the normalized configured public path prefix.
func (w *Wave) PublicPathPrefix() string {
	return w.runtime.PublicPathPrefix()
}

// DistDir returns the configured build output directory root.
func (w *Wave) DistDir() string {
	return w.runtime.DistDir()
}

// PrivateStaticDir returns the source directory for private static assets.
func (w *Wave) PrivateStaticDir() string {
	return w.runtime.PrivateStaticDir()
}

// ViteManifestLocation returns the expected path of the Vite manifest in build
// output.
func (w *Wave) ViteManifestLocation() string {
	return w.runtime.ViteManifestLocation()
}

// StaticPrivateOutDir returns the private static output directory.
func (w *Wave) StaticPrivateOutDir() string {
	return w.runtime.StaticPrivateOutDir()
}

// StaticPublicOutDir returns the public static output directory.
func (w *Wave) StaticPublicOutDir() string {
	return w.runtime.StaticPublicOutDir()
}

// ConfigFile returns the normalized config file path in parser path-basis
// semantics (the CWD-relative config discovery result used by wave.New).
func (w *Wave) ConfigFile() string {
	if w == nil {
		return ""
	}
	return w.configPath
}

// PrivateFS returns the runtime private-assets filesystem.
func (w *Wave) PrivateFS() (fs.FS, error) {
	return w.runtime.PrivateFS()
}

// MustPrivateFS returns the private filesystem or panics if unavailable.
func (w *Wave) MustPrivateFS() fs.FS {
	return w.runtime.MustPrivateFS()
}

// PublicURL resolves one source public asset path to its built URL.
func (w *Wave) PublicURL(original string) string {
	return w.runtime.PublicURL(original)
}

// CriticalCSS returns critical CSS content when available.
func (w *Wave) CriticalCSS() template.CSS {
	return w.runtime.CriticalCSS()
}

// CriticalCSSStyleElement returns one rendered critical-css <style> element.
func (w *Wave) CriticalCSSStyleElement() template.HTML {
	return w.runtime.CriticalCSSStyleElement()
}

// StyleSheetLinkElement returns one rendered non-critical stylesheet <link>.
func (w *Wave) StyleSheetLinkElement() template.HTML {
	return w.runtime.StyleSheetLinkElement()
}

// RefreshScript returns one rendered dev refresh script.
func (w *Wave) RefreshScript() template.HTML {
	return w.runtime.RefreshScript()
}

// MustStaticMiddleware returns middleware that serves static public assets and
// delegates all other requests to next.
func (w *Wave) MustStaticMiddleware(
	immutable bool,
) func(http.Handler) http.Handler {
	return w.runtime.MustStaticMiddleware(immutable)
}
