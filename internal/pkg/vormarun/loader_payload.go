package vormarun

import (
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/searchparams"
)

type ssr_payload struct {
	ClientBuildID string `json:",omitempty"`
	IsDev         bool   `json:",omitempty"`
	DeploymentID  string `json:",omitempty"`

	loader_payload
}

type loader_payload struct {
	/////// Sent no matter what:

	MatchedPatterns []string              `json:",omitempty"`
	Params          map[string]string     `json:",omitempty"`
	SplatValues     []string              `json:",omitempty"`
	SearchSchemas   []searchparams.Schema `json:",omitempty"`

	Title       *htmlutil.Element   `json:",omitempty"`
	MetaHeadEls []*htmlutil.Element `json:",omitempty"`
	RestHeadEls []*htmlutil.Element `json:",omitempty"`

	OutermostServerErr    string `json:",omitempty"`
	OutermostServerErrIdx *int   `json:",omitempty"`

	/////// Sent up to and including the error idx:

	ImportURLs []string `json:",omitempty"`
	Deps       []string `json:",omitempty"`
	CSSBundles []string `json:",omitempty"`

	/////// Sent up to but excluding the error idx:

	LoadersData []any `json:",omitempty"`
}
