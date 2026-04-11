package vormarun

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/set"
)

func InitRouter(
	v *Vorma,
	loaders Loaders,
	actions Actions,
	prod_static_fs fs.FS,
) (*mux.Router, error) {
	v.init_once.Do(func() {
		v.log = colorlog.New("vorma")

		if IsDev() {
			dist_dir := fsutil.SysNorm(v.DistDir)
			sfs_to_use := os.DirFS(filepath.ToSlash(filepath.Join(dist_dir, ".vorma", "static")))
			if _, err := fs.Stat(sfs_to_use, "."); err != nil {
				v.init_err = fmt.Errorf("error accessing static FS at %s: %w", dist_dir, err)
				return
			}
			v.static_fs = sfs_to_use
		} else {
			v.static_fs = prod_static_fs
		}

		manifest, err := read_manifest(v.static_fs)
		if err != nil {
			v.init_err = fmt.Errorf("error reading manifest: %w", err)
			return
		}
		v._manifest = manifest

		v.client_build_id, err = manifest.to_client_build_id()
		if err != nil {
			v.init_err = fmt.Errorf("error hashing manifest into client build id: %w", err)
			return
		}

		v.root_mux = mux.NewRouter(mux.Options{
			DynamicParamPrefix:     Dynamic_Param_Prefix,
			SplatSegmentIdentifier: Splat_Segment_Identifier,
		})

		if IsDev() {
			v.root_mux.AddHTTPHandlerFunc(
				"GET",
				"/.vorma/healthz",
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ok"))
				},
			)
		}

		v.loaders_mux = mux.NewNestedRouter(mux.NestedOptions{
			DynamicParamPrefix:             Dynamic_Param_Prefix,
			SplatSegmentIdentifier:         Splat_Segment_Identifier,
			ExplicitIndexSegmentIdentifier: Explicit_Index_Segment_Identifier,
		})

		for _, l := range loaders {
			l.register_to_mux(v.loaders_mux)
		}
		_ = v.root_mux.AddHTTPHandler("GET", "/*", v.loaders_handler())

		v.actions_mux = mux.NewRouter(mux.Options{
			MountRoot:              manifest.ActionsMountRoot,
			DynamicParamPrefix:     Dynamic_Param_Prefix,
			SplatSegmentIdentifier: Splat_Segment_Identifier,
			ParseInput:             parse_action_input,
		})

		supported_methods := set.New[string]()
		for _, a := range actions {
			supported_methods.Add(a.GetMethod())
			a.register_to_mux(v.actions_mux)
		}
		for m := range supported_methods.Range() {
			_ = v.root_mux.AddHTTPHandler(m, manifest.ActionsMountRoot+"*", v.actions_handler())
		}
		v.supported_methods = supported_methods
		allow_val_slice := supported_methods.Slice()
		slices.Sort(allow_val_slice)
		v.supported_methods_allow_val = strings.Join(allow_val_slice, ", ")

		root_html_template := v.HTMLConfig.Template
		if strings.TrimSpace(root_html_template) == "" {
			root_html_template = default_root_html_template
		}
		tmpl, err := template.New("root").Parse(root_html_template)
		if err != nil {
			v.init_err = fmt.Errorf("error parsing RootHTMLTemplate: %w", err)
			return
		}
		v.parsed_tmpl = tmpl

		v.headels_instance = headels.NewInstance("vorma")
		if v.HTMLConfig.HeadDedupeKeys != nil {
			h := headels.New()
			v.HTMLConfig.HeadDedupeKeys(h)
			v.headels_instance.InitUniqueRules(h)
		}

		fpf, err := v.make_final_public_filepaths()
		if err != nil {
			v.init_err = fmt.Errorf("error making final public filepaths: %w", err)
			return
		}
		v._final_public_filepaths = fpf
	})

	return v.root_mux, v.init_err
}

const default_root_html_template = `<!doctype html>
<html lang="en">
<head>
{{.VormaHead}}
</head>
<body>
{{.VormaBody}}
</body>
</html>
`
