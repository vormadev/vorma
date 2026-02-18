package bootstrap

import (
	"os"
	"strings"
	"text/template"
)

func (d *derivedOptions) mustWriteTmpl(target, name string) {
	tmplStr, err := tmplsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	tmpl := template.Must(template.New(target).Parse(string(tmplStr)))
	var sb strings.Builder
	if err := tmpl.Execute(&sb, d); err != nil {
		panic(err)
	}
	b := []byte(sb.String())
	if err := os.WriteFile(target, b, 0644); err != nil {
		panic(err)
	}
}

func mustWriteStr(target, name string) {
	content, err := tmplsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(target, content, 0644); err != nil {
		panic(err)
	}
}

func mustWriteFile(target, source string) {
	b, err := assetsFS.ReadFile(source)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(target, b, 0644); err != nil {
		panic(err)
	}
}
