package viteutil

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/vormadev/vorma/internal/pkg/stringutil"
	"github.com/vormadev/vorma/kit/htmlutil"
)

func ToDevScripts(port int, client_entry string, is_react bool) (template.HTML, error) {
	var html_builder strings.Builder

	if is_react {
		var b stringutil.Builder

		b.Return()
		b.Linef(`import RefreshRuntime from "http://localhost:%d/@react-refresh";`, port)
		b.Line("RefreshRuntime.injectIntoGlobalHook(window);")
		b.Line("window.$RefreshReg$ = () => {};")
		b.Line("window.$RefreshSig$ = () => (type) => type;")
		b.Line("window.__vite_plugin_react_preamble_installed__ = true;")

		el := &htmlutil.Element{
			Tag:                 "script",
			AttributesKnownSafe: map[string]string{"type": "module"},
			DangerousInnerHTML:  b.String(),
		}

		if err := htmlutil.RenderElementToBuilder(el, &html_builder); err != nil {
			return "", fmt.Errorf("could not render vite script: %w", err)
		}
		html_builder.WriteString("\n")
	}

	if err := htmlutil.RenderModuleScriptToBuilder(
		fmt.Sprintf("http://localhost:%d/@vite/client", port), &html_builder,
	); err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}
	html_builder.WriteString("\n")

	if err := htmlutil.RenderModuleScriptToBuilder(client_entry, &html_builder); err != nil {
		return "", fmt.Errorf("could not render vite script: %w", err)
	}

	return template.HTML(html_builder.String()), nil
}
