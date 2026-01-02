package vorma

import (
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
)

const (
	VormaSymbolStr = "__vorma_internal__"
)

var Log = colorlog.New("vorma")

type RouteType = string

var RouteTypes = struct {
	Loader   RouteType
	Query    RouteType
	Mutation RouteType
	NotFound RouteType
}{
	Loader:   "loader",
	Query:    "query",
	Mutation: "mutation",
	NotFound: "not-found",
}

type Path struct {
	NestedRoute mux.AnyNestedRoute `json:"-"`

	// both stages one and two
	OriginalPattern string `json:"originalPattern"`
	SrcPath         string `json:"srcPath"`
	ExportKey       string `json:"exportKey"`
	ErrorExportKey  string `json:"errorExportKey,omitempty"`

	// stage two only
	OutPath string   `json:"outPath,omitempty"`
	Deps    []string `json:"deps,omitempty"`
}

type UIVariant string

var UIVariants = struct {
	React  UIVariant
	Preact UIVariant
	Solid  UIVariant
}{
	React:  "react",
	Preact: "preact",
	Solid:  "solid",
}

type (
	GetDefaultHeadElsFunc    func(r *http.Request, app *Vorma) (*headels.HeadEls, error)
	GetHeadElUniqueRulesFunc func() *headels.HeadEls
	GetRootTemplateDataFunc  func(r *http.Request) (map[string]any, error)
)

type Vorma struct {
	*wave.Wave

	actionsRouter *ActionsRouter
	loadersRouter *LoadersRouter

	getDefaultHeadEls    GetDefaultHeadElsFunc
	getHeadElUniqueRules GetHeadElUniqueRulesFunc
	getRootTemplateData  GetRootTemplateDataFunc

	mu                 sync.RWMutex
	_isDev             bool
	_paths             map[string]*Path
	_clientEntrySrc    string
	_clientEntryOut    string
	_clientEntryDeps   []string
	_buildID           string
	_depToCSSBundleMap map[string]string
	_rootTemplate      *template.Template
	_privateFS         fs.FS
	_routeManifestFile string
	_serverAddr        string
}

func (h *Vorma) ServerAddr() string            { return h._serverAddr }
func (h *Vorma) LoadersRouter() *LoadersRouter { return h.loadersRouter }
func (h *Vorma) ActionsRouter() *ActionsRouter { return h.actionsRouter }
